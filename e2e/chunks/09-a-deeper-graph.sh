#!/usr/bin/env bash

. "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib/bootstrap.sh"
. "$E2E_DIR/lib/appsource.sh"
. "$E2E_DIR/lib/server.sh"
. "$E2E_DIR/lib/http.sh"
. "$E2E_DIR/lib/admin.sh"

chunk_begin "09" "A deeper graph"

trap server_cleanup EXIT

GRAPH_MIGRATION=""
STEPS_BEFORE=0

check_the_ladder_is_ready() {
	if app_table_exists shipments && [ "$(app_row_count shipments)" -ge 3 ]; then
		return 0
	fi
	check_failed "the shipments table holds rows" "run chunks 05 to 08 first"
	chunk_end
}

record_the_state_before() {
	STEPS_BEFORE="$(app_migration_files | wc -l | tr -d ' ')"
	SHIPMENTS_BEFORE="$(app_row_count shipments)"
	CUSTOMERS_BEFORE="$(app_row_count customers)"
	PORTS_BEFORE="$(app_row_count ports)"
	NOTES_BEFORE="$(app_sqlite_query "SELECT ifnull(customs_notes,'') FROM shipments WHERE reference='MRF-000001';")"

	note "state before the deeper graph" \
		"$STEPS_BEFORE migrations, $SHIPMENTS_BEFORE shipments, $CUSTOMERS_BEFORE customers, $PORTS_BEFORE ports"
}

generate_the_graph_migration() {
	local existed=0
	GRAPH_MIGRATION="$(app_migration_for_name containers_and_events)"
	if [ -n "$GRAPH_MIGRATION" ]; then
		existed=1
	fi

	app_write_container_models
	app_write_admin_resources 9
	app_write_main 9

	assert_file_exists "the container models are written" "$EXAMPLE_DIR/models_container.go"

	cd "$EXAMPLE_DIR"
	assert_succeeds "the project compiles with the deeper graph" go build ./...
	cd "$E2E_ROOT"

	app_run_command makemigrations --name=containers_and_events
	assert_equal "makemigrations exits cleanly" "0" "$CAPTURED_STATUS"

	if [ "$existed" = "1" ]; then
		check_skipped "both child tables go into one migration" \
			"the migration is already present from an earlier run"
	else
		local table
		for table in containers tracking_events; do
			case "$APP_OUTPUT" in
			*"create table $table"*)
				check_passed "the migration creates $table"
				;;
			*)
				check_failed "the migration creates $table" "$(truncated_output "$APP_OUTPUT")"
				;;
			esac
		done

		case "$APP_OUTPUT" in
		*"create table shipments"* | *"add column shipments"*)
			check_failed "the existing tables are left alone" "the migration also touches shipments"
			;;
		*)
			check_passed "the existing tables are left alone"
			;;
		esac
	fi

	GRAPH_MIGRATION="$(app_migration_for_name containers_and_events)"
	assert_file_exists "a migration for the deeper graph is written" "$GRAPH_MIGRATION"
	if [ "$existed" = "1" ]; then
		assert_equal "the ladder is unchanged because the step was already there" "$STEPS_BEFORE" \
			"$(app_migration_files | wc -l | tr -d ' ')"
	else
		assert_equal "the ladder gained one step" "$((STEPS_BEFORE + 1))" \
			"$(app_migration_files | wc -l | tr -d ' ')"
	fi
}

check_the_graph_migration_content() {
	if [ -z "$GRAPH_MIGRATION" ] || [ ! -f "$GRAPH_MIGRATION" ]; then
		check_skipped "the migration describes both foreign key columns" "no migration file"
		return
	fi

	assert_output_contains "containers reference their shipment" \
		'{Name: "shipment_id", Kind: model.KindString, Size: 36, References: migrate.Reference{Table: "shipments", Column: "id"}}' \
		cat "$GRAPH_MIGRATION"
	assert_output_contains "tracking events reference their container" \
		'{Name: "container_id", Kind: model.KindString, Size: 36, References: migrate.Reference{Table: "containers", Column: "id"}}' \
		cat "$GRAPH_MIGRATION"
	assert_output_contains "the container number is unique" \
		'idx_containers_number' cat "$GRAPH_MIGRATION"
	assert_output_contains "the migration drops both tables when reversed" \
		'migrate.DropTable{Name: "tracking_events"}' cat "$GRAPH_MIGRATION"

	cd "$EXAMPLE_DIR"
	assert_succeeds "the graph migration compiles" go build ./...
	cd "$E2E_ROOT"
}

apply_the_graph_migration() {
	app_run_command migrate
	assert_equal "migrate applies the deeper graph" "0" "$CAPTURED_STATUS"

	local table
	for table in containers tracking_events; do
		if app_table_exists "$table"; then
			check_passed "the $table table exists"
		else
			check_failed "the $table table exists" "$(truncated_output "$APP_OUTPUT")"
			chunk_end
		fi
	done

	assert_equal "the shipments survived the new tables" "$SHIPMENTS_BEFORE" "$(app_row_count shipments)"

	local indexes
	indexes="$(app_sqlite_query "SELECT name FROM sqlite_master WHERE type='index' AND tbl_name='tracking_events';" | tr '\n' ' ')"
	local column
	for column in shipment_id container_id kind occurred_at; do
		case "$indexes" in
		*"idx_tracking_events_$column"*)
			check_passed "tracking_events.$column is indexed"
			;;
		*)
			check_failed "tracking_events.$column is indexed" "indexes: $indexes"
			;;
		esac
	done
}

seed_many_children_per_parent() {
	app_sqlite_query "DELETE FROM tracking_events; DELETE FROM containers;"
	app_sqlite_query "INSERT INTO containers (id, number, shipment_id, size_feet, sealed, created_at, updated_at)
		SELECT 'con-' || s.id || '-' || n.n,
		       'MSCU' || substr('0000000' || (abs(random()) % 9999999), -7, 7) || n.n,
		       s.id, 40, 1, datetime('now'), datetime('now')
		FROM shipments s, (SELECT 1 AS n UNION SELECT 2 UNION SELECT 3) n;"
	app_sqlite_query "INSERT INTO tracking_events (id, shipment_id, container_id, kind, location, occurred_at, created_at)
		SELECT 'evt-' || c.id || '-' || k.n, c.shipment_id, c.id, 'gate_in', 'Rotterdam',
		       datetime('now'), datetime('now')
		FROM containers c, (SELECT 1 AS n UNION SELECT 2 UNION SELECT 3 UNION SELECT 4 UNION SELECT 5) k;"

	CONTAINERS_SEEDED="$(app_row_count containers)"
	EVENTS_SEEDED="$(app_row_count tracking_events)"

	assert_equal "each shipment has three containers" "$((SHIPMENTS_BEFORE * 3))" "$CONTAINERS_SEEDED"
	assert_equal "each container has five tracking events" "$((CONTAINERS_SEEDED * 5))" "$EVENTS_SEEDED"

	assert_equal "every container belongs to a real shipment" "0" \
		"$(app_sqlite_query "SELECT count(*) FROM containers c LEFT JOIN shipments s ON s.id = c.shipment_id WHERE s.id IS NULL;")"
	assert_equal "every event belongs to a real container" "0" \
		"$(app_sqlite_query "SELECT count(*) FROM tracking_events e LEFT JOIN containers c ON c.id = e.container_id WHERE c.id IS NULL;")"

	note "graph size" "$SHIPMENTS_BEFORE shipments, $CONTAINERS_SEEDED containers, $EVENTS_SEEDED tracking events"
}

check_rollback_asks_before_destroying_data() {
	app_run_command rollback

	if [ "$CAPTURED_STATUS" -eq 0 ] && app_table_exists containers; then
		check_passed "rollback refuses to destroy data without confirmation"
	else
		check_failed "rollback refuses to destroy data without confirmation" \
			"exit $CAPTURED_STATUS, containers table present: $(app_table_exists containers && echo yes || echo no)"
	fi

	local warning
	warning="$APP_OUTPUT"
	case "$warning" in
	*"every row in tracking_events"*)
		check_passed "rollback names the tables whose rows would be lost"
		;;
	*)
		check_failed "rollback names the tables whose rows would be lost" "$(truncated_output "$warning")"
		;;
	esac

	assert_equal "the containers are still there after the refusal" "$CONTAINERS_SEEDED" "$(app_row_count containers)"
	assert_equal "the tracking events are still there after the refusal" "$EVENTS_SEEDED" "$(app_row_count tracking_events)"
}

check_rollback_undoes_the_step() {
	app_run_command rollback --no-input
	assert_equal "rollback with --no-input exits cleanly" "0" "$CAPTURED_STATUS"

	local table
	for table in containers tracking_events; do
		if app_table_exists "$table"; then
			check_failed "rollback dropped the $table table" "the table is still present"
		else
			check_passed "rollback dropped the $table table"
		fi
	done

	assert_equal "the ledger rewound by one step" "0" \
		"$(app_sqlite_query "SELECT count(*) FROM coyote_migrations WHERE id LIKE '%containers_and_events';")"
}

check_the_parents_survived_the_rollback() {
	assert_equal "the shipments survived the rollback" "$SHIPMENTS_BEFORE" "$(app_row_count shipments)"
	assert_equal "the customers survived the rollback" "$CUSTOMERS_BEFORE" "$(app_row_count customers)"
	assert_equal "the ports survived the rollback" "$PORTS_BEFORE" "$(app_row_count ports)"
	assert_equal "the column added in an earlier step still holds its value" "$NOTES_BEFORE" \
		"$(app_sqlite_query "SELECT ifnull(customs_notes,'') FROM shipments WHERE reference='MRF-000001';")"
}

check_re_migrating_restores_the_schema() {
	app_run_command migrate
	assert_equal "migrate re-applies the rolled back step" "0" "$CAPTURED_STATUS"

	local table
	for table in containers tracking_events; do
		if app_table_exists "$table"; then
			check_passed "the $table table is back"
		else
			check_failed "the $table table is back" "$(truncated_output "$APP_OUTPUT")"
		fi
	done

	assert_equal "the child rows are gone, as a dropped table implies" "0" "$(app_row_count containers)"
	assert_equal "the parents are still intact after the round trip" "$SHIPMENTS_BEFORE" "$(app_row_count shipments)"
	note "rollback is not a backup" \
		"dropping and recreating a table loses its rows; only the parents survive a round trip"
}

check_the_admin_manages_the_new_models() {
	app_sqlite_query "INSERT INTO containers (id, number, shipment_id, size_feet, sealed, created_at, updated_at)
		SELECT 'con-admin-1', 'MSCU7654321', id, 20, 0, datetime('now'), datetime('now')
		FROM shipments WHERE reference='MRF-000001';"

	if ! port_is_free; then
		force_free_the_port
	fi
	server_start
	if ! server_wait_for_http; then
		check_failed "the application serves with five managed models" "$(tail -20 "$SERVER_LOG")"
		return
	fi
	check_passed "the application serves with five managed models"

	if ! admin_login; then
		check_failed "the superadmin can sign in" "status $HTTP_STATUS"
		return
	fi

	assert_http_status "the containers list answers 200" "200" "/admin/containers"
	assert_http_status "the tracking events list answers 200" "200" "/admin/tracking_events"
	note "admin slugs" "derived from the table name, so TrackingEvent is served at /admin/tracking_events"

	assert_body_contains "the containers list shows the seeded container" "MSCU7654321" "/admin/containers"
	assert_body_contains "the container list resolves its shipment by reference" \
		"MRF-000001" "/admin/containers"

	http_get "/admin/tracking_events/new"
	assert_equal "the tracking event form answers 200" "200" "$HTTP_STATUS"

	local relation
	for relation in shipment_id container_id; do
		case "$HTTP_BODY" in
		*"<select name=\"$relation\""*)
			check_passed "the event form offers $relation as a relation picker"
			;;
		*)
			check_failed "the event form offers $relation as a relation picker" "no select for $relation"
			;;
		esac
	done

	admin_submit "/admin/tracking_events/new" \
		"kind=customs_hold" "location=Rotterdam" "occurred_at=2026-03-04T09:30" \
		"shipment_id=$(app_sqlite_query "SELECT id FROM shipments WHERE reference='MRF-000001';")" \
		"container_id=con-admin-1"

	assert_equal "creating a tracking event redirects back to the list" "303" "$HTTP_STATUS"
	assert_equal "the tracking event was written with both relations" "1" \
		"$(app_sqlite_query "SELECT count(*) FROM tracking_events WHERE kind='customs_hold' AND container_id='con-admin-1';")"

	server_stop
}

check_the_ladder_is_ready
record_the_state_before
generate_the_graph_migration
check_the_graph_migration_content
apply_the_graph_migration
seed_many_children_per_parent
check_rollback_asks_before_destroying_data
check_rollback_undoes_the_step
check_the_parents_survived_the_rollback
check_re_migrating_restores_the_schema
check_the_admin_manages_the_new_models

if port_is_free; then
	check_passed "the port is free when the chunk ends"
else
	check_failed "the port is free when the chunk ends" "held by $(port_listener_pids | tr '\n' ' ')"
fi

chunk_end
