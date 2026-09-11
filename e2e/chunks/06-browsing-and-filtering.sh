#!/usr/bin/env bash

. "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib/bootstrap.sh"
. "$E2E_DIR/lib/services.sh"
. "$E2E_DIR/lib/shop.sh"
. "$E2E_DIR/lib/server.sh"
. "$E2E_DIR/lib/http.sh"

chunk_begin "06" "Browsing and filtering"

require_previous_chunk "the shop exists" "$SHOP_DIR/settings.go"

trap server_cleanup EXIT

cards_in_body() {
	printf '%s' "$HTTP_BODY" | grep -c 'class="card"'
}

first_product_name() {
	printf '%s' "$HTTP_BODY" | sed -n 's/.*<a class="name" href="\/products\/[^"]*">\([^<]*\)<.*/\1/p' | head -1
}

start_the_shop() {
	shop_start
	if server_wait_for_http; then
		check_passed "the shop is serving"
		return 0
	fi
	check_failed "the shop is serving" "$(tail -20 "$SERVER_LOG")"
	chunk_end
}

check_filtering_by_department() {
	http_get "/products?category=power-tools"
	assert_equal "a department filter answers 200" "200" "$HTTP_STATUS"
	assert_equal "power tools has five products" "5" "$(cards_in_body)"
	case "$HTTP_BODY" in
	*"Danish oil"*) check_failed "the department filter excludes other departments" "a finishing product leaked in" ;;
	*) check_passed "the department filter excludes other departments" ;;
	esac
}

check_filtering_by_price() {
	http_get "/products?min=200"
	assert_equal "a minimum price leaves the three dearest" "3" "$(cards_in_body)"

	http_get "/products?max=20"
	assert_equal "a maximum price leaves the four cheapest" "4" "$(cards_in_body)"

	http_get "/products?min=20&max=50"
	assert_equal "a price band of £20 to £50 leaves six" "6" "$(cards_in_body)"

	http_get "/products?min=900"
	case "$HTTP_BODY" in
	*"Nothing matches those filters"*) check_passed "an empty result says so instead of showing nothing" ;;
	*) check_failed "an empty result says so" "no empty-state message" ;;
	esac
}

check_filtering_by_stock() {
	http_get "/products?stock=in"
	assert_equal "the in-stock filter drops the two sold-out products" "12" "$(cards_in_body)"

	http_get "/products?stock=in&page=2"
	assert_equal "the second page of in-stock products has six" "6" "$(cards_in_body)"
}

check_sorting() {
	http_get "/products?sort=price-asc"
	assert_equal "cheapest first starts with the beeswax polish" "Beeswax polish" "$(first_product_name)"

	http_get "/products?sort=price-desc"
	assert_equal "dearest first starts with the track saw" "Track saw, 165mm" "$(first_product_name)"

	http_get "/products?sort=name"
	assert_equal "by name starts with the abrasive pack" "Abrasive pack, mixed grit" "$(first_product_name)"
}

check_filters_survive_pagination() {
	http_get "/products?category=hand-tools&sort=price-desc"
	local first
	first="$(first_product_name)"
	assert_equal "hand tools sorted dearest first starts with the block plane" "Low-angle block plane" "$first"

	http_get "/products?stock=in&sort=name"
	case "$HTTP_BODY" in
	*'class="pager"'*) check_passed "a filter that spans two pages gets a pager" ;;
	*) check_failed "a filter that spans two pages gets a pager" "no pager for 18 in-stock products" ;;
	esac
	case "$HTTP_BODY" in
	*"stock=in&amp;sort=name&amp;page="*) check_passed "the pager carries the filter and the sort into its links, correctly escaped" ;;
	*) check_failed "the pager carries the filter and the sort into its links" \
		"$(printf '%s' "$HTTP_BODY" | grep -oE 'href="/products\?[^"]*page=[0-9]*"' | head -3 || true)" ;;
	esac

	http_get "/products?stock=in&sort=name&page=2"
	assert_equal "following the pager keeps the filter applied" "6" "$(cards_in_body)"
	assert_equal "and keeps the sort" "Marking gauge, brass" "$(first_product_name)"
}

check_injected_parameters_are_refused() {
	local probe
	for probe in "1%3B%20DROP%20TABLE%20products" "price_cents%3B--" "..%2F..%2Fetc%2Fpasswd" "%00" "name%20desc"; do
		http_get "/products?sort=$probe"
		if [ "$HTTP_STATUS" = "200" ]; then
			check_passed "an injected sort of '$probe' falls back to the default order"
		else
			check_failed "an injected sort is handled" "sort=$probe answered $HTTP_STATUS"
		fi
	done

	for probe in "abc" "-5" "0" "99999999999999999999"; do
		http_get "/products?page=$probe"
		if [ "$HTTP_STATUS" = "200" ]; then
			check_passed "a nonsense page of '$probe' still answers"
		else
			check_failed "a nonsense page is handled" "page=$probe answered $HTTP_STATUS"
		fi
	done

	http_get "/products?min=notanumber&max=alsonot"
	assert_equal "a non-numeric price filter is ignored rather than fatal" "200" "$HTTP_STATUS"
	assert_equal "and the full catalogue comes back" "12" "$(cards_in_body)"
}

check_faceting_is_absent() {
	http_get "/products"
	case "$HTTP_BODY" in
	*"Hand tools (6)"* | *"Hand tools</a> (6)"*)
		check_passed "the department list shows how many products are in each"
		;;
	*)
		finding "there is no way to count matches per filter value without a query per facet" \
			"Every shop shows 'Hand tools (6)' beside its filters. model.Query can count one filtered
set at a time, so a four-department sidebar costs four extra Count queries per page render, and a
price histogram is not expressible at all. There is no GROUP BY in the generic store and no
aggregate other than Count. This is the single biggest thing a catalogue needs that the framework
does not offer; the shop's sidebar therefore shows bare department names."
		;;
	esac
}

start_the_shop
check_filtering_by_department
check_filtering_by_price
check_filtering_by_stock
check_sorting
check_filters_survive_pagination
check_injected_parameters_are_refused
check_faceting_is_absent

chunk_end
