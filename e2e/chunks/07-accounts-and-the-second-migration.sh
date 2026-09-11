#!/usr/bin/env bash

. "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib/bootstrap.sh"
. "$E2E_DIR/lib/services.sh"
. "$E2E_DIR/lib/shop.sh"
. "$E2E_DIR/lib/server.sh"
. "$E2E_DIR/lib/http.sh"

chunk_begin "07" "Accounts and a second migration"

require_previous_chunk "the shop exists" "$SHOP_DIR/settings.go"

trap server_cleanup EXIT

SHOPPER_USER="edith"
SHOPPER_PASSWORD="bench-plane-8821"
SHOPPER_EMAIL="edith@thornfield.test"

check_the_orders_stage_lands() {
	shop_apply_stage 2
	run_capturing shop_build
	if [ "$CAPTURED_STATUS" -eq 0 ]; then
		check_passed "the shop still compiles with orders, the cart and the delivery job"
	else
		check_failed "the shop compiles with the order models" "$(truncated_output "$CAPTURED_OUTPUT")"
		chunk_end
	fi
}

check_migrate_notices_unimported_migrations() {
	local files registered
	files="$(shop_migration_files | wc -l | tr -d ' ')"
	shop_command migrate >/dev/null 2>&1 || true
	registered="$(printf '%s' "$CAPTURED_OUTPUT" | sed -n 's/^migrations *\([0-9]*\) found.*/\1/p' | head -1)"

	if [ "${registered:-0}" = "$files" ]; then
		check_passed "migrate registers every migration file on disk ($files)"
		return 0
	fi
	finding "migrate exits zero and applies nothing when the migrations package is not imported" \
		"There are $files files in shop/migrations/ and migrate reported '${registered:-0} found, 0 pending',
then exited 0 with the message 'no migrations declared; run: coyote makemigrations'. The cause is a
missing blank import of the project's migrations package in main.go, which is easy to lose when
main.go is rewritten.
The failure is silent in the worst way: makemigrations keeps writing files, migrate keeps succeeding,
and the application runs against a schema that does not match its models until a query fails at
runtime. Migrations.Dir is already a setting, so the runner can count the files it finds there and
refuse to claim success when that count does not match migrate.Registered() - 'found 2 migration
files in migrations/ but none are registered; is the package imported from main?'"
}

check_the_second_migration() {
	if ! shop_command makemigrations --name=orders; then
		check_failed "makemigrations writes a second migration for the new models" \
			"$(truncated_output "$CAPTURED_OUTPUT")"
		chunk_end
	fi
	check_passed "makemigrations writes a second migration for the new models"
	assert_equal "there are now two migration files" "2" "$(shop_migration_files | wc -l | tr -d ' ')"

	if shop_command migrate; then
		check_passed "the second migration applies to postgres"
	else
		check_failed "the second migration applies to postgres" "$(truncated_output "$CAPTURED_OUTPUT")"
		chunk_end
	fi

	local table
	for table in addresses coupons orders order_lines shipment_events stock_movements; do
		if postgres_table_exists "$table"; then
			check_passed "postgres has the $table table"
		else
			check_failed "postgres has the $table table" "not created"
		fi
	done
	assert_equal "the ledger records two migrations" "2" "$(postgres_row_count coyote_migrations)"
	assert_equal "the products added earlier survived the migration" "20" "$(postgres_row_count products)"
}

check_the_foreign_keys_are_real() {
	local constraints
	constraints="$(postgres_query "SELECT count(*) FROM information_schema.table_constraints WHERE table_name='order_lines' AND constraint_type='FOREIGN KEY';")"
	if [ "${constraints:-0}" -ge 1 ]; then
		check_passed "order_lines declares $constraints foreign key constraint(s)"
	else
		finding "a belongs-to on a second migration produced no foreign key" \
			"order_lines has Order and Product pointers, so migrate should have emitted REFERENCES
clauses. information_schema reports ${constraints:-0} foreign key constraints on the table."
	fi

	local refused
	refused="$(postgres_attempt "INSERT INTO order_lines (id, order_id, product_id, title, quantity, unit_cents, line_cents) VALUES ('probe', 'no-such-order', 'no-such-product', 'probe', 1, 1, 1);" 2>&1)"
	local landed
	landed="$(postgres_query "SELECT count(*) FROM order_lines WHERE id='probe';")"
	if [ "${landed:-0}" = "0" ]; then
		check_passed "postgres refuses an order line pointing at no order"
	else
		finding "an order line can point at an order that does not exist" \
			"Inserting order_lines with order_id='no-such-order' succeeded, so the belongs-to
relation is an index rather than a constraint and orphan rows are possible."
		postgres_query "DELETE FROM order_lines WHERE id='probe';" >/dev/null
	fi
}

start_the_shop() {
	shop_start
	if server_wait_for_http; then
		check_passed "the shop restarts on the new schema"
		return 0
	fi
	check_failed "the shop restarts on the new schema" "$(tail -25 "$SERVER_LOG")"
	chunk_end
}

check_registration() {
	http_reset_session
	postgres_query "DELETE FROM users WHERE username='$SHOPPER_USER';" >/dev/null

	http_get "/accounts/register"
	assert_equal "the registration page answers 200" "200" "$HTTP_STATUS"

	local token
	token="$(http_csrf_token)"
	if [ -z "$token" ]; then
		check_failed "the registration form carries a CSRF token" "none found"
		chunk_end
	fi
	check_passed "the registration form carries a CSRF token"

	http_post "/accounts/register" \
		"csrf_token=$token" "username=$SHOPPER_USER" "email=$SHOPPER_EMAIL" \
		"first_name=Edith" "last_name=Pargeter" "password=$SHOPPER_PASSWORD"

	if [ "$HTTP_STATUS" = "303" ] || [ "$HTTP_STATUS" = "302" ]; then
		check_passed "a shopper can register"
	else
		check_failed "a shopper can register" "status $HTTP_STATUS
$(printf '%s' "$HTTP_BODY" | grep -oE 'class="error"[^<]*<[^>]*>[^<]*' | head -3)"
		chunk_end
	fi

	assert_equal "the shopper is in the users table" "1" \
		"$(postgres_query "SELECT count(*) FROM users WHERE username='$SHOPPER_USER';")"
	assert_equal "the shopper is not staff" "f" \
		"$(postgres_query "SELECT is_staff FROM users WHERE username='$SHOPPER_USER';")"

	local hash
	hash="$(postgres_query "SELECT password FROM users WHERE username='$SHOPPER_USER';")"
	case "$hash" in
	pbkdf2_sha256\$*) check_passed "the password is stored as a pbkdf2 hash" ;;
	*"$SHOPPER_PASSWORD"*) check_failed "the password is not stored in clear" "found the password in the column" ;;
	*) check_failed "the password is stored as a pbkdf2 hash" "got ${hash:0:24}" ;;
	esac
}

check_registration_signs_you_in() {
	http_get "/"
	case "$HTTP_BODY" in
	*"My orders"*) check_passed "registering signs the shopper straight in" ;;
	*) check_failed "registering signs the shopper straight in" "the header still offers Sign in" ;;
	esac
}

check_sign_out_and_in() {
	local token
	http_get "/accounts/profile"
	token="$(http_csrf_token)"
	http_post "/accounts/logout" "csrf_token=$token"

	http_get "/"
	case "$HTTP_BODY" in
	*"Sign in"*) check_passed "signing out returns the shopper to anonymous" ;;
	*) check_failed "signing out returns the shopper to anonymous" "still signed in" ;;
	esac

	http_get "/accounts/login"
	token="$(http_csrf_token)"
	http_post "/accounts/login" "csrf_token=$token" "username=$SHOPPER_USER" "password=$SHOPPER_PASSWORD"
	if [ "$HTTP_STATUS" = "303" ] || [ "$HTTP_STATUS" = "302" ]; then
		check_passed "the shopper signs back in"
	else
		check_failed "the shopper signs back in" "status $HTTP_STATUS"
	fi

	http_get "/accounts/profile"
	token="$(http_csrf_token)"
	http_post "/accounts/logout" "csrf_token=$token"

	http_get "/accounts/login"
	case "$HTTP_STATUS" in
	200) check_passed "the login page is served to an anonymous visitor" ;;
	*) check_failed "the login page is served to an anonymous visitor" "status $HTTP_STATUS" ;;
	esac

	token="$(http_csrf_token)"
	http_post "/accounts/login" "csrf_token=$token" "username=$SHOPPER_USER" "password=wrong-password-entirely"
	assert_equal "a wrong password is refused with 401" "401" "$HTTP_STATUS"
	case "$HTTP_BODY" in
	*"Invalid username or password"*) check_passed "and the message names neither the user nor the field" ;;
	*) check_failed "the refusal explains itself" "no invalid-credentials message" ;;
	esac

	http_get "/accounts/login"
	token="$(http_csrf_token)"
	http_post "/accounts/login" "csrf_token=$token" "username=$SHOPPER_USER" "password=$SHOPPER_PASSWORD"
	if [ "$HTTP_STATUS" = "303" ] || [ "$HTTP_STATUS" = "302" ]; then
		check_passed "a correct password still works after a wrong one"
	else
		check_failed "a correct password still works after a wrong one" "status $HTTP_STATUS"
	fi
}

check_orders_need_a_session() {
	http_reset_session
	http_get "/orders"
	if [ "$HTTP_STATUS" = "303" ] || [ "$HTTP_STATUS" = "302" ]; then
		check_passed "an anonymous visitor is sent to sign in for their orders"
		case "$(http_header_value Location)" in
		*"next="*) check_passed "the redirect remembers where they were going" ;;
		*) check_failed "the redirect remembers where they were going" "$(http_header_value Location)" ;;
		esac
	else
		check_failed "an anonymous visitor is sent to sign in" "status $HTTP_STATUS"
	fi
}

check_the_customer_is_the_user() {
	finding "a shopper and an administrator are the same kind of account" \
		"contrib/accounts registers into the same users table the admin portal authenticates
against, and auth.User has IsStaff and IsSuperadmin on it. A shop therefore cannot hold customer
fields (marketing consent, default address, loyalty tier) without a side table keyed on user id,
and any future admin bug that flips a flag turns a customer into staff. The swappable user model
is listed as phase 2 in CLAUDE.md; this is what it costs in the meantime."
}

check_the_orders_stage_lands
check_migrate_notices_unimported_migrations
check_the_second_migration
check_the_foreign_keys_are_real
start_the_shop
check_registration
check_registration_signs_you_in
check_sign_out_and_in
check_orders_need_a_session
check_the_customer_is_the_user

chunk_end
