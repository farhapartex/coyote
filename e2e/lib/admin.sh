ADMIN_PREFIX="/admin"

admin_login() {
	local username="$1" password="$2"
	http_reset_session
	http_get "$ADMIN_PREFIX/login"

	local token
	token="$(http_csrf_token)"
	if [ -z "$token" ]; then
		return 1
	fi

	http_post "$ADMIN_PREFIX/login" \
		"csrf_token=$token" \
		"username=$username" \
		"password=$password" \
		"next=$ADMIN_PREFIX/"

	[ "$HTTP_STATUS" = "303" ] || [ "$HTTP_STATUS" = "302" ]
}

admin_token_for() {
	http_get "$1"
	http_csrf_token
}

admin_submit() {
	local path="$1"
	shift
	local token
	token="$(admin_token_for "$path")"
	if [ -z "$token" ]; then
		return 1
	fi
	http_form "$path" "csrf_token=$token" "$@"
	[ "$HTTP_STATUS" = "303" ] || [ "$HTTP_STATUS" = "302" ]
}
