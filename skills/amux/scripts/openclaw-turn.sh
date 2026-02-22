#!/usr/bin/env bash
set -euo pipefail
SCRIPT_SOURCE="${BASH_SOURCE[0]:-$0}"
SCRIPT_DIR="$(cd "$(dirname "$SCRIPT_SOURCE")" >/dev/null 2>&1 && pwd -P)"
exec "$SCRIPT_DIR/assistant-turn.sh" "$@"
