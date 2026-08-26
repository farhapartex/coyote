#!/usr/bin/env bash

. "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib/bootstrap.sh"

chunk_begin "02" "Command surface"

OUTSIDE_PROJECT="$E2E_WORK_DIR/outside-a-project"

documented_commands() {
	sed -n '/^commands:$/,/^$/p' "$E2E_ROOT/cmd/coyote/usage.go" |
		sed -n 's/^  \([a-z][a-z]*\).*/\1/p' | sort -u
}

command_constant_value() {
	grep -hoE "$1[[:space:]]*=[[:space:]]*\"[a-z]+\"" "$E2E_ROOT"/contrib/cli/*.go |
		head -1 | sed 's/.*"\([a-z]*\)".*/\1/'
}

dispatched_commands() {
	local constant
	grep -oE 'case "[a-z-]+"' "$E2E_ROOT/cmd/coyote/main.go" | sed 's/case "//;s/"//'
	grep -oE 'case cli\.Name[A-Za-z]+' "$E2E_ROOT/cmd/coyote/main.go" | sed 's/case cli\.//' |
		while read -r constant; do
			command_constant_value "$constant"
		done
}

check_the_binary_is_present() {
	if [ -x "$COYOTE_BIN" ]; then
		return 0
	fi
	check_failed "the coyote binary exists" "run chunk 01 first: $COYOTE_BIN is missing"
	chunk_end
}

check_version_is_reported_once() {
	run_capturing_streams "$COYOTE_BIN" version
	assert_equal "coyote version prints one line on stdout" \
		"1" "$(printf '%s\n' "$CAPTURED_STDOUT" | wc -l | tr -d ' ')"
	assert_equal "coyote version and --version agree" \
		"$CAPTURED_STDOUT" "$("$COYOTE_BIN" --version 2>/dev/null)"
}

check_every_help_alias_prints_usage() {
	local alias_name
	for alias_name in help -h --help; do
		assert_stdout_contains "coyote $alias_name prints the usage text" \
			"usage:" "$COYOTE_BIN" "$alias_name"
		assert_exit_code "coyote $alias_name exits 0" 0 "$COYOTE_BIN" "$alias_name"
	done
}

check_no_arguments_prints_usage() {
	assert_stdout_contains "coyote with no arguments prints the usage text" \
		"usage:" "$COYOTE_BIN"
	assert_exit_code "coyote with no arguments exits 0" 0 "$COYOTE_BIN"
}

check_usage_matches_dispatch() {
	local documented dispatched difference
	documented="$(documented_commands)"
	dispatched="$(dispatched_commands | sort -u)"
	difference="$(diff <(printf '%s\n' "$documented") <(printf '%s\n' "$dispatched") || true)"

	if [ -z "$difference" ]; then
		check_passed "every dispatched command is documented, and nothing else"
		note "commands" "$(printf '%s\n' "$documented" | wc -l | tr -d ' ') documented and dispatched"
		return
	fi
	check_failed "every dispatched command is documented, and nothing else" \
		"lines starting < are documented but never dispatched
lines starting > are dispatched but undocumented
$difference"
}

check_usage_documents_flags() {
	local flag
	for flag in --module= --skip-deps --force --port= --fake --steps= --name= --locale= --strict; do
		assert_stdout_contains "the usage text documents $flag" "$flag" "$COYOTE_BIN" help
	done
}

check_an_unknown_flag_is_refused() {
	assert_exit_code "an unknown flag exits non-zero" 1 "$COYOTE_BIN" --nonsense
	assert_stderr_contains "an unknown flag is named on stderr" \
		'unknown flag "--nonsense"' "$COYOTE_BIN" --nonsense
	assert_stdout_contains "an unknown flag still shows the usage text" \
		"usage:" "$COYOTE_BIN" --nonsense
}

check_scaffolding_commands_need_a_name() {
	assert_exit_code "coyote new without a name exits non-zero" 1 "$COYOTE_BIN" new
	assert_stderr_contains "coyote new without a name explains the usage" \
		"coyote new <name>" "$COYOTE_BIN" new

	assert_exit_code "coyote startapp without a name exits non-zero" 1 "$COYOTE_BIN" startapp
	assert_stderr_contains "coyote startapp without a name explains the usage" \
		"coyote startapp <name>" "$COYOTE_BIN" startapp
}

check_project_commands_refuse_to_run_outside_a_project() {
	rm -rf "$OUTSIDE_PROJECT"
	mkdir -p "$OUTSIDE_PROJECT"
	cd "$OUTSIDE_PROJECT"

	local command_name
	for command_name in start migrate collectstatic shell; do
		assert_exit_code "coyote $command_name outside a project exits non-zero" \
			1 "$COYOTE_BIN" "$command_name"
		assert_stderr_contains "coyote $command_name says where it must be run" \
			"run this from the directory holding your main package" "$COYOTE_BIN" "$command_name"
	done

	assert_stdout_empty "a refusal writes nothing to stdout" "$COYOTE_BIN" migrate

	assert_stderr_contains "an unrecognised command is treated as an application command" \
		"run this from the directory holding your main package" "$COYOTE_BIN" frobnicate
	note "unknown commands" \
		"outside a project they report the project error, not \"unknown command\", because cli.Register commands are only knowable once the app runs"

	cd "$E2E_ROOT"
	rm -rf "$OUTSIDE_PROJECT"
}

check_the_binary_is_present
check_version_is_reported_once
check_no_arguments_prints_usage
check_every_help_alias_prints_usage
check_usage_matches_dispatch
check_usage_documents_flags
check_an_unknown_flag_is_refused
check_scaffolding_commands_need_a_name
check_project_commands_refuse_to_run_outside_a_project

chunk_end
