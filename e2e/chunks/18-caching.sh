#!/usr/bin/env bash

. "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib/bootstrap.sh"
. "$E2E_DIR/lib/appsource.sh"
. "$E2E_DIR/lib/server.sh"
. "$E2E_DIR/lib/http.sh"

chunk_begin "18" "Caching"

trap server_cleanup EXIT

PAGE_TTL=3
DEAD_REDIS="127.0.0.1:6399"

check_the_data_is_there() {
	if app_table_exists shipments && [ "$(app_row_count shipments)" -ge 3 ]; then
		return 0
	fi
	check_failed "there are shipments to report on" "run chunks 05 to 17 first"
	chunk_end
}

turn_caching_on() {
	app_write_report_handler
	app_write_public_templates 15
	app_write_settings 1 0 25 1 "$PAGE_TTL"
	app_write_main 18

	assert_output_contains "a memory cache is the default" 'settings.MemoryCache("default")' \
		cat "$EXAMPLE_DIR/settings.go"
	assert_output_contains "a file cache backs the page cache" \
		'Backend: settings.CacheInFile' cat "$EXAMPLE_DIR/settings.go"
	assert_output_contains "the page cache is enabled" "Enabled: true" cat "$EXAMPLE_DIR/settings.go"

	cd "$EXAMPLE_DIR"
	assert_succeeds "the project compiles with two caches" go build ./...
	assert_succeeds "the cached report passes go vet" go vet ./...
	cd "$E2E_ROOT"

	rm -rf "$EXAMPLE_DIR/cache"
}

boot_the_server() {
	if ! port_is_free; then
		force_free_the_port
	fi
	server_start
	if server_wait_for_http; then
		check_passed "the application serves with caching enabled"
		return 0
	fi
	check_failed "the application serves with caching enabled" "$(tail -20 "$SERVER_LOG")"
	chunk_end
}

check_both_caches_are_ready() {
	local alias
	for alias in default pages; do
		if grep -q "cache ready\" alias=$alias" "$SERVER_LOG" 2>/dev/null; then
			check_passed "the $alias cache reported itself ready at startup"
		else
			check_failed "the $alias cache reported itself ready at startup" \
				"$(grep -c 'cache ready' "$SERVER_LOG" || true) cache ready lines in the log"
		fi
	done
}

builds_reported() {
	printf '%s' "$HTTP_BODY" | grep -oE 'data-builds="[0-9]+"' | head -1 | sed 's/data-builds="//;s/"//'
}

cache_header() {
	http_get "$1"
	http_header_value X-Cache
}

check_remember_builds_once() {
	http_get "/report"
	assert_equal "the report answers 200" "200" "$HTTP_STATUS"

	local first
	first="$(builds_reported)"
	if [ -z "$first" ]; then
		check_failed "the report says which build produced it" "no build marker in the page"
		return
	fi
	check_passed "the report says which build produced it"

	local repeat
	for repeat in 1 2 3; do
		http_get "/report"
		assert_equal "request $repeat is served from the same build" "$first" "$(builds_reported)"
	done

	assert_body_contains "the report carries a real figure" 'data-status="booked"' "/report"
}

check_remember_costs_no_queries_on_a_hit() {
	local before after

	http_get "/report"
	before="$(grep -c 'gorm query' "$SERVER_LOG" || true)"
	http_get "/report"
	after="$(grep -c 'gorm query' "$SERVER_LOG" || true)"

	note "queries on a cached report" "$((after - before))"

	if [ "$((after - before))" -le 1 ]; then
		check_passed "a cached report does not go back to the database"
	else
		check_failed "a cached report does not go back to the database" \
			"$((after - before)) queries ran for a request that should have been a cache hit"
	fi
}

check_the_page_cache_hits() {
	rm -rf "$EXAMPLE_DIR/cache"

	local first second
	first="$(cache_header "/about")"
	second="$(cache_header "/about")"

	assert_equal "the first request to a cached path is a miss" "MISS" "$first"
	assert_equal "the second request is a hit" "HIT" "$second"

	http_get "/about"
	local first_hit
	first_hit="$(printf '%s' "$HTTP_BODY" | cksum)"
	http_get "/about"
	assert_equal "two hits return byte identical bodies" "$first_hit" \
		"$(printf '%s' "$HTTP_BODY" | cksum)"
}

check_the_page_cache_writes_to_disk() {
	local files
	files="$(find "$EXAMPLE_DIR/cache" -type f 2>/dev/null | wc -l | tr -d ' ')"

	if [ "$files" -ge 1 ]; then
		check_passed "the file backed page cache wrote an entry to disk"
	else
		check_failed "the file backed page cache wrote an entry to disk" "no files under cache/"
	fi

	local nested
	nested="$(find "$EXAMPLE_DIR/cache" -type f 2>/dev/null | head -1)"
	case "$nested" in
	*/pages/*)
		check_passed "the entry sits under the alias's own directory"
		;;
	*)
		check_failed "the entry sits under the alias's own directory" "found at ${nested:-nothing}"
		;;
	esac
}

check_the_page_cache_expires() {
	local before after
	before="$(cache_header "/about")"

	sleep $((PAGE_TTL + 1))
	after="$(cache_header "/about")"

	assert_equal "the entry was a hit before the ttl elapsed" "HIT" "$before"
	assert_equal "the entry is a miss once the ttl has elapsed" "MISS" "$after"
	note "page cache ttl" "${PAGE_TTL}s, waited $((PAGE_TTL + 1))s"
}

check_a_page_that_sets_a_cookie_is_never_cached() {
	local attempt header cookies
	for attempt in 1 2 3; do
		http_get "/quote"
		header="$(http_header_value X-Cache)"
		cookies="$(http_header_value Set-Cookie)"

		if [ "$header" = "HIT" ]; then
			check_failed "a page that sets a cookie is never cached" \
				"request $attempt was a HIT, so one visitor's page could reach another"
			return
		fi
	done

	check_passed "a page that sets a cookie is never cached"

	if [ -n "$cookies" ]; then
		check_passed "that page does set a cookie, which is why it is skipped"
	else
		check_skipped "that page does set a cookie, which is why it is skipped" \
			"no Set-Cookie was observed, so the rule was not the reason"
	fi

	note "the rule that matters" \
		"a response carrying Set-Cookie is never stored, so per visitor pages cannot leak through the cache"
}

check_an_uncached_path_has_no_header() {
	http_get "/services"
	local header
	header="$(http_header_value X-Cache)"

	if [ -z "$header" ]; then
		check_passed "a path outside the cached list is left alone entirely"
	else
		check_failed "a path outside the cached list is left alone entirely" "X-Cache: $header"
	fi
}

check_redis_configured_but_absent_refuses_to_boot() {
	server_stop

	if [ -n "$(lsof -ti:6399 2>/dev/null)" ]; then
		check_skipped "a configured but unreachable Redis stops the server" \
			"something is listening on $DEAD_REDIS"
		return
	fi

	app_write_settings 1 0 25 redis

	cd "$EXAMPLE_DIR"
	if ! go build ./... >/dev/null 2>&1; then
		cd "$E2E_ROOT"
		check_failed "the project compiles with a Redis cache configured" "build failed"
		return
	fi
	cd "$E2E_ROOT"
	check_passed "the project compiles with a Redis cache configured"

	app_run_command start "--port=$E2E_PORT"

	if [ "$CAPTURED_STATUS" -ne 0 ]; then
		check_passed "a configured but unreachable Redis stops the server"
	else
		check_failed "a configured but unreachable Redis stops the server" \
			"the command exited 0, so the server started without its cache"
	fi

	case "$APP_OUTPUT" in
	*"is unreachable"*)
		check_passed "the refusal says the cache is unreachable"
		;;
	*)
		check_failed "the refusal says the cache is unreachable" "$(truncated_output "$APP_OUTPUT")"
		;;
	esac

	case "$APP_OUTPUT" in
	*"$DEAD_REDIS"*)
		check_passed "the refusal names the address it tried"
		;;
	*)
		check_failed "the refusal names the address it tried" "$(truncated_output "$APP_OUTPUT")"
		;;
	esac

	case "$APP_OUTPUT" in
	*"start Redis, or set Caches[0].Backend"*)
		check_passed "the refusal says how to fix it"
		;;
	*)
		check_failed "the refusal says how to fix it" "$(truncated_output "$APP_OUTPUT")"
		;;
	esac

	if port_is_free; then
		check_passed "nothing is left listening after the refusal"
	else
		check_failed "nothing is left listening after the refusal" \
			"held by $(port_listener_pids | tr '\n' ' ')"
		force_free_the_port
	fi
}

restore_the_working_caches() {
	app_write_settings 1 0 25 1 "$PAGE_TTL"
	cd "$EXAMPLE_DIR"
	assert_succeeds "the project builds again with reachable caches" go build ./...
	cd "$E2E_ROOT"

	if ! port_is_free; then
		force_free_the_port
	fi
	server_start
	if server_wait_for_http; then
		check_passed "the application serves again once the cache is reachable"
	else
		check_failed "the application serves again once the cache is reachable" \
			"$(tail -20 "$SERVER_LOG")"
	fi
	server_stop
}

check_the_data_is_there
turn_caching_on
boot_the_server
check_both_caches_are_ready
check_remember_builds_once
check_remember_costs_no_queries_on_a_hit
check_the_page_cache_hits
check_the_page_cache_writes_to_disk
check_the_page_cache_expires
check_a_page_that_sets_a_cookie_is_never_cached
check_an_uncached_path_has_no_header
check_redis_configured_but_absent_refuses_to_boot
restore_the_working_caches

if port_is_free; then
	check_passed "the port is free when the chunk ends"
else
	check_failed "the port is free when the chunk ends" "held by $(port_listener_pids | tr '\n' ' ')"
fi

chunk_end
