#!/usr/bin/env bash
# Empaqueta un release firmado y una licencia para entrega manual. Sin red.
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
exec python3 "$ROOT/scripts/pack_delivery.py" "$@"
