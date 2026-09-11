#!/usr/bin/env bash

. "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib/bootstrap.sh"
. "$E2E_DIR/lib/services.sh"
. "$E2E_DIR/lib/shop.sh"

chunk_begin "02" "A shop from scratch"

SECRET_KEY=""

check_the_project_is_scaffolded() {
	if [ -d "$SHOP_DIR" ]; then
		note "shop directory" "already present, reused; run with --fresh to rebuild it"
	else
		run_capturing "$COYOTE_BIN" new "$SHOP_NAME" --skip-deps
		if [ "$CAPTURED_STATUS" -ne 0 ]; then
			check_failed "coyote new scaffolds a project" "$(truncated_output "$CAPTURED_OUTPUT")"
			chunk_end
		fi
		check_passed "coyote new scaffolds a project"
	fi

	assert_file_exists "the scaffold wrote settings.go" "$SHOP_DIR/settings.go"
	assert_file_exists "the scaffold wrote a git-ignored .env" "$SHOP_DIR/.env"

	SECRET_KEY="$(sed -n 's/^SECRET_KEY="\{0,1\}\([^"]*\)"\{0,1\}$/\1/p' "$SHOP_DIR/.env" | head -1)"
	if [ -n "$SECRET_KEY" ] && [ "${#SECRET_KEY}" -ge 32 ]; then
		check_passed "the scaffold generated a SecretKey of at least 32 characters"
	else
		check_failed "the scaffold generated a SecretKey of at least 32 characters" \
			"got ${#SECRET_KEY} characters"
		SECRET_KEY="e2e-shop-secret-key-long-enough-to-pass-validation"
	fi
}

check_the_shop_source_replaces_the_scaffold() {
	shop_apply_common
	shop_apply_stage 1
	shop_env_file "$SECRET_KEY"

	assert_file_exists "the shop settings are in place" "$SHOP_DIR/settings.go"
	assert_file_exists "the catalogue models are in place" "$SHOP_DIR/models_catalog.go"
	assert_file_exists "the custom stylesheet is in place" "$SHOP_DIR/static/shop.css"
	assert_file_exists "the layout is in place" "$SHOP_DIR/templates/layouts/base.html"

	if grep -q "settings.Postgres" "$SHOP_DIR/settings.go"; then
		check_passed "the shop is configured for postgres"
	else
		check_failed "the shop is configured for postgres" "settings.go does not mention settings.Postgres"
	fi
	if grep -q "RedisCache" "$SHOP_DIR/settings.go"; then
		check_passed "the shop is configured for a redis cache"
	else
		check_failed "the shop is configured for a redis cache" "settings.go does not mention RedisCache"
	fi
}

check_it_resolves_and_compiles() {
	if shop_wire_module; then
		check_passed "the shop resolves the framework from this checkout"
		assert_file_exists "go.mod exists once the module is initialised" "$SHOP_DIR/go.mod"
	else
		check_failed "the shop resolves the framework from this checkout" "go mod tidy failed"
		chunk_end
	fi

	run_capturing shop_build
	if [ "$CAPTURED_STATUS" -eq 0 ]; then
		check_passed "the shop compiles"
	else
		check_failed "the shop compiles" "$(truncated_output "$CAPTURED_OUTPUT")"
		chunk_end
	fi

	cd "$SHOP_DIR"
	assert_succeeds "the shop passes go vet" go vet ./...
	assert_output_empty "the shop is gofmt clean" gofmt -l .
	cd "$E2E_ROOT"
}

record_framework_findings() {
	finding "model.Record has no numeric accessor, so every application writes its own" \
		"model.Record offers Get, String and Bool and nothing else. An ecommerce catalogue reads
price_cents and stock on every page, and the generic store hands them back as an untyped any whose
concrete type depends on the driver. The shop had to add its own recordInt() with a type switch over
int64, int, int32, float64, string and []byte before it would compile.
This is worse than an inconvenience: the concrete type is engine-dependent, so code that works on
SQLite can silently return 0 on postgres or mysql. Record.Int, Record.Float and Record.Time belong
beside Record.String and Record.Bool in core/model/record.go."
}

check_no_comments_in_the_scaffolded_output() {
	local offenders
	offenders="$(grep -rn "^[[:space:]]*//" "$SHOP_DIR"/*.go 2>/dev/null | grep -v "go:embed" | head -5 || true)"
	if [ -z "$offenders" ]; then
		check_passed "no comments in the shop's Go source"
	else
		check_failed "no comments in the shop's Go source" "$offenders"
	fi
}

check_the_project_is_scaffolded
check_the_shop_source_replaces_the_scaffold
check_it_resolves_and_compiles
check_no_comments_in_the_scaffolded_output
record_framework_findings

chunk_end
