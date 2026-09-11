#!/usr/bin/env bash

. "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib/bootstrap.sh"
. "$E2E_DIR/lib/services.sh"
. "$E2E_DIR/lib/shop.sh"
. "$E2E_DIR/lib/server.sh"
. "$E2E_DIR/lib/http.sh"

chunk_begin "08" "The cart and the checkout"

require_previous_chunk "the shop exists" "$SHOP_DIR/settings.go"

trap server_cleanup EXIT

SHOPPER_USER="edith"
SHOPPER_PASSWORD="bench-plane-8821"
ORDER_REFERENCE=""

start_the_shop() {
	shop_start
	if server_wait_for_http; then
		check_passed "the shop is serving"
		return 0
	fi
	check_failed "the shop is serving" "$(tail -20 "$SERVER_LOG")"
	chunk_end
}

sign_in_as_the_shopper() {
	http_reset_session
	http_get "/accounts/login"
	local token
	token="$(http_csrf_token)"
	http_post "/accounts/login" "csrf_token=$token" "username=$SHOPPER_USER" "password=$SHOPPER_PASSWORD"
	if [ "$HTTP_STATUS" = "303" ] || [ "$HTTP_STATUS" = "302" ]; then
		check_passed "the shopper signs in"
		return 0
	fi
	check_failed "the shopper signs in" "status $HTTP_STATUS; run chunk 07 first"
	chunk_end
}

add_to_cart() {
	local slug="$1" quantity="$2"
	http_get "/products/$slug"
	local token
	token="$(http_csrf_token)"
	http_post "/cart/add" "csrf_token=$token" "product=$slug" "quantity=$quantity"
}

check_the_cart_starts_empty() {
	http_get "/cart"
	assert_equal "the cart page answers 200" "200" "$HTTP_STATUS"
	case "$HTTP_BODY" in
	*"Your cart is empty"*) check_passed "an empty cart says so" ;;
	*) check_failed "an empty cart says so" "no empty-cart message" ;;
	esac
	case "$HTTP_BODY" in
	*"Cart · 0"*) check_passed "the header shows a count of nothing" ;;
	*) check_failed "the header shows a count of nothing" "no zero count in the header" ;;
	esac
}

check_adding_to_the_cart() {
	add_to_cart "bevel-edge-chisel-25" 2
	if [ "$HTTP_STATUS" = "303" ] || [ "$HTTP_STATUS" = "302" ]; then
		check_passed "adding a product redirects to the cart"
	else
		check_failed "adding a product redirects to the cart" "status $HTTP_STATUS"
	fi

	http_get "/cart"
	case "$HTTP_BODY" in
	*"Bevel-edge chisel, 25mm"*) check_passed "the cart lists what was added" ;;
	*) check_failed "the cart lists what was added" "the chisel is not in the cart" ;;
	esac
	case "$HTTP_BODY" in
	*"Cart · 2"*) check_passed "the header count follows the quantity" ;;
	*) check_failed "the header count follows the quantity" "expected a count of 2" ;;
	esac
	case "$HTTP_BODY" in
	*"£69.00"*) check_passed "the line subtotal is two times the unit price" ;;
	*) check_failed "the line subtotal is two times the unit price" "expected £69.00" ;;
	esac

	add_to_cart "danish-oil-500" 3
	http_get "/cart"
	case "$HTTP_BODY" in
	*"Cart · 5"*) check_passed "a second product adds to the same cart" ;;
	*) check_failed "a second product adds to the same cart" "expected a count of 5" ;;
	esac
	case "$HTTP_BODY" in
	*"£117.45"*) check_passed "the total adds the goods and the delivery charge" ;;
	*) check_failed "the total adds the goods and the delivery charge" "expected £117.45 for 2 chisels and 3 oils plus delivery" ;;
	esac
}

check_stock_limits_the_cart() {
	add_to_cart "plunge-router-1400" 99
	http_get "/cart"
	case "$HTTP_BODY" in
	*"Only 4"*) check_passed "the cart refuses more than the stock and says how many there are" ;;
	*) check_failed "the cart refuses more than the stock" "no stock warning" ;;
	esac
}

check_an_out_of_stock_product_is_refused() {
	add_to_cart "biscuit-jointer" 1
	http_get "/products/biscuit-jointer"
	case "$HTTP_BODY" in
	*"is out of stock"*) check_passed "adding an out-of-stock product is refused with a flash message" ;;
	*) check_failed "adding an out-of-stock product is refused with a flash message" \
		"no out-of-stock flash on the page the shopper was sent back to" ;;
	esac

	http_get "/cart"
	case "$HTTP_BODY" in
	*'"/products/biscuit-jointer"'*) check_failed "the out-of-stock product is not in the cart" "it reached the cart" ;;
	*) check_passed "the out-of-stock product is not in the cart" ;;
	esac
}

check_updating_and_removing() {
	http_get "/cart"
	local token
	token="$(http_csrf_token)"
	http_post "/cart/update" "csrf_token=$token" \
		"quantity_bevel-edge-chisel-25=1" "quantity_danish-oil-500=3" "quantity_plunge-router-1400=4"
	http_get "/cart"
	case "$HTTP_BODY" in
	*"Cart · 8"*) check_passed "quantities can be changed from the cart" ;;
	*) check_failed "quantities can be changed from the cart" "expected a count of 8" ;;
	esac

	token="$(http_csrf_token)"
	http_post "/cart/remove" "csrf_token=$token" "product=plunge-router-1400"
	http_get "/cart"
	case "$HTTP_BODY" in
	*"Plunge router"*) check_failed "a line can be removed" "the router is still there" ;;
	*) check_passed "a line can be removed" ;;
	esac
	case "$HTTP_BODY" in
	*"Cart · 4"*) check_passed "the count drops with the removed line" ;;
	*) check_failed "the count drops with the removed line" "expected a count of 4" ;;
	esac
}

check_the_cart_needs_a_token() {
	http_post "/cart/add" "product=cabinet-scraper" "quantity=1"
	assert_equal "adding to the cart without a CSRF token is refused" "403" "$HTTP_STATUS"
	http_get "/cart"
	case "$HTTP_BODY" in
	*"Cabinet scraper"*) check_failed "the refused add changed nothing" "the scraper reached the cart" ;;
	*) check_passed "the refused add changed nothing" ;;
	esac
}

check_the_cart_survives_a_restart() {
	http_get "/cart"
	local before="$HTTP_BODY"

	server_stop >/dev/null 2>&1 || true
	shop_start
	if ! server_wait_for_http; then
		check_failed "the shop restarts" "$(tail -20 "$SERVER_LOG")"
		chunk_end
	fi

	http_get "/cart"
	case "$HTTP_BODY" in
	*"Cart · 4"*) check_passed "the cart survives a server restart because the session is in postgres" ;;
	*) check_failed "the cart survives a server restart" "the cart was lost across the restart" ;;
	esac
	case "$HTTP_BODY" in
	*"Bevel-edge chisel"*) check_passed "and the lines come back with it" ;;
	*) check_failed "and the lines come back with it" "the chisel is gone" ;;
	esac
	[ -n "$before" ] || true
}

check_a_coupon() {
	postgres_query "DELETE FROM coupons WHERE code='SPRING10';" >/dev/null
	postgres_query "INSERT INTO coupons (id, code, percent_off, is_active) VALUES ('coupon-spring', 'SPRING10', 10, true);" >/dev/null

	http_get "/checkout"
	assert_equal "the checkout page answers 200" "200" "$HTTP_STATUS"

	local token
	token="$(http_csrf_token)"
	http_post "/checkout/coupon" "csrf_token=$token" "code=NONSENSE"
	http_get "/checkout"
	case "$HTTP_BODY" in
	*"not valid"*) check_passed "an unknown coupon is refused" ;;
	*) check_failed "an unknown coupon is refused" "no rejection message" ;;
	esac

	token="$(http_csrf_token)"
	http_post "/checkout/coupon" "csrf_token=$token" "code=SPRING10"
	http_get "/checkout"
	case "$HTTP_BODY" in
	*"10% off"*) check_passed "a valid coupon is applied" ;;
	*) check_failed "a valid coupon is applied" "no discount line" ;;
	esac
	case "$HTTP_BODY" in
	*"-£7.80"*) check_passed "the discount is ten percent of the goods, not of the total" ;;
	*) check_failed "the discount is ten percent of the goods" "expected -£7.80 of £78.00 goods" ;;
	esac
}

check_checkout_validates() {
	http_get "/checkout"
	local token
	token="$(http_csrf_token)"
	http_post "/checkout" "csrf_token=$token" "email=" "line1=" "city=" "postcode="
	assert_equal "an empty checkout form is refused with 422" "422" "$HTTP_STATUS"

	local problem
	for problem in "email address to send the receipt" "street address is required" \
		"town or city is required" "postcode is required"; do
		case "$HTTP_BODY" in
		*"$problem"*) check_passed "the form complains that the $problem" ;;
		*) check_failed "the form complains about a missing field" "missing: $problem" ;;
		esac
	done
	assert_equal "nothing was ordered" "0" "$(postgres_row_count orders)"
}

check_placing_the_order() {
	local stock_before
	stock_before="$(postgres_query "SELECT stock FROM products WHERE slug='bevel-edge-chisel-25';")"

	http_get "/checkout"
	local token
	token="$(http_csrf_token)"
	http_post "/checkout" "csrf_token=$token" \
		"email=edith@thornfield.test" "line1=14 Shrewsbury Road" "line2=" \
		"city=Ludlow" "postcode=SY8 1AA"

	if [ "$HTTP_STATUS" = "303" ] || [ "$HTTP_STATUS" = "302" ]; then
		check_passed "the order is placed"
	else
		check_failed "the order is placed" "status $HTTP_STATUS
$(printf '%s' "$HTTP_BODY" | grep -oE 'flash error[^<]*<[^>]*>[^<]*' | head -3 || true)"
		chunk_end
	fi

	ORDER_REFERENCE="$(postgres_query "SELECT reference FROM orders LIMIT 1;")"
	assert_equal "there is exactly one order" "1" "$(postgres_row_count orders)"
	assert_equal "it has two lines" "2" "$(postgres_row_count order_lines)"
	assert_equal "it starts in the placed state" "placed" \
		"$(postgres_query "SELECT state FROM orders LIMIT 1;")"
	assert_equal "it belongs to the shopper" "$SHOPPER_USER" \
		"$(postgres_query "SELECT u.username FROM orders o JOIN users u ON u.id = o.user_id LIMIT 1;")"

	local stock_after
	stock_after="$(postgres_query "SELECT stock FROM products WHERE slug='bevel-edge-chisel-25';")"
	assert_equal "the chisel's stock fell by the one ordered" "$((stock_before - 1))" "$stock_after"
	assert_equal "a stock movement was recorded for each line" "2" "$(postgres_row_count stock_movements)"
	assert_equal "the first shipment event was written" "1" "$(postgres_row_count shipment_events)"

	local goods discount shipping total
	goods="$(postgres_query "SELECT goods_cents FROM orders LIMIT 1;")"
	discount="$(postgres_query "SELECT discount_cents FROM orders LIMIT 1;")"
	shipping="$(postgres_query "SELECT shipping_cents FROM orders LIMIT 1;")"
	total="$(postgres_query "SELECT total_cents FROM orders LIMIT 1;")"
	assert_equal "the stored total is goods minus discount plus delivery" \
		"$total" "$((goods - discount + shipping))"
	assert_equal "the coupon code is kept on the order" "SPRING10" \
		"$(postgres_query "SELECT coupon_code FROM orders LIMIT 1;")"
}

check_the_cart_is_emptied() {
	http_get "/cart"
	case "$HTTP_BODY" in
	*"Your cart is empty"*) check_passed "placing the order empties the cart" ;;
	*) check_failed "placing the order empties the cart" "the cart still has lines" ;;
	esac
	http_get "/checkout"
	if [ "$HTTP_STATUS" = "303" ] || [ "$HTTP_STATUS" = "302" ]; then
		check_passed "an empty cart cannot reach the checkout"
	else
		check_failed "an empty cart cannot reach the checkout" "status $HTTP_STATUS"
	fi
}

check_an_anonymous_checkout() {
	http_reset_session
	add_to_cart "cabinet-scraper" 1
	http_get "/checkout"
	local token
	token="$(http_csrf_token)"
	http_post "/checkout" "csrf_token=$token" "email=nobody@thornfield.test" \
		"line1=1 Nowhere" "city=Ludlow" "postcode=SY8 1AA"

	if [ "$HTTP_STATUS" = "303" ] || [ "$HTTP_STATUS" = "302" ]; then
		case "$(http_header_value Location)" in
		*"/accounts/login"*) check_passed "an anonymous shopper is sent to sign in at the checkout" ;;
		*) check_failed "an anonymous shopper is sent to sign in" "went to $(http_header_value Location)" ;;
		esac
	else
		check_failed "an anonymous shopper is sent to sign in" "status $HTTP_STATUS"
	fi
	assert_equal "and no order was created" "1" "$(postgres_row_count orders)"
}

start_the_shop
sign_in_as_the_shopper
check_the_cart_starts_empty
check_adding_to_the_cart
check_stock_limits_the_cart
check_an_out_of_stock_product_is_refused
check_updating_and_removing
check_the_cart_needs_a_token
check_the_cart_survives_a_restart
check_a_coupon
check_checkout_validates
check_placing_the_order
check_the_cart_is_emptied
check_an_anonymous_checkout

chunk_end
