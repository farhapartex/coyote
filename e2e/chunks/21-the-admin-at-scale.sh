#!/usr/bin/env bash

. "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib/bootstrap.sh"
. "$E2E_DIR/lib/appsource.sh"
. "$E2E_DIR/lib/server.sh"
. "$E2E_DIR/lib/http.sh"
. "$E2E_DIR/lib/admin.sh"

chunk_begin "21" "The admin at scale"

trap server_cleanup EXIT

MANAGED="customers ports shipments containers tracking_events invoices invoice_lines quote_requests"

check_the_schema_is_complete() {
	if app_table_exists quote_requests && app_table_exists invoices; then
		return 0
	fi
	check_failed "every model has been migrated" "run chunks 05 to 20 first"
	chunk_end
}

manage_every_model() {
	app_write_admin_action
	app_write_admin_resources 21
	app_write_main 21

	assert_file_exists "the bulk action is written" "$EXAMPLE_DIR/admin_actions.go"
	assert_output_contains "every model is handed to the portal" "quoteRequestResource{}" \
		cat "$EXAMPLE_DIR/main.go"
	assert_output_contains "containers declare a filter column" \
		'FilterColumns() []string { return []string{"sealed"} }' cat "$EXAMPLE_DIR/admin_resources.go"

	cd "$EXAMPLE_DIR"
	assert_succeeds "the project compiles with eight managed models" go build ./...
	assert_succeeds "the admin resources pass go vet" go vet ./...
	cd "$E2E_ROOT"
}

seed_containers() {
	app_sqlite_query "DELETE FROM containers;"
	app_sqlite_query "INSERT INTO containers (id, number, shipment_id, size_feet, sealed, created_at, updated_at)
		SELECT 'adm-1', 'MSCU1000001', id, 40, 0, datetime('now'), datetime('now') FROM shipments LIMIT 1;"
	app_sqlite_query "INSERT INTO containers (id, number, shipment_id, size_feet, sealed, created_at, updated_at)
		SELECT 'adm-2', 'MSCU1000002', id, 40, 0, datetime('now'), datetime('now') FROM shipments LIMIT 1;"
	app_sqlite_query "INSERT INTO containers (id, number, shipment_id, size_feet, sealed, created_at, updated_at)
		SELECT 'adm-3', 'TGHU2000003', id, 20, 1, datetime('now'), datetime('now') FROM shipments LIMIT 1;"

	assert_equal "three containers are ready" "3" "$(app_row_count containers)"
}

boot_the_server() {
	if ! port_is_free; then
		force_free_the_port
	fi
	server_start
	if server_wait_for_http; then
		check_passed "the application serves the whole portal"
		return 0
	fi
	check_failed "the application serves the whole portal" "$(tail -20 "$SERVER_LOG")"
	chunk_end
}

check_every_model_is_reachable() {
	if ! admin_login; then
		check_failed "the superadmin can sign in" "status $HTTP_STATUS"
		chunk_end
	fi

	http_get "/admin/"
	local offered slug
	offered="$(printf '%s' "$HTTP_BODY" | grep -oE 'href="/admin/[a-z_]+"' | sort -u | tr '\n' ' ')"

	for slug in $MANAGED; do
		case "$offered" in
		*"/admin/$slug\""*)
			check_passed "the dashboard offers $slug"
			;;
		*)
			check_failed "the dashboard offers $slug" "offered: $offered"
			;;
		esac
	done

	for slug in $MANAGED; do
		assert_http_status "the $slug list answers 200" "200" "/admin/$slug"
	done

	local builtin
	for builtin in users roles sessions; do
		assert_http_status "the framework's own $slug screen still answers 200" "200" "/admin/$builtin"
	done
}

check_search_narrows_a_list() {
	http_get "/admin/containers?q=TGHU"

	case "$HTTP_BODY" in
	*TGHU2000003*)
		check_passed "searching finds the matching container"
		;;
	*)
		check_failed "searching finds the matching container" "TGHU2000003 was not listed"
		;;
	esac

	case "$HTTP_BODY" in
	*MSCU1000001*)
		check_failed "searching excludes the containers that do not match" "MSCU1000001 was still listed"
		;;
	*)
		check_passed "searching excludes the containers that do not match"
		;;
	esac

	http_get "/admin/containers?q=NOTHINGMATCHESTHIS"
	assert_equal "a search with no matches still answers 200" "200" "$HTTP_STATUS"

	http_get "/admin/containers?q=%25"
	assert_equal "a bare wildcard is escaped rather than matching everything" "200" "$HTTP_STATUS"
	case "$HTTP_BODY" in
	*MSCU1000001*)
		check_failed "a bare wildcard is treated as text" "the percent sign matched every row"
		;;
	*)
		check_passed "a bare wildcard is treated as text"
		;;
	esac
}

check_a_filter_narrows_a_list() {
	http_get "/admin/containers"
	case "$HTTP_BODY" in
	*'name="filter.sealed"'*)
		check_passed "a declared filter column renders a control"
		;;
	*)
		check_failed "a declared filter column renders a control" "no filter widget on the list"
		;;
	esac

	http_get "/admin/containers?filter.sealed=no"
	local unsealed
	unsealed="$(printf '%s' "$HTTP_BODY" | grep -c 'MSCU100000' || true)"
	assert_equal "filtering to unsealed shows both unsealed containers" "2" "$unsealed"
	case "$HTTP_BODY" in
	*TGHU2000003*)
		check_failed "filtering to unsealed hides the sealed one" "TGHU2000003 was listed"
		;;
	*)
		check_passed "filtering to unsealed hides the sealed one"
		;;
	esac

	http_get "/admin/containers?filter.sealed=yes"
	case "$HTTP_BODY" in
	*TGHU2000003*)
		check_passed "filtering to sealed shows the sealed one"
		;;
	*)
		check_failed "filtering to sealed shows the sealed one" "TGHU2000003 was not listed"
		;;
	esac

	http_get "/admin/containers?filter.sealed=maybe"
	assert_equal "an unknown filter value is ignored rather than failing" "200" "$HTTP_STATUS"
}

check_sorting_a_list() {
	http_get "/admin/shipments?sort=reference"
	assert_equal "sorting by a real column answers 200" "200" "$HTTP_STATUS"
	local ascending
	ascending="$(printf '%s' "$HTTP_BODY" | grep -oE 'MRF-[0-9]+' | head -1)"

	http_get "/admin/shipments?sort=-reference"
	local descending
	descending="$(printf '%s' "$HTTP_BODY" | grep -oE 'MRF-[0-9]+' | head -1)"

	if [ -n "$ascending" ] && [ "$ascending" != "$descending" ]; then
		check_passed "the admin reverses the order on a leading minus"
	else
		check_failed "the admin reverses the order on a leading minus" \
			"both directions began with ${ascending:-nothing}"
	fi

	local before
	before="$(app_row_count shipments)"
	http_get "/admin/shipments?sort=reference%3B%20DROP%20TABLE%20shipments"
	assert_equal "an injected sort is ignored" "200" "$HTTP_STATUS"
	assert_equal "the table survives the injected sort" "$before" "$(app_row_count shipments)"
}

check_a_large_list_is_paged() {
	local total
	total="$(app_row_count shipments)"

	http_get "/admin/shipments"
	local shown
	shown="$(printf '%s' "$HTTP_BODY" | grep -oE 'MRF-[0-9]+' | sort -u | wc -l | tr -d ' ')"

	if [ "$shown" -gt 0 ] && [ "$shown" -lt "$total" ]; then
		check_passed "a list of $total rows renders a page, not everything"
	else
		check_failed "a list of $total rows renders a page, not everything" \
			"$shown references on one screen"
	fi
	note "admin page size" "$shown of $total shipments on the first screen"

	http_get "/admin/shipments?page=2"
	assert_equal "the admin honours a page number" "200" "$HTTP_STATUS"
	local second
	second="$(printf '%s' "$HTTP_BODY" | grep -oE 'MRF-[0-9]+' | head -1)"

	http_get "/admin/shipments?page=1"
	local first
	first="$(printf '%s' "$HTTP_BODY" | grep -oE 'MRF-[0-9]+' | head -1)"

	if [ -n "$first" ] && [ "$first" != "$second" ]; then
		check_passed "the second page shows different rows"
	else
		check_failed "the second page shows different rows" "both pages began with ${first:-nothing}"
	fi

	http_get "/admin/shipments?page=99999"
	assert_equal "a page past the end is still a page" "200" "$HTTP_STATUS"
}

check_a_custom_bulk_action() {
	http_get "/admin/containers"
	case "$HTTP_BODY" in
	*'value="seal"'*)
		check_passed "the custom action is offered on the list"
		;;
	*)
		check_failed "the custom action is offered on the list" "no seal action in the markup"
		;;
	esac

	app_sqlite_query "UPDATE containers SET sealed = 0;"
	assert_equal "every container starts unsealed" "0" \
		"$(app_sqlite_query "SELECT count(*) FROM containers WHERE sealed = 1;")"

	http_get "/admin/containers"
	admin_submit "/admin/containers/bulk" "action=seal" "ids=adm-1" "ids=adm-2"

	assert_equal "running the action redirects back to the list" "303" "$HTTP_STATUS"
	assert_equal "the action ran on both selected rows" "2" \
		"$(app_sqlite_query "SELECT count(*) FROM containers WHERE sealed = 1;")"
	assert_equal "the row that was not selected is untouched" "0" \
		"$(app_sqlite_query "SELECT sealed FROM containers WHERE id = 'adm-3';")"
}

check_bulk_delete_and_its_guards() {
	local before
	before="$(app_row_count containers)"

	http_get "/admin/containers"
	admin_submit "/admin/containers/bulk" "action=delete" "ids=adm-3"

	assert_equal "bulk delete redirects" "303" "$HTTP_STATUS"
	assert_equal "the selected row is gone" "$((before - 1))" "$(app_row_count containers)"

	local remaining
	remaining="$(app_row_count containers)"

	http_get "/admin/containers"
	admin_submit "/admin/containers/bulk" "action=delete"
	assert_equal "a bulk action with nothing selected is a no-op" "$remaining" "$(app_row_count containers)"

	http_post "/admin/containers/bulk" "action=delete" "ids=adm-1"
	assert_equal "a bulk action without a CSRF token is refused" "403" "$HTTP_STATUS"
	assert_equal "the refused bulk action deleted nothing" "$remaining" "$(app_row_count containers)"

	http_get "/admin/containers"
	admin_submit "/admin/containers/bulk" "action=no-such-action" "ids=adm-1"
	assert_equal "an unknown action deletes nothing" "$remaining" "$(app_row_count containers)"
}

check_relations_resolve_across_the_portal() {
	http_get "/admin/invoice_lines"
	local label
	for label in "Ocean freight" "INV-000042"; do
		case "$HTTP_BODY" in
		*"$label"*)
			check_passed "the invoice lines list still resolves $label"
			;;
		*)
			check_failed "the invoice lines list still resolves $label" "not shown"
			;;
		esac
	done

	http_get "/admin/quote_requests"
	assert_equal "the quote requests list answers 200" "200" "$HTTP_STATUS"
	case "$HTTP_BODY" in
	*"Acme Trading BV"* | *"Kestrel Trading"*)
		check_passed "the quote requests list shows a submitted request"
		;;
	*)
		check_failed "the quote requests list shows a submitted request" "no quote rows listed"
		;;
	esac
}

check_the_schema_is_complete
manage_every_model
seed_containers
boot_the_server
check_every_model_is_reachable
check_search_narrows_a_list
check_a_filter_narrows_a_list
check_sorting_a_list
check_a_large_list_is_paged
check_a_custom_bulk_action
check_bulk_delete_and_its_guards
check_relations_resolve_across_the_portal

server_stop
if port_is_free; then
	check_passed "the port is free when the chunk ends"
else
	check_failed "the port is free when the chunk ends" "held by $(port_listener_pids | tr '\n' ' ')"
fi

chunk_end
