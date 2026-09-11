chunk_begin() {
	CHUNK_ID="$1"
	CHUNK_NAME="$2"
	CHUNK_STARTED="$(seconds_now)"
	CHUNK_STANDALONE=""

	if [ -z "${E2E_RESULTS:-}" ]; then
		CHUNK_STANDALONE=1
		E2E_RESULTS="$(mktemp "${TMPDIR:-/tmp}/coyote-e2e-XXXXXX")"
	fi

	record_result META "name" "$CHUNK_NAME"
	printf '\n%s%s. %s%s\n' "$C_BOLD" "$CHUNK_ID" "$CHUNK_NAME" "$C_RESET"
}

chunk_end() {
	local passed failed skipped found elapsed
	passed="$(count_results_of_kind PASS)"
	failed="$(count_results_of_kind FAIL)"
	skipped="$(count_results_of_kind SKIP)"
	found="$(count_results_of_kind FIND)"
	elapsed="$(($(seconds_now) - CHUNK_STARTED))"

	record_result META "duration" "$elapsed"
	record_result META "totals" "$passed/$((passed + failed))"

	if [ "$failed" -gt 0 ]; then
		printf '  %s%d failed%s, %d passed, %d skipped, %s%d finding(s)%s in %ds\n' \
			"$C_RED" "$failed" "$C_RESET" "$passed" "$skipped" "$C_YELLOW" "$found" "$C_RESET" "$elapsed"
	else
		printf '  %s%d passed%s, %d skipped, %s%d finding(s)%s in %ds\n' \
			"$C_GREEN" "$passed" "$C_RESET" "$skipped" "$C_YELLOW" "$found" "$C_RESET" "$elapsed"
	fi

	if [ -n "$CHUNK_STANDALONE" ]; then
		rm -f "$E2E_RESULTS"
	fi

	if [ "$failed" -gt 0 ]; then
		exit 1
	fi
	exit 0
}

require_previous_chunk() {
	local description="$1" path="$2"
	if [ -e "$path" ]; then
		return 0
	fi
	check_failed "$description" "expected $path to exist; run the earlier chunks first"
	chunk_end
}
