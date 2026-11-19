#!/usr/bin/env bash
# F61: comprueba que la versión del binario, la API y el footer del login coinciden.
# Sin red, sin secretos y sin dejar servidores en marcha.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

VERSION="$(grep -m1 '^VERSION' Makefile | awk '{print $3}')"
COMMIT="$(git rev-parse --short HEAD 2>/dev/null || echo dev)"
BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
BUILDPKG="github.com/kr3s4l4/netdiag/internal/buildinfo"

TMP="$(mktemp -d /tmp/netdiag-version-stamp.XXXXXX)"
PID=""

cleanup() {
  if [[ -n "$PID" ]] && kill -0 "$PID" 2>/dev/null; then
    kill "$PID" 2>/dev/null || true
    wait "$PID" 2>/dev/null || true
  fi
  rm -rf "$TMP"
}
trap cleanup EXIT

echo "== Versión esperada: $VERSION =="

LDFLAGS="-s -w -X $BUILDPKG.Version=$VERSION -X $BUILDPKG.Commit=$COMMIT -X $BUILDPKG.BuildDate=$BUILD_DATE"
CGO_ENABLED=0 go build -trimpath -ldflags "$LDFLAGS" -o "$TMP/netdiag" ./cmd/netdiag

CLI="$($TMP/netdiag --version)"
case "$CLI" in
  "netdiag $VERSION "*) echo "OK: --version -> $CLI" ;;
  *) echo "ERROR: --version no usa $VERSION: $CLI" >&2; exit 1 ;;
esac

PORT="$(python3 - <<'PY'
import socket
s = socket.socket()
s.bind(("127.0.0.1", 0))
print(s.getsockname()[1])
s.close()
PY
)"

"$TMP/netdiag" serve \
  --config "$TMP/config.yaml" \
  --data-dir "$TMP/data" \
  --listen "127.0.0.1:$PORT" \
  --no-monitor \
  --log-level warn >"$TMP/server.log" 2>&1 &
PID=$!

BASE="http://127.0.0.1:$PORT"
STATUS="$(curl -fs --retry 10 --retry-connrefused --retry-delay 1 "$BASE/api/v1/auth/status")" || {
  echo "ERROR: el servidor no respondió en $BASE" >&2
  exit 1
}
WANT_STATUS="\"version\":\"$VERSION\""
if [[ "$STATUS" != *"$WANT_STATUS"* ]]; then
  echo "ERROR: /api/v1/auth/status no usa $VERSION: $STATUS" >&2
  exit 1
fi
echo "OK: /api/v1/auth/status -> version=$VERSION"

APPJS="$(curl -fs "$BASE/app.js")" || {
  echo "ERROR: no se pudo descargar app.js desde $BASE" >&2
  exit 1
}
if [[ "$APPJS" != *'NetDiag ${esc((st && st.version) || '* ]]; then
  echo "ERROR: el footer del login no lee st.version" >&2
  exit 1
fi
echo "OK: footer del login usa st.version"

echo "VÁLIDO: binario, API y UI comparten la versión $VERSION"
