#!/usr/bin/env bash

. "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib/bootstrap.sh"
. "$E2E_DIR/lib/appsource.sh"
. "$E2E_DIR/lib/server.sh"
. "$E2E_DIR/lib/http.sh"

chunk_begin "19" "Internationalisation"

trap server_cleanup EXIT

check_the_site_is_there() {
	if [ -f "$EXAMPLE_DIR/handlers_public.go" ]; then
		return 0
	fi
	check_failed "the public site is in place" "run chunks 14 to 18 first"
	chunk_end
}

write_the_catalogs() {
	app_write_locales
	app_write_public_templates 19
	app_write_settings 1 0 25 1 3 1
	app_write_main 19

	assert_file_exists "the French catalog is written" "$EXAMPLE_DIR/locales/fr.po"
	assert_file_exists "the Arabic catalog is written" "$EXAMPLE_DIR/locales/ar.po"
	if [ -f "$EXAMPLE_DIR/locales/en.po" ]; then
		check_failed "English needs no catalog of its own" "an en.po was written"
	else
		check_passed "English needs no catalog of its own"
	fi
	assert_output_contains "the catalogs are embedded, not read from disk" "FS:        locales" \
		cat "$EXAMPLE_DIR/settings.go"

	cd "$EXAMPLE_DIR"
	assert_succeeds "the project compiles with three locales" go build ./...
	assert_succeeds "the translated site passes go vet" go vet ./...
	cd "$E2E_ROOT"

	rm -rf "$EXAMPLE_DIR/cache"
}

boot_the_server() {
	if ! port_is_free; then
		force_free_the_port
	fi
	server_start
	if server_wait_for_http; then
		check_passed "the application serves in three locales"
		return 0
	fi
	check_failed "the application serves in three locales" "$(tail -20 "$SERVER_LOG")"
	chunk_end
}

fetch_in_locale() {
	local locale="$1" path="${2:-/about}"
	local header_file
	header_file="$(mktemp "${TMPDIR:-/tmp}/coyote-e2e-i18n-XXXXXX")"

	set +e
	HTTP_BODY="$(curl -sS --max-time 15 -H "Accept-Language: $locale" \
		-D "$header_file" "$E2E_BASE_URL$path" 2>/dev/null)"
	set -e

	HTTP_HEADERS="$(cat "$header_file")"
	HTTP_STATUS="$(printf '%s' "$HTTP_HEADERS" | awk '/^HTTP\//{code=$2} END{print code}')"
	rm -f "$header_file"
}

translated() {
	printf '%s' "$HTTP_BODY" | grep -oE "data-i18n=\"$1\">[^<]*" | sed "s/data-i18n=\"$1\">//"
}

plural_for() {
	printf '%s' "$HTTP_BODY" | grep -oE "data-plural=\"$1\">[^<]*" | sed "s/data-plural=\"$1\">//"
}

locale_tag() {
	printf '%s' "$HTTP_BODY" | grep -oE 'data-tag="[a-z-]+"' | sed 's/data-tag="//;s/"//'
}

locale_direction() {
	printf '%s' "$HTTP_BODY" | grep -oE 'data-dir="[a-z]+"' | sed 's/data-dir="//;s/"//'
}

check_the_default_locale() {
	fetch_in_locale ""
	assert_equal "the page answers 200" "200" "$HTTP_STATUS"
	assert_equal "the default locale is English" "en" "$(locale_tag)"
	assert_equal "an untranslated message falls back to the template text" "About Meridian" "$(translated about)"
	assert_equal "English reads left to right" "ltr" "$(locale_direction)"
}

check_accept_language_is_honoured() {
	fetch_in_locale "fr"
	assert_equal "a French visitor gets French" "fr" "$(locale_tag)"
	assert_equal "the heading is translated" "À propos de Meridian" "$(translated about)"
	assert_equal "a second message is translated too" "Suivre un envoi" "$(translated track)"

	fetch_in_locale "ar"
	assert_equal "an Arabic visitor gets Arabic" "ar" "$(locale_tag)"
	assert_equal "the Arabic heading is translated" "عن ميريديان" "$(translated about)"

	fetch_in_locale "fr-CA,fr;q=0.9,en;q=0.8"
	assert_equal "a regional tag falls back to its base language" "fr" "$(locale_tag)"

	fetch_in_locale "de,ja;q=0.9"
	assert_equal "an unsupported language falls back to the default" "en" "$(locale_tag)"
}

check_right_to_left() {
	fetch_in_locale "ar"
	assert_equal "Arabic reads right to left" "rtl" "$(locale_direction)"

	fetch_in_locale "fr"
	assert_equal "French reads left to right" "ltr" "$(locale_direction)"
}

check_plural_rules() {
	fetch_in_locale ""
	assert_equal "English uses the plural for none" "0 shipments on this lane" "$(plural_for 0)"
	assert_equal "English uses the singular for one" "1 shipment on this lane" "$(plural_for 1)"
	assert_equal "English uses the plural for two" "2 shipments on this lane" "$(plural_for 2)"

	fetch_in_locale "fr"
	assert_equal "French treats none as singular" "0 envoi sur cette ligne" "$(plural_for 0)"
	assert_equal "French treats one as singular" "1 envoi sur cette ligne" "$(plural_for 1)"
	assert_equal "French turns plural at two" "2 envois sur cette ligne" "$(plural_for 2)"
	note "the French rule" "nplurals=2; plural=(n > 1), so zero is singular, unlike English"

	fetch_in_locale "ar"
	assert_equal "Arabic has a form for none" "لا شحنات على هذا الخط" "$(plural_for 0)"
	assert_equal "Arabic has a form for one" "شحنة واحدة على هذا الخط" "$(plural_for 1)"
	assert_equal "Arabic uses the dual for exactly two" "شحنتان على هذا الخط" "$(plural_for 2)"
	assert_equal "Arabic uses the few form for seven" "7 شحنات على هذا الخط" "$(plural_for 7)"
	note "the Arabic rule" "six forms, and two is a dual rather than a plural"
}

check_the_locale_picker() {
	fetch_in_locale ""
	local offered
	offered="$(printf '%s' "$HTTP_BODY" | grep -oE 'data-locale="[a-z]+"' | sed 's/data-locale="//;s/"//' | tr '\n' ' ' | sed 's/ $//')"
	assert_equal "the picker offers every supported locale" "en fr ar" "$offered"
}

check_switching_locale_sticks() {
	http_reset_session
	http_get "/locale?locale=fr&next=/about"

	assert_equal "the switch redirects" "303" "$HTTP_STATUS"

	local cookie
	cookie="$(http_header_value Set-Cookie)"
	case "$cookie" in
	*locale=fr*)
		check_passed "the switch writes a locale cookie"
		;;
	*)
		check_failed "the switch writes a locale cookie" "Set-Cookie: ${cookie:-none}"
		;;
	esac

	http_get "/about"
	assert_equal "the choice sticks without an Accept-Language header" "fr" "$(locale_tag)"

	http_get "/locale?locale=zz&next=/about"
	http_get "/about"
	assert_equal "an unsupported locale is refused and the choice is kept" "fr" "$(locale_tag)"

	http_get "/locale?locale=en&next=/about"
	http_get "/about"
	assert_equal "switching back to English works" "en" "$(locale_tag)"
}

check_the_switch_returns_the_visitor_to_the_page_they_were_on() {
	http_reset_session
	http_get "/locale?locale=fr&next=/about"

	local target
	target="$(http_header_value Location)"

	case "$target" in
	*/about*)
		check_passed "a switch by link returns the visitor to the page named in next"
		;;
	*)
		check_failed "a switch by link returns the visitor to the page named in next" \
			"it redirected to ${target:-nothing} instead of /about
core/i18n/detector.go reads the locale from the query as well as the form, but reads next from
r.PostForm only, so a GET picker always lands on the fallback rather than the page being read
guide/34-internationalisation.md documents the GET form of the picker, and CLAUDE.md records that
the picker was changed from a POST form to a link because a form in a shared partial made every
page uncacheable, so the link is the shape a cacheable site has to use"
		note "next on a GET switch" \
			"reading next from r.URL.Query() as well would make the two forms of the picker behave alike"
		;;
	esac
}

check_the_page_cache_keeps_locales_apart() {
	rm -rf "$EXAMPLE_DIR/cache"

	local locale
	for locale in en fr ar; do
		fetch_in_locale "$locale"
		assert_equal "the first $locale request is a miss" "MISS" "$(http_header_value X-Cache)"
	done

	for locale in en fr ar; do
		fetch_in_locale "$locale"
		assert_equal "the second $locale request is a hit" "HIT" "$(http_header_value X-Cache)"
	done

	fetch_in_locale "en"
	assert_equal "an English hit still returns English" "About Meridian" "$(translated about)"

	fetch_in_locale "fr"
	assert_equal "a French hit still returns French" "À propos de Meridian" "$(translated about)"

	fetch_in_locale "ar"
	assert_equal "an Arabic hit still returns Arabic" "عن ميريديان" "$(translated about)"

	note "cache keys" "the page cache varies by locale, so one language's page never reaches another"
}

check_the_message_commands() {
	app_run_command checkmessages --locale=fr
	assert_equal "checkmessages exits cleanly for a complete catalog" "0" "$CAPTURED_STATUS"

	case "$APP_OUTPUT" in
	*"fr"*)
		check_passed "checkmessages reports on the locale it was given"
		;;
	*)
		check_failed "checkmessages reports on the locale it was given" "$(truncated_output "$APP_OUTPUT")"
		;;
	esac
}

check_the_site_is_there
write_the_catalogs
boot_the_server
check_the_default_locale
check_accept_language_is_honoured
check_right_to_left
check_plural_rules
check_the_locale_picker
check_switching_locale_sticks
check_the_switch_returns_the_visitor_to_the_page_they_were_on
check_the_page_cache_keeps_locales_apart
check_the_message_commands

server_stop
if port_is_free; then
	check_passed "the port is free when the chunk ends"
else
	check_failed "the port is free when the chunk ends" "held by $(port_listener_pids | tr '\n' ' ')"
fi

chunk_end
