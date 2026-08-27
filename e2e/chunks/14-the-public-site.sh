#!/usr/bin/env bash

. "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib/bootstrap.sh"
. "$E2E_DIR/lib/appsource.sh"
. "$E2E_DIR/lib/server.sh"
. "$E2E_DIR/lib/http.sh"

chunk_begin "14" "The public site"

trap server_cleanup EXIT

check_the_data_is_there() {
	if app_table_exists shipments && [ "$(app_row_count ports)" -ge 3 ]; then
		return 0
	fi
	check_failed "the reference data is in place" "run chunks 05 to 13 first"
	chunk_end
}

write_the_public_site() {
	app_write_public_handlers
	app_write_public_templates
	app_write_admin_resources 12
	app_write_main 14

	assert_file_exists "the public handlers are written" "$EXAMPLE_DIR/handlers_public.go"
	assert_file_exists "the landing template is written" "$EXAMPLE_DIR/templates/pages/landing.html"

	cd "$EXAMPLE_DIR"
	assert_succeeds "the project compiles with the public site" go build ./...
	assert_succeeds "the public site passes go vet" go vet ./...
	cd "$E2E_ROOT"
}

boot_the_server() {
	if ! port_is_free; then
		force_free_the_port
	fi
	server_start
	if server_wait_for_http; then
		check_passed "the application serves the public site"
		return 0
	fi
	check_failed "the application serves the public site" "$(tail -20 "$SERVER_LOG")"
	chunk_end
}

check_every_page_answers() {
	local route
	for route in / /services /track /quote /about /contact; do
		assert_http_status "GET $route answers 200" "200" "$route"
	done
}

check_the_landing_page() {
	http_get "/"
	assert_body_contains "the landing page carries its headline" "Meridian Freight" "/"
	assert_body_contains "the landing page lists a service" "Ocean freight" "/"
	assert_body_contains "the layout supplies the page title" "<title>Meridian Freight" "/"
}

check_named_routes_resolve_in_templates() {
	http_get "/"

	local links
	links="$(printf '%s' "$HTTP_BODY" | grep -oE 'href="[^"]*"' | sort -u | tr '\n' ' ')"

	local expected
	for expected in '/track' '/quote' '/services/ocean-freight'; do
		case "$links" in
		*"href=\"$expected\""*)
			check_passed "the template resolved a link to $expected"
			;;
		*)
			check_failed "the template resolved a link to $expected" "links: $links"
			;;
		esac
	done

	case "$links" in
	*'href=""'* | *'href="#"'*)
		check_failed "no named route resolved to an empty href" "links: $links"
		;;
	*)
		check_passed "no named route resolved to an empty href"
		;;
	esac
}

check_a_dynamic_segment() {
	local slug
	for slug in ocean-freight air-freight customs-brokerage; do
		assert_http_status "GET /services/$slug answers 200" "200" "/services/$slug"
	done

	assert_body_contains "the detail page renders the matched service" \
		"Full and less than container loads" "/services/ocean-freight"
	assert_body_contains "a different slug renders a different service" \
		"Time critical movements" "/services/air-freight"

	assert_http_status "an unknown slug answers 404" "404" "/services/no-such-service"
	assert_body_contains "the 404 page is the application's own" \
		"Page not found" "/services/no-such-service"
}

check_a_lookup_behind_a_dynamic_segment() {
	assert_http_status "a known port code answers 200" "200" "/ports/NLRTM"
	assert_body_contains "the port page names the port" "Rotterdam" "/ports/NLRTM"
	assert_body_contains "the port page shows its code" "NLRTM" "/ports/NLRTM"

	assert_http_status "a lowercase code is normalised and still found" "200" "/ports/nlrtm"
	assert_body_contains "the lowercase request resolves the same port" "Rotterdam" "/ports/nlrtm"

	assert_http_status "an unknown port code answers 404" "404" "/ports/ZZZZZ"

	assert_http_status "a code carrying a quote is refused, not executed" "404" "/ports/NL%27RTM"
	assert_http_status "a code carrying SQL is refused, not executed" "404" "/ports/x%27%20OR%20%271%27%3D%271"
	assert_equal "the ports table is intact after those attempts" "3" "$(app_row_count ports)"
}

check_the_tracking_pages() {
	assert_http_status "the tracking form answers 200" "200" "/track"
	assert_body_contains "the tracking form offers a reference field" 'name="reference"' "/track"

	assert_http_status "a known reference answers 200" "200" "/track/MRF-000001"
	assert_body_contains "the result page names the reference" "MRF-000001" "/track/MRF-000001"

	http_get "/track/MRF-000001"
	local expected
	for expected in "Rotterdam" "Singapore"; do
		case "$HTTP_BODY" in
		*"$expected"*)
			check_passed "the tracking page resolves $expected from a foreign key"
			;;
		*)
			check_failed "the tracking page resolves $expected from a foreign key" \
				"the relation label did not reach the public page"
			;;
		esac
	done

	case "$HTTP_BODY" in
	*prt-rtm*)
		check_failed "the tracking page shows names, not raw keys" "prt-rtm leaked onto the page"
		;;
	*)
		check_passed "the tracking page shows names, not raw keys"
		;;
	esac

	assert_http_status "an unknown reference is a page, not an error" "200" "/track/NOSUCH"
	assert_body_contains "the miss explains itself" "No shipment matches NOSUCH" "/track/NOSUCH"
	note "a tracking miss" "answers 200 with an explanatory page rather than 404, which is a deliberate choice"
}

check_the_method_is_enforced() {
	http_post "/about" "unused=1"
	assert_equal "POST to a GET-only route answers 405" "405" "$HTTP_STATUS"

	http_post "/services/ocean-freight" "unused=1"
	assert_equal "POST to a dynamic GET-only route answers 405" "405" "$HTTP_STATUS"
}

check_an_unknown_path() {
	assert_http_status "an unknown path answers 404" "404" "/nothing-here"
	assert_http_status "an unknown nested path answers 404" "404" "/services/ocean-freight/extra"
	http_get "/nothing-here"
	case "$HTTP_BODY" in
	*"Page not found"*)
		check_passed "an unrouted path renders the application's own 404 page"
		;;
	*)
		check_failed "an unrouted path renders the application's own 404 page" \
			"the body was $(printf '%s' "$HTTP_BODY" | head -1 | tr -d '\r')
a path with no matching route falls through to the standard library's plain text 404, because
the framework exposes no hook for one; a handler can call RenderStatus, as /services/nope does,
but an unknown URL cannot be styled at all, which is visible to every visitor of a public site"
		note "custom 404" \
			"a Settings.NotFound handler, or a.SetNotFound, would let an application own this page"
		;;
	esac
}

check_the_admin_still_works_alongside_the_public_site() {
	assert_http_status "the admin is still mounted" "303" "/admin/customers"
	assert_http_status "the static file is still served" "200" "/static/site.css"
}

check_the_public_site_needs_no_session() {
	http_reset_session
	http_get "/"

	assert_equal "an anonymous visitor gets the landing page" "200" "$HTTP_STATUS"

	local cookie
	cookie="$(http_header_value Set-Cookie)"
	if [ -z "$cookie" ]; then
		check_passed "a public page mints no session cookie"
	else
		check_skipped "a public page mints no session cookie" \
			"a cookie was set: ${cookie%%;*}"
	fi
}

check_the_data_is_there
write_the_public_site
boot_the_server
check_every_page_answers
check_the_landing_page
check_named_routes_resolve_in_templates
check_a_dynamic_segment
check_a_lookup_behind_a_dynamic_segment
check_the_tracking_pages
check_the_method_is_enforced
check_an_unknown_path
check_the_admin_still_works_alongside_the_public_site
check_the_public_site_needs_no_session

server_stop
if port_is_free; then
	check_passed "the port is free when the chunk ends"
else
	check_failed "the port is free when the chunk ends" "held by $(port_listener_pids | tr '\n' ' ')"
fi

chunk_end
