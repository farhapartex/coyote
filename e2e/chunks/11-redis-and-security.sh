#!/usr/bin/env bash

. "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib/bootstrap.sh"
. "$E2E_DIR/lib/services.sh"
. "$E2E_DIR/lib/shop.sh"
. "$E2E_DIR/lib/server.sh"
. "$E2E_DIR/lib/http.sh"
. "$E2E_DIR/lib/admin.sh"

chunk_begin "11" "Redis, security and production settings"

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

check_redis_is_actually_used() {
	redis_flush
	assert_equal "redis starts empty" "0" "$(redis_key_count)"

	http_get "/"
	local keys
	keys="$(redis_key_count)"
	if [ "${keys:-0}" -ge 1 ]; then
		check_passed "rendering a page put $keys key(s) in redis"
	else
		check_failed "the redis cache is used" "the cache is configured but nothing was written"
		return 0
	fi

	local stored
	stored="$(redis_command KEYS '*' | head -1)"
	case "$stored" in
	coyote:*) check_passed "the key is namespaced with the configured prefix ($stored)" ;;
	*) check_failed "cache keys are namespaced" "got '$stored'" ;;
	esac

	local before after
	before="$(redis_command GET "$stored")"
	http_get "/products"
	after="$(redis_command GET "$stored")"
	assert_equal "a second page read the cached value rather than rewriting it" "$before" "$after"

	local ttl
	ttl="$(redis_command TTL "$stored")"
	if [ "${ttl:-0}" -gt 0 ]; then
		check_passed "the entry carries the TTL the application asked for (${ttl}s)"
	else
		check_failed "the cache entry expires" "TTL came back as ${ttl:-none}"
	fi
}

check_the_cache_can_go_stale() {
	local id renamed
	id="$(postgres_query "SELECT id FROM categories WHERE slug='finishing';")"
	renamed="Finishing $(seconds_now)"

	http_reset_session
	http_get "/"
	case "$HTTP_BODY" in
	*"Finishing"*) check_passed "the department list is on the page and cached" ;;
	*) check_failed "the department list is on the page" "no department names rendered" ;;
	esac

	if admin_login "root" "thornfield-supply-2026"; then
		admin_submit "/admin/categories/$id" \
			"name=$renamed" "slug=finishing" \
			"blurb=Oils, waxes and abrasives" "position=4" >/dev/null
	fi
	assert_equal "the rename reached postgres" "$renamed" \
		"$(postgres_query "SELECT name FROM categories WHERE id='$id';")"

	http_reset_session
	http_get "/"
	case "$HTTP_BODY" in
	*"$renamed"*)
		check_passed "renaming a department shows up on the storefront at once"
		;;
	*)
		finding "a cached read has no way to be invalidated when the underlying row changes" \
			"The shop caches its department list with cache.Remember for five minutes, because it is
read on every page. Renaming a department through the admin does not appear on the storefront until
the entry expires - the admin has no idea the cache exists.
core/store has store.Cached, which does track a generation counter per table and invalidate on
write, but it only covers reads that go through model.Store. Anything an application caches itself
with cache.Remember is invisible to it. There is no event, hook or signal on a model write that an
application could subscribe to, so the only options are a short TTL (stale data) or no cache (slow
pages). A write-through invalidation hook, or making store.Cached's generation counter readable so an
application can build its own key from it, would close this."
		;;
	esac
}

check_the_session_store_is_postgres_not_redis() {
	local sessions
	sessions="$(postgres_row_count sessions)"
	if [ "${sessions:-0}" -ge 1 ]; then
		check_passed "sessions are in postgres ($sessions live)"
	else
		check_failed "sessions are in postgres" "the sessions table is empty"
	fi

	finding "redis cannot be used as the session store even when it is already configured" \
		"The shop runs redis for its cache and postgres for its sessions, so every request that
touches a session makes a database round trip while a perfectly good key-value store sits idle beside
it. Sessions.Backend accepts memory, database or cookie and there is no cache option, which
guide/27-architecture.md already lists under the missing seams. For a shop this is the hottest path
in the application: the cart lives in the session, so every page load reads it."
}

check_security_headers() {
	http_get "/"
	assert_equal "nosniff is set" "nosniff" "$(http_header_value X-Content-Type-Options)"
	assert_equal "frames are denied" "DENY" "$(http_header_value X-Frame-Options)"
	assert_equal "the referrer is kept same-origin" "same-origin" "$(http_header_value Referrer-Policy)"
	assert_equal "cross-origin opener policy is set" "same-origin" "$(http_header_value Cross-Origin-Opener-Policy)"

	if [ -z "$(http_header_value Content-Security-Policy)" ]; then
		note "content security policy" "not set, which is the documented default"
	else
		check_passed "a content security policy is set"
	fi
}

check_csrf_covers_every_unsafe_route() {
	http_reset_session
	local path
	for path in "/cart/add" "/cart/update" "/cart/remove" "/checkout" "/checkout/coupon" "/accounts/login" "/accounts/register"; do
		http_post "$path" "probe=1"
		if [ "$HTTP_STATUS" = "403" ]; then
			check_passed "POST $path without a token is refused"
		else
			check_failed "POST $path without a token is refused" "got $HTTP_STATUS"
		fi
	done
}

check_a_foreign_host_is_refused() {
	local status
	status="$(curl -sS -o /dev/null -w '%{http_code}' --max-time 10 \
		-H "Host: evil.example" "$E2E_BASE_URL/" 2>/dev/null || true)"
	assert_equal "a request for an unlisted host is refused" "400" "$status"
}

check_the_admin_is_closed_to_shoppers() {
	http_reset_session
	http_get "/accounts/login"
	local token
	token="$(http_csrf_token)"
	http_post "/accounts/login" "csrf_token=$token" "username=edith" "password=bench-plane-8821"

	http_get "/admin/"
	if [ "$HTTP_STATUS" = "403" ]; then
		check_passed "a signed-in shopper is refused the admin portal"
	else
		check_failed "a signed-in shopper is refused the admin portal" "got $HTTP_STATUS"
	fi

	http_get "/admin/products"
	if [ "$HTTP_STATUS" = "403" ]; then
		check_passed "and refused the product section too"
	else
		check_failed "and refused the product section" "got $HTTP_STATUS"
	fi
}

check_the_production_preset_hardens_the_app() {
	server_stop >/dev/null 2>&1 || true
	cp "$SHOP_DIR/.env" "$SHOP_DIR/.env.backup"
	sed -i '' 's/^APP_ENV=.*/APP_ENV=production/' "$SHOP_DIR/.env"

	shop_start
	if ! server_wait_for_http; then
		check_failed "the shop starts under the production preset" "$(tail -20 "$SERVER_LOG")"
		mv "$SHOP_DIR/.env.backup" "$SHOP_DIR/.env"
		return 0
	fi
	check_passed "the shop starts under the production preset"

	if grep -q "Debug is enabled" "$SERVER_LOG"; then
		check_failed "the production preset turns Debug off" "the debug warning is still logged"
	else
		check_passed "the production preset turns Debug off"
	fi

	if grep -q '"level":"INFO"' "$SERVER_LOG" || grep -q 'level=INFO' "$SERVER_LOG"; then
		check_passed "the production preset is logging"
	fi
	if grep -q '{"time"' "$SERVER_LOG"; then
		check_passed "and logging as JSON rather than text"
	else
		check_failed "the production preset logs JSON" "the log is still the development text format"
	fi

	http_reset_session
	http_get "/products"
	local cookie
	cookie="$(printf '%s' "$HTTP_HEADERS" | grep -i '^set-cookie:' | head -1 || true)"
	case "$cookie" in
	*Secure*) check_passed "the session cookie is marked Secure on a deployed profile" ;;
	"") check_skipped "the session cookie is marked Secure" "no cookie was set on this request" ;;
	*) check_failed "the session cookie is marked Secure on a deployed profile" "got: $cookie" ;;
	esac
	case "$cookie" in
	*HttpOnly*) check_passed "and HttpOnly" ;;
	"") : ;;
	*) check_failed "the session cookie is HttpOnly" "got: $cookie" ;;
	esac

	server_stop >/dev/null 2>&1 || true
	mv "$SHOP_DIR/.env.backup" "$SHOP_DIR/.env"
	shop_start
	server_wait_for_http >/dev/null || true
}

start_the_shop
check_redis_is_actually_used
check_the_cache_can_go_stale
check_the_session_store_is_postgres_not_redis
check_security_headers
check_csrf_covers_every_unsafe_route
check_a_foreign_host_is_refused
check_the_admin_is_closed_to_shoppers
check_the_production_preset_hardens_the_app

chunk_end
