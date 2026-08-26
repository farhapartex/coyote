#!/usr/bin/env bash

. "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib/bootstrap.sh"

chunk_begin "01" "Framework build"

unformatted_framework_files() {
	gofmt -l . | grep -v "^$EXAMPLE_NAME/" || true
}

check_the_race_detector() {
	if [ -z "${E2E_RACE:-}" ]; then
		check_skipped "the suite is clean under the race detector" \
			"set E2E_RACE=1 to run it; it takes around 15s"
		return
	fi
	assert_succeeds "the suite is clean under the race detector" \
		go test ./tests/ -race -count=1
}

check_the_binary_reports_a_version() {
	local reported expected
	expected="$(sed -n 's/^const Version = "\(.*\)"$/\1/p' "$E2E_ROOT/core/app/app.go" | head -1)"

	if [ ! -x "$COYOTE_BIN" ]; then
		check_skipped "the built binary reports the version in core/app" "the binary was not built"
		return
	fi

	run_capturing "$COYOTE_BIN" version
	reported="$(printf '%s' "$CAPTURED_OUTPUT" | awk '{print $2}')"

	assert_equal "the built binary reports the version in core/app" "$expected" "$reported"
}

assert_output_empty "gofmt reports nothing unformatted" unformatted_framework_files
assert_succeeds "go vet is clean" go vet ./...
assert_succeeds "every package builds" go build ./...
assert_succeeds "the framework's own suite passes" go test ./tests/ -count=1
check_the_race_detector

rm -f "$COYOTE_BIN"
assert_succeeds "the coyote command builds" go build -o "$COYOTE_BIN" ./cmd/coyote
assert_file_exists "the coyote binary was written" "$COYOTE_BIN"
assert_file_executable "the coyote binary is executable" "$COYOTE_BIN"
check_the_binary_reports_a_version

chunk_end
