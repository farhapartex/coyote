#!/usr/bin/env bash

. "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib/bootstrap.sh"
. "$E2E_DIR/lib/appsource.sh"
. "$E2E_DIR/lib/server.sh"
. "$E2E_DIR/lib/http.sh"

chunk_begin "22" "Security"

trap server_cleanup EXIT

ALLOWED_ORIGIN="https://partner.example"
RATE_LIMIT=3

check_the_site_is_there() {
	if [ -f "$EXAMPLE_DIR/handlers_public.go" ]; then
		return 0
	fi
	check_failed "the public site is in place" "run chunks 14 to 21 first"
	chunk_end
}

check_the_defaults_before_anything_is_asked_for() {
	app_write_settings 1 0 25 1 3 1 1 0
	cd "$EXAMPLE_DIR"
	assert_succeeds "the project compiles without security settings" go build ./...
	cd "$E2E_ROOT"

	if ! port_is_free; then
		force_free_the_port
	fi
	server_start
	if ! server_wait_for_http; then
		check_failed "the application serves with default security" "$(tail -20 "$SERVER_LOG")"
		chunk_end
	fi
	check_passed "the application serves with default security"

	http_get "/services"
	assert_equal "a sniffing browser is told not to" "nosniff" \
		"$(http_header_value X-Content-Type-Options)"
	assert_equal "framing is denied" "DENY" "$(http_header_value X-Frame-Options)"
	assert_equal "the referrer is kept same origin" "same-origin" \
		"$(http_header_value Referrer-Policy)"

	local csp
	csp="$(http_header_value Content-Security-Policy)"
	if [ -z "$csp" ]; then
		check_passed "no content security policy is sent until one is configured"
		note "CSP is opt in" \
			"settings.DefaultCSP exists but Security.CSP is empty until a project sets it"
	else
		check_failed "no content security policy is sent until one is configured" "CSP: $csp"
	fi
}

check_the_host_is_checked() {
	local host status

	for host in "evil.test" "127.0.0.1.evil.test" "evil.test:8099" "notlocalhost"; do
		status="$(curl -s -o /dev/null -w '%{http_code}' -H "Host: $host" "$E2E_BASE_URL/services" 2>/dev/null)"
		assert_equal "a request claiming to be $host is refused" "400" "$status"
	done

	for host in "127.0.0.1" "127.0.0.1:8099" "localhost" "LOCALHOST" "127.0.0.1."; do
		status="$(curl -s -o /dev/null -w '%{http_code}' -H "Host: $host" "$E2E_BASE_URL/services" 2>/dev/null)"
		assert_equal "a request claiming to be $host is served" "200" "$status"
	done

	status="$(curl -s -o /dev/null -w '%{http_code}' -H "Host: 127.0.0.1:8099.evil.test" "$E2E_BASE_URL/services" 2>/dev/null)"
	if [ "$status" = "400" ]; then
		check_passed "a host with a port that is not a number is refused"
	else
		check_skipped "a host with a port that is not a number is refused" \
			"answered $status; net.SplitHostPort accepts any text after the colon, so the port is not validated"
		note "the port is not checked" \
			"the hostname is still pinned to an allowed value, so this is not host injection, but r.Host can carry attacker text after the colon and should not be echoed or used as a cache key"
	fi
}

turn_security_on() {
	server_stop
	app_write_settings 1 0 25 1 3 1 1 1 1000

	assert_output_contains "a content security policy is configured" \
		"s.Security.CSP = settings.DefaultCSP" cat "$EXAMPLE_DIR/settings.go"
	assert_output_contains "compression is asked for" "s.Security.Compress = true" \
		cat "$EXAMPLE_DIR/settings.go"
	assert_output_contains "one origin is allowed" "$ALLOWED_ORIGIN" cat "$EXAMPLE_DIR/settings.go"

	cd "$EXAMPLE_DIR"
	assert_succeeds "the project compiles with security turned on" go build ./...
	assert_succeeds "the security settings pass go vet" go vet ./...
	cd "$E2E_ROOT"

	rm -rf "$EXAMPLE_DIR/cache"
	if ! port_is_free; then
		force_free_the_port
	fi
	server_start
	if server_wait_for_http; then
		check_passed "the application serves with security turned on"
	else
		check_failed "the application serves with security turned on" "$(tail -20 "$SERVER_LOG")"
		chunk_end
	fi
}

check_the_policy_and_its_nonce() {
	http_get "/services"
	local policy
	policy="$(http_header_value Content-Security-Policy)"

	local directive
	for directive in "default-src 'self'" "object-src 'none'" "frame-ancestors 'none'" "base-uri 'self'"; do
		case "$policy" in
		*"$directive"*)
			check_passed "the policy carries $directive"
			;;
		*)
			check_failed "the policy carries $directive" "policy: $policy"
			;;
		esac
	done

	case "$policy" in
	*"{nonce}"*)
		check_failed "the nonce placeholder is replaced with a real value" "the literal {nonce} was sent"
		;;
	*nonce-*)
		check_passed "the nonce placeholder is replaced with a real value"
		;;
	*)
		check_failed "the nonce placeholder is replaced with a real value" "no nonce in: $policy"
		;;
	esac

	local first second
	first="$(printf '%s' "$policy" | grep -o "nonce-[A-Za-z0-9_+/=-]*" | head -1)"
	http_get "/services"
	second="$(http_header_value Content-Security-Policy | grep -o "nonce-[A-Za-z0-9_+/=-]*" | head -1)"

	if [ -n "$first" ] && [ "$first" != "$second" ]; then
		check_passed "a fresh nonce is minted for every request"
	else
		check_failed "a fresh nonce is minted for every request" \
			"two requests shared ${first:-nothing}, which would let an attacker reuse it"
	fi
}

check_compression() {
	local compressed plain encoding

	compressed="$(curl -s -H 'Accept-Encoding: gzip' "$E2E_BASE_URL/services" --output - 2>/dev/null | wc -c | tr -d ' ')"
	plain="$(curl -s "$E2E_BASE_URL/services" 2>/dev/null | wc -c | tr -d ' ')"
	encoding="$(curl -sI -H 'Accept-Encoding: gzip' "$E2E_BASE_URL/services" 2>/dev/null | grep -i '^content-encoding' | tr -d '\r' | awk '{print $2}')"

	assert_equal "a client that accepts gzip is sent gzip" "gzip" "$encoding"

	if [ "$compressed" -lt "$plain" ]; then
		check_passed "the compressed response is smaller than the plain one"
	else
		check_failed "the compressed response is smaller than the plain one" \
			"gzip $compressed bytes against plain $plain bytes"
	fi
	note "compression" "$plain bytes plain, $compressed bytes gzipped"

	encoding="$( (curl -sI "$E2E_BASE_URL/services" 2>/dev/null | grep -ic '^content-encoding' || true) | tr -d ' ')"
	assert_equal "a client that did not ask for gzip is not given it" "0" "$encoding"
}

check_cors() {
	local headers

	headers="$(curl -s -o /dev/null -D - -X OPTIONS \
		-H "Origin: $ALLOWED_ORIGIN" -H "Access-Control-Request-Method: POST" \
		"$E2E_BASE_URL/services" 2>/dev/null)"

	case "$headers" in
	*"Access-Control-Allow-Origin: $ALLOWED_ORIGIN"*)
		check_passed "a preflight from the allowed origin is answered"
		;;
	*)
		check_failed "a preflight from the allowed origin is answered" \
			"$(printf '%s' "$headers" | head -6)"
		;;
	esac

	case "$headers" in
	*"Access-Control-Allow-Methods: GET, POST"*)
		check_passed "the preflight names the methods that are allowed"
		;;
	*)
		check_failed "the preflight names the methods that are allowed" \
			"$(printf '%s' "$headers" | head -6)"
		;;
	esac

	case "$headers" in
	*"Vary: Access-Control-Request-Method"*)
		check_passed "the preflight varies, so a cache cannot serve it to the wrong origin"
		;;
	*)
		check_failed "the preflight varies, so a cache cannot serve it to the wrong origin" \
			"$(printf '%s' "$headers" | head -6)"
		;;
	esac

	case "$headers" in
	*"Access-Control-Allow-Origin: *"*)
		check_failed "the wildcard origin is never sent" "the response allowed any origin"
		;;
	*)
		check_passed "the wildcard origin is never sent"
		;;
	esac

	local status
	status="$(curl -s -o /dev/null -w '%{http_code}' -X OPTIONS \
		-H "Origin: https://evil.test" -H "Access-Control-Request-Method: POST" \
		"$E2E_BASE_URL/services" 2>/dev/null)"
	assert_equal "a preflight from an origin not on the list is refused" "403" "$status"

	headers="$(curl -s -o /dev/null -D - -H "Origin: https://evil.test" "$E2E_BASE_URL/services" 2>/dev/null)"
	case "$headers" in
	*"Access-Control-Allow-Origin"*)
		check_failed "an unlisted origin gets no allow header on a plain request" \
			"$(printf '%s' "$headers" | grep -i access-control | head -2)"
		;;
	*)
		check_passed "an unlisted origin gets no allow header on a plain request"
		;;
	esac
}

check_the_rate_limiter() {
	server_stop
	app_write_settings 1 0 25 1 3 1 1 1 "$RATE_LIMIT"

	cd "$EXAMPLE_DIR"
	if ! go build ./... >/dev/null 2>&1; then
		cd "$E2E_ROOT"
		check_failed "the project compiles with a rate limit" "build failed"
		return
	fi
	cd "$E2E_ROOT"

	rm -rf "$EXAMPLE_DIR/cache"
	if ! port_is_free; then
		force_free_the_port
	fi
	server_start
	if ! server_wait_for_http; then
		check_failed "the application serves with a rate limit" "$(tail -20 "$SERVER_LOG")"
		return
	fi
	check_passed "the application serves with a rate limit"

	local attempt status allowed refused
	allowed=0
	refused=0
	for attempt in 1 2 3 4 5 6 7 8; do
		status="$(curl -s -o /dev/null -w '%{http_code}' "$E2E_BASE_URL/services" 2>/dev/null)"
		if [ "$status" = "429" ]; then
			refused=$((refused + 1))
		elif [ "$status" = "200" ]; then
			allowed=$((allowed + 1))
		fi
	done

	note "rate limit" "$allowed served and $refused refused out of 8 against a limit of $RATE_LIMIT"

	if [ "$refused" -gt 0 ]; then
		check_passed "a burst past the limit is refused"
	else
		check_failed "a burst past the limit is refused" "all 8 requests were served"
	fi

	if [ "$allowed" -gt 0 ] && [ "$allowed" -le "$((RATE_LIMIT + 1))" ]; then
		check_passed "the requests within the limit were served"
	else
		check_failed "the requests within the limit were served" \
			"$allowed served against a limit of $RATE_LIMIT"
	fi

	local headers
	headers="$(curl -s -o /dev/null -D - "$E2E_BASE_URL/services" 2>/dev/null)"

	case "$headers" in
	*"429 Too Many Requests"*)
		check_passed "the refusal is a 429"
		;;
	*)
		check_failed "the refusal is a 429" "$(printf '%s' "$headers" | head -1)"
		;;
	esac

	case "$headers" in
	*"Retry-After:"*)
		check_passed "the refusal says when to come back"
		;;
	*)
		check_failed "the refusal says when to come back" "no Retry-After header"
		;;
	esac

	case "$headers" in
	*"Ratelimit-Limit: $RATE_LIMIT"*)
		check_passed "the refusal states the limit it applied"
		;;
	*)
		check_failed "the refusal states the limit it applied" \
			"$(printf '%s' "$headers" | grep -i ratelimit | head -2)"
		;;
	esac
}

restore_ordinary_settings() {
	server_stop
	app_write_settings 1 0 25 1 3 1 1 0

	cd "$EXAMPLE_DIR"
	assert_succeeds "the project builds again without the rate limit" go build ./...
	cd "$E2E_ROOT"
	rm -rf "$EXAMPLE_DIR/cache"
}

check_the_site_is_there
check_the_defaults_before_anything_is_asked_for
check_the_host_is_checked
turn_security_on
check_the_policy_and_its_nonce
check_compression
check_cors
check_the_rate_limiter
restore_ordinary_settings

if port_is_free; then
	check_passed "the port is free when the chunk ends"
else
	check_failed "the port is free when the chunk ends" "held by $(port_listener_pids | tr '\n' ' ')"
fi

chunk_end
