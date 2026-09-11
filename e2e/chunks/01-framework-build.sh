#!/usr/bin/env bash

. "$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/lib/bootstrap.sh"

chunk_begin "01" "The framework itself"

check_it_builds() {
	assert_succeeds "the framework compiles" go build ./...
	assert_output_empty "gofmt has nothing to say about core, contrib, lib and cmd" \
		gofmt -l core contrib lib cmd
	assert_succeeds "go vet is clean" go vet ./...
}

check_the_suite_passes() {
	assert_succeeds "the go test suite passes" go test ./tests/ -count=1
}

check_the_binary_builds() {
	mkdir -p "$E2E_BIN_DIR"
	assert_succeeds "the coyote command builds" go build -o "$COYOTE_BIN" ./cmd/coyote
	assert_file_executable "the coyote command is executable" "$COYOTE_BIN"
	assert_stdout_contains "coyote version reports itself" "coyote" "$COYOTE_BIN" version
	assert_stdout_contains "coyote help lists the worker command" "worker" "$COYOTE_BIN" help
}

check_it_builds
check_the_suite_passes
check_the_binary_builds

chunk_end
