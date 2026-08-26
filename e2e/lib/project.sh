E2E_LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
E2E_DIR="$(cd "$E2E_LIB_DIR/.." && pwd)"
E2E_ROOT="$(cd "$E2E_DIR/.." && pwd)"

E2E_BIN_DIR="$E2E_DIR/.bin"
E2E_WORK_DIR="$E2E_DIR/.work"
E2E_REPORT="$E2E_DIR/REPORT.md"

COYOTE_BIN="$E2E_BIN_DIR/coyote"
EXAMPLE_DIR="$E2E_ROOT/example"
EXAMPLE_NAME="example"

E2E_PORT="${E2E_PORT:-8099}"
E2E_HOST="${E2E_HOST:-127.0.0.1}"
E2E_BASE_URL="http://$E2E_HOST:$E2E_PORT"

E2E_SEP=$'\037'

if [ -t 1 ]; then
	C_RESET=$'\033[0m'
	C_BOLD=$'\033[1m'
	C_DIM=$'\033[2m'
	C_GREEN=$'\033[32m'
	C_RED=$'\033[31m'
	C_YELLOW=$'\033[33m'
else
	C_RESET=""
	C_BOLD=""
	C_DIM=""
	C_GREEN=""
	C_RED=""
	C_YELLOW=""
fi

repository_commit() {
	git -C "$E2E_ROOT" rev-parse --short HEAD 2>/dev/null || echo "unknown"
}

repository_is_dirty() {
	[ -n "$(git -C "$E2E_ROOT" status --porcelain 2>/dev/null)" ]
}

go_version() {
	go version 2>/dev/null | awk '{print $3}' | sed 's/^go//'
}

platform_name() {
	printf '%s/%s' "$(go env GOOS 2>/dev/null || uname -s)" "$(go env GOARCH 2>/dev/null || uname -m)"
}

version_at_least() {
	local have="$1" want="$2"
	[ "$want" = "$(printf '%s\n%s\n' "$want" "$have" | sort -t. -k1,1n -k2,2n -k3,3n | head -1)" ]
}

seconds_now() {
	date +%s
}
