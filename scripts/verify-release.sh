#!/usr/bin/env bash
# Verifica por completo un release generado por scripts/release.sh. Sin red.
set -euo pipefail

usage() {
  cat <<'EOF'
Uso: scripts/verify-release.sh --dir DIR [opciones]

Obligatorias:
  --dir DIR             Directorio del release (p. ej. dist/release/1.5.1)

Opcionales:
  --key PUB             Clave pública (.pub o base64). Por defecto DIR/release.pub
  --version X           Comprueba que el manifiesto corresponde a esa versión
  --help                Muestra esta ayuda
EOF
}

DIR=""
KEY=""
VERSION=""
while [[ $# -gt 0 ]]; do
  case "$1" in
    --dir) DIR="$2"; shift 2 ;;
    --key) KEY="$2"; shift 2 ;;
    --version) VERSION="$2"; shift 2 ;;
    --help|-h) usage; exit 0 ;;
    *) echo "Opción desconocida: $1" >&2; usage >&2; exit 2 ;;
  esac
done

if [[ -z "$DIR" ]]; then
  usage >&2
  exit 2
fi
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
DIR="$(realpath "$DIR")"
[[ -d "$DIR" ]] || { echo "ERROR: no existe $DIR" >&2; exit 1; }

if [[ -z "$KEY" && -r "$DIR/release.pub" ]]; then
  KEY="$DIR/release.pub"
fi
if [[ -z "$KEY" ]]; then
  echo "ERROR: indica --key o incluye release.pub en el directorio." >&2
  exit 2
fi

fail() { echo "NO VÁLIDO: $*" >&2; exit 1; }
ok() { echo "OK: $*"; }

TOOLS="$(mktemp -d)"
trap 'rm -rf "$TOOLS"' EXIT
go build -o "$TOOLS/netdiag-release" ./cmd/netdiag-release
go build -o "$TOOLS/netdiag" ./cmd/netdiag

for name in SHA256SUMS SHA256SUMS.minisig manifest.json manifest.json.minisig \
  RELEASE_NOTES.md RELEASE_NOTES.md.minisig sbom.cdx.json sbom.cdx.json.minisig \
  provenance.intoto.jsonl provenance.intoto.jsonl.minisig; do
  [[ -r "$DIR/$name" ]] || fail "falta $name"
done

echo "== Verificando release en $DIR =="
"$TOOLS/netdiag-release" verify --in "$DIR/SHA256SUMS" \
  --sig "$DIR/SHA256SUMS.minisig" --key "$KEY" || fail "firma de SHA256SUMS"
ok "firma de SHA256SUMS"

(
  cd "$DIR"
  sha256sum -c SHA256SUMS >/dev/null
) || fail "algún fichero no coincide con SHA256SUMS"
ok "hashes de todos los artefactos"

"$TOOLS/netdiag-release" verify-manifest --in "$DIR/manifest.json" --key "$KEY" || fail "manifiesto F23"
ok "manifiesto F23"

for name in manifest.json RELEASE_NOTES.md sbom.cdx.json provenance.intoto.jsonl; do
  "$TOOLS/netdiag-release" verify --in "$DIR/$name" \
    --sig "$DIR/$name.minisig" --key "$KEY" || fail "firma de $name"
  ok "firma de $name"
done

NDC="$(find "$DIR" -maxdepth 1 -type f -name '*.ndc' -print -quit)"
[[ -n "$NDC" ]] || fail "no se encontró contenedor .ndc"
"$TOOLS/netdiag" container verify --key "$KEY" "$NDC" >/dev/null || fail "contenedor $NDC"
ok "contenedor $(basename "$NDC")"

BIN_SUM="$(awk '$2=="netdiag-linux-amd64" {print $1}' "$DIR/SHA256SUMS")"
MANIFEST_SUM="$(awk -F'"' '/"checksum"/ {print $4}' "$DIR/manifest.json")"
MANIFEST_VERSION="$(awk -F'"' '/"version"/ {print $4}' "$DIR/manifest.json")"
[[ -n "$BIN_SUM" ]] || fail "SHA256SUMS no incluye netdiag-linux-amd64"
[[ "$MANIFEST_SUM" == "$BIN_SUM" ]] || fail "el manifiesto no apunta al binario verificado"
ok "checksum del manifiesto coincide con el binario"
MIN_SAFE="$(awk -F'"' '/"min_version"/ {print $4}' "$DIR/manifest.json")"
[[ -z "$MIN_SAFE" ]] || ok "versión mínima segura: $MIN_SAFE"
if [[ -n "$VERSION" ]]; then
  [[ "$MANIFEST_VERSION" == "$VERSION" ]] || fail "el manifiesto indica $MANIFEST_VERSION, se esperaba $VERSION"
  ok "versión $VERSION"
fi

echo "VÁLIDO: release verificado sin conexión y sin ejecutar el binario."
