#!/usr/bin/env bash

. "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib/bootstrap.sh"
. "$E2E_DIR/lib/appsource.sh"
. "$E2E_DIR/lib/server.sh"
. "$E2E_DIR/lib/http.sh"
. "$E2E_DIR/lib/admin.sh"

chunk_begin "08" "Columns on a populated table"

trap server_cleanup EXIT

CUSTOMS_MIGRATION=""
ETA_MIGRATION=""
REQUIRED_MIGRATION=""
STEPS_BEFORE=0

check_the_ladder_is_at_step_two() {
	if app_table_exists shipments; then
		return 0
	fi
	check_failed "the shipments table exists" "run chunk 07 first"
	chunk_end
}

seed_more_shipments() {
	app_sqlite_query "INSERT OR IGNORE INTO shipments
		(id, reference, customer_id, origin_id, destination_id, status, created_at, updated_at) VALUES
		('shp-e2e-2','MRF-000002','cus-kestrel','prt-sin','prt-rtm','in_transit',datetime('now'),datetime('now')),
		('shp-e2e-3','MRF-000003','cus-alpine','prt-sha','prt-rtm','draft',datetime('now'),datetime('now'));"

	STEPS_BEFORE="$(app_migration_files | wc -l | tr -d ' ')"
	SHIPMENTS_BEFORE="$(app_row_count shipments)"
	REFERENCES_BEFORE="$(app_sqlite_query "SELECT reference FROM shipments ORDER BY reference;" | tr '\n' ',')"

	if [ "$SHIPMENTS_BEFORE" -ge 3 ]; then
		check_passed "the shipments table holds rows before the columns are added"
	else
		check_failed "the shipments table holds rows before the columns are added" \
			"only $SHIPMENTS_BEFORE rows; a populated table is the point of this chunk"
		chunk_end
	fi
	note "rows before the column additions" "$SHIPMENTS_BEFORE shipments"
}

generate_the_customs_migration() {
	local existed=0
	CUSTOMS_MIGRATION="$(app_migration_for_name shipment_customs)"
	if [ -n "$CUSTOMS_MIGRATION" ]; then
		existed=1
	fi

	app_write_shipment_model 2
	cd "$EXAMPLE_DIR"
	assert_succeeds "the project compiles with the two new fields" go build ./...
	cd "$E2E_ROOT"

	app_run_command makemigrations --name=shipment_customs
	assert_equal "makemigrations exits cleanly" "0" "$CAPTURED_STATUS"

	if [ "$existed" = "1" ]; then
		check_skipped "makemigrations adds columns rather than recreating the table" \
			"0003 is already present from an earlier run"
	else
		case "$APP_OUTPUT" in
		*"add column shipments.customs_notes"*)
			check_passed "makemigrations adds columns rather than recreating the table"
			;;
		*)
			check_failed "makemigrations adds columns rather than recreating the table" \
				"$(truncated_output "$APP_OUTPUT")"
			;;
		esac

		case "$APP_OUTPUT" in
		*"create table shipments"*)
			check_failed "the populated table is not recreated" "the migration recreates shipments"
			;;
		*)
			check_passed "the populated table is not recreated"
			;;
		esac
	fi

	CUSTOMS_MIGRATION="$(app_migration_for_name shipment_customs)"
	assert_file_exists "a migration for the added columns is written" "$CUSTOMS_MIGRATION"
}

check_the_added_columns_are_nullable() {
	if [ ! -f "$CUSTOMS_MIGRATION" ]; then
		check_skipped "the added columns are nullable" "no migration file"
		return
	fi

	assert_output_contains "customs_notes is added as a text column" \
		'migrate.AddColumn{Table: "shipments", Column: migrate.Column{Name: "customs_notes", Kind: model.KindText, Size: 2000}}' \
		cat "$CUSTOMS_MIGRATION"
	assert_output_contains "declared_value is added as a float column" \
		'migrate.AddColumn{Table: "shipments", Column: migrate.Column{Name: "declared_value", Kind: model.KindFloat, Size: 64}}' \
		cat "$CUSTOMS_MIGRATION"

	local added
	added="$(grep 'migrate.AddColumn' "$CUSTOMS_MIGRATION" || true)"
	case "$added" in
	*NotNull*)
		check_failed "a column added to a populated table is nullable" \
			"an AddColumn op carries NotNull, which cannot be applied to existing rows:
$added"
		;;
	*)
		check_passed "a column added to a populated table is nullable"
		;;
	esac

	assert_output_contains "the migration is reversible with a DropColumn" \
		'migrate.DropColumn{Table: "shipments", Column: "declared_value"}' cat "$CUSTOMS_MIGRATION"
}

apply_the_customs_migration() {
	app_run_command migrate
	assert_equal "migrate applies the column additions" "0" "$CAPTURED_STATUS"

	assert_equal "every existing shipment survived" "$SHIPMENTS_BEFORE" "$(app_row_count shipments)"
	assert_equal "the existing references are unchanged" "$REFERENCES_BEFORE" \
		"$(app_sqlite_query "SELECT reference FROM shipments ORDER BY reference;" | tr '\n' ',')"

	local columns
	columns="$(app_table_columns shipments | tr '\n' ' ')"
	local column
	for column in customs_notes declared_value; do
		case "$columns" in
		*"$column"*)
			check_passed "the $column column exists in the database"
			;;
		*)
			check_failed "the $column column exists in the database" "columns: $columns"
			;;
		esac
	done

	assert_equal "the new columns are NULL on every pre-existing row" "0" \
		"$(app_sqlite_query "SELECT count(*) FROM shipments WHERE customs_notes IS NOT NULL OR declared_value IS NOT NULL;")"
}

generate_the_eta_migration() {
	local existed=0
	ETA_MIGRATION="$(app_migration_for_name shipment_eta)"
	if [ -n "$ETA_MIGRATION" ]; then
		existed=1
	fi

	app_write_shipment_model 3
	cd "$EXAMPLE_DIR"
	assert_succeeds "the project compiles with the third field" go build ./...
	cd "$E2E_ROOT"

	app_run_command makemigrations --name=shipment_eta
	assert_equal "the second column addition exits cleanly" "0" "$CAPTURED_STATUS"

	if [ "$existed" = "1" ]; then
		check_skipped "a further column goes into its own migration" \
			"0004 is already present from an earlier run"
	else
		case "$APP_OUTPUT" in
		*"add column shipments.eta"*)
			check_passed "a further column goes into its own migration"
			;;
		*)
			check_failed "a further column goes into its own migration" \
				"$(truncated_output "$APP_OUTPUT")"
			;;
		esac
	fi

	ETA_MIGRATION="$(app_migration_for_name shipment_eta)"
	assert_file_exists "a separate migration for the third column is written" "$ETA_MIGRATION"
	assert_equal "the ladder gained exactly two steps" "$((STEPS_BEFORE + 2))" \
		"$(app_migration_files | wc -l | tr -d ' ')"

	app_run_command migrate
	assert_equal "the second column addition applies" "0" "$CAPTURED_STATUS"
	assert_equal "the shipments survived the second addition" "$SHIPMENTS_BEFORE" "$(app_row_count shipments)"

	local columns
	columns="$(app_table_columns shipments | tr '\n' ' ')"
	case "$columns" in
	*eta*)
		check_passed "the eta column exists in the database"
		;;
	*)
		check_failed "the eta column exists in the database" "columns: $columns"
		;;
	esac
}

check_a_required_column_is_refused_on_a_populated_table() {
	app_backup_snapshot

	app_write_shipment_model 4
	cd "$EXAMPLE_DIR"
	go build ./... >/dev/null 2>&1
	cd "$E2E_ROOT"

	app_run_command makemigrations --name=carrier_required

	REQUIRED_MIGRATION="$(app_migration_for_name carrier_required)"
	if [ -z "$REQUIRED_MIGRATION" ]; then
		check_skipped "a NOT NULL column on a populated table fails loudly" \
			"the migration was not generated: $(truncated_output "$APP_OUTPUT")"
		app_write_shipment_model 3
		app_restore_snapshot
		app_gofmt
		return
	fi

	local added
	added="$(grep 'migrate.AddColumn' "$REQUIRED_MIGRATION" || true)"
	case "$added" in
	*"NotNull: true"*)
		check_passed "makemigrations marks a non-pointer field NOT NULL"
		;;
	*)
		check_failed "makemigrations marks a non-pointer field NOT NULL" "$added"
		;;
	esac

	case "$APP_OUTPUT" in
	*"NOT NULL without a default"*)
		check_passed "makemigrations warns at generation time that existing rows need a value"
		;;
	*)
		check_failed "makemigrations warns at generation time that existing rows need a value" \
			"no warning in the output:
$(truncated_output "$APP_OUTPUT")"
		;;
	esac

	app_run_command migrate

	if [ "$CAPTURED_STATUS" -ne 0 ]; then
		check_passed "a NOT NULL column on a populated table fails loudly"
	else
		check_failed "a NOT NULL column on a populated table fails loudly" \
			"migrate exited 0; a NOT NULL column cannot be filled for existing rows"
	fi

	case "$APP_OUTPUT" in
	*"Cannot add a NOT NULL column"*)
		check_passed "the failure names the reason the database gave"
		;;
	*)
		check_failed "the failure names the reason the database gave" "$(truncated_output "$APP_OUTPUT")"
		;;
	esac

	assert_equal "the failed migration left the rows alone" "$SHIPMENTS_BEFORE" "$(app_row_count shipments)"
	assert_equal "the failed migration is not recorded as applied" "0" \
		"$(app_sqlite_query "SELECT count(*) FROM coyote_migrations WHERE id LIKE '%carrier_required';")"
	assert_equal "the column was not half added" "0" \
		"$(app_sqlite_query "SELECT count(*) FROM pragma_table_info('shipments') WHERE name='carrier_name';")"

	if grep -q carrier_name "$(app_snapshot_path)" 2>/dev/null; then
		check_failed "the snapshot does not record a change that failed to apply" \
			"snapshot.json lists carrier_name although migrate rejected 0005
the snapshot is written when the migration is generated, not when it is applied, so a failed
migration leaves the recorded model state ahead of the database"
		note "snapshot after a failure" \
			"the developer must delete the migration and hand-edit snapshot.json to get back in step"
	else
		check_passed "the snapshot does not record a change that failed to apply"
	fi

	rm -f "$REQUIRED_MIGRATION"
	app_write_shipment_model 3
	app_restore_snapshot
	cd "$EXAMPLE_DIR"
	assert_succeeds "the project builds again after backing the change out" go build ./...
	cd "$E2E_ROOT"
	assert_equal "the ladder is back to where it was" "$((STEPS_BEFORE + 2))" \
		"$(app_migration_files | wc -l | tr -d ' ')"
}

check_the_new_columns_are_writable_through_the_admin() {
	if ! port_is_free; then
		force_free_the_port
	fi
	server_start
	if ! server_wait_for_http; then
		check_failed "the application serves after the column additions" "$(tail -20 "$SERVER_LOG")"
		return
	fi
	check_passed "the application serves after the column additions"

	if ! admin_login; then
		check_failed "the superadmin can sign in" "status $HTTP_STATUS"
		return
	fi

	http_get "/admin/shipments/new"
	local field
	for field in customs_notes declared_value eta; do
		case "$HTTP_BODY" in
		*"name=\"$field\""*)
			check_passed "the admin form offers the new $field field"
			;;
		*)
			check_failed "the admin form offers the new $field field" "not present on the create form"
			;;
		esac
	done

	local target
	target="$(app_sqlite_query "SELECT id FROM shipments WHERE reference='MRF-000001';")"
	if [ -z "$target" ]; then
		check_skipped "a new column can be written through the admin" "no shipment to edit"
		return
	fi

	http_get "/admin/shipments/$target"
	admin_submit "/admin/shipments/$target" \
		"reference=MRF-000001" "customer_id=cus-nordwind" "origin_id=prt-rtm" \
		"destination_id=prt-sin" "status=booked" \
		"customs_notes=Cleared at Rotterdam on arrival" "declared_value=18500.50"

	assert_equal "editing a shipment with the new columns redirects" "303" "$HTTP_STATUS"
	assert_equal "the new text column was written" "Cleared at Rotterdam on arrival" \
		"$(app_sqlite_query "SELECT customs_notes FROM shipments WHERE reference='MRF-000001';")"
	assert_equal "the new numeric column was written" "18500.5" \
		"$(app_sqlite_query "SELECT declared_value FROM shipments WHERE reference='MRF-000001';")"
	assert_equal "the other shipments still have NULL in the new columns" "2" \
		"$(app_sqlite_query "SELECT count(*) FROM shipments WHERE customs_notes IS NULL;")"

	server_stop
}

check_the_ladder_is_at_step_two
seed_more_shipments
generate_the_customs_migration
check_the_added_columns_are_nullable
apply_the_customs_migration
generate_the_eta_migration
check_a_required_column_is_refused_on_a_populated_table
check_the_new_columns_are_writable_through_the_admin

if port_is_free; then
	check_passed "the port is free when the chunk ends"
else
	check_failed "the port is free when the chunk ends" "held by $(port_listener_pids | tr '\n' ' ')"
fi

chunk_end
