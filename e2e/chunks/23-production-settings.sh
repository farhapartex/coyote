#!/usr/bin/env bash

. "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib/bootstrap.sh"
. "$E2E_DIR/lib/appsource.sh"
. "$E2E_DIR/lib/server.sh"
. "$E2E_DIR/lib/http.sh"

chunk_begin "23" "Production settings"

trap server_cleanup EXIT

check_the_site_is_there() {
	if [ -f "$EXAMPLE_DIR/settings.go" ]; then
		return 0
	fi
	check_failed "the project has settings" "run chunk 03 first"
	chunk_end
}

start_with_environment() {
	cd "$EXAMPLE_DIR"
	set +e
	CAPTURED_OUTPUT="$(env "$@" timeout 12 "$COYOTE_BIN" start "--port=$E2E_PORT" 2>&1 </dev/null)"
	CAPTURED_STATUS=$?
	set -e
	cd "$E2E_ROOT"
	force_free_the_port >/dev/null 2>&1 || true
}

refusal_says() {
	case "$CAPTURED_OUTPUT" in
	*"$1"*)
		return 0
		;;
	esac
	return 1
}

write_ordinary_settings() {
	app_write_settings 1 0 25 1 3 1 1 0 5 0
	cd "$EXAMPLE_DIR"
	assert_succeeds "the project compiles" go build ./...
	cd "$E2E_ROOT"
}

check_production_boots_when_it_is_configured_properly() {
	if ! port_is_free; then
		force_free_the_port
	fi

	SERVER_LOG="$E2E_WORK_DIR/server.log"
	: >"$SERVER_LOG"
	set -m
	(
		cd "$EXAMPLE_DIR" || exit 1
		exec env APP_ENV=production "$COYOTE_BIN" start "--port=$E2E_PORT"
	) >"$SERVER_LOG" 2>&1 &
	SERVER_PID=$!
	SERVER_GROUP_KILL=1
	set +m

	if server_wait_for_http; then
		check_passed "a properly configured production app starts"
	else
		check_failed "a properly configured production app starts" "$(tail -20 "$SERVER_LOG")"
		return
	fi

	if grep -q '"environment":"production"' "$SERVER_LOG"; then
		check_passed "it reports itself as production"
	else
		check_failed "it reports itself as production" "$(grep -m1 listening "$SERVER_LOG")"
	fi

	if grep -q '"debug":false' "$SERVER_LOG"; then
		check_passed "the preset turned debug off"
	else
		check_failed "the preset turned debug off" "$(grep -m1 listening "$SERVER_LOG")"
	fi

	if grep -q '^{"time"' "$SERVER_LOG"; then
		check_passed "the preset switched logging to json"
	else
		check_failed "the preset switched logging to json" "$(head -1 "$SERVER_LOG")"
	fi
}

check_the_session_cookie_hardens() {
	http_get "/quote"
	local cookie
	cookie="$(http_header_value Set-Cookie)"

	case "$cookie" in
	*Secure*)
		check_passed "the session cookie is marked Secure in production"
		;;
	*)
		check_failed "the session cookie is marked Secure in production" "Set-Cookie: ${cookie:-none}"
		;;
	esac

	case "$cookie" in
	*HttpOnly*)
		check_passed "the session cookie stays HttpOnly"
		;;
	*)
		check_failed "the session cookie stays HttpOnly" "Set-Cookie: ${cookie:-none}"
		;;
	esac

	case "$cookie" in
	*SameSite=Lax* | *SameSite=Strict*)
		check_passed "the session cookie carries a SameSite policy"
		;;
	*)
		check_failed "the session cookie carries a SameSite policy" "Set-Cookie: ${cookie:-none}"
		;;
	esac

	server_stop
}

check_a_short_secret_is_refused() {
	start_with_environment APP_ENV=production SECRET_KEY=tooshortforproduction

	assert_equal "a short secret stops production from starting" "1" \
		"$([ "$CAPTURED_STATUS" -ne 0 ] && echo 1 || echo 0)"

	if refusal_says "SecretKey is shorter than 32 characters"; then
		check_passed "the refusal names the secret as the problem"
	else
		check_failed "the refusal names the secret as the problem" "$(truncated_output "$CAPTURED_OUTPUT")"
	fi

	if refusal_says "improperly configured"; then
		check_passed "the refusal is an improperly configured error"
	else
		check_failed "the refusal is an improperly configured error" "$(truncated_output "$CAPTURED_OUTPUT")"
	fi

	if port_is_free; then
		check_passed "nothing is left listening after that refusal"
	else
		check_failed "nothing is left listening after that refusal" "$(port_listener_pids | tr '\n' ' ')"
	fi
}

check_an_empty_secret_is_refused() {
	start_with_environment APP_ENV=production SECRET_KEY=

	assert_equal "an empty secret stops production from starting" "1" \
		"$([ "$CAPTURED_STATUS" -ne 0 ] && echo 1 || echo 0)"

	if refusal_says "SecretKey is empty"; then
		check_passed "the refusal says the secret is empty"
	else
		check_failed "the refusal says the secret is empty" "$(truncated_output "$CAPTURED_OUTPUT")"
	fi

	if refusal_says "GenerateSecretKey"; then
		check_passed "the refusal says how to make one"
	else
		check_failed "the refusal says how to make one" "$(truncated_output "$CAPTURED_OUTPUT")"
	fi
}

check_a_short_secret_is_allowed_in_development() {
	start_with_environment APP_ENV=development SECRET_KEY=tooshort E2E_STOP=1

	if [ "$CAPTURED_STATUS" -ne 0 ] && refusal_says "SecretKey"; then
		check_failed "development tolerates a short secret" \
			"development refused a short key, which would make local work awkward"
	else
		check_passed "development tolerates a short secret"
	fi
	note "why the rule is conditional" \
		"the length check is skipped while Debug is on, so a throwaway key is fine locally and never in production"

	force_free_the_port >/dev/null 2>&1 || true
}

check_an_unknown_environment_is_refused() {
	start_with_environment APP_ENV=produktion

	assert_equal "an unrecognised environment stops the app" "1" \
		"$([ "$CAPTURED_STATUS" -ne 0 ] && echo 1 || echo 0)"

	if refusal_says 'Environment "produktion" is not recognised'; then
		check_passed "the refusal quotes the value it did not understand"
	else
		check_failed "the refusal quotes the value it did not understand" \
			"$(truncated_output "$CAPTURED_OUTPUT")"
	fi

	if refusal_says '"development", "staging" or "production"'; then
		check_passed "the refusal lists the environments it accepts"
	else
		check_failed "the refusal lists the environments it accepts" \
			"$(truncated_output "$CAPTURED_OUTPUT")"
	fi
}

check_staging_is_held_to_the_same_rules() {
	start_with_environment APP_ENV=staging SECRET_KEY=tooshort

	assert_equal "staging refuses a short secret too" "1" \
		"$([ "$CAPTURED_STATUS" -ne 0 ] && echo 1 || echo 0)"
	if refusal_says "SecretKey is shorter than 32 characters"; then
		check_passed "staging gives the same reason as production"
	else
		check_failed "staging gives the same reason as production" \
			"$(truncated_output "$CAPTURED_OUTPUT")"
	fi
	note "staging" "Deployed() covers staging as well as production, so both are held to the deployed rules"
}

check_debug_cannot_be_left_on_in_production() {
	app_write_settings 1 0 25 1 3 1 1 0 5 1

	cd "$EXAMPLE_DIR"
	if ! go build ./... >/dev/null 2>&1; then
		cd "$E2E_ROOT"
		check_failed "the project compiles with debug forced on" "build failed"
		write_ordinary_settings
		return
	fi
	cd "$E2E_ROOT"

	start_with_environment APP_ENV=production

	assert_equal "debug left on stops production from starting" "1" \
		"$([ "$CAPTURED_STATUS" -ne 0 ] && echo 1 || echo 0)"

	if refusal_says "Debug must be off when Environment is production"; then
		check_passed "the refusal names debug as the problem"
	else
		check_failed "the refusal names debug as the problem" "$(truncated_output "$CAPTURED_OUTPUT")"
	fi

	start_with_environment APP_ENV=development
	note "the same settings in development" \
		"debug on is exactly what development wants, so only the deployed profiles object"

	write_ordinary_settings
}

check_a_wildcard_host_is_a_choice_not_an_accident() {
	start_with_environment APP_ENV=production ALLOWED_HOSTS=*

	if [ "$CAPTURED_STATUS" -eq 0 ] || ! refusal_says "AllowedHosts"; then
		check_passed "a wildcard host list is accepted when it is asked for"
		note "the wildcard" \
			"AllowedHosts of \\\"*\\\" is documented as the way to allow any host, so it is a decision rather than an oversight; the refusal only fires when the list is empty"
	else
		check_failed "a wildcard host list is accepted when it is asked for" \
			"$(truncated_output "$CAPTURED_OUTPUT")"
	fi

	force_free_the_port >/dev/null 2>&1 || true
}

check_every_problem_is_reported_at_once() {
	start_with_environment APP_ENV=production SECRET_KEY=short ALLOWED_HOSTS=

	if [ "$CAPTURED_STATUS" -eq 0 ]; then
		check_skipped "several problems are reported together" "that combination started successfully"
		return
	fi

	if refusal_says "SecretKey"; then
		check_passed "the report includes the secret problem"
	else
		check_failed "the report includes the secret problem" "$(truncated_output "$CAPTURED_OUTPUT")"
	fi
	note "how problems are reported" \
		"settings.Configure gathers every problem into one ImproperlyConfigured error rather than stopping at the first"
}

check_the_site_is_there
write_ordinary_settings
check_production_boots_when_it_is_configured_properly
check_the_session_cookie_hardens
check_a_short_secret_is_refused
check_an_empty_secret_is_refused
check_a_short_secret_is_allowed_in_development
check_an_unknown_environment_is_refused
check_staging_is_held_to_the_same_rules
check_debug_cannot_be_left_on_in_production
check_a_wildcard_host_is_a_choice_not_an_accident
check_every_problem_is_reported_at_once

if port_is_free; then
	check_passed "the port is free when the chunk ends"
else
	check_failed "the port is free when the chunk ends" "held by $(port_listener_pids | tr '\n' ' ')"
fi

chunk_end
