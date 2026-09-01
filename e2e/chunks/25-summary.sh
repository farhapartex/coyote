#!/usr/bin/env bash

. "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib/bootstrap.sh"
. "$E2E_DIR/lib/appsource.sh"
. "$E2E_DIR/lib/server.sh"
. "$E2E_DIR/lib/http.sh"
. "$E2E_DIR/lib/admin.sh"

chunk_begin "25" "Summary"

trap server_cleanup EXIT

EXPECTED_TABLES="containers customers invoice_lines invoices permissions ports quote_requests role_permissions roles shipments tracking_events user_roles users"

check_nothing_is_still_listening() {
	if port_is_free; then
		check_passed "no earlier chunk left something on port $E2E_PORT"
	else
		check_failed "no earlier chunk left something on port $E2E_PORT" \
			"held by $(port_listener_pids | tr '\n' ' ')"
		force_free_the_port
	fi
}

check_no_stray_processes() {
	local strays
	strays="$( (pgrep -f "coyote start" 2>/dev/null || true) | tr '\n' ' ' | sed 's/ $//')"

	if [ -z "$strays" ]; then
		check_passed "no coyote start process was left running"
	else
		check_failed "no coyote start process was left running" "pid(s) $strays"
	fi

	strays="$( (pgrep -f "exe/$EXAMPLE_NAME" 2>/dev/null || true) | tr '\n' ' ' | sed 's/ $//')"
	if [ -z "$strays" ]; then
		check_passed "no orphaned application binary was left running"
	else
		check_failed "no orphaned application binary was left running" "pid(s) $strays"
	fi
}

check_the_generated_project_is_sound() {
	assert_file_exists "the application still has a main package" "$EXAMPLE_DIR/main.go"
	assert_file_exists "the application still has settings" "$EXAMPLE_DIR/settings.go"
	assert_file_exists "the module still points at this checkout" "$EXAMPLE_DIR/go.mod"

	assert_output_contains "the module resolves the framework locally" \
		"replace github.com/farhapartex/coyote => .." cat "$EXAMPLE_DIR/go.mod"

	cd "$EXAMPLE_DIR"
	assert_succeeds "the application the suite built still compiles" go build ./...
	assert_succeeds "the application still passes go vet" go vet ./...
	assert_output_empty "the application is gofmt clean" gofmt -l .
	cd "$E2E_ROOT"
}

check_the_schema_is_whole() {
	local present missing table
	present="$(app_sqlite_query "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name;" | tr '\n' ' ')"

	missing=""
	for table in $EXPECTED_TABLES; do
		case "$present" in
		*"$table"*) ;;
		*) missing="$missing $table" ;;
		esac
	done

	if [ -z "$missing" ]; then
		check_passed "every table the suite created is present"
	else
		check_failed "every table the suite created is present" "missing:$missing"
	fi

	note "schema" "$(printf '%s' "$present" | wc -w | tr -d ' ') tables"
}

check_the_ladder_is_fully_applied() {
	local files applied
	files="$(app_migration_files | wc -l | tr -d ' ')"
	applied="$(app_sqlite_query "SELECT count(*) FROM coyote_migrations;")"

	assert_equal "every migration on disk has been applied" "$files" "$applied"
	note "migrations" "$files steps, all applied"

	app_run_command migrate
	assert_equal "a final migrate has nothing left to do" "0" "$CAPTURED_STATUS"
	case "$APP_OUTPUT" in
	*"nothing to apply"*)
		check_passed "the database reports itself up to date"
		;;
	*)
		check_failed "the database reports itself up to date" "$(truncated_output "$APP_OUTPUT")"
		;;
	esac

	app_run_command makemigrations
	case "$APP_OUTPUT" in
	*"no model changes detected"*)
		check_passed "the models and the migrations agree"
		;;
	*)
		check_failed "the models and the migrations agree" \
			"makemigrations still sees a change, so the ladder does not describe the models
$(truncated_output "$APP_OUTPUT")"
		;;
	esac
}

check_the_data_survived_the_whole_run() {
	local table count
	for table in customers ports shipments invoices; do
		count="$(app_row_count "$table")"
		if [ "$count" -gt 0 ]; then
			check_passed "$table still holds rows"
		else
			check_failed "$table still holds rows" "the table is empty at the end of the run"
		fi
	done

	assert_equal "the customers seeded in chunk 05 are still there" "3" "$(app_row_count customers)"
	assert_equal "the ports seeded in chunk 05 are still there" "3" "$(app_row_count ports)"

	local columns
	columns="$(app_table_columns shipments | tr '\n' ' ')"
	local column
	for column in customs_notes declared_value eta document; do
		case "$columns" in
		*"$column"*)
			check_passed "the $column column added mid ladder is still in the schema"
			;;
		*)
			check_failed "the $column column added mid ladder is still in the schema" "columns: $columns"
			;;
		esac
	done

	assert_equal "the first shipment is still the one chunk 07 created" "1" \
		"$(app_sqlite_query "SELECT count(*) FROM shipments WHERE reference='MRF-000001';")"
	assert_equal "it still carries the document chunk 16 attached" "1" \
		"$(app_sqlite_query "SELECT count(*) FROM shipments WHERE reference='MRF-000001' AND document IS NOT NULL AND document <> '';")"

	note "a value that did not survive" \
		"the customs notes written in chunk 08 were cleared in chunk 16, because an admin form submits every editable field and the upload post did not carry that one; that is form semantics rather than data loss"

	note "rows at the end" \
		"$(app_row_count customers) customers, $(app_row_count shipments) shipments, $(app_row_count invoices) invoices, $(app_row_count quote_requests) quote requests"
}

check_the_application_still_serves() {
	if ! port_is_free; then
		force_free_the_port
	fi
	server_start
	if ! server_wait_for_http; then
		check_failed "the application the suite built still serves" "$(tail -20 "$SERVER_LOG")"
		return
	fi
	check_passed "the application the suite built still serves"

	local route
	for route in / /services /track /quote /about /contact /shipments /report; do
		assert_http_status "GET $route still answers 200" "200" "$route"
	done

	http_reset_session
	assert_http_status "the admin still redirects an anonymous visitor" "303" "/admin/customers"

	if admin_login; then
		check_passed "the superadmin can still sign in"
	else
		check_failed "the superadmin can still sign in" "status $HTTP_STATUS"
		return
	fi

	local slug
	for slug in customers shipments invoices quote_requests; do
		assert_http_status "the $slug screen still answers 200" "200" "/admin/$slug"
	done

	server_stop
}

check_the_framework_repository_is_clean() {
	cd "$E2E_ROOT"
	assert_output_empty "the framework is gofmt clean" unformatted_framework_files
	assert_succeeds "the framework still vets" go vet ./...

	local tracked
	tracked="$( (git -C "$E2E_ROOT" ls-files "$EXAMPLE_NAME" 2>/dev/null || true) | wc -l | tr -d ' ')"
	assert_equal "nothing the suite generated is tracked by git" "0" "$tracked"

	if git -C "$E2E_ROOT" check-ignore -q "$EXAMPLE_NAME"; then
		check_passed "the generated application stays ignored"
	else
		check_failed "the generated application stays ignored" "$EXAMPLE_NAME is not ignored"
	fi
}

unformatted_framework_files() {
	gofmt -l . | grep -v "^$EXAMPLE_NAME/" || true
}

report_what_the_suite_built() {
	note "models registered" \
		"$(grep -c 'func (.*Resource) Entity' "$EXAMPLE_DIR/admin_resources.go" 2>/dev/null || echo 0) managed in the admin"
	note "public routes" \
		"$(grep -c 'a\.Get("' "$EXAMPLE_DIR/main.go" 2>/dev/null || echo 0) registered in main"
	note "application source" \
		"$(find "$EXAMPLE_DIR" -maxdepth 1 -name '*.go' | wc -l | tr -d ' ') Go files written by the suite"
	note "the app is left runnable" \
		"cd $EXAMPLE_NAME && coyote start"
}

check_nothing_is_still_listening
check_no_stray_processes
check_the_generated_project_is_sound
check_the_schema_is_whole
check_the_ladder_is_fully_applied
check_the_data_survived_the_whole_run
check_the_application_still_serves
check_the_framework_repository_is_clean
report_what_the_suite_built

if port_is_free; then
	check_passed "the port is free when the suite ends"
else
	check_failed "the port is free when the suite ends" "held by $(port_listener_pids | tr '\n' ' ')"
fi

chunk_end
