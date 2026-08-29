ADMIN_USERNAME="e2eadmin"
ADMIN_EMAIL="e2e@example.test"
ADMIN_PASSWORD="Harness-7712-Secret"
ADMIN_PREFIX="/admin"

admin_superadmin_exists() {
	[ -n "$(app_sqlite_query "SELECT id FROM users WHERE username='$ADMIN_USERNAME';")" ]
}

admin_create_superadmin() {
	COYOTE_SUPERADMIN_USERNAME="$ADMIN_USERNAME" \
		COYOTE_SUPERADMIN_EMAIL="$ADMIN_EMAIL" \
		COYOTE_SUPERADMIN_PASSWORD="$ADMIN_PASSWORD" \
		app_run_command createsuperadmin
}

admin_login() {
	http_reset_session
	http_get "$ADMIN_PREFIX/login"

	local token
	token="$(http_csrf_token)"
	if [ -z "$token" ]; then
		return 1
	fi

	http_post "$ADMIN_PREFIX/login" \
		"csrf_token=$token" \
		"username=$ADMIN_USERNAME" \
		"password=$ADMIN_PASSWORD" \
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
	token="$(http_csrf_token)"
	http_post "$path" "csrf_token=$token" "$@"
}
