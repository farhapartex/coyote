#!/usr/bin/env bash

. "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib/bootstrap.sh"
. "$E2E_DIR/lib/appsource.sh"
. "$E2E_DIR/lib/server.sh"
. "$E2E_DIR/lib/http.sh"

chunk_begin "20" "Email"

trap server_cleanup EXIT

OUTBOX="$EXAMPLE_DIR/mail"

check_the_quote_form_is_there() {
	if [ -f "$EXAMPLE_DIR/handlers_quote.go" ]; then
		return 0
	fi
	check_failed "the quote form is in place" "run chunk 15 first"
	chunk_end
}

turn_email_on() {
	app_write_quote_handlers 1
	app_write_locales
	app_write_public_templates 19
	app_write_settings 1 0 25 1 3 1 1
	app_write_main 20

	assert_file_exists "the application's own mailer is written" "$EXAMPLE_DIR/mailer.go"
	assert_output_contains "the file backend is configured" "Backend: settings.EmailToFile" \
		cat "$EXAMPLE_DIR/settings.go"
	assert_output_contains "the application opens the backend itself" "openMailer(a)" \
		cat "$EXAMPLE_DIR/main.go"

	cd "$EXAMPLE_DIR"
	assert_succeeds "the project compiles with a mailer" go build ./...
	assert_succeeds "the mailer passes go vet" go vet ./...
	cd "$E2E_ROOT"

	rm -rf "$OUTBOX"
}

boot_the_server() {
	if ! port_is_free; then
		force_free_the_port
	fi
	server_start
	if server_wait_for_http; then
		check_passed "the application serves with email configured"
		return 0
	fi
	check_failed "the application serves with email configured" "$(tail -20 "$SERVER_LOG")"
	chunk_end
}

messages_written() {
	find "$OUTBOX" -name '*.eml' -type f 2>/dev/null | wc -l | tr -d ' '
}

newest_message() {
	find "$OUTBOX" -name '*.eml' -type f 2>/dev/null | sort | tail -1
}

submit_quote() {
	http_get "/quote"
	local token
	token="$(http_csrf_token)"
	http_post "/quote" "csrf_token=$token" "$@"
}

check_a_quote_sends_one_confirmation() {
	assert_equal "the outbox starts empty" "0" "$(messages_written)"

	submit_quote "company=Acme Trading BV" "email=Buyer@ACME.example" \
		"origin_code=NLRTM" "service=ocean" "containers=4"

	assert_equal "the submission redirects" "303" "$HTTP_STATUS"
	assert_equal "exactly one message was written" "1" "$(messages_written)"

	local path
	path="$(newest_message)"
	case "$path" in
	*.eml)
		check_passed "the message is written as an .eml a mail client can open"
		;;
	*)
		check_failed "the message is written as an .eml a mail client can open" "wrote ${path:-nothing}"
		;;
	esac

	local mode
	mode="$(stat -f '%A' "$path" 2>/dev/null || stat -c '%a' "$path" 2>/dev/null)"
	assert_equal "the message is readable only by its owner" "600" "$mode"
}

check_the_message_is_well_formed() {
	local path
	path="$(newest_message)"
	if [ -z "$path" ]; then
		check_skipped "the message carries the headers it should" "nothing was written"
		return
	fi

	local expected
	for expected in \
		"From: quotes@meridian.test" \
		"To: buyer@acme.example" \
		"Subject: Quote request from Acme Trading BV" \
		"MIME-Version: 1.0" \
		"Content-Type: multipart/alternative"; do
		assert_output_contains "the message carries ${expected%%:*}" "$expected" cat "$path"
	done

	assert_output_contains "the address the visitor typed was lowercased" \
		"buyer@acme.example" cat "$path"

	case "$(cat "$path")" in
	*"Buyer@ACME.example"*)
		check_failed "the address is normalised, not repeated as typed" "the raw casing is in the file"
		;;
	*)
		check_passed "the address is normalised, not repeated as typed"
		;;
	esac

	assert_output_contains "the text part is present" "Content-Type: text/plain" cat "$path"
	assert_output_contains "the html part is present" "Content-Type: text/html" cat "$path"
	assert_output_contains "the body carries the request" "4 container" cat "$path"
}

check_an_independent_parser_accepts_it() {
	local path
	path="$(newest_message)"

	if ! command -v python3 >/dev/null 2>&1; then
		check_skipped "an independent parser reads the message back" "python3 is not installed"
		return
	fi
	if [ -z "$path" ]; then
		check_skipped "an independent parser reads the message back" "nothing was written"
		return
	fi

	local parsed
	parsed="$(python3 -c "
import email, sys
from email import policy
msg = email.message_from_binary_file(open(sys.argv[1], 'rb'), policy=policy.default)
parts = [p.get_content_type() for p in msg.walk() if p.get_content_maintype() != 'multipart']
print('%s|%s|%s|%s' % (msg['From'], msg['To'], msg['Subject'], ','.join(parts)))
" "$path" 2>/dev/null || true)"

	if [ -z "$parsed" ]; then
		check_failed "an independent parser reads the message back" "python could not parse $path"
		return
	fi
	check_passed "an independent parser reads the message back"

	assert_equal "every header survives a round trip through another implementation" \
		"quotes@meridian.test|buyer@acme.example|Quote request from Acme Trading BV|text/plain,text/html" \
		"$parsed"
}

check_a_header_injection_is_refused() {
	local before
	before="$(messages_written)"

	submit_quote "$(printf 'company=Acme\r\nBcc: attacker@evil.test')" "email=x@example.test" \
		"origin_code=NLRTM" "service=ocean" "containers=1"

	assert_equal "the visitor still gets their quote saved" "303" "$HTTP_STATUS"
	assert_equal "no message was written for the injected value" "$before" "$(messages_written)"

	local leaked
	leaked="$( (grep -rl "attacker@evil.test" "$OUTBOX" 2>/dev/null || true) | wc -l | tr -d ' ')"
	assert_equal "no message anywhere carries the attacker's address" "0" "$leaked"

	local bcc
	bcc="$( (grep -ril "^Bcc:" "$OUTBOX" 2>/dev/null || true) | wc -l | tr -d ' ')"
	assert_equal "no message carries a Bcc header at all" "0" "$bcc"

	if server_log_contains "a header value contains a line break"; then
		check_passed "the refusal names the reason"
	else
		check_failed "the refusal names the reason" "$(tail -5 "$SERVER_LOG")"
	fi

	note "where the guard sits" \
		"the form accepts a newline in a company name, and the mail layer refuses to put it in a header, so the quote is still saved while the message is not sent"
}

check_a_second_quote_writes_a_second_message() {
	local before
	before="$(messages_written)"

	submit_quote "company=Kestrel Trading" "email=ops@kestrel.example" \
		"origin_code=SGSIN" "service=air" "containers=2"

	assert_equal "a second valid quote redirects" "303" "$HTTP_STATUS"
	assert_equal "a second message joins the first" "$((before + 1))" "$(messages_written)"

	local names
	names="$(find "$OUTBOX" -name '*.eml' -type f | xargs -n1 basename | sort -u | wc -l | tr -d ' ')"
	assert_equal "each message gets its own filename" "$(messages_written)" "$names"

	assert_output_contains "the newest message is the second one" \
		"Subject: Quote request from Kestrel Trading" cat "$(newest_message)"
}

check_an_unconfigured_project_sends_nothing() {
	server_stop

	app_write_settings 1 0 25 1 3 1 0
	cd "$EXAMPLE_DIR"
	if go build ./... >/dev/null 2>&1; then
		check_passed "a project with no email backend still compiles"
	else
		check_failed "a project with no email backend still compiles" "build failed"
		app_write_settings 1 0 25 1 3 1 1
		cd "$E2E_ROOT"
		return
	fi
	cd "$E2E_ROOT"

	app_run_command start "--port=$E2E_PORT"

	if [ "$CAPTURED_STATUS" -ne 0 ]; then
		check_passed "an application that opens a backend it never configured is told so"
	else
		check_failed "an application that opens a backend it never configured is told so" \
			"the command exited 0"
	fi

	case "$APP_OUTPUT" in
	*"no email backend is configured"*)
		check_passed "the message says no backend is configured"
		;;
	*)
		check_failed "the message says no backend is configured" "$(truncated_output "$APP_OUTPUT")"
		;;
	esac

	note "sending nothing" \
		"an empty Backend is a valid configuration; Open reports ErrNotConfigured only when something asks for a backend"

	app_write_settings 1 0 25 1 3 1 1
	cd "$EXAMPLE_DIR"
	assert_succeeds "the project builds again with the file backend" go build ./...
	cd "$E2E_ROOT"
}

check_the_quote_form_is_there
turn_email_on
boot_the_server
check_a_quote_sends_one_confirmation
check_the_message_is_well_formed
check_an_independent_parser_accepts_it
check_a_header_injection_is_refused
check_a_second_quote_writes_a_second_message
check_an_unconfigured_project_sends_nothing

if port_is_free; then
	check_passed "the port is free when the chunk ends"
else
	check_failed "the port is free when the chunk ends" "held by $(port_listener_pids | tr '\n' ' ')"
fi

chunk_end
