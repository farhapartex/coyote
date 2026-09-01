#!/usr/bin/env bash

. "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib/bootstrap.sh"
. "$E2E_DIR/lib/appsource.sh"
. "$E2E_DIR/lib/server.sh"
. "$E2E_DIR/lib/http.sh"
. "$E2E_DIR/lib/admin.sh"

chunk_begin "12" "Invoicing"

trap server_cleanup EXIT

INVOICE_MIGRATION=""
STEPS_BEFORE=0
MIGRATION_WAS_NEW=0
ADMIN_NUMBER="INV-E2E-0001"

check_the_ladder_is_ready() {
	if app_table_exists shipments && [ "$(app_row_count shipments)" -ge 3 ]; then
		return 0
	fi
	check_failed "the shipment graph is in place" "run chunks 05 to 11 first"
	chunk_end
}

record_the_state_before() {
	STEPS_BEFORE="$(app_migration_files | wc -l | tr -d ' ')"
	SHIPMENTS_BEFORE="$(app_row_count shipments)"
	CUSTOMERS_BEFORE="$(app_row_count customers)"
	note "state before invoicing" "$STEPS_BEFORE migrations, $SHIPMENTS_BEFORE shipments"
}

generate_the_invoicing_migration() {
	local existed=0
	INVOICE_MIGRATION="$(app_migration_for_name invoicing)"
	if [ -n "$INVOICE_MIGRATION" ]; then
		existed=1
	fi
	MIGRATION_WAS_NEW=0
	if [ "$existed" = "0" ]; then
		MIGRATION_WAS_NEW=1
	fi

	app_write_invoice_models
	app_write_admin_resources 12
	app_write_main 12

	assert_file_exists "the invoicing models are written" "$EXAMPLE_DIR/models_invoice.go"

	cd "$EXAMPLE_DIR"
	assert_succeeds "the project compiles with seven models" go build ./...
	cd "$E2E_ROOT"

	app_run_command makemigrations --name=invoicing
	assert_equal "makemigrations exits cleanly" "0" "$CAPTURED_STATUS"

	if [ "$existed" = "1" ]; then
		check_skipped "both invoicing tables go into one migration" "the migration is already present"
	else
		local table
		for table in invoices invoice_lines; do
			case "$APP_OUTPUT" in
			*"create table $table"*)
				check_passed "the migration creates $table"
				;;
			*)
				check_failed "the migration creates $table" "$(truncated_output "$APP_OUTPUT")"
				;;
			esac
		done
	fi

	INVOICE_MIGRATION="$(app_migration_for_name invoicing)"
	assert_file_exists "a migration for invoicing is written" "$INVOICE_MIGRATION"
}

check_the_money_columns_are_integers() {
	if [ ! -f "$INVOICE_MIGRATION" ]; then
		check_skipped "the money columns are whole numbers" "no migration file"
		return
	fi

	local column
	for column in total_cents unit_cents amount_cents; do
		assert_output_contains "$column is an integer column" \
			"{Name: \"$column\", Kind: model.KindInt, Size: 64, NotNull: true}" cat "$INVOICE_MIGRATION"
	done

	assert_output_contains "the invoice number is unique" 'idx_invoices_number' cat "$INVOICE_MIGRATION"

	cd "$EXAMPLE_DIR"
	assert_succeeds "the invoicing migration compiles" go build ./...
	cd "$E2E_ROOT"
}

apply_the_invoicing_migration() {
	app_run_command migrate
	assert_equal "migrate applies the invoicing step" "0" "$CAPTURED_STATUS"

	local table
	for table in invoices invoice_lines; do
		if app_table_exists "$table"; then
			check_passed "the $table table exists"
		else
			check_failed "the $table table exists" "$(truncated_output "$APP_OUTPUT")"
			chunk_end
		fi
	done

	assert_equal "the shipments survived the invoicing step" "$SHIPMENTS_BEFORE" "$(app_row_count shipments)"
	assert_equal "the customers survived the invoicing step" "$CUSTOMERS_BEFORE" "$(app_row_count customers)"
	if [ "$MIGRATION_WAS_NEW" = "1" ]; then
		assert_equal "the ladder gained one step" "$((STEPS_BEFORE + 1))" \
			"$(app_migration_files | wc -l | tr -d ' ')"
	else
		assert_equal "the ladder is unchanged because the step was already there" "$STEPS_BEFORE" \
			"$(app_migration_files | wc -l | tr -d ' ')"
	fi

	local indexes
	indexes="$(app_sqlite_query "SELECT name FROM sqlite_master WHERE type='index' AND tbl_name='invoice_lines';" | tr '\n' ' ')"
	local column
	for column in invoice_id shipment_id; do
		case "$indexes" in
		*"idx_invoice_lines_$column"*)
			check_passed "invoice_lines.$column is indexed"
			;;
		*)
			check_failed "invoice_lines.$column is indexed" "indexes: $indexes"
			;;
		esac
	done
}

check_integer_money_is_exact() {
	app_sqlite_query "DELETE FROM invoice_lines; DELETE FROM invoices;"

	app_sqlite_query "INSERT INTO invoices (id, number, customer_id, currency, total_cents, status, issued_at, created_at, updated_at) VALUES
		('inv-max','INV-MAX','cus-nordwind','EUR',9223372036854775807,'draft',datetime('now'),datetime('now'),datetime('now')),
		('inv-credit','INV-CREDIT','cus-kestrel','EUR',-125075,'credit',datetime('now'),datetime('now'),datetime('now')),
		('inv-zero','INV-ZERO','cus-alpine','EUR',0,'draft',datetime('now'),datetime('now'),datetime('now'));"

	assert_equal "the largest int64 survives exactly" "9223372036854775807" \
		"$(app_sqlite_query "SELECT total_cents FROM invoices WHERE number='INV-MAX';")"
	assert_equal "a negative amount survives exactly" "-125075" \
		"$(app_sqlite_query "SELECT total_cents FROM invoices WHERE number='INV-CREDIT';")"
	assert_equal "zero survives as zero, not NULL" "0" \
		"$(app_sqlite_query "SELECT total_cents FROM invoices WHERE number='INV-ZERO';")"
}

record_how_float_money_is_stored() {
	app_sqlite_query "UPDATE shipments SET declared_value = 18500.50 WHERE reference='MRF-000001';"

	local stored cents
	stored="$(app_sqlite_query "SELECT printf('%.15f', declared_value) FROM shipments WHERE reference='MRF-000001';")"
	cents="$(app_sqlite_query "SELECT CAST(declared_value * 100 AS INTEGER) FROM shipments WHERE reference='MRF-000001';")"

	assert_equal "a float money column still converts to the right number of cents" "1850050" "$cents"
	note "float64 rendering" "sqlite printf renders 18500.50 as $stored, which is a formatting artefact and not a storage error: .5 is exact in binary and the cents recover"

	if grep -q 'minor units' "$E2E_ROOT/guide/10-models.md" 2>/dev/null; then
		check_passed "the guide tells you to keep money in integer minor units"
	else
		check_failed "the guide tells you to keep money in integer minor units" \
			"guide/10-models.md does not say how to store currency, so a reader will reach for float64"
	fi

	if grep -q 'Price *float64' "$E2E_ROOT/guide/10-models.md" 2>/dev/null; then
		check_failed "the guide's model example does not use float64 for money" \
			"guide/10-models.md still shows Price float64, which steers a developer toward floating point money"
	else
		check_passed "the guide's model example does not use float64 for money"
	fi

	note "no decimal kind, by decision" \
		"core/model has string, text, int, float, bool, time, bytes and file; currency belongs in an int64 of minor units, as the invoice lines in this chunk do"
}

seed_an_invoice_with_lines() {
	local shipment
	shipment="$(app_sqlite_query "SELECT id FROM shipments WHERE reference='MRF-000001';")"

	app_sqlite_query "DELETE FROM invoice_lines; DELETE FROM invoices WHERE number='INV-000042';"
	app_sqlite_query "INSERT INTO invoices (id, number, customer_id, currency, total_cents, status, issued_at, created_at, updated_at)
		VALUES ('inv-42','INV-000042','cus-nordwind','EUR',0,'draft',datetime('now'),datetime('now'),datetime('now'));"
	app_sqlite_query "INSERT INTO invoice_lines (id, invoice_id, shipment_id, description, quantity, unit_cents, amount_cents, created_at) VALUES
		('line-1','inv-42','$shipment','Ocean freight Rotterdam to Singapore',1,1850050,1850050,datetime('now')),
		('line-2','inv-42','$shipment','Terminal handling',2,12575,25150,datetime('now')),
		('line-3','inv-42','$shipment','Customs clearance',1,9900,9900,datetime('now'));"
	app_sqlite_query "UPDATE invoices SET total_cents = (SELECT sum(amount_cents) FROM invoice_lines WHERE invoice_id='inv-42') WHERE id='inv-42';"

	assert_equal "the invoice has three lines" "3" \
		"$(app_sqlite_query "SELECT count(*) FROM invoice_lines WHERE invoice_id='inv-42';")"
	assert_equal "the lines sum to the invoice total with no rounding drift" "1885100" \
		"$(app_sqlite_query "SELECT total_cents FROM invoices WHERE id='inv-42';")"
	assert_equal "each line's amount is its quantity times its unit price" "0" \
		"$(app_sqlite_query "SELECT count(*) FROM invoice_lines WHERE amount_cents <> quantity * unit_cents;")"
}

check_the_three_level_graph_resolves() {
	assert_equal "a line resolves through its invoice to a customer" "Nordwind Logistics" \
		"$(app_sqlite_query "SELECT c.name FROM invoice_lines l
			JOIN invoices i ON i.id = l.invoice_id
			JOIN customers c ON c.id = i.customer_id
			WHERE l.id = 'line-1';")"

	assert_equal "a line resolves through its shipment to a port" "Rotterdam" \
		"$(app_sqlite_query "SELECT p.name FROM invoice_lines l
			JOIN shipments s ON s.id = l.shipment_id
			JOIN ports p ON p.id = s.origin_id
			WHERE l.id = 'line-1';")"

	assert_equal "no line points at an invoice that does not exist" "0" \
		"$(app_sqlite_query "SELECT count(*) FROM invoice_lines l LEFT JOIN invoices i ON i.id = l.invoice_id WHERE l.invoice_id IS NOT NULL AND i.id IS NULL;")"
}

check_the_admin_manages_invoicing() {
	if ! port_is_free; then
		force_free_the_port
	fi
	server_start
	if ! server_wait_for_http; then
		check_failed "the application serves with seven managed models" "$(tail -20 "$SERVER_LOG")"
		return
	fi
	check_passed "the application serves with seven managed models"

	if ! admin_login; then
		check_failed "the superadmin can sign in" "status $HTTP_STATUS"
		return
	fi

	assert_http_status "the invoices list answers 200" "200" "/admin/invoices"
	assert_http_status "the invoice lines list answers 200" "200" "/admin/invoice_lines"

	assert_body_contains "the invoices list shows the seeded invoice" "INV-000042" "/admin/invoices"
	assert_body_contains "the invoices list resolves its customer" "Nordwind Logistics" "/admin/invoices"

	http_get "/admin/invoice_lines"
	local label
	for label in "Ocean freight" "INV-000042" "MRF-000001"; do
		case "$HTTP_BODY" in
		*"$label"*)
			check_passed "the invoice lines list resolves $label"
			;;
		*)
			check_failed "the invoice lines list resolves $label" "not shown in the list"
			;;
		esac
	done

	http_get "/admin/invoices/new"
	assert_equal "the invoice create form answers 200" "200" "$HTTP_STATUS"

	case "$HTTP_BODY" in
	*'<select name="customer_id"'*)
		check_passed "the invoice form offers its customer as a relation picker"
		;;
	*)
		check_failed "the invoice form offers its customer as a relation picker" "no select for customer_id"
		;;
	esac
}

check_money_written_through_the_admin_is_exact() {
	app_sqlite_query "DELETE FROM invoices WHERE number='$ADMIN_NUMBER';"

	http_get "/admin/invoices/new"
	admin_submit "/admin/invoices/new" \
		"number=$ADMIN_NUMBER" "customer_id=cus-kestrel" "currency=EUR" \
		"total_cents=1885100" "status=issued" "issued_at=2026-03-04T09:30"

	assert_equal "creating an invoice through the admin redirects" "303" "$HTTP_STATUS"
	assert_equal "the amount was stored to the cent" "1885100" \
		"$(app_sqlite_query "SELECT total_cents FROM invoices WHERE number='$ADMIN_NUMBER';")"

	local record_id
	record_id="$(app_sqlite_query "SELECT id FROM invoices WHERE number='$ADMIN_NUMBER';")"
	if [ -z "$record_id" ]; then
		check_skipped "editing an amount keeps it exact" "the invoice was not created"
		server_stop
		return
	fi

	http_get "/admin/invoices/$record_id"
	admin_submit "/admin/invoices/$record_id" \
		"number=$ADMIN_NUMBER" "customer_id=cus-kestrel" "currency=EUR" \
		"total_cents=-99999999" "status=credit" "issued_at=2026-03-04T09:30"

	assert_equal "a negative amount survives an edit" "-99999999" \
		"$(app_sqlite_query "SELECT total_cents FROM invoices WHERE number='$ADMIN_NUMBER';")"

	http_get "/admin/invoices/$record_id"
	admin_submit "/admin/invoices/$record_id/delete"
	assert_equal "the invoice can be deleted" "0" \
		"$(app_sqlite_query "SELECT count(*) FROM invoices WHERE number='$ADMIN_NUMBER';")"

	server_stop
}

check_the_ladder_is_ready
record_the_state_before
generate_the_invoicing_migration
check_the_money_columns_are_integers
apply_the_invoicing_migration
check_integer_money_is_exact
record_how_float_money_is_stored
seed_an_invoice_with_lines
check_the_three_level_graph_resolves
check_the_admin_manages_invoicing
check_money_written_through_the_admin_is_exact

if port_is_free; then
	check_passed "the port is free when the chunk ends"
else
	check_failed "the port is free when the chunk ends" "held by $(port_listener_pids | tr '\n' ' ')"
fi

chunk_end
