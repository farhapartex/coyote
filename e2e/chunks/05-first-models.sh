#!/usr/bin/env bash

. "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib/bootstrap.sh"
. "$E2E_DIR/lib/appsource.sh"

chunk_begin "05" "The first models"

MIGRATION_NAME="reference_data"
MIGRATION_FILE="$EXAMPLE_DIR/migrations/0001_${MIGRATION_NAME}.go"

require_sqlite_or_skip() {
	if command -v sqlite3 >/dev/null 2>&1; then
		return 0
	fi
	check_skipped "$1" "sqlite3 is not installed"
	return 1
}

check_the_example_exists() {
	if [ -f "$EXAMPLE_DIR/main.go" ]; then
		return 0
	fi
	check_failed "the example application exists" "run chunk 03 first"
	chunk_end
}

reset_the_migration_ladder() {
	app_reset_migration_state
	note "migration ladder" \
		"reset: migration files, the snapshot and the database were removed so step one is tested from scratch"

	if [ -f "$(app_database_path)" ]; then
		check_failed "the project has no database before the first migration" \
			"$(app_database_path) survived the reset"
		return
	fi
	check_passed "the project has no database before the first migration"
	assert_equal "no migration files remain before step one" "0" \
		"$(app_migration_files | wc -l | tr -d ' ')"
}

write_the_first_models() {
	app_write_reference_models
	app_write_main 5

	assert_file_exists "the reference models are written" "$EXAMPLE_DIR/models_reference.go"

	cd "$EXAMPLE_DIR"
	assert_succeeds "the project compiles with the new models" go build ./...
	cd "$E2E_ROOT"
}

check_makemigrations_generates_a_named_file() {
	rm -f "$MIGRATION_FILE"

	app_run_command makemigrations "--name=$MIGRATION_NAME"

	if [ "$CAPTURED_STATUS" -ne 0 ]; then
		check_failed "makemigrations writes a migration" "exit $CAPTURED_STATUS
$(truncated_output "$CAPTURED_STDERR")"
		chunk_end
	fi
	check_passed "makemigrations writes a migration"

	assert_file_exists "--name decides the migration filename" "$MIGRATION_FILE"

	case "$APP_OUTPUT" in
	*"create table customers"*)
		check_passed "makemigrations reports the tables it will create"
		;;
	*)
		check_failed "makemigrations reports the tables it will create" \
			"$(truncated_output "$APP_OUTPUT")"
		;;
	esac
}

check_the_generated_migration_describes_the_models() {
	if [ ! -f "$MIGRATION_FILE" ]; then
		check_skipped "the migration describes both tables" "no migration file"
		return
	fi

	local expected
	for expected in \
		'Name: "customers"' \
		'Name: "ports"' \
		'{Name: "code", Kind: model.KindString, Size: 20, NotNull: true}' \
		'{Name: "code", Kind: model.KindString, Size: 5, NotNull: true}' \
		'Unique: true'; do
		assert_output_contains "the migration declares $expected" "$expected" cat "$MIGRATION_FILE"
	done

	assert_output_contains "the migration registers itself with an id" \
		"ID: \"0001_$MIGRATION_NAME\"" cat "$MIGRATION_FILE"

	cd "$EXAMPLE_DIR"
	assert_succeeds "the generated migration compiles" go build ./...
	cd "$E2E_ROOT"
}

check_the_framework_models_come_along() {
	if [ ! -f "$MIGRATION_FILE" ]; then
		check_skipped "the framework's own tables are migrated too" "no migration file"
		return
	fi

	local table
	for table in users roles permissions; do
		assert_output_contains "the migration creates the framework's $table table" \
			"Name: \"$table\"" cat "$MIGRATION_FILE"
	done
	note "registered models" "$(printf '%s' "$APP_OUTPUT" | sed -n 's/^models *\([0-9]*\).*/\1/p' | head -1) including the framework's auth tables"
}

check_sqlmigrate_prints_without_applying() {
	app_run_command sqlmigrate

	assert_equal "sqlmigrate exits cleanly" "0" "$CAPTURED_STATUS"

	case "$APP_OUTPUT" in
	*'CREATE TABLE "customers"'*)
		check_passed "sqlmigrate prints the SQL it would run"
		;;
	*)
		check_failed "sqlmigrate prints the SQL it would run" "$(truncated_output "$APP_OUTPUT")"
		;;
	esac

	case "$APP_OUTPUT" in
	*'CREATE UNIQUE INDEX "idx_customers_code"'*)
		check_passed "sqlmigrate prints the unique index"
		;;
	*)
		check_failed "sqlmigrate prints the unique index" "$(truncated_output "$APP_OUTPUT")"
		;;
	esac

	if require_sqlite_or_skip "sqlmigrate does not create the tables it prints"; then
		if app_table_exists customers; then
			check_failed "sqlmigrate does not create the tables it prints" \
				"the customers table exists after sqlmigrate alone"
		else
			check_passed "sqlmigrate does not create the tables it prints"
			note "sqlmigrate" "it does create the coyote_migrations ledger, so the database is not left untouched"
		fi
	fi
}

check_migrate_applies_the_migration() {
	app_run_command migrate

	assert_equal "migrate exits cleanly" "0" "$CAPTURED_STATUS"

	case "$APP_OUTPUT" in
	*"$MIGRATION_NAME"*)
		check_passed "migrate names the migration it applied"
		;;
	*)
		check_failed "migrate names the migration it applied" "$(truncated_output "$APP_OUTPUT")"
		;;
	esac

	case "$APP_OUTPUT" in
	*"permission"*)
		check_passed "migrate creates the permissions for the new models"
		;;
	*)
		check_failed "migrate creates the permissions for the new models" \
			"$(truncated_output "$APP_OUTPUT")"
		;;
	esac
}

check_the_schema_matches_the_models() {
	require_sqlite_or_skip "the customers table has the columns the model describes" || return

	local table
	for table in customers ports; do
		if app_table_exists "$table"; then
			check_passed "the $table table exists"
		else
			check_failed "the $table table exists" \
				"tables: $(app_sqlite_query "SELECT name FROM sqlite_master WHERE type='table';" | tr '\n' ' ')"
		fi
	done

	assert_equal "the customers table has the columns the model describes" \
		"code country created_at id name updated_at" \
		"$(app_table_columns customers | tr '\n' ' ' | sed 's/ $//')"

	assert_equal "the ports table has the columns the model describes" \
		"code country created_at id name updated_at" \
		"$(app_table_columns ports | tr '\n' ' ' | sed 's/ $//')"

	local indexes
	indexes="$(app_sqlite_query "SELECT name FROM sqlite_master WHERE type='index' AND tbl_name='customers';")"
	case "$indexes" in
	*idx_customers_code*)
		check_passed "the unique index on customers.code was created"
		;;
	*)
		check_failed "the unique index on customers.code was created" "indexes: $(printf '%s' "$indexes" | tr '\n' ' ')"
		;;
	esac
}

check_the_unique_index_is_enforced() {
	require_sqlite_or_skip "a duplicate customer code is refused by the database" || return

	app_sqlite_query "INSERT INTO customers (id, name, code, country, created_at, updated_at)
		VALUES ('probe-1', 'Probe', 'PROBE-DUP', 'NL', datetime('now'), datetime('now'));"

	local failure
	failure="$(sqlite3 "$(app_database_path)" "INSERT INTO customers (id, name, code, country, created_at, updated_at)
		VALUES ('probe-2', 'Probe Two', 'PROBE-DUP', 'NL', datetime('now'), datetime('now'));" 2>&1 || true)"

	case "$failure" in
	*UNIQUE*)
		check_passed "a duplicate customer code is refused by the database"
		;;
	*)
		check_failed "a duplicate customer code is refused by the database" \
			"the second insert was accepted: ${failure:-no error}"
		;;
	esac

	app_sqlite_query "DELETE FROM customers WHERE id IN ('probe-1','probe-2');"
}

check_migrate_is_idempotent() {
	app_run_command migrate

	assert_equal "a second migrate exits cleanly" "0" "$CAPTURED_STATUS"

	case "$APP_OUTPUT" in
	*"nothing to apply"*)
		check_passed "a second migrate applies nothing"
		;;
	*)
		check_failed "a second migrate applies nothing" "$(truncated_output "$APP_OUTPUT")"
		;;
	esac
}

check_makemigrations_detects_no_changes() {
	app_run_command makemigrations

	case "$APP_OUTPUT" in
	*"no model changes detected"*)
		check_passed "makemigrations writes nothing when the models have not changed"
		;;
	*)
		check_failed "makemigrations writes nothing when the models have not changed" \
			"$(truncated_output "$APP_OUTPUT")"
		;;
	esac

	assert_equal "no second migration file appeared" "1" "$(app_migration_files | wc -l | tr -d ' ')"
}

seed_the_reference_data() {
	require_sqlite_or_skip "reference rows are readable after the migration" || return

	app_sqlite_query "DELETE FROM customers; DELETE FROM ports;"
	app_sqlite_query "INSERT INTO customers (id, name, code, country, created_at, updated_at) VALUES
		('cus-nordwind', 'Nordwind Logistics', 'NORDWIND', 'DE', datetime('now'), datetime('now')),
		('cus-kestrel', 'Kestrel Trading', 'KESTREL', 'GB', datetime('now'), datetime('now')),
		('cus-alpine', 'Alpine Freight', 'ALPINE', 'CH', datetime('now'), datetime('now'));"
	app_sqlite_query "INSERT INTO ports (id, name, code, country, created_at, updated_at) VALUES
		('prt-rtm', 'Rotterdam', 'NLRTM', 'NL', datetime('now'), datetime('now')),
		('prt-sin', 'Singapore', 'SGSIN', 'SG', datetime('now'), datetime('now')),
		('prt-sha', 'Shanghai', 'CNSHA', 'CN', datetime('now'), datetime('now'));"

	assert_equal "three customers are readable after the migration" "3" "$(app_row_count customers)"
	assert_equal "three ports are readable after the migration" "3" "$(app_row_count ports)"
	note "seed data" "customers and ports seeded so later migrations can be checked for data loss"
}

check_the_example_exists
reset_the_migration_ladder
write_the_first_models
check_makemigrations_generates_a_named_file
check_the_generated_migration_describes_the_models
check_the_framework_models_come_along
check_sqlmigrate_prints_without_applying
check_migrate_applies_the_migration
check_the_schema_matches_the_models
check_the_unique_index_is_enforced
check_migrate_is_idempotent
check_makemigrations_detects_no_changes
seed_the_reference_data

chunk_end
