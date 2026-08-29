#!/usr/bin/env bash

. "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib/bootstrap.sh"
. "$E2E_DIR/lib/server.sh"
. "$E2E_DIR/lib/http.sh"

chunk_begin "04" "First run"

trap server_cleanup EXIT

check_the_example_exists() {
	if [ -f "$EXAMPLE_DIR/main.go" ]; then
		return 0
	fi
	check_failed "the example application exists" "run chunk 03 first"
	chunk_end
}

check_the_port_starts_free() {
	if port_is_free; then
		check_passed "port $E2E_PORT is free before starting"
		return
	fi
	force_free_the_port
	if port_is_free; then
		check_skipped "port $E2E_PORT is free before starting" "a stale listener was killed first"
		return
	fi
	check_failed "port $E2E_PORT is free before starting" "still held by $(port_listener_pids | tr '\n' ' ')"
	chunk_end
}

check_the_server_boots() {
	server_start
	if server_wait_for_http; then
		check_passed "coyote start serves a request"
		return 0
	fi
	check_failed "coyote start serves a request" "no response on $E2E_BASE_URL after 60s
$(tail -20 "$SERVER_LOG")"
	return 1
}

check_only_one_process_holds_the_port() {
	local holders
	holders="$(port_listener_pids | wc -l | tr -d ' ')"
	assert_equal "exactly one process holds port $E2E_PORT" "1" "$holders"
}

check_the_startup_log() {
	if server_log_contains "coyote listening"; then
		check_passed "the server logs that it is listening"
	else
		check_failed "the server logs that it is listening" "$(tail -10 "$SERVER_LOG")"
	fi

	if server_log_contains "address already in use"; then
		check_failed "no bind conflict was logged" "$(grep -n 'address already in use' "$SERVER_LOG")"
	else
		check_passed "no bind conflict was logged"
	fi

	if server_log_contains "url=$E2E_BASE_URL"; then
		check_passed "--port overrides the port in .env"
	else
		check_failed "--port overrides the port in .env" \
			"expected url=$E2E_BASE_URL in the log
$(grep -m1 listening "$SERVER_LOG")"
	fi
}

check_the_home_page() {
	assert_http_status "the home page answers 200" "200" "/"
	assert_body_contains "the home page renders the scaffolded content" "Example is running" "/"
	assert_body_contains "the layout supplies the page title" "<title>Home" "/"
	assert_body_contains "the named route resolves in the template" 'href="/"' "/"
}

check_an_unknown_path_is_a_404() {
	assert_http_status "an unknown path answers 404" "404" "/no-such-page"
}

check_a_request_is_logged() {
	if server_log_contains "path=/ status=200"; then
		check_passed "the request log records the served request"
	else
		check_failed "the request log records the served request" "$(tail -5 "$SERVER_LOG")"
	fi
}

check_the_app_shuts_down_gracefully() {
	local listener
	listener="$(port_listener_pids | head -1)"

	if [ -z "$listener" ]; then
		check_skipped "the server shuts down gracefully when signalled directly" "nothing is listening"
		return
	fi

	kill -TERM "$listener" 2>/dev/null || true

	if wait_for_port_to_free 40; then
		check_passed "the server shuts down gracefully when signalled directly"
	else
		check_failed "the server shuts down gracefully when signalled directly" \
			"port still held after 10s by $(port_listener_pids | tr '\n' ' ')"
		force_free_the_port
	fi

	if server_log_contains "shutting down"; then
		check_passed "a graceful shutdown is logged"
	else
		check_failed "a graceful shutdown is logged" "$(tail -5 "$SERVER_LOG")"
	fi
}

check_the_scaffold_returns_a_clean_exit_code() {
	local template="$E2E_ROOT/contrib/scaffold/files/main.go.tmpl"

	if grep -q 'log\.Fatal(a\.Run())' "$template"; then
		check_failed "the scaffolded main.go exits zero on a clean shutdown" \
			"contrib/scaffold/files/main.go.tmpl ends with log.Fatal(a.Run())
Run() returns nil after a graceful shutdown, so log.Fatal(nil) prints <nil> and exits 1
every scaffolded project therefore reports failure on success; it should read
    if err := a.Run(); err != nil {
        log.Fatal(err)
    }"
		return
	fi
	check_passed "the scaffolded main.go exits zero on a clean shutdown"
}

check_the_running_app_exits_zero() {
	local binary="$E2E_WORK_DIR/example-binary" status=0

	if ! grep -q 'log\.Fatal(a\.Run())' "$EXAMPLE_DIR/main.go" 2>/dev/null; then
		check_skipped "the built application exits zero on a clean shutdown" \
			"example/main.go is no longer the scaffolded one, so this would not test the template"
		return
	fi

	cd "$EXAMPLE_DIR"
	if ! go build -o "$binary" . >/dev/null 2>&1; then
		cd "$E2E_ROOT"
		check_failed "the built application exits zero on a clean shutdown" "the example did not build"
		return
	fi
	cd "$E2E_ROOT"

	PORT="$E2E_PORT" "$binary" >"$E2E_WORK_DIR/direct.log" 2>&1 &
	local direct_pid=$!

	local attempt=0
	while [ "$attempt" -lt 40 ] && port_is_free; do
		sleep 0.25
		attempt=$((attempt + 1))
	done

	kill -TERM "$direct_pid" 2>/dev/null || true
	set +e
	wait "$direct_pid"
	status=$?
	set -e
	rm -f "$binary"

	if [ "$status" -eq 0 ]; then
		check_passed "the built application exits zero on a clean shutdown"
		return
	fi
	check_failed "the built application exits zero on a clean shutdown" \
		"the process exited $status after logging a graceful shutdown
$(tail -2 "$E2E_WORK_DIR/direct.log")"
}

check_coyote_start_does_not_orphan_the_server() {
	server_start 0
	if ! server_wait_for_http; then
		check_failed "coyote start releases the port when it is signalled" \
			"the server never came up
$(tail -10 "$SERVER_LOG")"
		return
	fi

	kill -TERM "$SERVER_PID" 2>/dev/null || true

	if wait_for_port_to_free 40; then
		check_passed "coyote start releases the port when it is signalled"
		SERVER_PID=""
		return
	fi

	local orphans
	orphans="$(port_listener_pids | tr '\n' ' ')"
	check_failed "coyote start releases the port when it is signalled" \
		"SIGTERM to the coyote process left pid(s) $orphans holding port $E2E_PORT
coyote signals the go run process, which exits without forwarding to the compiled
binary, so the server is reparented to pid 1 and keeps the port
signalling the process group, or the app directly, does shut it down"

	force_free_the_port
	SERVER_PID=""
}

check_the_port_is_free_afterwards() {
	if port_is_free; then
		check_passed "port $E2E_PORT is free when the chunk ends"
		return
	fi
	check_failed "port $E2E_PORT is free when the chunk ends" \
		"still held by $(port_listener_pids | tr '\n' ' ')"
}

check_the_example_exists
check_the_port_starts_free

if check_the_server_boots; then
	check_only_one_process_holds_the_port
	check_the_startup_log
	check_the_home_page
	check_an_unknown_path_is_a_404
	check_a_request_is_logged
	check_the_app_shuts_down_gracefully
else
	force_free_the_port
fi

check_the_scaffold_returns_a_clean_exit_code
check_the_running_app_exits_zero
check_coyote_start_does_not_orphan_the_server
check_the_port_is_free_afterwards

chunk_end
