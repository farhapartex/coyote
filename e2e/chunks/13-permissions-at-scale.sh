#!/usr/bin/env bash

. "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib/bootstrap.sh"
. "$E2E_DIR/lib/appsource.sh"
. "$E2E_DIR/lib/server.sh"
. "$E2E_DIR/lib/http.sh"
. "$E2E_DIR/lib/admin.sh"

chunk_begin "13" "Permissions at scale"

trap server_cleanup EXIT

STAFF_USERNAME="deskclerk"
STAFF_PASSWORD="Brackish-Fjord-8891"
PLAIN_USERNAME="warehouse"
PLAIN_PASSWORD="Quartz-Lantern-5567"
ROLE_NAME="Customer Desk"

MANAGED_RESOURCES="customers ports shipments containers tracking_events invoices invoice_lines"
FRAMEWORK_RESOURCES="users roles permissions"

check_the_ladder_is_ready() {
	if app_table_exists invoices && app_table_exists permissions; then
		return 0
	fi
	check_failed "the schema includes invoicing and permissions" "run chunks 05 to 12 first"
	chunk_end
}

check_syncpermissions_is_complete() {
	app_run_command syncpermissions
	assert_equal "syncpermissions exits cleanly" "0" "$CAPTURED_STATUS"

	local resource
	for resource in $MANAGED_RESOURCES; do
		assert_equal "$resource has all four permissions" "create,delete,read,update" \
			"$(app_sqlite_query "SELECT action FROM permissions WHERE resource='$resource' ORDER BY action;" | tr '\n' ',' | sed 's/,$//')"
	done

	for resource in $FRAMEWORK_RESOURCES; do
		assert_equal "the framework's own $resource has four permissions" "4" \
			"$(app_sqlite_query "SELECT count(*) FROM permissions WHERE resource='$resource';")"
	done

	local total resources
	total="$(app_row_count permissions)"
	resources="$(app_sqlite_query "SELECT count(DISTINCT resource) FROM permissions;")"
	assert_equal "there are exactly four permissions per resource" "$((resources * 4))" "$total"
	note "permission scale" "$total permissions across $resources resources"
}

check_syncpermissions_is_idempotent() {
	local before
	before="$(app_row_count permissions)"

	app_run_command syncpermissions
	assert_equal "a second syncpermissions exits cleanly" "0" "$CAPTURED_STATUS"
	assert_equal "a second syncpermissions creates no duplicates" "$before" "$(app_row_count permissions)"

	assert_equal "no resource and action pair is duplicated" "0" \
		"$(app_sqlite_query "SELECT count(*) FROM (SELECT resource, action FROM permissions GROUP BY resource, action HAVING count(*) > 1);")"
}

boot_the_server() {
	if ! port_is_free; then
		force_free_the_port
	fi
	server_start
	if server_wait_for_http; then
		check_passed "the application serves with the permission schema in place"
		return 0
	fi
	check_failed "the application serves with the permission schema in place" "$(tail -20 "$SERVER_LOG")"
	chunk_end
}

create_a_read_only_role() {
	if ! admin_login; then
		check_failed "the superadmin can sign in" "status $HTTP_STATUS"
		chunk_end
	fi

	app_sqlite_query "DELETE FROM role_permissions WHERE role_id IN (SELECT id FROM roles WHERE name='$ROLE_NAME');"
	app_sqlite_query "DELETE FROM roles WHERE name='$ROLE_NAME';"

	local read_permission
	read_permission="$(app_sqlite_query "SELECT id FROM permissions WHERE resource='customers' AND action='read';")"

	http_get "/admin/roles/new"
	assert_equal "the role form answers 200" "200" "$HTTP_STATUS"

	admin_submit "/admin/roles/new" \
		"name=$ROLE_NAME" "description=may read customers and nothing else" \
		"permissions=$read_permission"

	assert_equal "creating a role redirects back to the list" "303" "$HTTP_STATUS"
	assert_equal "the role holds exactly one permission" "1" \
		"$(app_sqlite_query "SELECT count(*) FROM role_permissions rp JOIN roles r ON r.id = rp.role_id WHERE r.name='$ROLE_NAME';")"
}

create_a_staff_user() {
	app_sqlite_query "DELETE FROM user_roles WHERE user_id IN (SELECT id FROM users WHERE username='$STAFF_USERNAME');"
	app_sqlite_query "DELETE FROM users WHERE username='$STAFF_USERNAME';"

	http_get "/admin/users/new"
	admin_submit "/admin/users/new" \
		"username=$STAFF_USERNAME" "email=desk@example.test" \
		"password=$STAFF_PASSWORD" "is_active=on" "is_staff=on"

	assert_equal "creating a staff user redirects back to the list" "303" "$HTTP_STATUS"
	assert_equal "the new user is staff but not a superadmin" "1|0" \
		"$(app_sqlite_query "SELECT is_staff || '|' || is_superadmin FROM users WHERE username='$STAFF_USERNAME';")"
}

check_a_weak_password_is_refused() {
	http_get "/admin/users/new"
	admin_submit "/admin/users/new" \
		"username=clerktwo" "email=two@example.test" \
		"password=clerktwo-secret" "is_active=on" "is_staff=on"

	if [ "$HTTP_STATUS" = "303" ]; then
		check_failed "a password resembling the username is refused" \
			"the account was created with a password derived from the username"
		app_sqlite_query "DELETE FROM users WHERE username='clerktwo';"
		return
	fi

	case "$HTTP_BODY" in
	*"too similar to the account details"*)
		check_passed "a password resembling the username is refused"
		;;
	*)
		check_failed "a password resembling the username is refused" \
			"status $HTTP_STATUS without the expected explanation"
		;;
	esac

	assert_equal "the rejected account was not created" "0" \
		"$(app_sqlite_query "SELECT count(*) FROM users WHERE username='clerktwo';")"
}

check_you_cannot_change_your_own_roles() {
	local own_id
	own_id="$(app_sqlite_query "SELECT id FROM users WHERE username='$ADMIN_USERNAME';")"

	http_get "/admin/users/$own_id"

	case "$HTTP_BODY" in
	*"cannot change your own roles"*)
		check_passed "the admin cannot change its own roles"
		;;
	*)
		check_failed "the admin cannot change its own roles" \
			"the form offered role checkboxes on the signed-in user's own record"
		;;
	esac
}

assign_the_role_to_the_staff_user() {
	local staff_id role_id
	staff_id="$(app_sqlite_query "SELECT id FROM users WHERE username='$STAFF_USERNAME';")"
	role_id="$(app_sqlite_query "SELECT id FROM roles WHERE name='$ROLE_NAME';")"

	http_get "/admin/users/$staff_id"
	case "$HTTP_BODY" in
	*'name="roles"'*)
		check_passed "another user's form offers role checkboxes"
		;;
	*)
		check_failed "another user's form offers role checkboxes" "no roles field rendered"
		;;
	esac

	admin_submit "/admin/users/$staff_id" \
		"username=$STAFF_USERNAME" "email=desk@example.test" \
		"is_active=on" "is_staff=on" "roles=$role_id"

	assert_equal "assigning a role redirects back to the list" "303" "$HTTP_STATUS"
	assert_equal "the staff user now holds the role" "1" \
		"$(app_sqlite_query "SELECT count(*) FROM user_roles WHERE user_id='$staff_id';")"
}

staff_login() {
	http_reset_session
	http_get "/admin/login"

	local token
	token="$(http_csrf_token)"
	http_post "/admin/login" "csrf_token=$token" \
		"username=$STAFF_USERNAME" "password=$STAFF_PASSWORD" "next=/admin/"

	[ "$HTTP_STATUS" = "303" ] || [ "$HTTP_STATUS" = "302" ]
}

check_the_staff_user_is_held_to_its_permissions() {
	if staff_login; then
		check_passed "the staff user can sign in"
	else
		check_failed "the staff user can sign in" "status $HTTP_STATUS"
		return
	fi

	assert_http_status "the dashboard is open to a staff user" "200" "/admin/"
	assert_http_status "the granted resource is readable" "200" "/admin/customers"
	assert_http_status "creating is refused without the create permission" "403" "/admin/customers/new"

	local resource
	for resource in shipments invoices containers tracking_events users; do
		assert_http_status "$resource is refused entirely" "403" "/admin/$resource"
	done

	http_get "/admin/"
	local offered
	offered="$(printf '%s' "$HTTP_BODY" | grep -oE 'href="/admin/[a-z_]+"' | sort -u | tr '\n' ' ')"

	case "$offered" in
	*"/admin/customers"*)
		check_passed "the navigation offers the resource the user may read"
		;;
	*)
		check_failed "the navigation offers the resource the user may read" "offered: $offered"
		;;
	esac

	case "$offered" in
	*"/admin/shipments"* | *"/admin/invoices"*)
		check_failed "the navigation hides what the user cannot reach" "offered: $offered"
		;;
	*)
		check_passed "the navigation hides what the user cannot reach"
		;;
	esac
	note "staff navigation" "offered: ${offered:-nothing}"
}

check_a_write_is_refused_not_merely_hidden() {
	local before
	before="$(app_row_count customers)"

	http_post "/admin/customers/new" \
		"name=Smuggled Customer" "code=SMUGGLED" "country=NL"

	assert_equal "a create posted without the permission is refused" "403" "$HTTP_STATUS"
	assert_equal "the refused create wrote nothing" "$before" "$(app_row_count customers)"
}

check_granting_a_permission_takes_effect() {
	local role_id create_permission
	role_id="$(app_sqlite_query "SELECT id FROM roles WHERE name='$ROLE_NAME';")"
	create_permission="$(app_sqlite_query "SELECT id FROM permissions WHERE resource='customers' AND action='create';")"

	app_sqlite_query "INSERT OR IGNORE INTO role_permissions (role_id, permission_id) VALUES ('$role_id','$create_permission');"

	assert_http_status "granting create opens the form to the same session" "200" "/admin/customers/new"
	assert_http_status "the resources still refused stay refused" "403" "/admin/shipments"

	app_sqlite_query "DELETE FROM role_permissions WHERE role_id='$role_id' AND permission_id='$create_permission';"
	assert_http_status "revoking create closes the form again" "403" "/admin/customers/new"
	note "permission changes" "take effect on the next request, without a new sign in"
}

check_a_non_staff_user_cannot_reach_the_admin() {
	if ! admin_login; then
		check_failed "the superadmin can sign in again" "status $HTTP_STATUS"
		return
	fi

	app_sqlite_query "DELETE FROM users WHERE username='$PLAIN_USERNAME';"

	http_get "/admin/users/new"
	admin_submit "/admin/users/new" \
		"username=$PLAIN_USERNAME" "email=wh@example.test" \
		"password=$PLAIN_PASSWORD" "is_active=on"

	assert_equal "creating a non-staff user redirects back to the list" "303" "$HTTP_STATUS"
	assert_equal "the new user is neither staff nor superadmin" "0|0" \
		"$(app_sqlite_query "SELECT is_staff || '|' || is_superadmin FROM users WHERE username='$PLAIN_USERNAME';")"

	http_reset_session
	http_get "/admin/login"
	local token
	token="$(http_csrf_token)"
	http_post "/admin/login" "csrf_token=$token" \
		"username=$PLAIN_USERNAME" "password=$PLAIN_PASSWORD" "next=/admin/"

	assert_equal "a non-staff account cannot sign in to the admin" "403" "$HTTP_STATUS"
	assert_http_status "and the dashboard stays closed to it" "303" "/admin/"
}

check_the_superadmin_bypasses_permission_checks() {
	if ! admin_login; then
		check_failed "the superadmin can sign in a third time" "status $HTTP_STATUS"
		return
	fi

	assert_equal "the superadmin holds no roles at all" "0" \
		"$(app_sqlite_query "SELECT count(*) FROM user_roles WHERE user_id = (SELECT id FROM users WHERE username='$ADMIN_USERNAME');")"

	local resource
	for resource in customers shipments invoices invoice_lines containers tracking_events users roles; do
		assert_http_status "the superadmin reaches $resource without a grant" "200" "/admin/$resource"
	done
}

check_the_ladder_is_ready
check_syncpermissions_is_complete
check_syncpermissions_is_idempotent
boot_the_server
create_a_read_only_role
create_a_staff_user
check_a_weak_password_is_refused
check_you_cannot_change_your_own_roles
assign_the_role_to_the_staff_user
check_the_staff_user_is_held_to_its_permissions
check_a_write_is_refused_not_merely_hidden
check_granting_a_permission_takes_effect
check_a_non_staff_user_cannot_reach_the_admin
check_the_superadmin_bypasses_permission_checks

server_stop
if port_is_free; then
	check_passed "the port is free when the chunk ends"
else
	check_failed "the port is free when the chunk ends" "held by $(port_listener_pids | tr '\n' ' ')"
fi

chunk_end
