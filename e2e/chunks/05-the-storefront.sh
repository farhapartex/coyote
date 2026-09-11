#!/usr/bin/env bash

. "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib/bootstrap.sh"
. "$E2E_DIR/lib/services.sh"
. "$E2E_DIR/lib/shop.sh"
. "$E2E_DIR/lib/server.sh"
. "$E2E_DIR/lib/http.sh"

chunk_begin "05" "The storefront"

require_previous_chunk "the shop exists" "$SHOP_DIR/settings.go"

trap server_cleanup EXIT

start_the_shop() {
	shop_start
	if server_wait_for_http; then
		check_passed "the shop is serving"
		return 0
	fi
	check_failed "the shop is serving" "$(tail -25 "$SERVER_LOG")"
	chunk_end
}

check_there_is_a_catalogue() {
	local total
	total="$(postgres_row_count products)"
	if [ "${total:-0}" -ge 20 ]; then
		check_passed "the catalogue has $total products to show"
		return 0
	fi
	check_failed "the catalogue has products to show" "found ${total:-0}; run chunk 04 first"
	chunk_end
}

check_the_landing_page() {
	http_get "/"
	assert_equal "the landing page answers 200" "200" "$HTTP_STATUS"

	local needle
	for needle in "Thornfield Supply" "Workshop tools" "Featured" "Departments"; do
		case "$HTTP_BODY" in
		*"$needle"*) check_passed "the landing page shows $needle" ;;
		*) check_failed "the landing page shows $needle" "not in the body" ;;
		esac
	done

	local featured
	featured="$(printf '%s' "$HTTP_BODY" | grep -c 'class="card"')"
	if [ "$featured" -ge 5 ]; then
		check_passed "the landing page renders the featured products and the departments as cards ($featured)"
	else
		check_failed "the landing page renders cards" "found $featured"
	fi

	case "$HTTP_BODY" in
	*"£118.00"*) check_passed "money is formatted for humans, not printed in pence" ;;
	*) check_failed "money is formatted for humans" "expected £118.00 for the block plane" ;;
	esac
}

check_the_custom_stylesheet_is_served() {
	http_get "/static/shop.css"
	assert_equal "the custom stylesheet answers 200" "200" "$HTTP_STATUS"
	assert_equal "it is served as CSS" "text/css; charset=utf-8" "$(http_header_value Content-Type)"

	case "$HTTP_BODY" in
	*"--brand:"*) check_passed "the stylesheet is the shop's own, not the scaffold's" ;;
	*) check_failed "the stylesheet is the shop's own" "no custom properties found" ;;
	esac

	http_get "/"
	case "$HTTP_BODY" in
	*'href="/static/shop.css"'*) check_passed "the layout links the stylesheet by its static URL" ;;
	*) finding "the static template function did not produce the expected URL" \
		"The layout calls {{static \"shop.css\"}} and the rendered page did not contain
href=\"/static/shop.css\". Check core/app/static.go against Static.URL." ;;
	esac
}

check_the_listing() {
	http_get "/products"
	assert_equal "the product listing answers 200" "200" "$HTTP_STATUS"

	case "$HTTP_BODY" in
	*"20 products"*) check_passed "the listing reports the full count" ;;
	*) check_failed "the listing reports the full count" "expected '20 products' in the toolbar" ;;
	esac

	local cards
	cards="$(printf '%s' "$HTTP_BODY" | grep -c 'class="card"')"
	assert_equal "the first page shows twelve products" "12" "$cards"

	case "$HTTP_BODY" in
	*'class="pager"'*) check_passed "a pager appears for the second page" ;;
	*) check_failed "a pager appears for the second page" "no pager in the body" ;;
	esac

	http_get "/products?page=2"
	cards="$(printf '%s' "$HTTP_BODY" | grep -c 'class="card"')"
	assert_equal "the second page shows the remaining eight" "8" "$cards"
}

check_the_product_page() {
	http_get "/products/low-angle-block-plane"
	assert_equal "a product page answers 200" "200" "$HTTP_STATUS"

	local needle
	for needle in "Low-angle block plane" "£118.00" "6 in stock" "Add to cart" "Hand tools"; do
		case "$HTTP_BODY" in
		*"$needle"*) check_passed "the product page shows $needle" ;;
		*) check_failed "the product page shows $needle" "not in the body" ;;
		esac
	done

	case "$HTTP_BODY" in
	*"Also in Hand tools"*) check_passed "the product page cross-sells from the same department" ;;
	*) check_failed "the product page cross-sells from the same department" "no related block" ;;
	esac

	case "$HTTP_BODY" in
	*'name="csrf_token"'*) check_passed "the add-to-cart form carries a CSRF token" ;;
	*) check_failed "the add-to-cart form carries a CSRF token" "no token in the form" ;;
	esac
}

check_an_out_of_stock_product() {
	http_get "/products/biscuit-jointer"
	assert_equal "an out-of-stock product still has a page" "200" "$HTTP_STATUS"
	case "$HTTP_BODY" in
	*"Out of stock"*) check_passed "it says it is out of stock" ;;
	*) check_failed "it says it is out of stock" "no out-of-stock notice" ;;
	esac
	case "$HTTP_BODY" in
	*"disabled"*) check_passed "its add-to-cart button is disabled" ;;
	*) check_failed "its add-to-cart button is disabled" "the button was not disabled" ;;
	esac
}

check_the_department_pages() {
	http_get "/categories/finishing"
	assert_equal "a department page answers 200" "200" "$HTTP_STATUS"
	case "$HTTP_BODY" in
	*"Danish oil"*) check_passed "the department page lists its own products" ;;
	*) check_failed "the department page lists its own products" "no finishing product found" ;;
	esac
	case "$HTTP_BODY" in
	*"Cordless drill"*) check_failed "the department page excludes other departments" "a power tool leaked in" ;;
	*) check_passed "the department page excludes other departments" ;;
	esac

	assert_http_status "an unknown department is a 404" "404" "/categories/nonsense"
	assert_http_status "an unknown product is a 404" "404" "/products/nonsense"

	http_get "/categories/nonsense"
	case "$HTTP_BODY" in
	*"Nothing here"*) check_passed "the 404 page is the shop's own, not the framework's" ;;
	*) check_failed "the 404 page is the shop's own" "SetNotFound did not render pages/notfound.html" ;;
	esac
}

check_security_headers_on_the_storefront() {
	http_get "/"
	assert_equal "nosniff is set" "nosniff" "$(http_header_value X-Content-Type-Options)"
	assert_equal "frame options are set" "DENY" "$(http_header_value X-Frame-Options)"
	if [ -n "$(http_header_value X-Request-Id)" ]; then
		check_passed "every response carries a request id"
	else
		check_failed "every response carries a request id" "no X-Request-Id header"
	fi
}

start_the_shop
check_there_is_a_catalogue
check_the_landing_page
check_the_custom_stylesheet_is_served
check_the_listing
check_the_product_page
check_an_out_of_stock_product
check_the_department_pages
check_security_headers_on_the_storefront

chunk_end
