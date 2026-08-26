record_result() {
	local kind="$1" description="$2" detail="${3:-}"
	detail="${detail//$'\n'/\\n}"
	printf '%s%s%s%s%s\n' "$kind" "$E2E_SEP" "$description" "$E2E_SEP" "$detail" >>"$E2E_RESULTS"
}

count_results_of_kind() {
	grep -c "^$1$E2E_SEP" "$E2E_RESULTS" 2>/dev/null || true
}

check_passed() {
	LAST_CHECK_OK=1
	record_result PASS "$1"
	printf '  %s✓%s %s\n' "$C_GREEN" "$C_RESET" "$1"
}

check_failed() {
	LAST_CHECK_OK=0
	record_result FAIL "$1" "${2:-}"
	printf '  %s✗%s %s\n' "$C_RED" "$C_RESET" "$1"
	if [ -n "${2:-}" ]; then
		printf '%b\n' "$2" | sed "s/^/      $C_DIM/;s/\$/$C_RESET/"
	fi
}

check_skipped() {
	LAST_CHECK_OK=0
	record_result SKIP "$1" "${2:-}"
	printf '  %s○%s %s %s(%s)%s\n' "$C_YELLOW" "$C_RESET" "$1" "$C_DIM" "${2:-skipped}" "$C_RESET"
}

note() {
	record_result NOTE "$1" "${2:-}"
	printf '  %s·%s %s: %s\n' "$C_DIM" "$C_RESET" "$1" "${2:-}"
}

run_capturing() {
	CAPTURED_OUTPUT=""
	CAPTURED_STATUS=0
	set +e
	CAPTURED_OUTPUT="$("$@" 2>&1)"
	CAPTURED_STATUS=$?
	set -e
}

truncated_output() {
	printf '%s' "$1" | head -20
}

failure_detail() {
	local command_line="$1" output="$2"
	printf 'command: %s\n%s' "$command_line" "$(truncated_output "$output")"
}

assert_succeeds() {
	local description="$1"
	shift
	run_capturing "$@"
	if [ "$CAPTURED_STATUS" -eq 0 ]; then
		check_passed "$description"
		return 0
	fi
	check_failed "$description" "$(failure_detail "$*" "$CAPTURED_OUTPUT")"
	return 0
}

assert_fails() {
	local description="$1"
	shift
	run_capturing "$@"
	if [ "$CAPTURED_STATUS" -ne 0 ]; then
		check_passed "$description"
		return 0
	fi
	check_failed "$description" "$(failure_detail "$*" "expected a non-zero exit, got 0")"
	return 0
}

assert_output_empty() {
	local description="$1"
	shift
	run_capturing "$@"
	if [ "$CAPTURED_STATUS" -eq 0 ] && [ -z "$CAPTURED_OUTPUT" ]; then
		check_passed "$description"
		return 0
	fi
	check_failed "$description" "$(failure_detail "$*" "$CAPTURED_OUTPUT")"
	return 0
}

assert_output_contains() {
	local description="$1" needle="$2"
	shift 2
	run_capturing "$@"
	case "$CAPTURED_OUTPUT" in
	*"$needle"*)
		check_passed "$description"
		return 0
		;;
	esac
	check_failed "$description" "$(failure_detail "$*" "expected to contain: $needle
got: $(truncated_output "$CAPTURED_OUTPUT")")"
	return 0
}

assert_equal() {
	local description="$1" expected="$2" actual="$3"
	if [ "$expected" = "$actual" ]; then
		check_passed "$description"
		return 0
	fi
	check_failed "$description" "expected: $expected
actual:   $actual"
	return 0
}

assert_file_exists() {
	local description="$1" path="$2"
	if [ -f "$path" ]; then
		check_passed "$description"
		return 0
	fi
	check_failed "$description" "no such file: $path"
	return 0
}

assert_file_executable() {
	local description="$1" path="$2"
	if [ -x "$path" ]; then
		check_passed "$description"
		return 0
	fi
	check_failed "$description" "not executable: $path"
	return 0
}

assert_command_exists() {
	local description="$1" name="$2"
	if command -v "$name" >/dev/null 2>&1; then
		check_passed "$description"
		return 0
	fi
	check_failed "$description" "$name is not on PATH"
	return 0
}

last_check_passed() {
	[ "${LAST_CHECK_OK:-0}" = "1" ]
}
