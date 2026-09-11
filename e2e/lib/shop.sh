SHOP_DIR="$E2E_ROOT/shop"
SHOP_NAME="shop"
SHOP_SOURCE="$E2E_DIR/_shop"

shop_env_file() {
	cat >"$SHOP_DIR/.env" <<ENV
APP_ENV=development
SECRET_KEY=$1
HOST=$E2E_HOST
PORT=$E2E_PORT
DB_HOST=$SHOP_DB_HOST
DB_PORT=$SHOP_DB_PORT
DB_NAME=$SHOP_DB_NAME
DB_USER=$SHOP_DB_USER
DB_PASSWORD=$SHOP_DB_PASSWORD
REDIS_ADDR=$SHOP_REDIS_ADDR
ENV
}

shop_wire_module() {
	cd "$SHOP_DIR" || return 1
	go mod init "$SHOP_NAME" >/dev/null 2>&1 || true
	go mod edit -require=github.com/farhapartex/coyote@v0.0.0
	go mod edit -replace="github.com/farhapartex/coyote=$E2E_ROOT"
	local status=0
	go mod tidy >/dev/null 2>&1 || status=$?
	cd "$E2E_ROOT"
	return $status
}

shop_apply_common() {
	rm -rf "$SHOP_DIR/templates" "$SHOP_DIR/static"
	cp -R "$SHOP_SOURCE/common/templates" "$SHOP_DIR/templates"
	cp -R "$SHOP_SOURCE/common/static" "$SHOP_DIR/static"
	cp "$SHOP_SOURCE"/common/*.go "$SHOP_DIR/"
	rm -f "$SHOP_DIR/main.go"
}

shop_apply_stage() {
	local stage="$1"
	cp "$SHOP_SOURCE/stage$stage"/*.go "$SHOP_DIR/"
	gofmt -w "$SHOP_DIR" 2>/dev/null || true
}

shop_build() {
	cd "$SHOP_DIR" && go build ./... 2>&1
	local status=$?
	cd "$E2E_ROOT"
	return $status
}

shop_run() {
	local command="$1"
	shift
	cd "$SHOP_DIR" || return 1
	set +e
	COYOTE_COMMAND="$command" "$@" go run . 2>&1
	local status=$?
	set -e
	cd "$E2E_ROOT"
	return $status
}

shop_command() {
	cd "$SHOP_DIR" || return 1
	set +e
	CAPTURED_OUTPUT="$("$COYOTE_BIN" "$@" 2>&1 </dev/null)"
	CAPTURED_STATUS=$?
	set -e
	cd "$E2E_ROOT"
	return $CAPTURED_STATUS
}

shop_migration_files() {
	find "$SHOP_DIR/migrations" -name '[0-9]*.go' 2>/dev/null | sort
}

shop_start() {
	SERVER_LOG="$E2E_WORK_DIR/shop.log"
	: >"$SERVER_LOG"
	set -m
	(
		cd "$SHOP_DIR" || exit 1
		exec "$COYOTE_BIN" start --port="$E2E_PORT"
	) >"$SERVER_LOG" 2>&1 &
	SERVER_PID=$!
	set +m
	SERVER_GROUP_KILL=1
}

