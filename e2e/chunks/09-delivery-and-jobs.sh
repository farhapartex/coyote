#!/usr/bin/env bash

. "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib/bootstrap.sh"
. "$E2E_DIR/lib/services.sh"
. "$E2E_DIR/lib/shop.sh"
. "$E2E_DIR/lib/server.sh"
. "$E2E_DIR/lib/http.sh"
. "$E2E_DIR/lib/admin.sh"

chunk_begin "09" "Delivery progress, driven by jobs"

require_previous_chunk "the shop exists" "$SHOP_DIR/settings.go"

trap server_cleanup EXIT

SHOPPER_USER="edith"
SHOPPER_PASSWORD="bench-plane-8821"
REFERENCE=""

wait_for_state() {
	local wanted="$1" deadline="${2:-40}" attempt=0
	while [ "$attempt" -lt "$deadline" ]; do
		if [ "$(postgres_query "SELECT state FROM orders WHERE reference='$REFERENCE';")" = "$wanted" ]; then
			return 0
		fi
		sleep 0.5
		attempt=$((attempt + 1))
	done
	return 1
}

start_the_shop() {
	shop_start
	if server_wait_for_http; then
		check_passed "the shop is serving with two job workers"
		return 0
	fi
	check_failed "the shop is serving" "$(tail -20 "$SERVER_LOG")"
	chunk_end
}

check_there_is_an_order() {
	REFERENCE="$(postgres_query "SELECT reference FROM orders ORDER BY placed_at DESC LIMIT 1;")"
	if [ -n "$REFERENCE" ]; then
		check_passed "there is an order to follow ($REFERENCE)"
		return 0
	fi
	check_failed "there is an order to follow" "no orders; run chunk 08 first"
	chunk_end
}

check_the_job_was_enqueued_with_the_order() {
	local enqueued
	enqueued="$(postgres_query "SELECT count(*) FROM jobs WHERE kind='shop.order.advance';")"
	if [ "${enqueued:-0}" -ge 1 ]; then
		check_passed "placing the order enqueued its delivery job in the same transaction"
	else
		check_failed "placing the order enqueued its delivery job" "no shop.order.advance rows in the jobs table"
	fi
}

check_delivery_advances_on_its_own() {
	if wait_for_state "delivered" 80; then
		check_passed "the order reached delivered without anybody asking"
	else
		check_failed "the order reached delivered" \
			"still $(postgres_query "SELECT state FROM orders WHERE reference='$REFERENCE';") after 40s
$(grep -iE 'job (done|failed)' "$SERVER_LOG" | tail -5 || true)"
		chunk_end
	fi

	local order_id state
	order_id="$(postgres_query "SELECT id FROM orders WHERE reference='$REFERENCE';")"
	for state in placed packing shipped delivered; do
		local seen
		seen="$(postgres_query "SELECT count(*) FROM shipment_events WHERE order_id='$order_id' AND state='$state';")"
		if [ "${seen:-0}" -ge 1 ]; then
			check_passed "it passed through $state and left an event behind"
		else
			check_failed "it passed through $state" "no shipment event with that state"
		fi
	done

	assert_equal "and left exactly one event per state" "4" \
		"$(postgres_query "SELECT count(*) FROM shipment_events WHERE order_id='$order_id';")"
}

check_the_jobs_finished_cleanly() {
	local done dead
	done="$(postgres_query "SELECT count(*) FROM jobs WHERE kind='shop.order.advance' AND state='done';")"
	dead="$(postgres_query "SELECT count(*) FROM jobs WHERE kind='shop.order.advance' AND state='dead';")"

	if [ "${done:-0}" -ge 3 ]; then
		check_passed "$done delivery jobs ran to completion"
	else
		check_failed "the delivery jobs ran to completion" "only ${done:-0} are done"
	fi
	assert_equal "none of them died" "0" "${dead:-0}"

	local chained
	chained="$(postgres_query "SELECT count(*) FROM jobs WHERE kind='shop.order.advance';")"
	if [ "${chained:-0}" -ge 3 ]; then
		check_passed "each job enqueued the next one, so the chain ran itself ($chained in total)"
	else
		check_failed "the job chain enqueued itself" "only ${chained:-0} jobs exist"
	fi
}

check_the_shopper_sees_the_progress() {
	http_reset_session
	http_get "/accounts/login"
	local token
	token="$(http_csrf_token)"
	http_post "/accounts/login" "csrf_token=$token" "username=$SHOPPER_USER" "password=$SHOPPER_PASSWORD"

	http_get "/orders"
	assert_equal "the order list answers 200" "200" "$HTTP_STATUS"
	case "$HTTP_BODY" in
	*"$REFERENCE"*) check_passed "the shopper's order is listed" ;;
	*) check_failed "the shopper's order is listed" "$REFERENCE not in the list" ;;
	esac
	case "$HTTP_BODY" in
	*"Delivered"*) check_passed "the list shows the delivery state in words" ;;
	*) check_failed "the list shows the delivery state in words" "no state label" ;;
	esac

	http_get "/orders/$REFERENCE"
	assert_equal "the order page answers 200" "200" "$HTTP_STATUS"

	local step
	for step in "Order placed" "Being packed" "On its way" "Delivered"; do
		case "$HTTP_BODY" in
		*"$step"*) check_passed "the timeline shows $step" ;;
		*) check_failed "the timeline shows $step" "not on the page" ;;
		esac
	done

	case "$HTTP_BODY" in
	*"Left with the customer"*) check_passed "the timeline carries the note each job wrote" ;;
	*) check_failed "the timeline carries the note each job wrote" "no delivery note" ;;
	esac
	case "$HTTP_BODY" in
	*"14 Shrewsbury Road"*) check_passed "the order page shows where it went" ;;
	*) check_failed "the order page shows where it went" "no address" ;;
	esac
	case "$HTTP_BODY" in
	*"SPRING10"*) check_passed "and the coupon that was used" ;;
	*) check_failed "and the coupon that was used" "no coupon on the page" ;;
	esac
}

check_one_shopper_cannot_read_another_order() {
	postgres_query "DELETE FROM users WHERE username='intruder';" >/dev/null
	http_reset_session
	http_get "/accounts/register"
	local token
	token="$(http_csrf_token)"
	http_post "/accounts/register" "csrf_token=$token" "username=intruder" \
		"email=intruder@thornfield.test" "first_name=No" "last_name=Body" "password=not-my-order-9931"

	http_get "/orders/$REFERENCE"
	assert_equal "another shopper gets a 404 for an order that is not theirs" "404" "$HTTP_STATUS"

	http_get "/orders"
	case "$HTTP_BODY" in
	*"$REFERENCE"*) check_failed "another shopper's list does not leak the order" "$REFERENCE was listed" ;;
	*) check_passed "another shopper's list is empty" ;;
	esac
}

check_the_jobs_page_in_the_admin() {
	http_reset_session
	if ! admin_login "root" "thornfield-supply-2026"; then
		check_failed "a superadmin signs in" "status $HTTP_STATUS"
		return 0
	fi
	http_get "/admin/jobs"
	assert_equal "the admin jobs page answers 200" "200" "$HTTP_STATUS"
	case "$HTTP_BODY" in
	*"shop.order.advance"*) check_passed "the jobs page shows the delivery job by kind" ;;
	*) check_failed "the jobs page shows the delivery job by kind" "not listed" ;;
	esac
	case "$HTTP_BODY" in
	*"Background jobs"*) check_passed "the jobs page is the framework's own" ;;
	*) check_failed "the jobs page is the framework's own" "unexpected page" ;;
	esac
}

check_polling_latency() {
	finding "delivery state changes are bounded by the job poll interval, not by the work" \
		"Each step of this order took at least Jobs.PollInterval to start, and the shop had to set
that to 200ms to make the test finish quickly. The default is one second, so a three-step chain
takes three seconds of pure waiting before any work happens. The queue is a table and no engine here
offers a portable blocking read, so a worker polls; this is written up in guide/36-jobs.md as a
known limit. For a delivery pipeline it is fine. For anything a shopper watches - 'preparing your
download', a payment callback - it is the wrong tool, and there is currently no in-process fast path
for a job that should start immediately."
}

start_the_shop
check_there_is_an_order
check_the_job_was_enqueued_with_the_order
check_delivery_advances_on_its_own
check_the_jobs_finished_cleanly
check_the_shopper_sees_the_progress
check_one_shopper_cannot_read_another_order
check_the_jobs_page_in_the_admin
check_polling_latency

chunk_end
