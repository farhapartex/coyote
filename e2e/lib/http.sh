HTTP_STATUS=""
HTTP_BODY=""
HTTP_HEADERS=""
COOKIE_JAR="$E2E_WORK_DIR/cookies.txt"

http_reset_session() {
	rm -f "$COOKIE_JAR"
	: >"$COOKIE_JAR"
}

http_get() {
	local path="$1"
	local header_file
	header_file="$(mktemp "${TMPDIR:-/tmp}/coyote-e2e-head-XXXXXX")"

	set +e
	HTTP_BODY="$(curl -sS --max-time 15 -b "$COOKIE_JAR" -c "$COOKIE_JAR" \
		-D "$header_file" "$E2E_BASE_URL$path" 2>/dev/null)"
	set -e

	HTTP_HEADERS="$(cat "$header_file")"
	HTTP_STATUS="$(printf '%s' "$HTTP_HEADERS" | awk 'NR==1{print $2}')"
	rm -f "$header_file"
}

http_post() {
	local path="$1"
	shift
	local header_file curl_arguments=()
	header_file="$(mktemp "${TMPDIR:-/tmp}/coyote-e2e-head-XXXXXX")"

	local field
	for field in "$@"; do
		curl_arguments+=(--data-urlencode "$field")
	done

	set +e
	HTTP_BODY="$(curl -sS --max-time 15 -b "$COOKIE_JAR" -c "$COOKIE_JAR" \
		-D "$header_file" "${curl_arguments[@]}" "$E2E_BASE_URL$path" 2>/dev/null)"
	set -e

	HTTP_HEADERS="$(cat "$header_file")"
	HTTP_STATUS="$(printf '%s' "$HTTP_HEADERS" | awk 'NR==1{print $2}')"
	rm -f "$header_file"
}

http_header_value() {
	printf '%s' "$HTTP_HEADERS" | awk -v name="$1" 'BEGIN{IGNORECASE=1} $1 == name":" {sub($1" ","");print;exit}' |
		tr -d '\r'
}

http_csrf_token() {
	printf '%s' "$HTTP_BODY" |
		sed -n 's/.*name="csrf_token"[^>]*value="\([^"]*\)".*/\1/p' | head -1
}

assert_http_status() {
	local description="$1" expected="$2" path="$3"
	http_get "$path"
	if [ "$HTTP_STATUS" = "$expected" ]; then
		check_passed "$description"
		return 0
	fi
	check_failed "$description" "GET $path
expected status $expected, got ${HTTP_STATUS:-no response}"
	return 0
}

assert_body_contains() {
	local description="$1" needle="$2" path="$3"
	http_get "$path"
	case "$HTTP_BODY" in
	*"$needle"*)
		check_passed "$description"
		return 0
		;;
	esac
	check_failed "$description" "GET $path (status ${HTTP_STATUS:-none})
expected the body to contain: $needle"
	return 0
}
