SHOP_DB_HOST="127.0.0.1"
SHOP_DB_PORT="55432"
SHOP_DB_NAME="shop"
SHOP_DB_USER="shop"
SHOP_DB_PASSWORD="shop"
SHOP_REDIS_ADDR="127.0.0.1:56379"

COMPOSE_FILE="$E2E_DIR/docker-compose.yml"
COMPOSE_PROJECT="coyote-shop-e2e"

compose() {
	docker compose -f "$COMPOSE_FILE" -p "$COMPOSE_PROJECT" "$@"
}

services_available() {
	command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1
}

services_up() {
	compose up -d --wait --wait-timeout 120 >/dev/null 2>&1
}

services_down() {
	compose down -v >/dev/null 2>&1
}

service_state() {
	compose ps --format '{{.Service}} {{.State}} {{.Health}}' 2>/dev/null
}

postgres_ready() {
	compose exec -T postgres pg_isready -U "$SHOP_DB_USER" -d "$SHOP_DB_NAME" >/dev/null 2>&1
}

redis_ready() {
	[ "$(compose exec -T redis redis-cli ping 2>/dev/null | tr -d '\r')" = "PONG" ]
}

postgres_query() {
	compose exec -T postgres psql -U "$SHOP_DB_USER" -d "$SHOP_DB_NAME" -tA -c "$1" 2>/dev/null | tr -d '\r'
}

redis_command() {
	compose exec -T redis redis-cli "$@" 2>/dev/null | tr -d '\r'
}

postgres_table_exists() {
	[ "$(postgres_query "SELECT to_regclass('public.$1') IS NOT NULL;")" = "t" ]
}

postgres_row_count() {
	postgres_query "SELECT count(*) FROM $1;"
}

postgres_columns() {
	postgres_query "SELECT column_name FROM information_schema.columns WHERE table_name = '$1' ORDER BY column_name;"
}

redis_key_count() {
	redis_command DBSIZE
}
