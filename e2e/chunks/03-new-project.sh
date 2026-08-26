#!/usr/bin/env bash

. "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib/bootstrap.sh"

chunk_begin "03" "A project from scratch"

SCAFFOLD_DIR="$E2E_WORK_DIR/scaffold"
SCAFFOLD_PROJECT="$SCAFFOLD_DIR/probe"

SCAFFOLDED_FILES="main.go
settings.go
migrations/migrations.go
templates/layouts/base.html
templates/pages/home.html
static/site.css
.env
.env.example
.gitignore
README.md"

check_the_binary_is_present() {
	if [ -x "$COYOTE_BIN" ]; then
		return 0
	fi
	check_failed "the coyote binary exists" "run chunk 01 first: $COYOTE_BIN is missing"
	chunk_end
}

scaffold_a_probe_project() {
	rm -rf "$SCAFFOLD_DIR"
	mkdir -p "$SCAFFOLD_DIR"
	cd "$SCAFFOLD_DIR"

	assert_exit_code "coyote new writes a project without touching the network" \
		0 "$COYOTE_BIN" new probe --skip-deps
	cd "$E2E_ROOT"
}

check_every_scaffolded_file_exists() {
	local relative_path
	while read -r relative_path; do
		assert_file_exists "coyote new writes $relative_path" "$SCAFFOLD_PROJECT/$relative_path"
	done <<<"$SCAFFOLDED_FILES"
}

check_the_scaffold_carries_no_comments() {
	local offenders
	offenders="$(grep -rn '//' "$SCAFFOLD_PROJECT" --include='*.go' | grep -v '://' || true)"

	if [ -z "$offenders" ]; then
		check_passed "the scaffolded Go files carry no comments"
		return
	fi
	check_failed "the scaffolded Go files carry no comments" "$offenders"
}

check_the_scaffold_is_formatted() {
	assert_output_empty "the scaffolded Go files are gofmt clean" gofmt -l "$SCAFFOLD_PROJECT"
}

check_the_scaffolded_gitignore() {
	local entry
	for entry in ".env" "*.db" "/media" "/staticfiles" "/probe"; do
		assert_output_contains "the scaffolded .gitignore covers $entry" "$entry" \
			cat "$SCAFFOLD_PROJECT/.gitignore"
	done
}

check_the_generated_secret_key() {
	local generated placeholder
	generated="$(sed -n 's/^SECRET_KEY="\(.*\)"$/\1/p' "$SCAFFOLD_PROJECT/.env")"
	placeholder="$(sed -n 's/^SECRET_KEY="\(.*\)"$/\1/p' "$SCAFFOLD_PROJECT/.env.example")"

	if [ "${#generated}" -ge 32 ]; then
		check_passed "the generated .env holds a secret key of at least 32 characters"
	else
		check_failed "the generated .env holds a secret key of at least 32 characters" \
			"found ${#generated} characters"
	fi

	if [ "$generated" != "$placeholder" ]; then
		check_passed ".env.example carries a placeholder, not the real key"
	else
		check_failed ".env.example carries a placeholder, not the real key" \
			"both files hold the same value"
	fi
}

check_two_projects_get_different_keys() {
	local second
	cd "$SCAFFOLD_DIR"
	"$COYOTE_BIN" new second --skip-deps >/dev/null 2>&1
	cd "$E2E_ROOT"

	second="$(sed -n 's/^SECRET_KEY="\(.*\)"$/\1/p' "$SCAFFOLD_DIR/second/.env")"
	assert_equal "two scaffolded projects do not share a secret key" \
		"different" "$([ "$second" != "$(sed -n 's/^SECRET_KEY="\(.*\)"$/\1/p' "$SCAFFOLD_PROJECT/.env")" ] && echo different || echo identical)"
}

check_the_module_flag_reaches_the_source() {
	cd "$SCAFFOLD_DIR"
	"$COYOTE_BIN" new shop --module=example.test/shop --skip-deps >/dev/null 2>&1
	cd "$E2E_ROOT"

	assert_output_contains "--module reaches the migrations import" \
		'_ "example.test/shop/migrations"' cat "$SCAFFOLD_DIR/shop/main.go"
}

check_a_used_directory_is_protected() {
	mkdir -p "$SCAFFOLD_DIR/taken"
	printf 'keep me\n' >"$SCAFFOLD_DIR/taken/keep.txt"
	cd "$SCAFFOLD_DIR"

	assert_exit_code "coyote new refuses a directory that is not empty" \
		1 "$COYOTE_BIN" new taken --skip-deps
	assert_exit_code "coyote new --force scaffolds into it anyway" \
		0 "$COYOTE_BIN" new taken --skip-deps --force

	cd "$E2E_ROOT"
	assert_file_exists "--force leaves existing files alone" "$SCAFFOLD_DIR/taken/keep.txt"
	assert_file_exists "--force still writes the project" "$SCAFFOLD_DIR/taken/main.go"
}

wire_module_to_the_local_checkout() {
	local project_dir="$1" module_path="$2" replace_path="$3"

	cd "$project_dir"
	go mod init "$module_path" >/dev/null 2>&1
	go mod edit -require=github.com/farhapartex/coyote@v0.0.0
	go mod edit -replace="github.com/farhapartex/coyote=$replace_path"
	local status=0
	go mod tidy >/dev/null 2>&1 || status=$?
	cd "$E2E_ROOT"
	return $status
}

check_the_scaffold_compiles() {
	if ! wire_module_to_the_local_checkout "$SCAFFOLD_PROJECT" probe ../../../..; then
		check_failed "the scaffolded project resolves its dependencies" "go mod tidy failed"
		return
	fi
	check_passed "the scaffolded project resolves its dependencies"

	cd "$SCAFFOLD_PROJECT"
	assert_succeeds "the scaffolded project compiles as it stands" go build ./...
	assert_succeeds "the scaffolded project passes go vet" go vet ./...
	cd "$E2E_ROOT"
}

create_the_example_application() {
	if [ -d "$EXAMPLE_DIR" ]; then
		note "example directory" "already present, reused; run with --fresh to build it from scratch"
		return 0
	fi

	cd "$E2E_ROOT"
	assert_exit_code "coyote new creates the example application" \
		0 "$COYOTE_BIN" new "$EXAMPLE_NAME" --skip-deps

	if ! wire_module_to_the_local_checkout "$EXAMPLE_DIR" "$EXAMPLE_NAME" ..; then
		check_failed "the example application resolves its dependencies" "go mod tidy failed"
		return 1
	fi
	check_passed "the example application resolves its dependencies"
}

check_the_example_application_builds() {
	if [ ! -f "$EXAMPLE_DIR/go.mod" ]; then
		check_failed "the example application has a go.mod" "no go.mod in $EXAMPLE_DIR"
		return
	fi

	assert_output_contains "the example resolves the framework from this checkout" \
		"replace github.com/farhapartex/coyote => .." cat "$EXAMPLE_DIR/go.mod"

	cd "$EXAMPLE_DIR"
	assert_succeeds "the example application compiles" go build ./...
	assert_succeeds "the example application passes go vet" go vet ./...
	cd "$E2E_ROOT"
}

check_the_example_is_ignored_by_git() {
	local tracked
	tracked="$(git -C "$E2E_ROOT" ls-files "$EXAMPLE_NAME" | head -5)"

	if [ -z "$tracked" ]; then
		check_passed "the generated example is not tracked by git"
	else
		check_failed "the generated example is not tracked by git" "still tracked:
$tracked"
	fi

	if git -C "$E2E_ROOT" check-ignore -q "$EXAMPLE_NAME"; then
		check_passed "the example directory is covered by .gitignore"
	else
		check_failed "the example directory is covered by .gitignore" \
			"add /$EXAMPLE_NAME/ to .gitignore"
	fi
}

check_the_binary_is_present
scaffold_a_probe_project
check_every_scaffolded_file_exists
check_the_scaffold_carries_no_comments
check_the_scaffold_is_formatted
check_the_scaffolded_gitignore
check_the_generated_secret_key
check_two_projects_get_different_keys
check_the_module_flag_reaches_the_source
check_a_used_directory_is_protected
check_the_scaffold_compiles
create_the_example_application
check_the_example_application_builds
check_the_example_is_ignored_by_git

rm -rf "$SCAFFOLD_DIR"

chunk_end
