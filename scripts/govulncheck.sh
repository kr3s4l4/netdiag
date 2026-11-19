#!/usr/bin/env bash
# Comprueba vulnerabilidades conocidas en las dependencias Go. Sin red de
# reporting: solo imprime el resultado en la terminal local.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

if ! command -v govulncheck >/dev/null 2>&1; then
  echo "govulncheck no está instalado." >&2
  echo "Instálalo en local con:" >&2
  echo "  go install golang.org/x/vuln/cmd/govulncheck@latest" >&2
  exit 127
fi

echo "== govulncheck ./... =="
govulncheck ./...
