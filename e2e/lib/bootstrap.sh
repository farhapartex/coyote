set -euo pipefail

E2E_BOOTSTRAP_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

. "$E2E_BOOTSTRAP_DIR/project.sh"
. "$E2E_BOOTSTRAP_DIR/assert.sh"
. "$E2E_BOOTSTRAP_DIR/chunk.sh"

cd "$E2E_ROOT"
