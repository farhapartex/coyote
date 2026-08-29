#!/usr/bin/env bash

. "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib/bootstrap.sh"
. "$E2E_DIR/lib/appsource.sh"
. "$E2E_DIR/lib/server.sh"
. "$E2E_DIR/lib/http.sh"
. "$E2E_DIR/lib/admin.sh"

chunk_begin "17" "Pagination"

trap server_cleanup EXIT

PER_PAGE=25
SEEDED=250
TOTAL=0
LAST_PAGE=0

check_the_shipments_exist() {
	if app_table_exists shipments; then
		return 0
	fi
	check_failed "the shipments table exists" "run chunks 05 to 16 first"
	chunk_end
}

seed_a_few_hundred_shipments() {
	app_sqlite_query "DELETE FROM shipments WHERE id LIKE 'bulk-%';"
	app_sqlite_query "WITH RECURSIVE n(i) AS (SELECT 1 UNION ALL SELECT i+1 FROM n WHERE i<$SEEDED)
		INSERT INTO shipments (id, reference, customer_id, origin_id, destination_id, status, created_at, updated_at)
		SELECT 'bulk-' || i, 'MRF-' || substr('000000' || (1000 + i), -6, 6),
		       'cus-nordwind', 'prt-rtm', 'prt-sin',
		       CASE i % 3 WHEN 0 THEN 'booked' WHEN 1 THEN 'in_transit' ELSE 'draft' END,
		       datetime('now'), datetime('now')
		FROM n;"

	TOTAL="$(app_row_count shipments)"
	LAST_PAGE=$(((TOTAL + PER_PAGE - 1) / PER_PAGE))

	if [ "$TOTAL" -ge "$SEEDED" ]; then
		check_passed "the table holds a few hundred shipments"
	else
		check_failed "the table holds a few hundred shipments" "only $TOTAL rows"
		chunk_end
	fi
	note "list size" "$TOTAL shipments, $PER_PAGE per page, $LAST_PAGE pages"
}

turn_pagination_on() {
	app_write_shipment_list_handler
	app_write_settings 1 0 "$PER_PAGE"
	app_write_public_templates 15
	app_write_admin_resources 12
	app_write_main 17

	assert_output_contains "the per page setting is applied" \
		"s.Pagination.PerPage = $PER_PAGE" cat "$EXAMPLE_DIR/settings.go"

	cd "$EXAMPLE_DIR"
	assert_succeeds "the project compiles with a paged list" go build ./...
	assert_succeeds "the paged list passes go vet" go vet ./...
	cd "$E2E_ROOT"
}

boot_the_server() {
	if ! port_is_free; then
		force_free_the_port
	fi
	server_start
	if server_wait_for_http; then
		check_passed "the application serves the paged list"
		return 0
	fi
	check_failed "the application serves the paged list" "$(tail -20 "$SERVER_LOG")"
	chunk_end
}

rows_on_page() {
	printf '%s' "$HTTP_BODY" | grep -c 'data-reference=' || true
}

first_reference() {
	printf '%s' "$HTTP_BODY" | grep -oE 'data-reference="[^"]*"' | head -1 | sed 's/data-reference="//;s/"//'
}

page_summary() {
	printf '%s' "$HTTP_BODY" | grep -oE 'Page [0-9]+ of [0-9]+ · [0-9]+ total' | head -1
}

check_the_first_page() {
	http_get "/shipments"

	assert_equal "the list answers 200" "200" "$HTTP_STATUS"
	assert_equal "the first page holds exactly the page size" "$PER_PAGE" "$(rows_on_page)"
	assert_equal "the summary reports the true total" "Page 1 of $LAST_PAGE · $TOTAL total" "$(page_summary)"

	case "$HTTP_BODY" in
	*'rel="next"'*)
		check_passed "the pager offers a next link"
		;;
	*)
		check_failed "the pager offers a next link" "no next link on page one"
		;;
	esac

	case "$HTTP_BODY" in
	*'rel="prev"'*)
		check_failed "page one offers no previous link" "a previous link was rendered on page one"
		;;
	*)
		check_passed "page one offers no previous link"
		;;
	esac
}

check_a_later_page_shows_other_rows() {
	http_get "/shipments?page=1"
	local first_of_one
	first_of_one="$(first_reference)"

	http_get "/shipments?page=2"
	local first_of_two
	first_of_two="$(first_reference)"

	assert_equal "the second page is also full" "$PER_PAGE" "$(rows_on_page)"
	assert_equal "the second page reports itself" "Page 2 of $LAST_PAGE · $TOTAL total" "$(page_summary)"

	if [ -n "$first_of_one" ] && [ "$first_of_one" != "$first_of_two" ]; then
		check_passed "the second page shows different rows from the first"
	else
		check_failed "the second page shows different rows from the first" \
			"both pages start at ${first_of_one:-nothing}"
	fi

	case "$HTTP_BODY" in
	*'rel="prev"'*)
		check_passed "a later page offers a previous link"
		;;
	*)
		check_failed "a later page offers a previous link" "no previous link on page two"
		;;
	esac
}

check_the_last_page() {
	http_get "/shipments?page=$LAST_PAGE"

	local expected
	expected=$((TOTAL - (LAST_PAGE - 1) * PER_PAGE))
	assert_equal "the last page holds the remainder" "$expected" "$(rows_on_page)"

	case "$HTTP_BODY" in
	*'rel="next"'*)
		check_failed "the last page offers no next link" "a next link was rendered on the last page"
		;;
	*)
		check_passed "the last page offers no next link"
		;;
	esac
}

check_an_out_of_range_page_is_clamped() {
	http_get "/shipments?page=99999"
	assert_equal "a page past the end answers 200" "200" "$HTTP_STATUS"
	assert_equal "a page past the end is clamped to the last one" \
		"Page $LAST_PAGE of $LAST_PAGE · $TOTAL total" "$(page_summary)"

	if [ "$(rows_on_page)" -gt 0 ]; then
		check_passed "a hand edited page number never shows an empty screen"
	else
		check_failed "a hand edited page number never shows an empty screen" "no rows rendered"
	fi

	local probe
	for probe in 0 -5 abc "1'" "%20"; do
		http_get "/shipments?page=$probe"
		assert_equal "page=$probe falls back to the first page" \
			"Page 1 of $LAST_PAGE · $TOTAL total" "$(page_summary)"
	done
}

check_a_sort_is_honoured_when_it_names_a_real_column() {
	http_get "/shipments?sort=reference"
	local ascending
	ascending="$(first_reference)"
	assert_equal "sorting by a real column answers 200" "200" "$HTTP_STATUS"

	http_get "/shipments?sort=-reference"
	local descending
	descending="$(first_reference)"
	assert_equal "sorting the other way answers 200" "200" "$HTTP_STATUS"

	if [ -n "$ascending" ] && [ "$ascending" != "$descending" ]; then
		check_passed "a leading minus reverses the order"
	else
		check_failed "a leading minus reverses the order" \
			"both directions start at ${ascending:-nothing}"
	fi
}

check_an_injected_sort_reaches_no_sql() {
	local before
	before="$(app_row_count shipments)"

	local payload
	for payload in \
		"nonexistent_column" \
		"reference%3B%20DROP%20TABLE%20shipments" \
		"reference%20--" \
		"%28SELECT%201%29" \
		"reference%3Bdelete%20from%20shipments" \
		"reference%20UNION%20SELECT%20password%20FROM%20users"; do
		http_get "/shipments?sort=$payload"

		if [ "$HTTP_STATUS" = "200" ]; then
			check_passed "sort=$payload is ignored rather than executed"
		else
			check_failed "sort=$payload is ignored rather than executed" "status $HTTP_STATUS"
		fi
	done

	assert_equal "the shipments table is intact after every payload" "$before" "$(app_row_count shipments)"
	assert_equal "the users table is intact too" "1" \
		"$(app_sqlite_query "SELECT count(*) FROM users WHERE username='$ADMIN_USERNAME';")"

	case "$HTTP_BODY" in
	*'$2a$'* | *pbkdf2* | *"password"*)
		check_failed "no password material leaked into the page" "the page mentions password material"
		;;
	*)
		check_passed "no password material leaked into the page"
		;;
	esac

	note "sort safety" \
		"schema.SortColumn accepts only a column the model declares, so anything else falls back to the default order"
}

check_a_page_costs_a_constant_number_of_queries() {
	local before after full remainder
	before="$(grep -c 'gorm query' "$SERVER_LOG" || true)"
	http_get "/shipments?page=3"
	after="$(grep -c 'gorm query' "$SERVER_LOG" || true)"
	full=$((after - before))

	before="$after"
	http_get "/shipments?page=$LAST_PAGE"
	after="$(grep -c 'gorm query' "$SERVER_LOG" || true)"
	remainder=$((after - before))

	note "queries per page" "$full for a page of $PER_PAGE, $remainder for the last page"

	if [ "$full" -lt 10 ]; then
		check_passed "a page of $PER_PAGE rows costs a handful of queries, not one per row"
	else
		check_failed "a page of $PER_PAGE rows costs a handful of queries, not one per row" \
			"$full queries for $PER_PAGE rows suggests a query per row"
	fi

	if [ "$full" = "$remainder" ]; then
		check_passed "the query count does not grow with the number of rows"
	else
		check_failed "the query count does not grow with the number of rows" \
			"a full page cost $full and a short page cost $remainder"
	fi

	note "paging by hand" \
		"the handler counts and then lists, and List counts again, so a hand rolled pager pays for two counts"
}

check_the_admin_pages_the_same_data() {
	if ! admin_login; then
		check_failed "the superadmin can sign in" "status $HTTP_STATUS"
		return
	fi

	http_get "/admin/shipments"
	assert_equal "the admin list answers 200" "200" "$HTTP_STATUS"

	local admin_rows
	admin_rows="$(printf '%s' "$HTTP_BODY" | grep -c "MRF-" || true)"
	if [ "$admin_rows" -gt 0 ] && [ "$admin_rows" -le $((PER_PAGE * 2)) ]; then
		check_passed "the admin list is paged rather than rendering every row"
	else
		check_failed "the admin list is paged rather than rendering every row" \
			"$admin_rows references on one screen out of $TOTAL rows"
	fi

	http_get "/admin/shipments?page=2"
	assert_equal "the admin honours a page number" "200" "$HTTP_STATUS"

	http_get "/admin/shipments?sort=reference%3B%20DROP%20TABLE%20shipments"
	assert_equal "the admin ignores an injected sort" "200" "$HTTP_STATUS"
	assert_equal "the admin left the table intact" "$TOTAL" "$(app_row_count shipments)"
}

check_the_shipments_exist
seed_a_few_hundred_shipments
turn_pagination_on
boot_the_server
check_the_first_page
check_a_later_page_shows_other_rows
check_the_last_page
check_an_out_of_range_page_is_clamped
check_a_sort_is_honoured_when_it_names_a_real_column
check_an_injected_sort_reaches_no_sql
check_a_page_costs_a_constant_number_of_queries
check_the_admin_pages_the_same_data

server_stop
if port_is_free; then
	check_passed "the port is free when the chunk ends"
else
	check_failed "the port is free when the chunk ends" "held by $(port_listener_pids | tr '\n' ' ')"
fi

chunk_end
