#!/usr/bin/env bash

. "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib/bootstrap.sh"
. "$E2E_DIR/lib/services.sh"
. "$E2E_DIR/lib/shop.sh"
. "$E2E_DIR/lib/server.sh"
. "$E2E_DIR/lib/http.sh"
. "$E2E_DIR/lib/admin.sh"

chunk_begin "04" "The catalogue, seeded through the admin"

require_previous_chunk "the shop exists" "$SHOP_DIR/settings.go"

trap server_cleanup EXIT

ADMIN_USER="root"
ADMIN_PASSWORD="thornfield-supply-2026"

reset_the_catalogue() {
	postgres_query "TRUNCATE products, categories CASCADE;" >/dev/null
	assert_equal "the catalogue starts empty" "0" "$(postgres_row_count categories)"
}

start_the_shop() {
	shop_start
	if server_wait_for_http; then
		check_passed "the shop serves a request on postgres and redis"
		return 0
	fi
	check_failed "the shop serves a request on postgres and redis" \
		"no response on $E2E_BASE_URL after 60s
$(tail -25 "$SERVER_LOG")"
	chunk_end
}

sign_in_to_the_admin() {
	http_reset_session
	if admin_login "$ADMIN_USER" "$ADMIN_PASSWORD"; then
		check_passed "a superadmin signs in to the admin portal"
		return 0
	fi
	check_failed "a superadmin signs in to the admin portal" \
		"status ${HTTP_STATUS:-none}
$(printf '%s' "$HTTP_BODY" | head -5 || true)"
	chunk_end
}

create_category() {
	local name="$1" slug="$2" blurb="$3" position="$4"
	admin_submit "/admin/categories/new" \
		"name=$name" "slug=$slug" "blurb=$blurb" "position=$position"
}

create_product() {
	local name="$1" slug="$2" category="$3" price="$4" stock="$5" featured="$6" summary="$7"
	local fields=(
		"name=$name" "slug=$slug" "category_id=$category"
		"price_cents=$price" "stock=$stock" "summary=$summary"
		"description=$summary Turned from a single billet and finished by hand."
		"is_active=1"
	)
	if [ "$featured" = "yes" ]; then
		fields+=("featured=1")
	fi
	admin_submit "/admin/products/new" "${fields[@]}"
}

seed_the_departments() {
	local created=0 slug
	while IFS='|' read -r name slug blurb position; do
		[ -n "$name" ] || continue
		if create_category "$name" "$slug" "$blurb" "$position"; then
			created=$((created + 1))
		fi
	done <<'ROWS'
Hand tools|hand-tools|Chisels, planes and saws that hold an edge|1
Power tools|power-tools|Corded and cordless, all serviceable|2
Workshop|workshop|Benches, vices and the things that hold work still|3
Finishing|finishing|Oils, waxes and abrasives|4
ROWS

	assert_equal "four departments were created through the admin" "4" "$(postgres_row_count categories)"
	if [ "$created" -eq 4 ]; then
		check_passed "each department form submission was accepted"
	else
		check_failed "each department form submission was accepted" "$created of 4 succeeded"
	fi
}

seed_the_products() {
	local hand power workshop finishing
	hand="$(postgres_query "SELECT id FROM categories WHERE slug='hand-tools';")"
	power="$(postgres_query "SELECT id FROM categories WHERE slug='power-tools';")"
	workshop="$(postgres_query "SELECT id FROM categories WHERE slug='workshop';")"
	finishing="$(postgres_query "SELECT id FROM categories WHERE slug='finishing';")"

	local created=0
	while IFS='|' read -r name slug category price stock featured summary; do
		[ -n "$name" ] || continue
		local id=""
		case "$category" in
		hand) id="$hand" ;;
		power) id="$power" ;;
		workshop) id="$workshop" ;;
		finishing) id="$finishing" ;;
		esac
		if create_product "$name" "$slug" "$id" "$price" "$stock" "$featured" "$summary"; then
			created=$((created + 1))
		fi
	done <<'ROWS'
Bevel-edge chisel, 25mm|bevel-edge-chisel-25|hand|3450|18|yes|A bench chisel for paring and chopping.
Bevel-edge chisel, 12mm|bevel-edge-chisel-12|hand|2950|24|no|The narrow chisel you reach for most.
Low-angle block plane|low-angle-block-plane|hand|11800|6|yes|End grain, chamfers and fitting work.
Dovetail saw, 10in|dovetail-saw-10|hand|8900|9|no|Filed rip for clean shoulders.
Cabinet scraper|cabinet-scraper|hand|1650|40|no|Cheaper than sandpaper and better.
Marking gauge, brass|marking-gauge-brass|hand|4200|15|no|Locks where you put it.
Cordless drill driver|cordless-drill-driver|power|18900|12|yes|Two batteries, metal chuck.
Random orbital sander|random-orbital-sander|power|13400|7|no|Dust extraction that works.
Plunge router, 1400W|plunge-router-1400|power|27500|4|yes|Fine height adjustment under load.
Biscuit jointer|biscuit-jointer|power|21000|0|no|Out of stock until the spring.
Track saw, 165mm|track-saw-165|power|39900|3|no|Splinter-free both sides of the cut.
Cast iron vice, 6in|cast-iron-vice-6|workshop|9800|11|no|Bolts down and stays there.
Bench dogs, pair|bench-dogs-pair|workshop|2200|30|no|Brass, so they will not mark work.
Holdfast, forged|holdfast-forged|workshop|3600|22|yes|One tap holds, one tap frees.
Sawbench, beech|sawbench-beech|workshop|16500|0|no|Knocked down flat for delivery.
Shop apron, waxed|shop-apron-waxed|workshop|5400|19|no|Pockets that do not collect shavings.
Danish oil, 500ml|danish-oil-500|finishing|1450|60|no|Three coats and a week to cure.
Beeswax polish|beeswax-polish|finishing|1200|45|no|Turpentine and beeswax, nothing else.
Abrasive pack, mixed grit|abrasive-pack-mixed|finishing|1850|38|no|80 through 400, ten sheets each.
Shellac flakes, 250g|shellac-flakes-250|finishing|2600|16|no|Dewaxed, dissolves in an hour.
ROWS

	local total
	total="$(postgres_row_count products)"
	assert_equal "twenty products were created through the admin" "20" "$total"
	if [ "$created" -eq 20 ]; then
		check_passed "each product form submission was accepted"
	else
		check_failed "each product form submission was accepted" "$created of 20 succeeded"
	fi
}

check_the_admin_lists_what_it_created() {
	http_get "/admin/products"
	assert_equal "the admin product list answers 200" "200" "$HTTP_STATUS"

	http_get "/admin/products?q=block+plane"
	case "$HTTP_BODY" in
	*"Low-angle block plane"*) check_passed "the admin search finds a product by name" ;;
	*) check_failed "the admin search finds a product by name" \
		"searching for 'block plane' did not return it (status $HTTP_STATUS)" ;;
	esac

	case "$HTTP_BODY" in
	*"Hand tools"*) check_passed "the admin list resolves the category relation to its name" ;;
	*) finding "the admin list shows a foreign key but not the name behind it" \
		"The product list renders the category_id column. A belongs-to relation should resolve to the
target's label so the operator sees 'Hand tools' rather than a UUID." ;;
	esac
}

check_a_file_field_blocks_a_urlencoded_save() {
	local token
	token="$(admin_token_for "/admin/products/new")"
	local category
	category="$(postgres_query "SELECT id FROM categories WHERE slug='hand-tools';")"
	http_post "/admin/products/new" "csrf_token=$token" "name=Urlencoded probe" \
		"slug=urlencoded-probe" "category_id=$category" "price_cents=100" "stock=1" \
		"summary=s" "description=d" "is_active=1"

	if [ "$HTTP_STATUS" = "303" ] || [ "$HTTP_STATUS" = "302" ]; then
		check_passed "a resource with a file field saves from a urlencoded form"
		return 0
	fi
	finding "a resource with a file field cannot be saved except as multipart, and says so unhelpfully" \
		"POSTing the product form as application/x-www-form-urlencoded is refused with 400 and the
message 'could not be uploaded' against the image field. The cause is upload.Service.AcceptTo
calling r.ParseMultipartForm unconditionally: on a urlencoded request that returns
http.ErrNotMultipart, which contrib/admin/files.go maps through uploadProblem to a generic string.
The admin's own form sets enctype=multipart/form-data so a browser is unaffected, but any
programmatic client is told neither the cause nor the fix. AcceptTo should treat a non-multipart
request as 'no file supplied' rather than a failure, or upload should export a distinct error the
caller can recognise."
}

check_prices_are_awkward_to_enter() {
	http_get "/admin/products/new"
	case "$HTTP_BODY" in
	*'name="price_cents"'*)
		finding "money has to be entered in minor units in the admin" \
			"The product form asks for price_cents, so an operator adding a £118.00 plane must type
11800. There is no decimal or money field kind, so every commerce application either trains its
staff to think in pence or writes a custom admin form. core/model/kind.go has string, text, int,
float, bool, time, bytes and file; a decimal kind with a scale would remove the whole class of
rounding bug that float invites."
		;;
	esac
}

start_the_shop
sign_in_to_the_admin
reset_the_catalogue
seed_the_departments
seed_the_products
check_the_admin_lists_what_it_created
check_a_file_field_blocks_a_urlencoded_save
check_prices_are_awkward_to_enter

chunk_end
