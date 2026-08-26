report_reset() {
	: >"$E2E_WORK_DIR/summary.tsv"
	: >"$E2E_WORK_DIR/details.md"
	REPORT_STARTED="$(seconds_now)"
}

report_meta_value() {
	local file="$1" key="$2"
	awk -F"$E2E_SEP" -v key="$key" '$1 == "META" && $2 == key {print $3}' "$file" | tail -1
}

report_add_chunk() {
	local id="$1" file="$2" status="$3"
	local name totals duration

	name="$(report_meta_value "$file" name)"
	totals="$(report_meta_value "$file" totals)"
	duration="$(report_meta_value "$file" duration)"

	printf '%s\t%s\t%s\t%s\t%s\n' "$id" "$name" "${totals:---}" "$status" "${duration:-0}" \
		>>"$E2E_WORK_DIR/summary.tsv"

	report_render_chunk_section "$id" "$name" "$file" "$status" >>"$E2E_WORK_DIR/details.md"
}

report_add_blocked_chunk() {
	local id="$1" name="$2" reason="$3"

	printf '%s\t%s\t%s\t%s\t%s\n' "$id" "$name" "--" "blocked" "0" >>"$E2E_WORK_DIR/summary.tsv"
	{
		printf '\n## %s. %s — blocked\n\n' "$id" "$name"
		printf 'Not run: %s\n' "$reason"
	} >>"$E2E_WORK_DIR/details.md"
}

report_render_chunk_section() {
	local id="$1" name="$2" file="$3" status="$4"

	printf '\n## %s. %s — %s\n\n' "$id" "$name" "$status"

	local kind description detail
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

report_status_label() {
	case "$1" in
	pass) printf 'pass' ;;
	fail) printf '**fail**' ;;
	blocked) printf '_blocked_' ;;
	*) printf '%s' "$1" ;;
	esac
}

report_render() {
	local elapsed dirty
	local row_id row_name row_totals row_status row_duration
	elapsed="$(($(seconds_now) - REPORT_STARTED))"
	dirty=""
	if repository_is_dirty; then
		dirty=" · working tree dirty"
	fi

	{
		printf '# Coyote end-to-end run\n\n'
		printf '%s · commit `%s`%s · go %s · %s · %ds\n\n' \
			"$(date '+%Y-%m-%d %H:%M')" "$(repository_commit)" "$dirty" \
			"$(go_version)" "$(platform_name)" "$elapsed"

		printf '| # | Chunk | Checks | Result |\n'
		printf '| --- | --- | --- | --- |\n'
		while IFS=$'\t' read -r row_id row_name row_totals row_status row_duration; do
			printf '| %s | %s | %s | %s |\n' \
				"$row_id" "$row_name" "$row_totals" "$(report_status_label "$row_status")"
		done <"$E2E_WORK_DIR/summary.tsv"

		report_render_totals
		cat "$E2E_WORK_DIR/details.md"
	} >"$E2E_REPORT"
}

report_render_totals() {
	local chunks passed failed blocked
	chunks="$(wc -l <"$E2E_WORK_DIR/summary.tsv" | tr -d ' ')"
	passed="$(awk -F'\t' '$4 == "pass"' "$E2E_WORK_DIR/summary.tsv" | wc -l | tr -d ' ')"
	failed="$(awk -F'\t' '$4 == "fail"' "$E2E_WORK_DIR/summary.tsv" | wc -l | tr -d ' ')"
	blocked="$(awk -F'\t' '$4 == "blocked"' "$E2E_WORK_DIR/summary.tsv" | wc -l | tr -d ' ')"

	printf '\n%s of %s chunks passed' "$passed" "$chunks"
	if [ "$failed" -gt 0 ]; then
		printf ', %s failed' "$failed"
	fi
	if [ "$blocked" -gt 0 ]; then
		printf ', %s blocked' "$blocked"
	fi
	printf '.\n'
}
