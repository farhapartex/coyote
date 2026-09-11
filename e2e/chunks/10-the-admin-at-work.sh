#!/usr/bin/env bash

. "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib/bootstrap.sh"
. "$E2E_DIR/lib/services.sh"
. "$E2E_DIR/lib/shop.sh"
. "$E2E_DIR/lib/server.sh"
. "$E2E_DIR/lib/http.sh"
. "$E2E_DIR/lib/admin.sh"

chunk_begin "10" "The admin at work"

require_previous_chunk "the shop exists" "$SHOP_DIR/settings.go"

trap server_cleanup EXIT

start_the_shop() {
	shop_start
	if server_wait_for_http; then
		check_passed "the shop is serving"
		return 0
	fi
	check_failed "the shop is serving" "$(tail -20 "$SERVER_LOG")"
	chunk_end
}

sign_in() {
	if admin_login "root" "thornfield-supply-2026"; then
		check_passed "a superadmin signs in"
		return 0
	fi
	check_failed "a superadmin signs in" "status $HTTP_STATUS"
	chunk_end
}

check_every_resource_is_reachable() {
	local slug
	for slug in categories products orders order_lines shipment_events coupons stock_movements addresses; do
		http_get "/admin/$slug"
		if [ "$HTTP_STATUS" = "200" ]; then
			check_passed "the $slug section answers 200"
		else
			check_failed "the $slug section answers 200" "got $HTTP_STATUS"
		fi
	done
}

check_the_dashboard() {
	http_get "/admin/"
	assert_equal "the dashboard answers 200" "200" "$HTTP_STATUS"
	case "$HTTP_BODY" in
	*"Thornfield Supply"*) check_passed "the dashboard carries the shop's name" ;;
	*) check_failed "the dashboard carries the shop's name" "no site name" ;;
	esac
}

check_inventory_can_be_adjusted() {
	local id before
	id="$(postgres_query "SELECT id FROM products WHERE slug='cabinet-scraper';")"
	before="$(postgres_query "SELECT stock FROM products WHERE id='$id';")"

	local category
	category="$(postgres_query "SELECT category_id FROM products WHERE id='$id';")"

	if admin_submit "/admin/products/$id" \
		"name=Cabinet scraper" "slug=cabinet-scraper" "category_id=$category" \
		"price_cents=1650" "stock=99" "summary=Cheaper than sandpaper and better." \
		"description=Restocked." "is_active=1"; then
		check_passed "an operator can change a product's stock"
	else
		check_failed "an operator can change a product's stock" "status $HTTP_STATUS"
	fi

	assert_equal "the new stock is in postgres" "99" \
		"$(postgres_query "SELECT stock FROM products WHERE id='$id';")"
	[ -n "$before" ] || true

	http_get "/products/cabinet-scraper"
	case "$HTTP_BODY" in
	*"99 in stock"*) check_passed "the storefront shows the new stock immediately" ;;
	*) check_failed "the storefront shows the new stock immediately" "the page still shows the old figure" ;;
	esac
}

check_a_price_change_reaches_the_shop() {
	local id category
	id="$(postgres_query "SELECT id FROM products WHERE slug='beeswax-polish';")"
	category="$(postgres_query "SELECT category_id FROM products WHERE id='$id';")"

	admin_submit "/admin/products/$id" \
		"name=Beeswax polish" "slug=beeswax-polish" "category_id=$category" \
		"price_cents=1395" "stock=45" "summary=Turpentine and beeswax, nothing else." \
		"description=Repriced." "is_active=1" >/dev/null

	http_get "/products/beeswax-polish"
	case "$HTTP_BODY" in
	*"£13.95"*) check_passed "a repriced product shows its new price" ;;
	*) check_failed "a repriced product shows its new price" "expected £13.95" ;;
	esac
}

check_a_boolean_cannot_be_cleared() {
	local id category before after
	id="$(postgres_query "SELECT id FROM products WHERE slug='shop-apron-waxed';")"
	category="$(postgres_query "SELECT category_id FROM products WHERE id='$id';")"
	before="$(postgres_query "SELECT is_active FROM products WHERE id='$id';")"

	admin_submit "/admin/products/$id" \
		"name=Shop apron, waxed" "slug=shop-apron-waxed" "category_id=$category" \
		"price_cents=5400" "stock=19" "summary=Pockets." "description=Withdrawn." >/dev/null
	local accepted="$HTTP_STATUS"
	after="$(postgres_query "SELECT is_active FROM products WHERE id='$id';")"

	if [ "$after" = "f" ]; then
		check_passed "clearing a checkbox in the admin turns the flag off"
		assert_http_status "an inactive product is gone from the storefront" "404" "/products/shop-apron-waxed"
		return 0
	fi

	finding "a boolean on an admin-managed model can be switched on but never off" \
		"Submitting the product form with is_active omitted - which is exactly what a browser sends
for an unchecked checkbox - was accepted with $accepted and left is_active as '$after'. The same is
true of 'featured'. There is no way to withdraw a product, unfeature it, or deactivate a coupon from
the admin portal.
The cause is form.RecordOf in core/form/record.go. When a column is absent from the submission it
looks only at nullability:
      submitted, present := values[f.Column]
      if !present {
              if !f.Nullable { out.Problems.Add(f.Column, f.Label+\" was not submitted\") }
              continue
      }
A bool tagged gorm:\"index\" or gorm:\"default:true\" is nullable, so an absent checkbox is skipped
silently and the column keeps its old value. Make the column NOT NULL instead and the same
submission fails validation with 'was not submitted', so neither shape works.
HTML has no way to send 'this checkbox is off', which is why an absent value must mean false for a
boolean. RecordOf already special-cases f.Kind == model.KindBool when the value is present
(anyTruthy); it needs the matching branch when it is absent. The built-in user form sidesteps this
by reading r.PostForm.Get(\"is_staff\") != \"\" directly, which is why the framework's own tests do
not catch it."
}

check_the_order_advance_action() {
	postgres_query "UPDATE orders SET state='placed' WHERE state='delivered';" >/dev/null
	local id
	id="$(postgres_query "SELECT id FROM orders LIMIT 1;")"

	http_get "/admin/orders"
	local token
	token="$(http_csrf_token)"
	http_form "/admin/orders/bulk" "csrf_token=$token" "action=advance" "ids=$id"

	if [ "$HTTP_STATUS" = "303" ] || [ "$HTTP_STATUS" = "302" ]; then
		check_passed "the custom bulk action runs from the order list"
	else
		check_failed "the custom bulk action runs from the order list" "status $HTTP_STATUS"
		return 0
	fi

	assert_equal "the order moved one state forward" "packing" \
		"$(postgres_query "SELECT state FROM orders WHERE id='$id';")"
}

check_stock_movements_are_read_only() {
	http_get "/admin/stock_movements"
	assert_equal "the stock movement list answers 200" "200" "$HTTP_STATUS"
	case "$HTTP_BODY" in
	*"order TS-"*) check_passed "each movement records which order caused it" ;;
	*) check_failed "each movement records which order caused it" "no order reference in the reason" ;;
	esac

	http_get "/admin/stock_movements/new"
	if [ "$HTTP_STATUS" = "200" ]; then
		finding "a resource declared read-only still serves its create form" \
			"stockMovementResource implements ReadOnly() returning true, and GET /admin/stock-movements/new
answers 200 with a fully populated form. Submitting it is refused, so nothing is lost, but the
operator is shown a form that cannot work. A read-only resource should not route its new and edit
forms at all."
	else
		check_passed "a read-only resource does not serve a create form"
	fi
}

check_customers_are_manageable() {
	http_get "/admin/users"
	assert_equal "the user list answers 200" "200" "$HTTP_STATUS"
	case "$HTTP_BODY" in
	*"edith"*) check_passed "the shopper appears in the admin user list" ;;
	*) check_failed "the shopper appears in the admin user list" "edith not listed" ;;
	esac

	http_get "/admin/users?q=edith"
	case "$HTTP_BODY" in
	*"edith"*) check_passed "customers can be searched by username" ;;
	*) check_failed "customers can be searched by username" "search returned nothing" ;;
	esac
}

check_the_sessions_page() {
	http_get "/admin/sessions"
	assert_equal "the session list answers 200" "200" "$HTTP_STATUS"
	local live
	live="$(postgres_row_count sessions)"
	if [ "${live:-0}" -ge 1 ]; then
		check_passed "there are $live live sessions in postgres to show"
	else
		check_failed "sessions are stored in postgres" "the sessions table is empty"
	fi
}

check_an_inline_list_is_impossible() {
	local id
	id="$(postgres_query "SELECT id FROM orders LIMIT 1;")"
	http_get "/admin/orders/$id"
	assert_equal "the order edit form answers 200" "200" "$HTTP_STATUS"

	case "$HTTP_BODY" in
	*"Bevel-edge chisel"*)
		check_passed "the order form shows the lines it contains"
		;;
	*)
		finding "an order cannot show its own lines in the admin" \
			"Opening an order in the admin shows its scalar columns and nothing else. The lines that
make up the order live in order_lines with a belongs-to pointing back, and core/model describes only
the belongs-to direction, so there is no has-many for the admin to render. The operator has to open
the Order lines section separately and filter it by hand - and the order id is a UUID, so that means
copying it between two pages.
This is the single biggest gap for this domain. An order without its lines, a product without its
images, a customer without their addresses: every one of them is the same missing relation. CLAUDE.md
lists inline editing as blocked on exactly this."
		;;
	esac
}

check_permissions_cover_the_new_models() {
	local total
	total="$(postgres_row_count permissions)"
	shop_command syncpermissions >/dev/null 2>&1 || true
	local after
	after="$(postgres_row_count permissions)"
	if [ "${after:-0}" -gt "${total:-0}" ]; then
		check_passed "syncpermissions adds permissions for the models added since ($total to $after)"
	elif [ "${after:-0}" -ge 40 ]; then
		check_passed "every model has its four permissions ($after in total)"
	else
		check_failed "permissions cover the new models" "only ${after:-0} permissions exist"
	fi
}

start_the_shop
sign_in
check_every_resource_is_reachable
check_the_dashboard
check_inventory_can_be_adjusted
check_a_price_change_reaches_the_shop
check_a_boolean_cannot_be_cleared
check_the_order_advance_action
check_stock_movements_are_read_only
check_customers_are_manageable
check_the_sessions_page
check_an_inline_list_is_impossible
check_permissions_cover_the_new_models

chunk_end
