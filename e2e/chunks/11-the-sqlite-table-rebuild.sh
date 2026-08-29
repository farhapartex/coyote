#!/usr/bin/env bash

. "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib/bootstrap.sh"
. "$E2E_DIR/lib/appsource.sh"

chunk_begin "11" "The SQLite table rebuild"

REBUILD_MIGRATION=""
STEPS_BEFORE=0

check_the_ladder_is_ready() {
	if app_table_exists containers && [ "$(app_row_count shipments)" -ge 3 ]; then
		return 0
	fi
	check_failed "the graph and its rows are in place" "run chunks 05 to 10 first"
	chunk_end
}

record_the_state_before() {
	STEPS_BEFORE="$(app_migration_files | wc -l | tr -d ' ')"
	SHIPMENTS_BEFORE="$(app_row_count shipments)"
	ROWS_BEFORE="$(app_sqlite_query "SELECT reference || '|' || status || '|' || ifnull(customs_notes,'') || '|' || ifnull(declared_value,'') FROM shipments ORDER BY reference;" | tr '\n' ';')"
	INDEXES_BEFORE="$(app_sqlite_query "SELECT name FROM sqlite_master WHERE type='index' AND tbl_name='shipments' ORDER BY name;" | tr '\n' ',')"

	app_sqlite_query "DELETE FROM containers WHERE id='con-orphan';"
	CHILDREN_BEFORE="$(app_row_count containers)"

	note "state before the rebuild" \
		"$SHIPMENTS_BEFORE shipments, $CHILDREN_BEFORE containers, $STEPS_BEFORE migrations"

	case "$INDEXES_BEFORE" in
	*uq_shipments_reference*)
		check_passed "the unique index from the previous step is in place"
		;;
	*)
		check_failed "the unique index from the previous step is in place" "indexes: $INDEXES_BEFORE"
		;;
	esac
}

generate_the_rebuild_migration() {
	local existed=0
	REBUILD_MIGRATION="$(app_migration_for_name widen_reference)"
	if [ -n "$REBUILD_MIGRATION" ]; then
		existed=1
	fi

	app_write_shipment_model 3 2 40
	cd "$EXAMPLE_DIR"
	assert_succeeds "the project compiles with the widened column" go build ./...
	cd "$E2E_ROOT"

	app_run_command makemigrations --name=widen_reference
	assert_equal "makemigrations exits cleanly" "0" "$CAPTURED_STATUS"

	if [ "$existed" = "1" ]; then
		check_skipped "makemigrations emits an AlterColumn" "the migration is already present"
	else
		case "$APP_OUTPUT" in
		*"alter column shipments.reference"*)
			check_passed "makemigrations emits an AlterColumn"
			;;
		*)
			check_failed "makemigrations emits an AlterColumn" "$(truncated_output "$APP_OUTPUT")"
			;;
		esac
	fi

	REBUILD_MIGRATION="$(app_migration_for_name widen_reference)"
	assert_file_exists "a migration for the altered column is written" "$REBUILD_MIGRATION"

	if [ -f "$REBUILD_MIGRATION" ]; then
		assert_output_contains "the AlterColumn carries the whole table definition" \
			'Columns: []migrate.Column{{Name: "id"' cat "$REBUILD_MIGRATION"
		assert_output_contains "the AlterColumn carries the indexes to recreate" \
			'uq_shipments_reference' cat "$REBUILD_MIGRATION"
		assert_output_contains "the step is reversible back to the old width" \
			'To: migrate.Column{Name: "reference", Kind: model.KindString, Size: 30' cat "$REBUILD_MIGRATION"
	fi
}

check_the_generated_migration_compiles() {
	if [ ! -f "$REBUILD_MIGRATION" ]; then
		check_skipped "the generated AlterColumn migration compiles" "no migration file"
		return
	fi

	cd "$EXAMPLE_DIR"
	run_capturing go build ./...
	cd "$E2E_ROOT"

	if [ "$CAPTURED_STATUS" -eq 0 ]; then
		check_passed "the generated AlterColumn migration compiles"
		return
	fi

	case "$CAPTURED_OUTPUT" in
	*"undefined: model"*)
		check_failed "the generated AlterColumn migration compiles" \
			"the file uses model.KindString but never imports core/model, so the project will not build
contrib/migrate/generate.go:125 usesModelPackage lists only CreateTable and AddColumn, while
AlterColumn.Source() also emits model.Kind values; a migration made only of AlterColumn ops is
therefore dead on arrival and migrate cannot even run
$(truncated_output "$CAPTURED_OUTPUT")"
		note "workaround" \
			"the harness adds the missing import so the rest of the rebuild can be tested"
		;;
	*)
		check_failed "the generated AlterColumn migration compiles" "$(truncated_output "$CAPTURED_OUTPUT")"
		;;
	esac

	if app_add_missing_model_import "$REBUILD_MIGRATION"; then
		cd "$EXAMPLE_DIR"
		assert_succeeds "adding the missing import is enough to make it build" go build ./...
		cd "$E2E_ROOT"
	fi
}

check_sqlmigrate_can_preview_a_rebuild() {
	app_run_command sqlmigrate

	case "$APP_OUTPUT" in
	*"CREATE TABLE"*)
		check_passed "sqlmigrate previews the statements a rebuild will run"
		;;
	*"widen_reference"*)
		check_failed "sqlmigrate previews the statements a rebuild will run" \
			"sqlmigrate printed the migration header and no statements at all
a rebuild is assembled by the dialect at apply time, so the one operation where previewing the
SQL matters most cannot be previewed
$(truncated_output "$APP_OUTPUT")"
		;;
	*)
		check_failed "sqlmigrate previews the statements a rebuild will run" \
			"$(truncated_output "$APP_OUTPUT")"
		;;
	esac
}

apply_the_rebuild() {
	app_run_command migrate
	assert_equal "the rebuild applies cleanly" "0" "$CAPTURED_STATUS"

	if [ "$CAPTURED_STATUS" -ne 0 ]; then
		check_failed "the shipments table survived the rebuild" "$(truncated_output "$APP_OUTPUT")"
		chunk_end
	fi

	assert_equal "every shipment survived the rebuild" "$SHIPMENTS_BEFORE" "$(app_row_count shipments)"
	assert_equal "every column value survived the rebuild byte for byte" "$ROWS_BEFORE" \
		"$(app_sqlite_query "SELECT reference || '|' || status || '|' || ifnull(customs_notes,'') || '|' || ifnull(declared_value,'') FROM shipments ORDER BY reference;" | tr '\n' ';')"
	assert_equal "the indexes were all recreated" "$INDEXES_BEFORE" \
		"$(app_sqlite_query "SELECT name FROM sqlite_master WHERE type='index' AND tbl_name='shipments' ORDER BY name;" | tr '\n' ',')"

	local failure
	failure="$(sqlite3 "$(app_database_path)" "INSERT INTO shipments (id, reference, status, created_at, updated_at)
		VALUES ('shp-after-rebuild','MRF-000001','draft',datetime('now'),datetime('now'));" 2>&1 || true)"
	case "$failure" in
	*UNIQUE*)
		check_passed "the unique constraint still bites after the rebuild"
		;;
	*)
		check_failed "the unique constraint still bites after the rebuild" \
			"a duplicate reference was accepted: ${failure:-no error}"
		;;
	esac
	app_sqlite_query "DELETE FROM shipments WHERE id='shp-after-rebuild';"

	assert_equal "the child rows still resolve to their parents" "0" \
		"$(app_sqlite_query "SELECT count(*) FROM containers c LEFT JOIN shipments s ON s.id = c.shipment_id WHERE c.shipment_id IS NOT NULL AND s.id IS NULL;")"
	assert_equal "no container was lost" "$CHILDREN_BEFORE" "$(app_row_count containers)"

	note "column width in SQLite" \
		"the DDL records TEXT with no length, so the widening is visible in the migration and the snapshot rather than in the database"
}

check_the_foreign_key_guard_can_fire() {
	app_sqlite_query "DELETE FROM containers WHERE id='con-orphan';"
	app_sqlite_query "INSERT INTO containers (id, number, shipment_id, size_feet, sealed, created_at, updated_at)
		VALUES ('con-orphan','MSCU0000001','no-such-shipment',20,0,datetime('now'),datetime('now'));"

	local orphans
	orphans="$(app_sqlite_query "SELECT count(*) FROM containers c LEFT JOIN shipments s ON s.id = c.shipment_id WHERE c.shipment_id IS NOT NULL AND s.id IS NULL;")"
	if [ "$orphans" -lt 1 ]; then
		check_skipped "the rebuild's foreign key check notices a dangling child" "could not create an orphan"
		return
	fi
	note "orphan planted" "$orphans container points at a shipment that does not exist"

	app_run_command rollback --no-input

	if [ "$CAPTURED_STATUS" -ne 0 ]; then
		check_passed "the rebuild's foreign key check notices a dangling child"
	else
		check_failed "the rebuild's foreign key check notices a dangling child" \
			"the rebuild completed and was recorded with a dangling child row present
contrib/migrate/runner.go:158 runs PRAGMA foreign_key_check after a rebuild, but the generated
schema declares no REFERENCES clauses, so the check has nothing to check and always returns clean
the guard is wired to a constraint the framework never emits"
		note "inert guard" \
			"the foreign_key_check will only ever fire once contrib/migrate emits foreign keys"
	fi

	app_sqlite_query "DELETE FROM containers WHERE id='con-orphan';"
}

check_the_rebuild_reverses_and_reapplies() {
	if [ -n "$(app_sqlite_query "SELECT id FROM coyote_migrations WHERE id LIKE '%widen_reference';")" ]; then
		app_run_command rollback --no-input
		assert_equal "the rebuild can be rolled back" "0" "$CAPTURED_STATUS"
	else
		check_passed "the rebuild can be rolled back"
	fi

	assert_equal "the rows survived the reverse rebuild" "$SHIPMENTS_BEFORE" "$(app_row_count shipments)"
	assert_equal "the values survived the reverse rebuild" "$ROWS_BEFORE" \
		"$(app_sqlite_query "SELECT reference || '|' || status || '|' || ifnull(customs_notes,'') || '|' || ifnull(declared_value,'') FROM shipments ORDER BY reference;" | tr '\n' ';')"

	app_run_command migrate
	assert_equal "the rebuild re-applies after a rollback" "0" "$CAPTURED_STATUS"
	assert_equal "the rows survived the round trip" "$SHIPMENTS_BEFORE" "$(app_row_count shipments)"
	assert_equal "the indexes survived the round trip" "$INDEXES_BEFORE" \
		"$(app_sqlite_query "SELECT name FROM sqlite_master WHERE type='index' AND tbl_name='shipments' ORDER BY name;" | tr '\n' ',')"
	assert_equal "the ladder gained one step" "$((STEPS_BEFORE + 1))" \
		"$(app_migration_files | wc -l | tr -d ' ')"
}

check_the_ladder_is_ready
record_the_state_before
generate_the_rebuild_migration
check_the_generated_migration_compiles
check_sqlmigrate_can_preview_a_rebuild
apply_the_rebuild
check_the_foreign_key_guard_can_fire
check_the_rebuild_reverses_and_reapplies

chunk_end
