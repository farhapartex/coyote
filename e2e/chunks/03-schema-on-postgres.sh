#!/usr/bin/env bash

. "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib/bootstrap.sh"
. "$E2E_DIR/lib/services.sh"
. "$E2E_DIR/lib/shop.sh"

chunk_begin "03" "Schema on postgres"

require_previous_chunk "the shop exists" "$SHOP_DIR/settings.go"

check_migrations_are_written() {
	if ! shop_command makemigrations --name=catalogue; then
		check_failed "makemigrations writes the first migration" "$(truncated_output "$CAPTURED_OUTPUT")"
		chunk_end
	fi
	check_passed "makemigrations writes the first migration"

	local files
	files="$(shop_migration_files | wc -l | tr -d ' ')"
	assert_equal "exactly one migration file exists" "1" "$files"

	if shop_command makemigrations; then
		case "$CAPTURED_OUTPUT" in
		*"no model changes"* | *"no changes"* | *"up to date"*)
			check_passed "a second makemigrations finds no changes"
			;;
		*)
			finding "makemigrations does not say clearly that there is nothing to do" \
				"Running makemigrations twice with no model change printed:\n$(truncated_output "$CAPTURED_OUTPUT")\nA developer cannot tell from this whether a migration was written."
			;;
		esac
	else
		check_failed "a second makemigrations exits zero" "$(truncated_output "$CAPTURED_OUTPUT")"
	fi
}

check_the_migration_applies() {
	if shop_command migrate; then
		check_passed "migrate applies the schema to postgres"
	else
		check_failed "migrate applies the schema to postgres" "$(truncated_output "$CAPTURED_OUTPUT")"
		chunk_end
	fi

	local table
	for table in categories products users sessions jobs permissions roles; do
		if postgres_table_exists "$table"; then
			check_passed "postgres has the $table table"
		else
			check_failed "postgres has the $table table" \
				"to_regclass('public.$table') came back null"
		fi
	done
}

check_the_ledger_is_recorded() {
	if postgres_table_exists "coyote_migrations"; then
		check_passed "the migration ledger exists"
	else
		check_failed "the migration ledger exists" "no coyote_migrations table"
		return 0
	fi
	assert_equal "the ledger records one applied migration" "1" "$(postgres_row_count coyote_migrations)"

	if shop_command migrate; then
		check_passed "a second migrate is a no-op"
	else
		check_failed "a second migrate is a no-op" "$(truncated_output "$CAPTURED_OUTPUT")"
	fi
	assert_equal "the ledger still records one migration" "1" "$(postgres_row_count coyote_migrations)"
}

check_postgres_types_are_sensible() {
	local kind
	kind="$(postgres_query "SELECT data_type FROM information_schema.columns WHERE table_name='products' AND column_name='price_cents';")"
	assert_equal "an int64 price became bigint" "bigint" "$kind"

	kind="$(postgres_query "SELECT data_type FROM information_schema.columns WHERE table_name='products' AND column_name='created_at';")"
	case "$kind" in
	"timestamp with time zone") check_passed "a time.Time became timestamptz" ;;
	*) finding "a time.Time column is not timestamptz on postgres" \
		"products.created_at has type '$kind'. Without a time zone, timestamps written by one process and read by another in a different zone will not agree." ;;
	esac

	kind="$(postgres_query "SELECT data_type FROM information_schema.columns WHERE table_name='products' AND column_name='is_active';")"
	assert_equal "a bool became boolean" "boolean" "$kind"
}

check_the_superadmin_is_created() {
	if COYOTE_SUPERADMIN_USERNAME=root \
		COYOTE_SUPERADMIN_EMAIL=root@thornfield.test \
		COYOTE_SUPERADMIN_PASSWORD=thornfield-supply-2026 \
		shop_command createsuperadmin; then
		check_passed "createsuperadmin makes an account from the environment"
	else
		check_failed "createsuperadmin makes an account from the environment" "$(truncated_output "$CAPTURED_OUTPUT")"
	fi
	assert_equal "postgres has exactly one user" "1" "$(postgres_row_count users)"
	assert_equal "that user is a superadmin" "t" "$(postgres_query "SELECT is_superadmin FROM users LIMIT 1;")"
}

check_permissions_sync() {
	if shop_command syncpermissions; then
		check_passed "syncpermissions creates the model permissions"
	else
		check_failed "syncpermissions creates the model permissions" "$(truncated_output "$CAPTURED_OUTPUT")"
	fi
	local total
	total="$(postgres_row_count permissions)"
	if [ "${total:-0}" -ge 8 ]; then
		check_passed "there are at least four permissions per catalogue model ($total in total)"
	else
		check_failed "there are at least four permissions per catalogue model" "found $total"
	fi
}

check_migrations_are_written
check_the_migration_applies
check_the_ledger_is_recorded
check_postgres_types_are_sensible
check_the_superadmin_is_created
check_permissions_sync

chunk_end
