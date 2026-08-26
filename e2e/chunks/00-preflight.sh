#!/usr/bin/env bash

. "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib/bootstrap.sh"

chunk_begin "00" "Preflight"

MINIMUM_GO_VERSION="1.25"

check_bash_is_new_enough() {
	if [ "${BASH_VERSINFO[0]}" -ge 4 ]; then
		check_passed "bash is version 4 or newer"
	else
		check_failed "bash is version 4 or newer" \
			"found ${BASH_VERSION}; macOS ships 3.2 at /bin/bash, install a newer one with brew install bash"
	fi
}

check_go_is_new_enough() {
	local found
	found="$(go_version)"

	if [ -z "$found" ]; then
		check_failed "go is on PATH" "go could not be executed"
		return
	fi
	if version_at_least "$found" "$MINIMUM_GO_VERSION"; then
		check_passed "go is $MINIMUM_GO_VERSION or newer"
	else
		check_failed "go is $MINIMUM_GO_VERSION or newer" "found $found"
	fi
}

check_port_is_free() {
	local holder
	holder="$(lsof -ti:"$E2E_PORT" 2>/dev/null || true)"

	if [ -z "$holder" ]; then
		check_passed "port $E2E_PORT is free"
		return
	fi
	check_failed "port $E2E_PORT is free" \
		"held by pid(s) $(printf '%s' "$holder" | tr '\n' ' ')
free it with: lsof -ti:$E2E_PORT | xargs kill -9"
}

report_optional_tool() {
	local name="$1" purpose="$2"
	if command -v "$name" >/dev/null 2>&1; then
		note "$name" "available, $purpose"
	else
		check_skipped "$name is available" "not installed; $purpose"
	fi
}

check_bash_is_new_enough
assert_command_exists "go is on PATH" go
check_go_is_new_enough
assert_command_exists "curl is on PATH" curl
assert_command_exists "git is on PATH" git
check_port_is_free

note "commit" "$(repository_commit)"
note "platform" "$(platform_name)"
note "bash" "$BASH_VERSION"

if repository_is_dirty; then
	note "working tree" "dirty, so this run does not describe a clean commit"
else
	note "working tree" "clean"
fi

if [ -d "$EXAMPLE_DIR" ]; then
	note "example directory" "present; chunk 03 owns creating it"
else
	note "example directory" "absent"
fi

report_optional_tool jq "used for JSON assertions"
report_optional_tool sqlite3 "used for direct schema assertions"
report_optional_tool redis-server "used by the Redis cache checks"

chunk_end
