#!/usr/bin/env bash

. "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib/bootstrap.sh"
. "$E2E_DIR/lib/appsource.sh"
. "$E2E_DIR/lib/server.sh"
. "$E2E_DIR/lib/http.sh"
. "$E2E_DIR/lib/admin.sh"

chunk_begin "10" "A unique index after the fact"

trap server_cleanup EXIT

REFERENCE_MIGRATION=""
COLLIDING_MIGRATION=""
STEPS_BEFORE=0

check_the_ladder_is_ready() {
	if app_table_exists tracking_events && [ "$(app_row_count shipments)" -ge 3 ]; then
		return 0
	fi
	check_failed "the deeper graph is in place" "run chunks 05 to 09 first"
	chunk_end
}

record_the_state_before() {
	STEPS_BEFORE="$(app_migration_files | wc -l | tr -d ' ')"
	SHIPMENTS_BEFORE="$(app_row_count shipments)"
	REFERENCES_BEFORE="$(app_sqlite_query "SELECT reference FROM shipments ORDER BY reference;" | tr '\n' ',')"

	assert_equal "the shipment references are already distinct" "$SHIPMENTS_BEFORE" \
		"$(app_sqlite_query "SELECT count(DISTINCT reference) FROM shipments;")"
	note "state before the unique index" "$STEPS_BEFORE migrations, $SHIPMENTS_BEFORE shipments"
}

check_making_an_index_unique_is_noticed() {
	app_backup_snapshot
	app_write_shipment_model 3 1

	cd "$EXAMPLE_DIR"
	assert_succeeds "the project compiles with the index marked unique" go build ./...
	cd "$E2E_ROOT"

	app_run_command makemigrations --name=reference_uniqueness

	case "$APP_OUTPUT" in
	*"no model changes detected"*)
		check_failed "changing an index to unique produces a migration" \
			"the tag changed from index to uniqueIndex and the project rebuilt, but makemigrations
reported no model changes, so an existing index can never be made unique through a migration
contrib/migrate/diff.go:130 matches indexes by name only: an index whose name is unchanged is
treated as unchanged, so neither Unique nor Columns is ever compared
snapshot.json does record \"unique\": true for other tables, so the information is present but unused"
		note "index diffing" \
			"the workaround is to give the index a new name, which the diff then sees as a drop and an add"
		;;
	*)
		check_passed "changing an index to unique produces a migration"
		;;
	esac

	app_write_shipment_model 3 0
	app_restore_snapshot
	app_gofmt
}

generate_the_renamed_unique_index() {
	local existed=0
	REFERENCE_MIGRATION="$(app_migration_for_name unique_reference)"
	if [ -n "$REFERENCE_MIGRATION" ]; then
		existed=1
	fi

	app_write_shipment_model 3 2
	cd "$EXAMPLE_DIR"
	assert_succeeds "the project compiles with a renamed unique index" go build ./...
	cd "$E2E_ROOT"

	app_run_command makemigrations --name=unique_reference
	assert_equal "makemigrations exits cleanly" "0" "$CAPTURED_STATUS"

	if [ "$existed" = "1" ]; then
		check_skipped "a renamed unique index is detected" "the migration is already present"
	else
		case "$APP_OUTPUT" in
		*"create unique index uq_shipments_reference"*)
			check_passed "a renamed unique index is detected"
			;;
		*)
			check_failed "a renamed unique index is detected" "$(truncated_output "$APP_OUTPUT")"
			;;
		esac

		case "$APP_OUTPUT" in
		*"drop index idx_shipments_reference"*)
			check_passed "the old non-unique index is dropped in the same step"
			;;
		*)
			check_failed "the old non-unique index is dropped in the same step" \
				"$(truncated_output "$APP_OUTPUT")"
			;;
		esac
	fi

	REFERENCE_MIGRATION="$(app_migration_for_name unique_reference)"
	assert_file_exists "a migration for the unique index is written" "$REFERENCE_MIGRATION"

	if [ -f "$REFERENCE_MIGRATION" ]; then
		assert_output_contains "the index is created with Unique set" \
			'Name: "uq_shipments_reference", Columns: []string{"reference"}, Unique: true' \
			cat "$REFERENCE_MIGRATION"
		assert_output_contains "the step is reversible" \
			'migrate.DropIndex{Table: "shipments", Name: "uq_shipments_reference"}' \
			cat "$REFERENCE_MIGRATION"
	fi
}

apply_the_unique_index() {
	app_run_command migrate
	assert_equal "the unique index applies to a populated table" "0" "$CAPTURED_STATUS"

	local indexes
	indexes="$(app_sqlite_query "SELECT name FROM sqlite_master WHERE type='index' AND tbl_name='shipments';" | tr '\n' ' ')"

	case "$indexes" in
	*uq_shipments_reference*)
		check_passed "the unique index exists in the database"
		;;
	*)
		check_failed "the unique index exists in the database" "indexes: $indexes"
		;;
	esac

	case "$indexes" in
	*idx_shipments_reference*)
		check_failed "the superseded index is gone" "idx_shipments_reference is still present"
		;;
	*)
		check_passed "the superseded index is gone"
		;;
	esac

	assert_equal "every shipment survived the index change" "$SHIPMENTS_BEFORE" "$(app_row_count shipments)"
	assert_equal "the references are unchanged" "$REFERENCES_BEFORE" \
		"$(app_sqlite_query "SELECT reference FROM shipments ORDER BY reference;" | tr '\n' ',')"
}

check_a_duplicate_is_now_refused() {
	app_sqlite_query "DELETE FROM shipments WHERE id='shp-duplicate';"

	local failure
	failure="$(sqlite3 "$(app_database_path)" "INSERT INTO shipments (id, reference, status, created_at, updated_at)
		VALUES ('shp-duplicate','MRF-000001','draft',datetime('now'),datetime('now'));" 2>&1 || true)"

	case "$failure" in
	*UNIQUE*)
		check_passed "a duplicate reference is refused by the database"
		;;
	*)
		check_failed "a duplicate reference is refused by the database" \
			"the insert was accepted: ${failure:-no error}"
		;;
	esac

	app_sqlite_query "DELETE FROM shipments WHERE id='shp-duplicate';"
	assert_equal "the refused insert left the table alone" "$SHIPMENTS_BEFORE" "$(app_row_count shipments)"
}

check_the_admin_reports_a_duplicate_without_a_crash() {
	if ! port_is_free; then
		force_free_the_port
	fi
	server_start
	if ! server_wait_for_http; then
		check_failed "the application serves with the unique index in place" "$(tail -20 "$SERVER_LOG")"
		return
	fi
	check_passed "the application serves with the unique index in place"

	if ! admin_login; then
		check_failed "the superadmin can sign in" "status $HTTP_STATUS"
		return
	fi

	http_get "/admin/shipments/new"
	admin_submit "/admin/shipments/new" \
		"reference=MRF-000001" "customer_id=cus-nordwind" "origin_id=prt-rtm" \
		"destination_id=prt-sin" "status=draft"

	if [ "$HTTP_STATUS" -ge 500 ]; then
		check_failed "a duplicate submitted through the admin is not a server error" \
			"status $HTTP_STATUS; a constraint violation should be reported to the user"
	else
		check_passed "a duplicate submitted through the admin is not a server error"
	fi
	note "duplicate through the admin" "status $HTTP_STATUS"

	assert_equal "the duplicate was not written" "$SHIPMENTS_BEFORE" "$(app_row_count shipments)"

	server_stop
}

check_a_unique_index_over_colliding_data_fails_cleanly() {
	app_backup_snapshot

	app_sqlite_query "DELETE FROM tracking_events WHERE id LIKE 'evt-collide-%';"
	app_sqlite_query "INSERT INTO tracking_events (id, shipment_id, container_id, kind, location, occurred_at, created_at)
		SELECT 'evt-collide-' || n.n, (SELECT id FROM shipments LIMIT 1), NULL,
		       'gate_in', 'Rotterdam', datetime('now'), datetime('now')
		FROM (SELECT 1 AS n UNION SELECT 2 UNION SELECT 3) n;"

	local colliding
	colliding="$(app_sqlite_query "SELECT count(*) FROM tracking_events WHERE kind='gate_in';")"
	if [ "$colliding" -lt 2 ]; then
		check_skipped "a unique index over colliding data fails cleanly" "could not create a collision"
		app_restore_snapshot
		return
	fi
	note "colliding rows" "$colliding tracking events share the kind gate_in"

	EVENTS_BEFORE="$(app_row_count tracking_events)"
	INDEXES_BEFORE="$(app_sqlite_query "SELECT name FROM sqlite_master WHERE type='index' AND tbl_name='tracking_events' ORDER BY name;" | tr '\n' ',')"

	app_write_container_models 2
	cd "$EXAMPLE_DIR"
	go build ./... >/dev/null 2>&1
	cd "$E2E_ROOT"

	app_run_command makemigrations --name=unique_kind
	COLLIDING_MIGRATION="$(app_migration_for_name unique_kind)"

	if [ -z "$COLLIDING_MIGRATION" ]; then
		check_skipped "a unique index over colliding data fails cleanly" \
			"the migration was not generated: $(truncated_output "$APP_OUTPUT")"
		app_write_container_models 0
		app_restore_snapshot
		app_gofmt
		return
	fi

	app_run_command migrate

	if [ "$CAPTURED_STATUS" -ne 0 ]; then
		check_passed "a unique index over colliding data fails cleanly"
	else
		check_failed "a unique index over colliding data fails cleanly" \
			"migrate exited 0 although $colliding rows share the same kind"
	fi

	case "$APP_OUTPUT" in
	*"UNIQUE constraint failed: tracking_events.kind"*)
		check_passed "the failure names the column that collides"
		;;
	*)
		check_failed "the failure names the column that collides" "$(truncated_output "$APP_OUTPUT")"
		;;
	esac

	assert_equal "the colliding rows are untouched" "$EVENTS_BEFORE" "$(app_row_count tracking_events)"
	assert_equal "the failed step is not recorded as applied" "0" \
		"$(app_sqlite_query "SELECT count(*) FROM coyote_migrations WHERE id LIKE '%unique_kind';")"

	assert_equal "the drop that ran first was rolled back with it" "$INDEXES_BEFORE" \
		"$(app_sqlite_query "SELECT name FROM sqlite_master WHERE type='index' AND tbl_name='tracking_events' ORDER BY name;" | tr '\n' ',')"

	local after
	after="$(app_sqlite_query "SELECT name FROM sqlite_master WHERE type='index' AND tbl_name='tracking_events';" | tr '\n' ' ')"
	case "$after" in
	*uq_tracking_events_kind*)
		check_failed "the half-created index is absent" "uq_tracking_events_kind exists after a failed migration"
		;;
	*)
		check_passed "the half-created index is absent"
		;;
	esac

	rm -f "$COLLIDING_MIGRATION"
	app_sqlite_query "DELETE FROM tracking_events WHERE id LIKE 'evt-collide-%';"
	app_write_container_models 0
	app_restore_snapshot

	cd "$EXAMPLE_DIR"
	assert_succeeds "the project builds again after backing the change out" go build ./...
	cd "$E2E_ROOT"
	assert_equal "the ladder is back to one added step" "$((STEPS_BEFORE + 1))" \
		"$(app_migration_files | wc -l | tr -d ' ')"
}

check_the_ladder_is_ready
record_the_state_before
check_making_an_index_unique_is_noticed
generate_the_renamed_unique_index
apply_the_unique_index
check_a_duplicate_is_now_refused
check_the_admin_reports_a_duplicate_without_a_crash
check_a_unique_index_over_colliding_data_fails_cleanly

if port_is_free; then
	check_passed "the port is free when the chunk ends"
else
	check_failed "the port is free when the chunk ends" "held by $(port_listener_pids | tr '\n' ' ')"
fi

chunk_end
