#!/usr/bin/env bash
# Comprueba vulnerabilidades conocidas en las dependencias Go. Sin red de
# reporting: solo imprime el resultado en la terminal local.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

GOVULNCHECK="$(command -v govulncheck || true)"
if [[ -z "$GOVULNCHECK" ]]; then
  GOBIN="$(go env GOPATH)/bin/govulncheck"
  [[ -x "$GOBIN" ]] && GOVULNCHECK="$GOBIN"
fi
if [[ -z "$GOVULNCHECK" ]]; then
  echo "govulncheck no está instalado." >&2
  echo "Instálalo en local con:" >&2
  echo "  go install golang.org/x/vuln/cmd/govulncheck@latest" >&2
  exit 127
fi

echo "== govulncheck ./... =="
"$GOVULNCHECK" ./...
