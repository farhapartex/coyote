#!/usr/bin/env bash

E2E_RUN_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
. "$E2E_RUN_DIR/lib/bootstrap.sh"
. "$E2E_RUN_DIR/lib/report.sh"

OPT_FRESH=""
OPT_FAIL_FAST=""
OPT_ONLY=""
OPT_FROM=""
OPT_LIST=""

usage_text() {
	cat <<'TEXT'
usage: ./e2e/run.sh [flags]

flags:
  --fresh        remove the generated example project before running
  --fail-fast    stop at the first failing chunk
  --only=NN      run a single chunk
  --from=NN      start at a chunk and run the rest
  --list         print the chunks and exit
  --help         print this message

the report is written to e2e/REPORT.md and overwritten on every run
TEXT
}

parse_flags() {
	local argument
	for argument in "$@"; do
		case "$argument" in
		--fresh) OPT_FRESH=1 ;;
		--fail-fast) OPT_FAIL_FAST=1 ;;
		--only=*) OPT_ONLY="${argument#--only=}" ;;
		--from=*) OPT_FROM="${argument#--from=}" ;;
		--list) OPT_LIST=1 ;;
		--help | -h)
			usage_text
			exit 0
			;;
		*)
			printf 'unknown flag %s\n\n' "$argument" >&2
			usage_text >&2
			exit 2
			;;
		esac
	done
}

chunk_scripts() {
	find "$E2E_RUN_DIR/chunks" -name '[0-9][0-9]-*.sh' -type f 2>/dev/null | sort
}

chunk_id_of() {
	basename "$1" | cut -d- -f1
}

chunk_name_of() {
	sed -n 's/^[[:space:]]*chunk_begin "[0-9][0-9]" "\(.*\)"$/\1/p' "$1" | head -1
}

list_chunks() {
	local script
	for script in $(chunk_scripts); do
		printf '  %s  %s\n' "$(chunk_id_of "$script")" "$(chunk_name_of "$script")"
	done
}

chunk_is_selected() {
	local id="$1"
	if [ -n "$OPT_ONLY" ]; then
		[ "$id" = "$OPT_ONLY" ]
		return
	fi
	if [ -n "$OPT_FROM" ]; then
		[ "$id" \> "$OPT_FROM" ] || [ "$id" = "$OPT_FROM" ]
		return
	fi
	return 0
}

prepare_workspace() {
	mkdir -p "$E2E_BIN_DIR" "$E2E_WORK_DIR"
	rm -f "$E2E_WORK_DIR"/chunk-*.result

	if [ -n "$OPT_FRESH" ] && [ -d "$EXAMPLE_DIR" ]; then
		printf '%sremoving %s%s\n' "$C_DIM" "$EXAMPLE_DIR" "$C_RESET"
		rm -rf "$EXAMPLE_DIR"
	fi

	report_reset "$(run_scope_description)"
}

run_scope_description() {
	if [ -n "$OPT_ONLY" ]; then
		printf 'only chunk %s' "$OPT_ONLY"
		return
	fi
	if [ -n "$OPT_FROM" ]; then
		printf 'chunk %s onwards' "$OPT_FROM"
		return
	fi
	printf 'every chunk'
}

run_one_chunk() {
	local chunk_script="$1" chunk_number chunk_status
	chunk_number="$(chunk_id_of "$chunk_script")"

	E2E_RESULTS="$E2E_WORK_DIR/chunk-$chunk_number.result"
	: >"$E2E_RESULTS"
	export E2E_RESULTS

	chunk_status="pass"
	if ! bash "$chunk_script"; then
		chunk_status="fail"
	fi

	report_add_chunk "$chunk_number" "$E2E_RESULTS" "$chunk_status"
	report_render

	[ "$chunk_status" = "pass" ]
}

block_remaining_chunks() {
	local reason="$1" script
	shift
	for script in "$@"; do
		report_add_blocked_chunk "$(chunk_id_of "$script")" "$(chunk_name_of "$script")" "$reason"
	done
	report_render
}

main() {
	parse_flags "$@"

	if [ -n "$OPT_LIST" ]; then
		list_chunks
		exit 0
	fi

	prepare_workspace

	local selected=()
	local script
	for script in $(chunk_scripts); do
		if chunk_is_selected "$(chunk_id_of "$script")"; then
			selected+=("$script")
		fi
	done

	if [ "${#selected[@]}" -eq 0 ]; then
		printf 'no chunks matched\n' >&2
		exit 2
	fi

	for script in $(chunk_scripts); do
		if ! chunk_is_selected "$(chunk_id_of "$script")"; then
			report_add_unrun_chunk "$(chunk_id_of "$script")" "$(chunk_name_of "$script")"
		fi
	done

	local index=0
	local failures=0
	for script in "${selected[@]}"; do
		index=$((index + 1))
		if run_one_chunk "$script"; then
			continue
		fi

		failures=$((failures + 1))
		if [ -n "$OPT_FAIL_FAST" ]; then
			block_remaining_chunks "stopped by --fail-fast after $(chunk_id_of "$script")" \
				"${selected[@]:$index}"
			break
		fi
	done

	printf '\n'
	if [ "$failures" -gt 0 ]; then
		printf '%s%d chunk(s) failed%s · %s\n' "$C_RED" "$failures" "$C_RESET" "$E2E_REPORT"
		exit 1
	fi
	printf '%sall chunks passed%s · %s\n' "$C_GREEN" "$C_RESET" "$E2E_REPORT"
}

main "$@"
