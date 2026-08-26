#!/usr/bin/env bash

. "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib/bootstrap.sh"
. "$E2E_DIR/lib/appsource.sh"
. "$E2E_DIR/lib/server.sh"
. "$E2E_DIR/lib/http.sh"
. "$E2E_DIR/lib/admin.sh"

chunk_begin "06" "Admin for the first models"

trap server_cleanup EXIT

CREATED_CODE="MERIDIAN-E2E"

check_the_schema_is_ready() {
	if app_table_exists customers; then
		return 0
	fi
	check_failed "the customers table exists" "run chunk 05 first"
	chunk_end
}

register_the_admin_resources() {
	app_write_admin_resources
	app_write_main 6

	assert_file_exists "admin resources are written" "$EXAMPLE_DIR/admin_resources.go"
	assert_output_contains "the portal manages both resources" \
		"portal.MustManage(customerResource{}, portResource{})" cat "$EXAMPLE_DIR/main.go"

	cd "$EXAMPLE_DIR"
	assert_succeeds "the project compiles with the admin resources" go build ./...
	cd "$E2E_ROOT"
}

check_createsuperadmin() {
	if admin_superadmin_exists; then
		check_skipped "createsuperadmin creates an account without prompting" \
			"$ADMIN_USERNAME already exists from an earlier run"
	else
		admin_create_superadmin
		if [ "$CAPTURED_STATUS" -eq 0 ]; then
			check_passed "createsuperadmin creates an account without prompting"
		else
			check_failed "createsuperadmin creates an account without prompting" \
				"exit $CAPTURED_STATUS
$(truncated_output "$APP_OUTPUT")"
			chunk_end
		fi
	fi

	assert_equal "the account is a superadmin and staff" "1|1" \
		"$(app_sqlite_query "SELECT is_superadmin || '|' || is_staff FROM users WHERE username='$ADMIN_USERNAME';")"

	local stored
	stored="$(app_sqlite_query "SELECT password FROM users WHERE username='$ADMIN_USERNAME';")"
	case "$stored" in
	*"$ADMIN_PASSWORD"*)
		check_failed "the password is stored hashed, never in the clear" "the column contains the password"
		;;
	"")
		check_failed "the password is stored hashed, never in the clear" "no password column value"
		;;
	*)
		check_passed "the password is stored hashed, never in the clear"
		;;
	esac
}

check_a_duplicate_superadmin_is_refused() {
	admin_create_superadmin
	if [ "$CAPTURED_STATUS" -ne 0 ]; then
		check_passed "a second superadmin with the same username is refused"
	else
		check_failed "a second superadmin with the same username is refused" \
			"the command exited 0
$(truncated_output "$APP_OUTPUT")"
	fi
}

boot_the_server() {
	if ! port_is_free; then
		force_free_the_port
	fi
	server_start
	if server_wait_for_http; then
		check_passed "the application serves with the admin portal mounted"
		return 0
	fi
	check_failed "the application serves with the admin portal mounted" \
		"no response on $E2E_BASE_URL
$(tail -20 "$SERVER_LOG")"
	chunk_end
}

check_the_portal_requires_a_login() {
	http_reset_session
	http_get "$ADMIN_PREFIX/customers"

	assert_equal "an anonymous visitor cannot list customers" "303" "$HTTP_STATUS"

	local location
	location="$(http_header_value Location)"
	case "$location" in
	*"/admin/login"*)
		check_passed "the guard redirects to the admin login"
		;;
	*)
		check_failed "the guard redirects to the admin login" "Location: ${location:-none}"
		;;
	esac
}

check_signing_in() {
	if admin_login; then
		check_passed "the superadmin can sign in"
	else
		check_failed "the superadmin can sign in" "status $HTTP_STATUS
$(printf '%s' "$HTTP_BODY" | head -5)"
		chunk_end
	fi

	http_get "$ADMIN_PREFIX/"
	assert_equal "the dashboard answers 200 once signed in" "200" "$HTTP_STATUS"
}

check_both_models_are_listed() {
	http_get "$ADMIN_PREFIX/"

	local model_name
	for model_name in Customer Port; do
		case "$HTTP_BODY" in
		*"$model_name"*)
			check_passed "the dashboard offers $model_name"
			;;
		*)
			check_failed "the dashboard offers $model_name" "not mentioned on the dashboard"
			;;
		esac
	done

	assert_http_status "the customers list answers 200" "200" "$ADMIN_PREFIX/customers"
	assert_http_status "the ports list answers 200" "200" "$ADMIN_PREFIX/ports"
}

check_the_seeded_rows_are_visible() {
	http_get "$ADMIN_PREFIX/customers"

	local expected
	for expected in "Nordwind Logistics" "Kestrel Trading" "Alpine Freight"; do
		case "$HTTP_BODY" in
		*"$expected"*)
			check_passed "the list shows $expected"
			;;
		*)
			check_failed "the list shows $expected" "missing from the customers list"
			;;
		esac
	done

	http_get "$ADMIN_PREFIX/ports"
	assert_body_contains "the ports list shows a seeded port" "Rotterdam" "$ADMIN_PREFIX/ports"
}

check_the_create_form_omits_the_primary_key() {
	http_get "$ADMIN_PREFIX/customers/new"
	assert_equal "the create form answers 200" "200" "$HTTP_STATUS"

	local fields
	fields="$(printf '%s' "$HTTP_BODY" | grep -oE 'name="[a-z_]+"' | sort -u | tr '\n' ' ')"

	case "$fields" in
	*'name="id"'*)
		check_failed "the create form does not offer the primary key" "fields: $fields"
		;;
	*)
		check_passed "the create form does not offer the primary key"
		;;
	esac

	local field
	for field in name code country; do
		case "$fields" in
		*"name=\"$field\""*)
			check_passed "the create form offers $field"
			;;
		*)
			check_failed "the create form offers $field" "fields: $fields"
			;;
		esac
	done
}

check_creating_a_record() {
	app_sqlite_query "DELETE FROM customers WHERE code='$CREATED_CODE';"

	http_get "$ADMIN_PREFIX/customers/new"
	admin_submit "$ADMIN_PREFIX/customers/new" \
		"name=Meridian Freight" "code=$CREATED_CODE" "country=NL"

	assert_equal "creating a customer redirects back to the list" "303" "$HTTP_STATUS"
	assert_equal "the new customer is in the database" "1" \
		"$(app_sqlite_query "SELECT count(*) FROM customers WHERE code='$CREATED_CODE';")"

	local generated
	generated="$(app_sqlite_query "SELECT id FROM customers WHERE code='$CREATED_CODE';")"
	if [ ${#generated} -ge 32 ]; then
		check_passed "the primary key was generated by the framework"
	else
		check_failed "the primary key was generated by the framework" "id was ${generated:-empty}"
	fi

	assert_body_contains "the new customer appears in the list" "Meridian Freight" "$ADMIN_PREFIX/customers"
}

check_editing_a_record() {
	local record_id
	record_id="$(app_sqlite_query "SELECT id FROM customers WHERE code='$CREATED_CODE';")"

	if [ -z "$record_id" ]; then
		check_skipped "editing a customer saves the change" "the record was not created"
		return
	fi

	http_get "$ADMIN_PREFIX/customers/$record_id"
	assert_equal "the edit form answers 200" "200" "$HTTP_STATUS"
	assert_body_contains "the edit form is filled in with the record" "$CREATED_CODE" "$ADMIN_PREFIX/customers/$record_id"

	http_get "$ADMIN_PREFIX/customers/$record_id"
	admin_submit "$ADMIN_PREFIX/customers/$record_id" \
		"name=Meridian Freight BV" "code=$CREATED_CODE" "country=BE"

	assert_equal "editing a customer redirects back to the list" "303" "$HTTP_STATUS"
	assert_equal "editing a customer saves the change" "Meridian Freight BV|BE" \
		"$(app_sqlite_query "SELECT name || '|' || country FROM customers WHERE code='$CREATED_CODE';")"
	assert_equal "editing did not create a second row" "1" \
		"$(app_sqlite_query "SELECT count(*) FROM customers WHERE code='$CREATED_CODE';")"
}

check_the_search_box() {
	http_get "$ADMIN_PREFIX/customers?q=KESTREL"

	case "$HTTP_BODY" in
	*"Kestrel Trading"*)
		check_passed "searching finds the matching customer"
		;;
	*)
		check_failed "searching finds the matching customer" "Kestrel Trading was not in the results"
		;;
	esac

	case "$HTTP_BODY" in
	*"Nordwind Logistics"*)
		check_failed "searching excludes the customers that do not match" \
			"Nordwind Logistics was still listed for q=KESTREL"
		;;
	*)
		check_passed "searching excludes the customers that do not match"
		;;
	esac
}

check_a_write_without_csrf_is_refused() {
	local record_id
	record_id="$(app_sqlite_query "SELECT id FROM customers WHERE code='$CREATED_CODE';")"

	if [ -z "$record_id" ]; then
		check_skipped "a write without a CSRF token is refused" "no record to target"
		return
	fi

	http_post "$ADMIN_PREFIX/customers/$record_id" "name=Tampered" "code=$CREATED_CODE" "country=XX"
	assert_equal "a write without a CSRF token is refused" "403" "$HTTP_STATUS"
	assert_equal "the refused write changed nothing" "Meridian Freight BV" \
		"$(app_sqlite_query "SELECT name FROM customers WHERE code='$CREATED_CODE';")"
}

check_deleting_a_record() {
	local record_id
	record_id="$(app_sqlite_query "SELECT id FROM customers WHERE code='$CREATED_CODE';")"

	if [ -z "$record_id" ]; then
		check_skipped "deleting a customer removes it" "no record to delete"
		return
	fi

	http_get "$ADMIN_PREFIX/customers/$record_id"
	admin_submit "$ADMIN_PREFIX/customers/$record_id/delete"

	assert_equal "deleting a customer redirects back to the list" "303" "$HTTP_STATUS"
	assert_equal "deleting a customer removes it" "0" \
		"$(app_sqlite_query "SELECT count(*) FROM customers WHERE code='$CREATED_CODE';")"
	assert_equal "the seeded customers are untouched by the delete" "3" "$(app_row_count customers)"
}

check_signing_out() {
	http_get "$ADMIN_PREFIX/"
	admin_submit "$ADMIN_PREFIX/logout"

	http_get "$ADMIN_PREFIX/customers"
	assert_equal "after signing out the portal is guarded again" "303" "$HTTP_STATUS"
}

check_the_schema_is_ready
register_the_admin_resources
check_createsuperadmin
check_a_duplicate_superadmin_is_refused
boot_the_server
check_the_portal_requires_a_login
check_signing_in
check_both_models_are_listed
check_the_seeded_rows_are_visible
check_the_create_form_omits_the_primary_key
check_creating_a_record
check_editing_a_record
check_the_search_box
check_a_write_without_csrf_is_refused
check_deleting_a_record
check_signing_out

server_stop
check_the_port_is_free() {
	if port_is_free; then
		check_passed "the port is free when the chunk ends"
	else
		check_failed "the port is free when the chunk ends" "held by $(port_listener_pids | tr '\n' ' ')"
	fi
}
check_the_port_is_free

chunk_end
