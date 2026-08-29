#!/usr/bin/env bash

. "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib/bootstrap.sh"
. "$E2E_DIR/lib/appsource.sh"
. "$E2E_DIR/lib/server.sh"
. "$E2E_DIR/lib/http.sh"
. "$E2E_DIR/lib/admin.sh"

chunk_begin "16" "Uploads"

trap upload_cleanup EXIT

FIXTURES="$E2E_WORK_DIR/uploads"
SHIPMENT_ID=""
STORED_KEY=""

upload_cleanup() {
	server_cleanup
	rm -rf "$FIXTURES"
}

check_the_shipment_is_there() {
	if app_table_exists shipments && [ "$(app_row_count shipments)" -ge 1 ]; then
		return 0
	fi
	check_failed "a shipment exists to attach a document to" "run chunks 05 to 15 first"
	chunk_end
}

build_the_fixtures() {
	rm -rf "$FIXTURES"
	mkdir -p "$FIXTURES"

	printf '%%PDF-1.4\n1 0 obj\n<< /Type /Catalog >>\nendobj\ntrailer\n<< /Root 1 0 R >>\n%%%%EOF\n' \
		>"$FIXTURES/bill.pdf"
	printf '%%PDF-1.4\n' >"$FIXTURES/big.pdf"
	dd if=/dev/zero bs=1024 count=1200 >>"$FIXTURES/big.pdf" 2>/dev/null
	printf '#!/bin/sh\necho pwned\n' >"$FIXTURES/fake.pdf"
	: >"$FIXTURES/empty.pdf"

	assert_file_exists "a valid pdf fixture exists" "$FIXTURES/bill.pdf"
	if [ "$(wc -c <"$FIXTURES/big.pdf")" -gt 1048576 ]; then
		check_passed "the oversize fixture is larger than the configured limit"
	else
		check_failed "the oversize fixture is larger than the configured limit" \
			"$(wc -c <"$FIXTURES/big.pdf") bytes"
	fi
}

turn_uploads_on() {
	app_write_settings 1 0
	app_write_shipment_model 3 2 40 1
	app_write_admin_resources 12
	app_write_main 15

	assert_output_contains "uploads are enabled in settings" "s.Uploads.Enabled = true" \
		cat "$EXAMPLE_DIR/settings.go"
	assert_output_contains "the document column asks for a path and an accept" \
		'coyote:"path=shipments/documents,accept=application/pdf"' cat "$EXAMPLE_DIR/models_shipment.go"

	cd "$EXAMPLE_DIR"
	assert_succeeds "the project compiles with an upload column" go build ./...
	cd "$E2E_ROOT"
}

migrate_the_document_column() {
	app_run_command makemigrations --name=shipment_document
	assert_equal "makemigrations exits cleanly" "0" "$CAPTURED_STATUS"

	local migration
	migration="$(app_migration_for_name shipment_document)"
	assert_file_exists "a migration for the document column is written" "$migration"

	if [ -n "$migration" ]; then
		assert_output_contains "an upload reference is an ordinary string column" \
			'{Name: "document", Kind: model.KindString, Size: 200}' cat "$migration"
	fi

	app_run_command migrate
	assert_equal "migrate applies the document column" "0" "$CAPTURED_STATUS"

	local columns
	columns="$(app_table_columns shipments | tr '\n' ' ')"
	case "$columns" in
	*document*)
		check_passed "the document column exists in the database"
		;;
	*)
		check_failed "the document column exists in the database" "columns: $columns"
		chunk_end
		;;
	esac

	app_sqlite_query "UPDATE shipments SET document = NULL;"
	rm -rf "$EXAMPLE_DIR/media"
}

boot_the_server() {
	if ! port_is_free; then
		force_free_the_port
	fi
	server_start
	if server_wait_for_http; then
		check_passed "the application serves with uploads enabled"
		return 0
	fi
	check_failed "the application serves with uploads enabled" "$(tail -20 "$SERVER_LOG")"
	chunk_end
}

attach_document() {
	local file="$1" filename="$2"
	local token

	http_get "/admin/shipments/$SHIPMENT_ID"
	token="$(http_csrf_token)"

	local header_file
	header_file="$(mktemp "${TMPDIR:-/tmp}/coyote-e2e-up-XXXXXX")"

	set +e
	HTTP_BODY="$(curl -sS --max-time 30 -b "$COOKIE_JAR" -c "$COOKIE_JAR" -D "$header_file" \
		-F "csrf_token=$token" -F "reference=MRF-000001" -F "customer_id=cus-nordwind" \
		-F "origin_id=prt-rtm" -F "destination_id=prt-sin" -F "status=booked" \
		-F "document=@$file;filename=$filename" \
		"$E2E_BASE_URL/admin/shipments/$SHIPMENT_ID" 2>/dev/null)"
	set -e

	HTTP_HEADERS="$(cat "$header_file")"
	HTTP_STATUS="$(printf '%s' "$HTTP_HEADERS" | awk '/^HTTP\//{code=$2} END{print code}')"
	rm -f "$header_file"
}

stored_document() {
	app_sqlite_query "SELECT ifnull(document,'') FROM shipments WHERE id='$SHIPMENT_ID';"
}

sign_in_and_find_a_shipment() {
	if ! admin_login; then
		check_failed "the superadmin can sign in" "status $HTTP_STATUS"
		chunk_end
	fi

	SHIPMENT_ID="$(app_sqlite_query "SELECT id FROM shipments WHERE reference='MRF-000001';")"
	if [ -z "$SHIPMENT_ID" ]; then
		check_failed "a shipment is available" "no shipment with reference MRF-000001"
		chunk_end
	fi
}

check_the_admin_renders_a_file_input() {
	http_get "/admin/shipments/$SHIPMENT_ID"
	assert_equal "the shipment form answers 200" "200" "$HTTP_STATUS"

	case "$HTTP_BODY" in
	*'type="file" name="document"'*)
		check_passed "an upload column renders a file input with no extra work"
		;;
	*)
		check_failed "an upload column renders a file input with no extra work" \
			"no file input for document on the form"
		;;
	esac

	case "$HTTP_BODY" in
	*'accept="application/pdf"'*)
		check_passed "the accept from the struct tag reaches the input"
		;;
	*)
		check_failed "the accept from the struct tag reaches the input" "no accept attribute"
		;;
	esac
}

check_a_valid_upload_is_stored() {
	attach_document "$FIXTURES/bill.pdf" "bill-of-lading.pdf"

	assert_equal "attaching a pdf redirects" "303" "$HTTP_STATUS"

	STORED_KEY="$(stored_document)"
	if [ -z "$STORED_KEY" ]; then
		check_failed "the reference was written to the row" "the document column is still empty"
		chunk_end
	fi
	check_passed "the reference was written to the row"

	case "$STORED_KEY" in
	shipments/documents/*)
		check_passed "the file went to the path the struct tag asked for"
		;;
	*)
		check_failed "the file went to the path the struct tag asked for" "stored at $STORED_KEY"
		;;
	esac

	case "$STORED_KEY" in
	*bill-of-lading*)
		check_failed "the key is the content hash, not the name the client sent" "stored at $STORED_KEY"
		;;
	*.pdf)
		check_passed "the key is the content hash, not the name the client sent"
		;;
	*)
		check_failed "the extension comes from the sniffed type" "stored at $STORED_KEY"
		;;
	esac

	assert_file_exists "the bytes are on disk under that key" "$EXAMPLE_DIR/media/$STORED_KEY"
	assert_equal "nothing was left behind in staging" "0" \
		"$(find "$EXAMPLE_DIR/media/staged" -type f 2>/dev/null | wc -l | tr -d ' ')"
}

check_the_refusals() {
	local before
	before="$(stored_document)"

	attach_document "$FIXTURES/big.pdf" "big.pdf"
	assert_equal "a file over the size limit is refused" "400" "$HTTP_STATUS"
	assert_equal "the oversize attempt left the stored file alone" "$before" "$(stored_document)"

	attach_document "$FIXTURES/fake.pdf" "fake.pdf"
	assert_equal "a shell script named .pdf is refused" "400" "$HTTP_STATUS"
	assert_equal "the spoofed type left the stored file alone" "$before" "$(stored_document)"

	attach_document "$FIXTURES/empty.pdf" "empty.pdf"
	assert_equal "an empty file is refused" "400" "$HTTP_STATUS"
	assert_equal "the empty attempt left the stored file alone" "$before" "$(stored_document)"

	note "why the spoof fails" \
		"the declared type is ignored and the first bytes are sniffed, so a script named .pdf never lands"
}

check_a_traversal_filename_cannot_escape() {
	attach_document "$FIXTURES/bill.pdf" "../../etc/passwd.pdf"

	assert_equal "a traversal filename does not break the upload" "303" "$HTTP_STATUS"
	assert_equal "it stored under the same content hash as before" "$STORED_KEY" "$(stored_document)"

	assert_equal "no file was written outside the media directory" "0" \
		"$(find "$EXAMPLE_DIR" -maxdepth 2 -name 'passwd*' 2>/dev/null | wc -l | tr -d ' ')"
	assert_equal "no file inside media carries the attacker's name" "0" \
		"$(find "$EXAMPLE_DIR/media" -name 'passwd*' 2>/dev/null | wc -l | tr -d ' ')"

	note "the filename" "kept only for display; the path is derived from the hash and the sniffed type"
}

check_the_file_is_served() {
	assert_http_status "the stored file is served" "200" "/media/$STORED_KEY"

	http_get "/media/$STORED_KEY"
	assert_equal "it is served as a pdf" "application/pdf" "$(http_header_value Content-Type)"
	assert_equal "sniffing is turned off for user content" "nosniff" \
		"$(http_header_value X-Content-Type-Options)"

	case "$(http_header_value Cache-Control)" in
	*immutable*)
		check_passed "a content addressed file may be cached forever"
		;;
	*)
		check_failed "a content addressed file may be cached forever" \
			"Cache-Control: $(http_header_value Cache-Control)"
		;;
	esac
}

check_the_reserved_areas_are_not_served() {
	assert_http_status "staging is not reachable over HTTP" "404" "/media/staged/$STORED_KEY"
	assert_http_status "the trash is not reachable over HTTP" "404" "/media/trash/$STORED_KEY"
	assert_http_status "a path climbing out of media is refused" "404" "/media/../settings.go"
	assert_http_status "an encoded climb is refused too" "404" "/media/..%2Fsettings.go"
	assert_http_status "an unknown key answers 404" "404" "/media/shipments/documents/00/00/nope.pdf"
}

check_private_files_need_a_signature() {
	server_stop
	app_write_settings 1 1

	cd "$EXAMPLE_DIR"
	if ! go build ./... >/dev/null 2>&1; then
		cd "$E2E_ROOT"
		check_failed "the project compiles with private uploads" "build failed"
		return
	fi
	cd "$E2E_ROOT"
	check_passed "the project compiles with private uploads"

	if ! port_is_free; then
		force_free_the_port
	fi
	server_start
	if ! server_wait_for_http; then
		check_failed "the application serves with private uploads" "$(tail -20 "$SERVER_LOG")"
		return
	fi

	assert_http_status "an unsigned request is refused when files are private" "403" "/media/$STORED_KEY"

	if ! admin_login; then
		check_failed "the superadmin can sign in again" "status $HTTP_STATUS"
		return
	fi

	http_get "/admin/shipments/$SHIPMENT_ID"
	local signed
	signed="$(printf '%s' "$HTTP_BODY" | grep -oE '/media/[^"]*' | head -1 | sed 's/&amp;/\&/g')"

	case "$signed" in
	*signature=*)
		check_passed "the admin links to a signed url"
		;;
	*)
		check_failed "the admin links to a signed url" "link was ${signed:-none}"
		return
		;;
	esac

	case "$signed" in
	*expires=*)
		check_passed "the signed url carries an expiry"
		;;
	*)
		check_failed "the signed url carries an expiry" "link was $signed"
		;;
	esac

	assert_http_status "the signed url is served" "200" "$signed"

	local base
	base="${signed%\?*}"
	assert_http_status "a tampered signature is refused" "403" \
		"$base?expires=4102444800&signature=tampered"
	assert_http_status "an expired signature is refused" "403" \
		"$base?expires=1000000000&signature=$(printf '%s' "$signed" | sed 's/.*signature=//')"
	assert_http_status "dropping the query string is refused" "403" "$base"
}

restore_public_uploads() {
	server_stop
	app_write_settings 1 0
	cd "$EXAMPLE_DIR"
	assert_succeeds "the project builds again with public uploads" go build ./...
	cd "$E2E_ROOT"
}

check_the_shipment_is_there
build_the_fixtures
turn_uploads_on
migrate_the_document_column
boot_the_server
sign_in_and_find_a_shipment
check_the_admin_renders_a_file_input
check_a_valid_upload_is_stored
check_the_refusals
check_a_traversal_filename_cannot_escape
check_the_file_is_served
check_the_reserved_areas_are_not_served
check_private_files_need_a_signature
restore_public_uploads

if port_is_free; then
	check_passed "the port is free when the chunk ends"
else
	check_failed "the port is free when the chunk ends" "held by $(port_listener_pids | tr '\n' ' ')"
fi

chunk_end
