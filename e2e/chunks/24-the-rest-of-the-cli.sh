#!/usr/bin/env bash

. "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib/bootstrap.sh"
. "$E2E_DIR/lib/appsource.sh"

chunk_begin "24" "The rest of the CLI"

STATICFILES="$EXAMPLE_DIR/staticfiles"

check_the_project_is_there() {
	if app_table_exists shipments; then
		return 0
	fi
	check_failed "the project has a schema" "run chunks 05 to 23 first"
	chunk_end
}

run_shell() {
	cd "$EXAMPLE_DIR"
	set +e
	SHELL_OUTPUT="$(printf '%s\n' "$@" | "$COYOTE_BIN" shell 2>&1 | grep -v 'level=')"
	SHELL_STATUS=$?
	set -e
	cd "$E2E_ROOT"
}

shell_says() {
	case "$SHELL_OUTPUT" in
	*"$1"*)
		return 0
		;;
	esac
	return 1
}

check_collectstatic() {
	rm -rf "$STATICFILES"

	app_run_command collectstatic
	assert_equal "collectstatic exits cleanly" "0" "$CAPTURED_STATUS"

	case "$APP_OUTPUT" in
	*"collected"*)
		check_passed "collectstatic says what it collected"
		;;
	*)
		check_failed "collectstatic says what it collected" "$(truncated_output "$APP_OUTPUT")"
		;;
	esac

	assert_file_exists "a manifest is written" "$STATICFILES/manifest.json"

	local fingerprinted
	fingerprinted="$(find "$STATICFILES" -name 'site.*.css' -type f 2>/dev/null | wc -l | tr -d ' ')"
	assert_equal "the stylesheet is written under a fingerprinted name" "1" "$fingerprinted"

	local name
	name="$(basename "$(find "$STATICFILES" -name 'site.*.css' -type f | head -1)")"
	case "$name" in
	site.css)
		check_failed "the fingerprint is part of the filename" "the file kept its plain name"
		;;
	site.*.css)
		check_passed "the fingerprint is part of the filename"
		;;
	*)
		check_failed "the fingerprint is part of the filename" "found $name"
		;;
	esac

	assert_output_contains "the manifest maps the plain name to the fingerprinted one" \
		"site.css" cat "$STATICFILES/manifest.json"

	local first
	first="$name"
	app_run_command collectstatic
	name="$(basename "$(find "$STATICFILES" -name 'site.*.css' -type f | head -1)")"
	assert_equal "collecting twice gives the same fingerprint" "$first" "$name"
	note "fingerprints" "$first, derived from the contents so a changed file gets a new name"
}

check_dbshell_reports_the_connection() {
	app_run_command dbshell

	case "$APP_OUTPUT" in
	*sqlite3*)
		check_passed "dbshell names the client it would run"
		;;
	*)
		check_failed "dbshell names the client it would run" "$(truncated_output "$APP_OUTPUT")"
		;;
	esac

	case "$APP_OUTPUT" in
	*example.db*)
		check_passed "dbshell names the database it would open"
		;;
	*)
		check_failed "dbshell names the database it would open" "$(truncated_output "$APP_OUTPUT")"
		;;
	esac

	case "$APP_OUTPUT" in
	*password*)
		check_failed "dbshell prints no password material" "$(truncated_output "$APP_OUTPUT")"
		;;
	*)
		check_passed "dbshell prints no password material"
		;;
	esac
}

check_the_shell_opens() {
	run_shell ".quit"

	if shell_says "coyote shell"; then
		check_passed "the shell announces itself"
	else
		check_failed "the shell announces itself" "$(truncated_output "$SHELL_OUTPUT")"
	fi

	if shell_says "model(s)"; then
		check_passed "the shell says how many models it knows"
	else
		check_failed "the shell says how many models it knows" "$(truncated_output "$SHELL_OUTPUT")"
	fi
}

check_the_shell_lists_models() {
	run_shell ".models" ".quit"

	local name
	for name in customers ports shipments containers invoices quote_requests users; do
		if shell_says "$name"; then
			check_passed ".models includes $name"
		else
			check_failed ".models includes $name" "$(truncated_output "$SHELL_OUTPUT")"
		fi
	done
}

check_the_shell_counts_and_lists() {
	local customers
	customers="$(app_row_count customers)"

	run_shell "count customers" ".quit"
	if shell_says "$customers"; then
		check_passed "count agrees with the database"
	else
		check_failed "count agrees with the database" \
			"the database holds $customers, the shell said $(truncated_output "$SHELL_OUTPUT")"
	fi

	run_shell "list ports 2" ".quit"
	if shell_says "Rotterdam"; then
		check_passed "list shows a row"
	else
		check_failed "list shows a row" "$(truncated_output "$SHELL_OUTPUT")"
	fi

	if shell_says "2 of"; then
		check_passed "list honours the row limit it was given"
	else
		check_failed "list honours the row limit it was given" "$(truncated_output "$SHELL_OUTPUT")"
	fi
}

check_the_shell_describes_a_model() {
	run_shell ".describe ports" ".quit"

	local column
	for column in "id" "name" "code" "country"; do
		if shell_says "$column"; then
			check_passed ".describe lists the $column column"
		else
			check_failed ".describe lists the $column column" "$(truncated_output "$SHELL_OUTPUT")"
		fi
	done

	if shell_says "primary key"; then
		check_passed ".describe marks the primary key"
	else
		check_failed ".describe marks the primary key" "$(truncated_output "$SHELL_OUTPUT")"
	fi
}

check_the_shell_refuses_nonsense() {
	run_shell "count nosuchmodel" ".quit"
	if shell_says "no model called"; then
		check_passed "an unknown model is reported, not guessed at"
	else
		check_failed "an unknown model is reported, not guessed at" "$(truncated_output "$SHELL_OUTPUT")"
	fi

	run_shell "drop table customers" ".quit"
	if shell_says "unknown command"; then
		check_passed "a command the shell does not have is refused"
	else
		check_failed "a command the shell does not have is refused" "$(truncated_output "$SHELL_OUTPUT")"
	fi
	assert_equal "the customers table survived that attempt" "3" "$(app_row_count customers)"

	run_shell "count customers; drop table customers" ".quit"
	assert_equal "a statement smuggled after a model name changes nothing" "3" "$(app_row_count customers)"
	note "the shell is not a SQL prompt" \
		"it exposes count, list, get and describe over registered models, so there is no statement to inject into"
}

check_a_custom_command() {
	app_write_custom_command

	cd "$EXAMPLE_DIR"
	assert_succeeds "the project compiles with a registered command" go build ./...
	cd "$E2E_ROOT"

	app_run_command lanes
	assert_equal "the custom command runs" "0" "$CAPTURED_STATUS"

	local status total
	for status in booked in_transit draft; do
		total="$(app_sqlite_query "SELECT count(*) FROM shipments WHERE status='$status';")"
		case "$APP_OUTPUT" in
		*"$status $total"*)
			check_passed "the custom command counted $status correctly"
			;;
		*)
			check_failed "the custom command counted $status correctly" \
				"expected $status $total in: $(truncated_output "$APP_OUTPUT")"
			;;
		esac
	done
}

check_a_custom_command_receives_its_arguments() {
	app_run_command lanes booked

	assert_equal "the command runs with an argument" "0" "$CAPTURED_STATUS"

	local booked
	booked="$(app_sqlite_query "SELECT count(*) FROM shipments WHERE status='booked';")"
	case "$APP_OUTPUT" in
	*"booked $booked"*)
		check_passed "the argument reached the command"
		;;
	*)
		check_failed "the argument reached the command" "$(truncated_output "$APP_OUTPUT")"
		;;
	esac

	case "$APP_OUTPUT" in
	*in_transit*)
		check_failed "only the requested status was counted" "the command ignored its argument"
		;;
	*)
		check_passed "only the requested status was counted"
		;;
	esac
}

check_a_custom_command_is_listed() {
	cd "$EXAMPLE_DIR"
	run_capturing_streams "$COYOTE_BIN" nosuchcommand
	cd "$E2E_ROOT"

	assert_equal "an unknown command still fails" "1" \
		"$([ "$CAPTURED_STATUS" -ne 0 ] && echo 1 || echo 0)"

	note "where a custom command lives" \
		"cli.Register in an init function is enough; the coyote binary passes any name it does not know through to the application"
}

check_the_project_is_there
check_collectstatic
check_dbshell_reports_the_connection
check_the_shell_opens
check_the_shell_lists_models
check_the_shell_counts_and_lists
check_the_shell_describes_a_model
check_the_shell_refuses_nonsense
check_a_custom_command
check_a_custom_command_receives_its_arguments
check_a_custom_command_is_listed

chunk_end
