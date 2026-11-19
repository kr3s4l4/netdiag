#!/usr/bin/env bash
# Firma informes, diffs y snapshots en un contenedor .ndc.
# La clave de firma es local y distinta de la clave de release.
# Nada sale de esta máquina: no hay envío, telemetría ni nube.
set -euo pipefail

usage() {
  cat <<'EOF'
Uso: sign-evidence.sh --seed-file CLAVE.json --key-pub CLAVE.pub \
       --version VERSION --out SALIDA.ndc FICHERO... [FICHERO...]

  --seed-file   JSON local {"seed":"base64","key_id":"hex16"} (nunca compartir)
  --key-pub     Clave pública .pub para verificar el contenedor
  --version     Versión del producto o del informe (ej. 1.5.2)
  --out         Archivo .ndc de salida
  --kind        evidence (por defecto) o site
  --producer    Nombre del productor (por defecto netdiag-operator)

Ejemplo:
  NETDIAG_BIN=./bin/netdiag scripts/sign-evidence.sh \
    --seed-file /ruta/segura/evidence-seed.json \
    --key-pub /ruta/publica/evidence.pub \
    --version 1.5.2 --out evidencia.ndc \
    informe.pdf diff.json snapshot.json
EOF
}

KIND=evidence
PRODUCER=netdiag-operator
SEED_FILE=""
KEY_PUB=""
VERSION=""
OUT=""
FILES=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --seed-file) SEED_FILE="${2:-}"; shift 2 ;;
    --key-pub) KEY_PUB="${2:-}"; shift 2 ;;
    --version) VERSION="${2:-}"; shift 2 ;;
    --out) OUT="${2:-}"; shift 2 ;;
    --kind) KIND="${2:-}"; shift 2 ;;
    --producer) PRODUCER="${2:-}"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    -*) echo "Opción desconocida: $1" >&2; usage >&2; exit 2 ;;
    *) FILES+=("$1"); shift ;;
  esac
done

if [[ -z "$SEED_FILE" || -z "$KEY_PUB" || -z "$VERSION" || -z "$OUT" || ${#FILES[@]} -eq 0 ]]; then
  usage >&2
  exit 2
fi
case "$KIND" in
  evidence|site) ;;
  *) echo "kind debe ser evidence o site" >&2; exit 2 ;;
esac
for f in "${FILES[@]}"; do
  [[ -f "$f" ]] || { echo "No existe: $f" >&2; exit 2; }
done

NETDIAG_BIN="${NETDIAG_BIN:-netdiag}"
command -v "$NETDIAG_BIN" >/dev/null 2>&1 || { echo "No se encontró $NETDIAG_BIN" >&2; exit 2; }

mkdir -p "$(dirname "$OUT")"
"$NETDIAG_BIN" container build \
  --version "$VERSION" --kind "$KIND" --producer "$PRODUCER" \
  --seed-file "$SEED_FILE" --out "$OUT" "${FILES[@]}"
"$NETDIAG_BIN" container verify --key "$KEY_PUB" "$OUT"
echo "Firma verificada: $OUT"
