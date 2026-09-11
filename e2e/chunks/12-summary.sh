#!/usr/bin/env bash

. "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib/bootstrap.sh"
. "$E2E_DIR/lib/services.sh"
. "$E2E_DIR/lib/shop.sh"
. "$E2E_DIR/lib/server.sh"
. "$E2E_DIR/lib/http.sh"

chunk_begin "12" "What the framework carried"

require_previous_chunk "the shop exists" "$SHOP_DIR/settings.go"

trap server_cleanup EXIT

count_lines() {
	find "$SHOP_DIR" -maxdepth 1 -name '*.go' -exec cat {} + 2>/dev/null | grep -c '' || true
}

count_templates() {
	find "$SHOP_DIR/templates" -name '*.html' 2>/dev/null | wc -l | tr -d ' '
}

start_the_shop() {
	shop_start
	if server_wait_for_http; then
		check_passed "the finished shop is serving"
		return 0
	fi
	check_failed "the finished shop is serving" "$(tail -20 "$SERVER_LOG")"
	chunk_end
}

check_the_whole_journey_still_works() {
	local path
	for path in "/" "/products" "/products/low-angle-block-plane" "/categories/hand-tools" "/cart"; do
		http_get "$path"
		if [ "$HTTP_STATUS" = "200" ]; then
			check_passed "GET $path answers 200"
		else
			check_failed "GET $path answers 200" "got $HTTP_STATUS"
		fi
	done
}

record_the_shape_of_it() {
	note "application Go" "$(count_lines) lines across $(find "$SHOP_DIR" -maxdepth 1 -name '*.go' | wc -l | tr -d ' ') files"
	note "templates" "$(count_templates) files, one hand-written stylesheet of $(grep -c '' "$SHOP_DIR/static/shop.css") lines"
	note "models" "$(grep -ho '^type [A-Z][A-Za-z]* struct' "$SHOP_DIR"/*.go | wc -l | tr -d ' ') entities"
	note "migrations" "$(shop_migration_files | wc -l | tr -d ' ') files"
	note "postgres tables" "$(postgres_query "SELECT count(*) FROM information_schema.tables WHERE table_schema='public';")"
	note "products" "$(postgres_row_count products)"
	note "orders" "$(postgres_row_count orders)"
	note "order lines" "$(postgres_row_count order_lines)"
	note "shipment events" "$(postgres_row_count shipment_events)"
	note "stock movements" "$(postgres_row_count stock_movements)"
	note "jobs run" "$(postgres_query "SELECT count(*) FROM jobs WHERE state='done';")"
	note "permissions" "$(postgres_row_count permissions)"
	note "users" "$(postgres_row_count users)"
	note "live sessions" "$(postgres_row_count sessions)"
	note "redis keys" "$(redis_key_count)"
}

check_what_the_framework_supplied() {
	local supplied
	supplied="admin portal, sessions in postgres, authentication, permissions, self-service accounts, \
migrations, forms, uploads, pagination, redis caching, background jobs, CSRF, secure headers, host \
validation, request ids, graceful shutdown"
	note "came from the framework" "$supplied"

	local written
	written="cart, coupon arithmetic, money formatting and parsing, order placement, stock movement, \
delivery state machine, storefront templates, custom CSS, category and price filtering, a numeric \
accessor for model.Record"
	note "had to be written" "$written"
}

record_the_verdict() {
	note "verdict" "A complete shop is buildable on this framework. Nothing in the domain was impossible \
and no part of the framework had to be worked around structurally; the two real defects found are \
both small and local. What costs the most is the missing has-many relation, which pushes every \
parent-and-children screen out of the admin and into hand-written pages."

	finding "the shape of the gap, in one place" \
		"Ten findings are recorded across this run. Grouped by what they cost:
  Defects, small and local:
    - a string column with a default: tag emits invalid DDL and migrate fails on postgres (chunk 07)
    - a boolean on an admin-managed model can be switched on but never off (chunk 10)
    - a read-only resource still serves its create and edit forms (chunk 10)
    - a file field refuses a urlencoded save with an unhelpful message (chunk 04)
    - migrate exits zero when the migrations package is not imported (chunk 07)
  Missing capability, felt on every screen:
    - no has-many, so an order cannot show its lines (chunk 10)
    - no numeric accessor on model.Record (chunk 02)
    - no decimal or money kind, so prices are entered in pence (chunk 04)
    - no aggregate beyond Count, so filter facets cost a query each (chunk 06)
    - no cache invalidation hook, and redis cannot back sessions (chunk 11)
The first group is an afternoon. The second is the roadmap."
}

start_the_shop
check_the_whole_journey_still_works
record_the_shape_of_it
check_what_the_framework_supplied
record_the_verdict

chunk_end
