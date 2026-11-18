#!/usr/bin/env bash
# Construcción reproducible de un release de NetDiag. Sin red, sin telemetría.
set -euo pipefail

usage() {
  cat <<'EOF'
Uso: scripts/release.sh --version X --signing-json CLAVE.json --public-key PUB --notes-file NOTAS.md [opciones]

Obligatorias:
  --version X           Versión del release (p. ej. 1.5.1)
  --signing-json RUTA   JSON con {"seed":"base64","key_id":"hex16"} de la clave offline
  --public-key PUB      Clave pública minisign (.pub o base64). Si se omita, se
                        busca release.pub junto al JSON de firma.
  --notes-file RUTA     RELEASE_NOTES.md de esta versión

Opcionales:
  --out DIR             Directorio de salida (por defecto dist/release/<versión>)
  --base-url URL        Raíz HTTPS del espejo inmutable
  --pcap                Incluir la variante con libpcap (CGO, no bit a bit reproducible)
  --help                Muestra esta ayuda

El árbol de trabajo debe estar limpio. La clave de firma es material offline:
nunca se sube al repositorio y se guarda con permisos 0600.
EOF
}

VERSION=""
SIGNING_JSON=""
PUBKEY=""
NOTES_FILE=""
OUT=""
BASE_URL=""
WITH_PCAP=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --version) VERSION="$2"; shift 2 ;;
    --signing-json) SIGNING_JSON="$2"; shift 2 ;;
    --public-key) PUBKEY="$2"; shift 2 ;;
    --notes-file) NOTES_FILE="$2"; shift 2 ;;
    --out) OUT="$2"; shift 2 ;;
    --base-url) BASE_URL="$2"; shift 2 ;;
    --pcap) WITH_PCAP=1; shift ;;
    --help|-h) usage; exit 0 ;;
    *) echo "Opción desconocida: $1" >&2; usage >&2; exit 2 ;;
  esac
done

if [[ -z "$VERSION" || -z "$SIGNING_JSON" || -z "$NOTES_FILE" ]]; then
  usage >&2
  exit 2
fi

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

if ! git diff --quiet || ! git diff --cached --quiet; then
  echo "ERROR: el árbol de trabajo tiene cambios. Confirma o descarta antes de publicar." >&2
  exit 1
fi
if [[ ! -r "$SIGNING_JSON" || ! -r "$NOTES_FILE" ]]; then
  echo "ERROR: no se puede leer la clave o las notas." >&2
  exit 1
fi
SIGNING_JSON="$(realpath "$SIGNING_JSON")"
NOTES_FILE="$(realpath "$NOTES_FILE")"

if [[ -z "$PUBKEY" && -r "$(dirname "$SIGNING_JSON")/release.pub" ]]; then
  PUBKEY="$(dirname "$SIGNING_JSON")/release.pub"
fi
if [[ -z "$PUBKEY" ]]; then
  echo "ERROR: indica --public-key o coloca release.pub junto a la clave de firma." >&2
  exit 2
fi

GO_VERSION="$(go env GOVERSION)"
if [[ "$GO_VERSION" != go1.27* ]]; then
  echo "AVISO: se esperaba Go 1.27.x, se encontró $GO_VERSION." >&2
fi

hash_file() { sha256sum "$1" | awk '{print $1}'; }
fail() { echo "ERROR: $*" >&2; exit 1; }

echo "== 1/10 Verificando módulos =="
go mod verify

COMMIT="$(git rev-parse HEAD)"
SHORT="$(git rev-parse --short HEAD)"
if [[ -z "${SOURCE_DATE_EPOCH:-}" ]]; then
  SOURCE_DATE_EPOCH="$(git log -1 --format=%ct "$COMMIT")"
fi
export SOURCE_DATE_EPOCH
BUILD_DATE="$(date -u -d "@$SOURCE_DATE_EPOCH" +%Y-%m-%dT%H:%M:%SZ)"
if [[ -z "$BASE_URL" ]]; then
  BASE_URL="https://updates.netdiag.example/releases/$VERSION"
fi
OUT="$(realpath -m "${OUT:-dist/release/$VERSION}")"
rm -rf "$OUT"
mkdir -p "$OUT"

LDFLAGS="-s -w -buildid= -X main.version=$VERSION -X main.commit=$SHORT -X main.buildDate=$BUILD_DATE"
TOOLS="$(mktemp -d)"
trap 'rm -rf "$TOOLS"' EXIT
go build -o "$TOOLS/netdiag-release" ./cmd/netdiag-release
go build -o "$TOOLS/netdiag" ./cmd/netdiag

echo "== 2/10 Compilando binarios reproducibles =="
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -buildvcs=false \
  -ldflags "$LDFLAGS" -o "$OUT/netdiag-linux-amd64" ./cmd/netdiag
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -buildvcs=false \
  -ldflags "$LDFLAGS" -o "$OUT/netdiag-linux-arm64" ./cmd/netdiag

if [[ "$WITH_PCAP" -eq 1 ]]; then
  echo "== 2b/10 Compilando variante pcap (CGO) =="
  CGO_ENABLED=1 GOOS=linux GOARCH=amd64 go build -tags pcap -trimpath -buildvcs=false \
    -ldflags "$LDFLAGS" -o "$OUT/netdiag-linux-amd64-pcap" ./cmd/netdiag
fi

echo "== 3/10 SBOM y procedencia =="
NETDIAG_VERSION="$VERSION" SOURCE_DATE_EPOCH="$SOURCE_DATE_EPOCH" \
  python3 scripts/sbom.py "$OUT/sbom.cdx.json"
(
  cd "$OUT"
  NETDIAG_VERSION="$VERSION" SOURCE_DATE_EPOCH="$SOURCE_DATE_EPOCH" \
    python3 "$ROOT/scripts/provenance.py" provenance.intoto.jsonl
)

echo "== 4/10 Notas firmadas =="
cp "$NOTES_FILE" "$OUT/RELEASE_NOTES.md"

echo "== 5/10 Manifiesto F23 =="
BIN_SUM="$(hash_file "$OUT/netdiag-linux-amd64")"
"$TOOLS/netdiag-release" manifest \
  --version "$VERSION" \
  --url "$BASE_URL/netdiag-linux-amd64" \
  --checksum "$BIN_SUM" \
  --notes "NetDiag $VERSION" \
  --released-at "$BUILD_DATE" \
  --seed-file "$SIGNING_JSON" \
  --out "$OUT/manifest.json"

echo "== 6/10 Contenedor .ndc reproducible =="
(
  cd "$OUT"
  "$TOOLS/netdiag" container build \
    --version "$VERSION" --kind release --producer netdiag \
    --created-at "$BUILD_DATE" \
    --sbom sbom.cdx.json --seed-file "$SIGNING_JSON" \
    --out "netdiag_${VERSION}_linux_amd64.ndc" \
    netdiag-linux-amd64 RELEASE_NOTES.md manifest.json provenance.intoto.jsonl
)

echo "== 7/10 SHA256SUMS =="
(
  cd "$OUT"
  FILES=(netdiag-linux-amd64 netdiag-linux-arm64 manifest.json RELEASE_NOTES.md \
    sbom.cdx.json provenance.intoto.jsonl "netdiag_${VERSION}_linux_amd64.ndc")
  [[ "$WITH_PCAP" -eq 1 ]] && FILES+=(netdiag-linux-amd64-pcap)
  sha256sum "${FILES[@]}" > SHA256SUMS
)

echo "== 8/10 Firmas =="
SUMS_SUM="$(hash_file "$OUT/SHA256SUMS")"
"$TOOLS/netdiag-release" sign --in "$OUT/SHA256SUMS" \
  --out "$OUT/SHA256SUMS.minisig" --seed-file "$SIGNING_JSON" \
  --comment "version:$VERSION sha256:$SUMS_SUM"

MANIFEST_SUM="$(hash_file "$OUT/manifest.json")"
"$TOOLS/netdiag-release" sign --in "$OUT/manifest.json" \
  --out "$OUT/manifest.json.minisig" --seed-file "$SIGNING_JSON" \
  --comment "version:$VERSION sha256:$MANIFEST_SUM"

NOTES_SUM="$(hash_file "$OUT/RELEASE_NOTES.md")"
"$TOOLS/netdiag-release" sign --in "$OUT/RELEASE_NOTES.md" \
  --out "$OUT/RELEASE_NOTES.md.minisig" --seed-file "$SIGNING_JSON" \
  --comment "version:$VERSION notes:$NOTES_SUM"

SBOM_SUM="$(hash_file "$OUT/sbom.cdx.json")"
"$TOOLS/netdiag-release" sign --in "$OUT/sbom.cdx.json" \
  --out "$OUT/sbom.cdx.json.minisig" --seed-file "$SIGNING_JSON" \
  --comment "version:$VERSION sbom:$SBOM_SUM"

PROV_SUM="$(hash_file "$OUT/provenance.intoto.jsonl")"
"$TOOLS/netdiag-release" sign --in "$OUT/provenance.intoto.jsonl" \
  --out "$OUT/provenance.intoto.jsonl.minisig" --seed-file "$SIGNING_JSON" \
  --comment "version:$VERSION provenance:$PROV_SUM"

if [[ -f "$PUBKEY" ]]; then
  cp "$PUBKEY" "$OUT/release.pub"
else
  printf '%s\n' "$PUBKEY" > "$OUT/release.pub"
fi
chmod 0644 "$OUT/release.pub"

echo "== 9/10 Verificación interna =="
"$TOOLS/netdiag-release" verify --in "$OUT/SHA256SUMS" \
  --sig "$OUT/SHA256SUMS.minisig" --key "$OUT/release.pub"
"$TOOLS/netdiag-release" verify-manifest --in "$OUT/manifest.json" --key "$OUT/release.pub"
for name in RELEASE_NOTES.md sbom.cdx.json provenance.intoto.jsonl manifest.json; do
  "$TOOLS/netdiag-release" verify --in "$OUT/$name" \
    --sig "$OUT/$name.minisig" --key "$OUT/release.pub"
done
"$TOOLS/netdiag" container verify --key "$OUT/release.pub" \
  "$OUT/netdiag_${VERSION}_linux_amd64.ndc" >/dev/null
(
  cd "$OUT"
  sha256sum -c SHA256SUMS >/dev/null
)
REPORTED_SUM="$(awk -F'"' '/"checksum"/ {print $4}' "$OUT/manifest.json")"
REPORTED_VERSION="$(awk -F'"' '/"version"/ {print $4}' "$OUT/manifest.json")"
[[ "$REPORTED_SUM" == "$BIN_SUM" ]] || fail "el manifiesto no apunta al binario compilado"
[[ "$REPORTED_VERSION" == "$VERSION" ]] || fail "el manifiesto indica otra versión"
echo "Firmas, hashes, manifiesto y contenedor verificados."

echo "== 10/10 Resumen =="
echo "Release: $OUT"
echo "Commit:  $SHORT"
echo "Época:   $SOURCE_DATE_EPOCH ($BUILD_DATE)"
echo "Clave:   $(head -n 1 "$OUT/release.pub")"
ls -1 "$OUT"
echo "Listo. Publica $OUT en el espejo inmutable y comprueba con scripts/verify-release.sh."
