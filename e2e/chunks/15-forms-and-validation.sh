#!/usr/bin/env bash

. "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib/bootstrap.sh"
. "$E2E_DIR/lib/appsource.sh"
. "$E2E_DIR/lib/server.sh"
. "$E2E_DIR/lib/http.sh"

chunk_begin "15" "Forms and validation"

trap server_cleanup EXIT

QUOTE_MIGRATION=""
STEPS_BEFORE=0
MIGRATION_WAS_NEW=0

check_the_site_is_there() {
	if [ -f "$EXAMPLE_DIR/handlers_public.go" ]; then
		return 0
	fi
	check_failed "the public site is in place" "run chunk 14 first"
	chunk_end
}

write_the_quote_form() {
	STEPS_BEFORE="$(app_migration_files | wc -l | tr -d ' ')"
	QUOTE_MIGRATION="$(app_migration_for_name quote_requests)"
	if [ -z "$QUOTE_MIGRATION" ]; then
		MIGRATION_WAS_NEW=1
	fi

	app_write_quote_model
	app_write_quote_handlers
	app_write_public_handlers
	app_write_public_templates 15
	app_write_admin_resources 12
	app_write_main 15

	assert_file_exists "the quote model is written" "$EXAMPLE_DIR/models_quote.go"
	assert_file_exists "the quote handlers are written" "$EXAMPLE_DIR/handlers_quote.go"

	cd "$EXAMPLE_DIR"
	assert_succeeds "the project compiles with the quote form" go build ./...
	assert_succeeds "the quote form passes go vet" go vet ./...
	cd "$E2E_ROOT"
}

migrate_the_quote_table() {
	app_run_command makemigrations --name=quote_requests
	assert_equal "makemigrations exits cleanly" "0" "$CAPTURED_STATUS"

	QUOTE_MIGRATION="$(app_migration_for_name quote_requests)"
	assert_file_exists "a migration for the quote table is written" "$QUOTE_MIGRATION"

	app_run_command migrate
	assert_equal "migrate applies the quote table" "0" "$CAPTURED_STATUS"

	if app_table_exists quote_requests; then
		check_passed "the quote_requests table exists"
	else
		check_failed "the quote_requests table exists" "$(truncated_output "$APP_OUTPUT")"
		chunk_end
	fi

	app_sqlite_query "DELETE FROM quote_requests;"
	assert_equal "the table starts empty for this chunk" "0" "$(app_row_count quote_requests)"
}

boot_the_server() {
	if ! port_is_free; then
		force_free_the_port
	fi
	server_start
	if server_wait_for_http; then
		check_passed "the application serves the quote form"
		return 0
	fi
	check_failed "the application serves the quote form" "$(tail -20 "$SERVER_LOG")"
	chunk_end
}

submit_quote() {
	http_get "/quote"
	local token
	token="$(http_csrf_token)"
	http_post "/quote" "csrf_token=$token" "$@"
}

problem_fields() {
	printf '%s' "$HTTP_BODY" | grep -oE 'data-field="[a-z_]+"' | sed 's/data-field="//;s/"//' | sort -u | tr '\n' ' ' | sed 's/ $//'
}

check_the_form_renders() {
	assert_http_status "the quote form answers 200" "200" "/quote"

	http_get "/quote"
	local field
	for field in company email origin_code service containers notes; do
		case "$HTTP_BODY" in
		*"name=\"$field\""*)
			check_passed "the form offers the $field field"
			;;
		*)
			check_failed "the form offers the $field field" "not present on the form"
			;;
		esac
	done

	case "$HTTP_BODY" in
	*'name="csrf_token"'*)
		check_passed "the form carries a CSRF token"
		;;
	*)
		check_failed "the form carries a CSRF token" "no token on the form"
		;;
	esac
}

check_an_empty_submission_reports_every_field() {
	submit_quote

	assert_equal "an empty submission is unprocessable, not a redirect" "422" "$HTTP_STATUS"
	assert_equal "every missing required field is reported at once" \
		"company containers email origin_code service" "$(problem_fields)"
	assert_equal "the empty submission wrote nothing" "0" "$(app_row_count quote_requests)"

	case "$HTTP_BODY" in
	*"is required"*)
		check_passed "the messages say what is wrong"
		;;
	*)
		check_failed "the messages say what is wrong" "no explanation in the response"
		;;
	esac
}

check_each_rule_in_turn() {
	local before
	before="$(app_row_count quote_requests)"

	submit_quote "company=Acme" "email=not-an-email" "origin_code=NLRTM" "service=ocean" "containers=2"
	assert_equal "an unparseable email is refused" "email" "$(problem_fields)"

	submit_quote "company=Acme" "email=a@b.co" "origin_code=NL" "service=ocean" "containers=2"
	assert_equal "a code of the wrong length is refused" "origin_code" "$(problem_fields)"

	submit_quote "company=Acme" "email=a@b.co" "origin_code=NL-TM" "service=ocean" "containers=2"
	assert_equal "a code with punctuation is refused" "origin_code" "$(problem_fields)"

	submit_quote "company=Acme" "email=a@b.co" "origin_code=NLRTM" "service=rail" "containers=2"
	assert_equal "a service outside the list is refused" "service" "$(problem_fields)"
	case "$HTTP_BODY" in
	*"one of ocean, air, customs"*)
		check_passed "the message names the services that are allowed"
		;;
	*)
		check_failed "the message names the services that are allowed" "the list was not offered"
		;;
	esac

	submit_quote "company=Acme" "email=a@b.co" "origin_code=NLRTM" "service=ocean" "containers=0"
	assert_equal "a count below the minimum is refused" "containers" "$(problem_fields)"

	submit_quote "company=Acme" "email=a@b.co" "origin_code=NLRTM" "service=ocean" "containers=9999"
	assert_equal "a count above the maximum is refused" "containers" "$(problem_fields)"

	assert_equal "no rejected submission wrote a row" "$before" "$(app_row_count quote_requests)"
}

check_an_oversize_field_is_refused() {
	local before long
	before="$(app_row_count quote_requests)"
	long="$(printf 'x%.0s' $(seq 1 2100))"

	submit_quote "company=Acme" "email=a@b.co" "origin_code=NLRTM" "service=ocean" \
		"containers=2" "notes=$long"

	assert_equal "notes longer than the limit are refused" "notes" "$(problem_fields)"
	assert_equal "the oversize submission wrote nothing" "$before" "$(app_row_count quote_requests)"
}

check_the_form_keeps_what_was_typed() {
	submit_quote "company=Kestrel Trading" "email=broken" "origin_code=NLRTM" \
		"service=ocean" "containers=7"

	assert_equal "a failed submission is still 422" "422" "$HTTP_STATUS"

	local expected
	for expected in 'value="Kestrel Trading"' 'value="NLRTM"' 'value="7"'; do
		case "$HTTP_BODY" in
		*"$expected"*)
			check_passed "the form redisplays $expected"
			;;
		*)
			check_failed "the form redisplays $expected" "the visitor would have to retype it"
			;;
		esac
	done
}

check_a_dangerous_value_is_escaped() {
	submit_quote "company=<script>alert(1)</script>" "email=broken" "origin_code=NLRTM" \
		"service=ocean" "containers=1"

	case "$HTTP_BODY" in
	*"<script>alert(1)</script>"*)
		check_failed "a script tag is escaped when the form is redisplayed" \
			"the raw tag was written back into the page"
		;;
	*)
		check_passed "a script tag is escaped when the form is redisplayed"
		;;
	esac

	case "$HTTP_BODY" in
	*"&lt;script&gt;"*)
		check_passed "the value survives as escaped text"
		;;
	*)
		check_failed "the value survives as escaped text" "the value was dropped rather than escaped"
		;;
	esac
}

check_a_valid_submission_persists() {
	app_sqlite_query "DELETE FROM quote_requests;"

	submit_quote "company=Acme Trading BV" "email=Buyer@ACME.example" "origin_code=nlrtm" \
		"service=ocean" "containers=4" "notes=Two reefers please"

	assert_equal "a valid submission redirects" "303" "$HTTP_STATUS"
	assert_equal "exactly one row was written" "1" "$(app_row_count quote_requests)"
	assert_equal "the values were stored as the handler normalised them" \
		"Acme Trading BV|buyer@acme.example|NLRTM|ocean|4" \
		"$(app_sqlite_query "SELECT company || '|' || email || '|' || origin_code || '|' || service || '|' || containers FROM quote_requests;")"

	local generated
	generated="$(app_sqlite_query "SELECT id FROM quote_requests;")"
	if [ ${#generated} -ge 32 ]; then
		check_passed "the primary key was generated, not submitted"
	else
		check_failed "the primary key was generated, not submitted" "id was ${generated:-empty}"
	fi

	http_get "/quote"
	case "$HTTP_BODY" in
	*"we will come back with a rate"*)
		check_passed "the visitor is thanked on the page they land on"
		;;
	*)
		check_failed "the visitor is thanked on the page they land on" \
			"no flash message after the redirect"
		;;
	esac

	assert_equal "the flash appears exactly once" "1" \
		"$(printf '%s' "$HTTP_BODY" | grep -c 'flash flash-success' || true)"

	http_get "/quote"
	case "$HTTP_BODY" in
	*"we will come back with a rate"*)
		check_failed "the flash is shown once and then cleared" "it survived a second request"
		;;
	*)
		check_passed "the flash is shown once and then cleared"
		;;
	esac
}

check_a_submission_without_csrf_is_refused() {
	local before
	before="$(app_row_count quote_requests)"

	http_post "/quote" "company=Sneaky Ltd" "email=a@b.co" "origin_code=NLRTM" \
		"service=ocean" "containers=1"

	assert_equal "a submission without a CSRF token is refused" "403" "$HTTP_STATUS"
	assert_equal "the refused submission wrote nothing" "$before" "$(app_row_count quote_requests)"

	http_get "/quote"
	http_post "/quote" "csrf_token=not-the-real-token" "company=Sneaky Ltd" "email=a@b.co" \
		"origin_code=NLRTM" "service=ocean" "containers=1"

	assert_equal "a forged CSRF token is refused" "403" "$HTTP_STATUS"
	assert_equal "the forged submission wrote nothing" "$before" "$(app_row_count quote_requests)"
}

check_an_unexpected_field_is_ignored() {
	local before
	before="$(app_row_count quote_requests)"

	submit_quote "company=Padding Ltd" "email=pad@example.test" "origin_code=SGSIN" \
		"service=air" "containers=1" "id=chosen-by-the-client" "created_at=1999-01-01"

	assert_equal "a submission carrying extra fields still succeeds" "303" "$HTTP_STATUS"
	assert_equal "one more row was written" "$((before + 1))" "$(app_row_count quote_requests)"
	assert_equal "the client could not choose the primary key" "0" \
		"$(app_sqlite_query "SELECT count(*) FROM quote_requests WHERE id='chosen-by-the-client';")"
}

check_the_site_is_there
write_the_quote_form
migrate_the_quote_table
boot_the_server
check_the_form_renders
check_an_empty_submission_reports_every_field
check_each_rule_in_turn
check_an_oversize_field_is_refused
check_the_form_keeps_what_was_typed
check_a_dangerous_value_is_escaped
check_a_valid_submission_persists
check_a_submission_without_csrf_is_refused
check_an_unexpected_field_is_ignored

server_stop
if port_is_free; then
	check_passed "the port is free when the chunk ends"
else
	check_failed "the port is free when the chunk ends" "held by $(port_listener_pids | tr '\n' ' ')"
fi

chunk_end
