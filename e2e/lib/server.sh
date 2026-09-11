SERVER_LOG="$E2E_WORK_DIR/server.log"
SERVER_PID=""
SERVER_GROUP_KILL=1

port_listener_pids() {
	lsof -ti:"$E2E_PORT" 2>/dev/null || true
}

port_is_free() {
	[ -z "$(port_listener_pids)" ]
}

wait_for_port_to_free() {
	local deadline="${1:-40}" attempt=0
	while [ "$attempt" -lt "$deadline" ]; do
		if port_is_free; then
			return 0
		fi
		sleep 0.25
		attempt=$((attempt + 1))
	done
	return 1
}

force_free_the_port() {
	local pid
	for pid in $(port_listener_pids); do
		kill -KILL "$pid" 2>/dev/null || true
	done
	wait_for_port_to_free 20
}

server_start() {
	SERVER_GROUP_KILL="${1:-1}"
	: >"$SERVER_LOG"

	set -m
	(
		cd "$SHOP_DIR" || exit 1
		exec "$COYOTE_BIN" start --port="$E2E_PORT"
	) >"$SERVER_LOG" 2>&1 &
	SERVER_PID=$!
	set +m
}

server_wait_for_http() {
	local deadline="${1:-120}" attempt=0
	while [ "$attempt" -lt "$deadline" ]; do
		if curl -fsS -o /dev/null --max-time 2 "$E2E_BASE_URL/" 2>/dev/null; then
			return 0
		fi
		sleep 0.5
		attempt=$((attempt + 1))
	done
	return 1
}

server_stop() {
	if [ -n "$SERVER_PID" ]; then
		if [ "$SERVER_GROUP_KILL" = "1" ]; then
			kill -TERM -"$SERVER_PID" 2>/dev/null || true
		else
			kill -TERM "$SERVER_PID" 2>/dev/null || true
		fi
	fi

	if wait_for_port_to_free 40; then
		SERVER_PID=""
		return 0
	fi

	force_free_the_port
	SERVER_PID=""
	return 1
}

server_log_contains() {
	grep -q "$1" "$SERVER_LOG" 2>/dev/null
}

server_cleanup() {
	if [ -n "$SERVER_PID" ]; then
		kill -TERM -"$SERVER_PID" 2>/dev/null || true
		kill -TERM "$SERVER_PID" 2>/dev/null || true
	fi
	force_free_the_port >/dev/null 2>&1 || true
}
