#!/usr/bin/env bash

. "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib/bootstrap.sh"
. "$E2E_DIR/lib/appsource.sh"
. "$E2E_DIR/lib/server.sh"
. "$E2E_DIR/lib/http.sh"
. "$E2E_DIR/lib/admin.sh"

chunk_begin "07" "Foreign keys"

trap server_cleanup EXIT

MIGRATION_FILE="$EXAMPLE_DIR/migrations/0002_shipments.go"
SHIPMENT_REFERENCE="MRF-000001"

check_the_ladder_is_at_step_one() {
	if app_table_exists customers && [ -f "$EXAMPLE_DIR/migrations/0001_reference_data.go" ]; then
		return 0
	fi
	check_failed "the first migration has been applied" "run chunks 05 and 06 first"
	chunk_end
}

record_the_parent_rows() {
	CUSTOMERS_BEFORE="$(app_row_count customers)"
	PORTS_BEFORE="$(app_row_count ports)"
	CUSTOMER_NAMES_BEFORE="$(app_sqlite_query "SELECT name FROM customers ORDER BY name;" | tr '\n' ',')"

	note "rows before the migration" "$CUSTOMERS_BEFORE customers, $PORTS_BEFORE ports"

	if [ "$CUSTOMERS_BEFORE" = "3" ] && [ "$PORTS_BEFORE" = "3" ]; then
		check_passed "the parent tables hold the seeded rows before the migration"
	else
		check_failed "the parent tables hold the seeded rows before the migration" \
			"customers=$CUSTOMERS_BEFORE ports=$PORTS_BEFORE, expected 3 and 3"
	fi
}

write_the_shipment_model() {
	app_write_shipment_model
	app_write_admin_resources 7
	app_write_main 7

	assert_file_exists "the shipment model is written" "$EXAMPLE_DIR/models_shipment.go"

	cd "$EXAMPLE_DIR"
	assert_succeeds "the project compiles with the shipment model" go build ./...
	cd "$E2E_ROOT"
}

check_the_second_migration_is_generated() {
	local existed=0
	if [ -f "$MIGRATION_FILE" ]; then
		existed=1
	fi

	app_run_command makemigrations --name=shipments
	assert_equal "makemigrations exits cleanly" "0" "$CAPTURED_STATUS"

	if [ "$existed" = "1" ]; then
		check_skipped "makemigrations generates the second step" \
			"0002 is already present from an earlier run; a full suite run regenerates it because chunk 05 resets the ladder"
	elif [ ! -f "$MIGRATION_FILE" ]; then
		check_failed "makemigrations generates the second step" \
			"the migration file is absent but makemigrations reports no model changes
snapshot.json still records Shipment, so the snapshot and the migration files have drifted apart
a developer who deletes a migration to regenerate it lands here and has to edit snapshot.json by hand
$(truncated_output "$APP_OUTPUT")"
		note "snapshot drift" \
			"the snapshot is authoritative for change detection, so deleting a migration file does not let makemigrations rewrite it"
	else
		case "$APP_OUTPUT" in
		*"create table shipments"*)
			check_passed "makemigrations generates the second step"
			;;
		*)
			check_failed "makemigrations generates the second step" "$(truncated_output "$APP_OUTPUT")"
			;;
		esac

		case "$APP_OUTPUT" in
		*"create table customers"*)
			check_failed "the migration does not recreate the existing tables" \
				"customers is mentioned again in migration 0002"
			;;
		*)
			check_passed "the migration does not recreate the existing tables"
			;;
		esac
	fi

	assert_file_exists "a second migration file is present" "$MIGRATION_FILE"
	assert_file_exists "the first migration is left alone" \
		"$EXAMPLE_DIR/migrations/0001_reference_data.go"
	assert_equal "the ladder now has two steps" "2" "$(app_migration_files | wc -l | tr -d ' ')"
}

check_the_migration_describes_the_relations() {
	if [ ! -f "$MIGRATION_FILE" ]; then
		check_skipped "the migration carries the foreign key columns" "no migration file"
		return
	fi

	local column target
	for column in customer_id:customers origin_id:ports destination_id:ports; do
		target="${column#*:}"
		column="${column%%:*}"
		assert_output_contains "the migration adds the $column column" \
			"{Name: \"$column\", Kind: model.KindString, Size: 36, References: migrate.Reference{Table: \"$target\", Column: \"id\"}}" \
			cat "$MIGRATION_FILE"
		assert_output_contains "the migration indexes $column" \
			"idx_shipments_$column" cat "$MIGRATION_FILE"
	done

	case "$(cat "$MIGRATION_FILE")" in
	*Customer*)
		check_failed "the embedded relation fields do not become columns" \
			"the migration mentions the Customer struct field"
		;;
	*)
		check_passed "the embedded relation fields do not become columns"
		;;
	esac

	cd "$EXAMPLE_DIR"
	assert_succeeds "the second migration compiles" go build ./...
	cd "$E2E_ROOT"
}

apply_the_second_migration() {
	app_run_command migrate

	assert_equal "migrate applies the second step cleanly" "0" "$CAPTURED_STATUS"

	if app_table_exists shipments; then
		check_passed "the shipments table exists"
	else
		check_failed "the shipments table exists" "$(truncated_output "$APP_OUTPUT")"
		chunk_end
	fi

	assert_equal "the ledger records both migrations" "0001_reference_data 0002_shipments" \
		"$(app_sqlite_query "SELECT id FROM coyote_migrations ORDER BY id;" | tr '\n' ' ' | sed 's/ $//')"
}

check_the_parent_rows_survived() {
	assert_equal "the customers survived the migration" "$CUSTOMERS_BEFORE" "$(app_row_count customers)"
	assert_equal "the ports survived the migration" "$PORTS_BEFORE" "$(app_row_count ports)"
	assert_equal "the customer rows are unchanged, not merely the same count" \
		"$CUSTOMER_NAMES_BEFORE" "$(app_sqlite_query "SELECT name FROM customers ORDER BY name;" | tr '\n' ',')"
}

check_the_shipment_schema() {
	assert_equal "the shipments table has the columns the model describes" \
		"created_at customer_id destination_id id origin_id reference status updated_at" \
		"$(app_table_columns shipments | tr '\n' ' ' | sed 's/ $//')"

	local indexes
	indexes="$(app_sqlite_query "SELECT name FROM sqlite_master WHERE type='index' AND tbl_name='shipments';" | tr '\n' ' ')"
	local column
	for column in customer_id origin_id destination_id; do
		case "$indexes" in
		*"idx_shipments_$column"*)
			check_passed "$column is indexed in the database"
			;;
		*)
			check_failed "$column is indexed in the database" "indexes: $indexes"
			;;
		esac
	done
}

check_referential_integrity_in_the_schema() {
	local ddl
	ddl="$(app_sqlite_query "SELECT sql FROM sqlite_master WHERE name='shipments';")"

	case "$ddl" in
	*REFERENCES*"customers"*)
		check_passed "the shipments schema declares a foreign key to customers"
		;;
	*)
		check_failed "the shipments schema declares a foreign key to customers" "ddl: $ddl"
		;;
	esac

	app_sqlite_query "DELETE FROM shipments WHERE id='shp-dangling';"
	sqlite3 "$(app_database_path)" "PRAGMA foreign_keys=ON;
		INSERT INTO shipments (id, reference, customer_id, origin_id, destination_id, status, created_at, updated_at)
		VALUES ('shp-dangling','DANGLING','no-such-customer','no-such-port','no-such-port','draft',datetime('now'),datetime('now'));" \
		>/dev/null 2>&1 || true

	local dangling
	dangling="$(app_sqlite_query "SELECT count(*) FROM shipments WHERE id='shp-dangling';")"
	app_sqlite_query "DELETE FROM shipments WHERE id='shp-dangling';"

	if [ "$dangling" = "0" ]; then
		check_passed "the schema enforces that a shipment points at a real customer"
	else
		check_failed "the schema enforces that a shipment points at a real customer" \
			"a row pointing at customer_id 'no-such-customer' was accepted even with PRAGMA foreign_keys=ON"
	fi

	local kept
	kept="$(app_sqlite_query "SELECT count(*) FROM shipments WHERE customer_id IS NULL;")"
	sqlite3 "$(app_database_path)" "PRAGMA foreign_keys=ON;
		INSERT INTO shipments (id, reference, status, created_at, updated_at)
		VALUES ('shp-loose','LOOSE','draft',datetime('now'),datetime('now'));" >/dev/null 2>&1 || true
	local loose
	loose="$(app_sqlite_query "SELECT count(*) FROM shipments WHERE id='shp-loose';")"
	app_sqlite_query "DELETE FROM shipments WHERE id='shp-loose';"

	if [ "$loose" = "1" ]; then
		check_passed "a shipment with no customer at all is still allowed"
	else
		check_failed "a shipment with no customer at all is still allowed" \
			"the constraint should reject dangling keys, not require the column to be filled in (had $kept null rows)"
	fi

	note "referential integrity" \
		"a described belongs-to becomes a named FOREIGN KEY, so the database refuses a key that points at nothing"
}

boot_the_server() {
	if ! port_is_free; then
		force_free_the_port
	fi
	server_start
	if server_wait_for_http; then
		check_passed "the application serves with the shipment resource mounted"
		return 0
	fi
	check_failed "the application serves with the shipment resource mounted" \
		"$(tail -20 "$SERVER_LOG")"
	chunk_end
}

check_the_admin_resolves_the_relations() {
	if ! admin_login; then
		check_failed "the superadmin can sign in" "status $HTTP_STATUS"
		chunk_end
	fi
	check_passed "the superadmin can sign in"

	assert_http_status "the shipments list answers 200" "200" "/admin/shipments"

	http_get "/admin/shipments/new"
	assert_equal "the shipment create form answers 200" "200" "$HTTP_STATUS"

	local relation
	for relation in customer_id origin_id destination_id; do
		case "$HTTP_BODY" in
		*"<select name=\"$relation\""*)
			check_passed "$relation is offered as a relation picker"
			;;
		*)
			check_failed "$relation is offered as a relation picker" "no select rendered for $relation"
			;;
		esac
	done

	local label
	for label in "Nordwind Logistics" "Rotterdam" "Singapore"; do
		case "$HTTP_BODY" in
		*"$label"*)
			check_passed "the pickers offer $label by name, not by id"
			;;
		*)
			check_failed "the pickers offer $label by name, not by id" "$label is not among the options"
			;;
		esac
	done
}

check_creating_a_shipment_through_the_admin() {
	app_sqlite_query "DELETE FROM shipments WHERE reference='$SHIPMENT_REFERENCE';"

	http_get "/admin/shipments/new"
	admin_submit "/admin/shipments/new" \
		"reference=$SHIPMENT_REFERENCE" \
		"customer_id=cus-nordwind" \
		"origin_id=prt-rtm" \
		"destination_id=prt-sin" \
		"status=booked"

	assert_equal "creating a shipment redirects back to the list" "303" "$HTTP_STATUS"
	assert_equal "the shipment was written with both relations set" \
		"cus-nordwind|prt-rtm|prt-sin|booked" \
		"$(app_sqlite_query "SELECT customer_id || '|' || origin_id || '|' || destination_id || '|' || status FROM shipments WHERE reference='$SHIPMENT_REFERENCE';")"
}

check_the_list_shows_labels_not_keys() {
	http_get "/admin/shipments"

	local label
	for label in "Nordwind Logistics" "Rotterdam" "Singapore"; do
		case "$HTTP_BODY" in
		*"$label"*)
			check_passed "the shipments list resolves $label"
			;;
		*)
			check_failed "the shipments list resolves $label" "the list does not show it"
			;;
		esac
	done

	case "$HTTP_BODY" in
	*cus-nordwind*)
		check_failed "the list does not fall back to raw keys" "cus-nordwind appears in the list"
		;;
	*)
		check_passed "the list does not fall back to raw keys"
		;;
	esac
}

check_two_relations_to_one_table() {
	local origin destination
	origin="$(app_sqlite_query "SELECT origin_id FROM shipments WHERE reference='$SHIPMENT_REFERENCE';")"
	destination="$(app_sqlite_query "SELECT destination_id FROM shipments WHERE reference='$SHIPMENT_REFERENCE';")"

	if [ -n "$origin" ] && [ -n "$destination" ] && [ "$origin" != "$destination" ]; then
		check_passed "two relations to the same table stay independent"
	else
		check_failed "two relations to the same table stay independent" \
			"origin=$origin destination=$destination"
	fi
}

check_the_ladder_is_at_step_one
record_the_parent_rows
write_the_shipment_model
check_the_second_migration_is_generated
check_the_migration_describes_the_relations
apply_the_second_migration
check_the_parent_rows_survived
check_the_shipment_schema
check_referential_integrity_in_the_schema
boot_the_server
check_the_admin_resolves_the_relations
check_creating_a_shipment_through_the_admin
check_the_list_shows_labels_not_keys
check_two_relations_to_one_table

server_stop
if port_is_free; then
	check_passed "the port is free when the chunk ends"
else
	check_failed "the port is free when the chunk ends" "held by $(port_listener_pids | tr '\n' ' ')"
fi

chunk_end
