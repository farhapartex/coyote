RESULTS_DIR="$E2E_WORK_DIR/results"

report_open() {
	mkdir -p "$RESULTS_DIR"
	REPORT_STARTED="$(seconds_now)"
}

report_clear_all() {
	rm -rf "$RESULTS_DIR"
	mkdir -p "$RESULTS_DIR"
}

report_results_path() {
	printf '%s/%s.result' "$RESULTS_DIR" "$1"
}

report_stamp_chunk() {
	local id="$1" status="$2" file
	file="$(report_results_path "$id")"

	printf 'META%sstatus%s%s\n' "$E2E_SEP" "$E2E_SEP" "$status" >>"$file"
	printf 'META%swhen%s%s\n' "$E2E_SEP" "$E2E_SEP" "$(date '+%Y-%m-%d %H:%M')" >>"$file"
	printf 'META%scommit%s%s\n' "$E2E_SEP" "$E2E_SEP" "$(repository_commit)" >>"$file"
}

report_meta_value() {
	local file="$1" key="$2"
	awk -F"$E2E_SEP" -v key="$key" '$1 == "META" && $2 == key {print $3}' "$file" 2>/dev/null | tail -1
}

report_status_label() {
	case "$1" in
	pass) printf 'pass' ;;
	fail) printf '**fail**' ;;
	blocked) printf '_blocked_' ;;
	aborted) printf '**stopped early**' ;;
	none) printf '_never run_' ;;
	*) printf '_incomplete_' ;;
	esac
}

report_recorded_commits() {
	local file
	for file in "$RESULTS_DIR"/*.result; do
		[ -f "$file" ] || continue
		report_meta_value "$file" commit
	done | sort -u
}

report_render() {
	local elapsed dirty
	elapsed="$(($(seconds_now) - REPORT_STARTED))"
	dirty=""
	if repository_is_dirty; then
		dirty=" · working tree dirty"
	fi

	{
		printf '# Coyote end-to-end run\n\n'
		printf 'Last pass %s%s · go %s · %s · %ds\n\n' \
			"$(date '+%Y-%m-%d %H:%M')" "$dirty" "$(go_version)" "$(platform_name)" "$elapsed"
		printf 'Every chunk keeps its own findings here. A chunk section is rewritten only when that\n'
		printf 'chunk runs again, so this stays a whole-suite report after `--only` or `--from`.\n\n'

		report_render_summary
		report_render_totals
		report_render_consistency
		report_render_sections
	} >"$E2E_REPORT"
}

report_render_summary() {
	printf '| # | Chunk | Checks | Result | Recorded |\n'
	printf '| --- | --- | --- | --- | --- |\n'

	local script id name file status totals when
	for script in $(chunk_scripts); do
		id="$(chunk_id_of "$script")"
		name="$(chunk_name_of "$script")"
		file="$(report_results_path "$id")"

		if [ ! -f "$file" ]; then
			printf '| %s | %s | -- | %s | -- |\n' "$id" "$name" "$(report_status_label none)"
			continue
		fi

		status="$(report_meta_value "$file" status)"
		totals="$(report_meta_value "$file" totals)"
		when="$(report_meta_value "$file" when)"
		if [ -z "$totals" ]; then
			status="aborted"
		fi
		printf '| %s | %s | %s | %s | %s |\n' \
			"$id" "$name" "${totals:---}" "$(report_status_label "$status")" "${when:---}"
	done
	printf '\n'
}

report_render_totals() {
	local script id file status chunks=0 passed=0 failed=0 missing=0

	for script in $(chunk_scripts); do
		chunks=$((chunks + 1))
		id="$(chunk_id_of "$script")"
		file="$(report_results_path "$id")"
		if [ ! -f "$file" ]; then
			missing=$((missing + 1))
			continue
		fi
		status="$(report_meta_value "$file" status)"
		case "$status" in
		pass) passed=$((passed + 1)) ;;
		fail) failed=$((failed + 1)) ;;
		esac
	done

	printf '%s of %s chunks passing' "$passed" "$chunks"
	if [ "$failed" -gt 0 ]; then
		printf ', %s failing' "$failed"
	fi
	if [ "$missing" -gt 0 ]; then
		printf ', %s never run' "$missing"
	fi
	printf '.\n'
}

report_render_consistency() {
	local commits count
	commits="$(report_recorded_commits | tr '\n' ' ' | sed 's/ $//')"
	count="$(report_recorded_commits | wc -l | tr -d ' ')"

	if [ "$count" -gt 1 ]; then
		printf '\nThese results were recorded at more than one commit (%s), so they do not describe a\n' "$commits"
		printf 'single state of the tree. Run `./e2e/run.sh` for one consistent picture.\n'
	fi
	printf '\n'
}

report_render_sections() {
	local script id name file status

	for script in $(chunk_scripts); do
		id="$(chunk_id_of "$script")"
		name="$(chunk_name_of "$script")"
		file="$(report_results_path "$id")"

		if [ ! -f "$file" ]; then
			printf '\n## %s. %s — never run\n\n' "$id" "$name"
			printf 'No results recorded yet.\n'
			continue
		fi

		status="$(report_meta_value "$file" status)"
		printf '\n## %s. %s — %s\n\n' "$id" "$name" "${status:-unknown}"
		printf '_Recorded %s at commit %s._\n\n' \
			"$(report_meta_value "$file" when)" "$(report_meta_value "$file" commit)"
		report_render_checks "$file"
	done
}

report_render_checks() {
	local file="$1" kind description detail

	while IFS="$E2E_SEP" read -r kind description detail; do
		case "$kind" in
		PASS)
			printf -- '- [x] %s\n' "$description"
			;;
		FAIL)
			printf -- '- [ ] **%s**\n' "$description"
			if [ -n "$detail" ]; then
				printf '%b\n' "$detail" | sed 's/^/      /'
			fi
			;;
		SKIP)
			printf -- '- [ ] %s _(skipped: %s)_\n' "$description" "${detail:-no reason given}"
			;;
		NOTE)
			printf -- '- %s: `%s`\n' "$description" "$detail"
			;;
		esac
	done <"$file"
}
