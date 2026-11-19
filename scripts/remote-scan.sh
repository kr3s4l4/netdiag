#!/usr/bin/env bash
# Escaneo distribuido por SSH.
# SSH es solo el transporte: el escaneo se ejecuta dentro de la red remota.
# Sin contraseñas (BatchMode). Sin telemetría. Sin agente en los hosts.
set -euo pipefail

usage() {
  cat <<'EOF'
Uso: remote-scan.sh <usuario@host> <ruta-netdiag-remoto> <red> [perfil] [salida.json]

  red      ID de red configurada (3) o CIDR (192.168.10.0/24)
  perfil   quick | common | full | custom   (por defecto common)
  salida   archivo JSON local (por defecto scan-<host>-<red>-<fecha>.json)

Ejemplos:
  scripts/remote-scan.sh operator@172.20.50.10 /opt/netdiag/netdiag 3 common sede-a.json
  scripts/remote-scan.sh operator@172.20.50.10 /opt/netdiag/netdiag 192.168.10.0/24 quick
EOF
}

if [[ $# -lt 3 || "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  [[ $# -lt 3 ]] && exit 2
  exit 0
fi

TARGET="$1"
REMOTE_BIN="$2"
NET="$3"
PROFILE="${4:-common}"
OUT="${5:-}"

case "$PROFILE" in
  quick|common|full|custom) ;;
  *)
    echo "Perfil no válido: $PROFILE" >&2
    usage >&2
    exit 2
    ;;
esac

SSH_OPTS=(-o BatchMode=yes -o ConnectTimeout=10)

if [[ "$NET" =~ ^[0-9]+$ ]]; then
  REMOTE_CMD="$REMOTE_BIN scan --network $NET --json"
  LABEL="red-$NET"
elif [[ "$NET" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}/[0-9]{1,2}$ ]]; then
  REMOTE_CMD="$REMOTE_BIN scan '$NET' --profile $PROFILE --json"
  LABEL="${NET//\//-}"
else
  echo "La red debe ser un ID numérico o un CIDR: $NET" >&2
  exit 2
fi

if [[ -z "$OUT" ]]; then
  HOSTPART="${TARGET##*@}"
  OUT="scan-${HOSTPART}-${LABEL}-$(date +%F).json"
fi

mkdir -p "$(dirname "$OUT")"
TMP="${OUT}.tmp.$$"
trap 'rm -f "$TMP"' EXIT
ssh "${SSH_OPTS[@]}" "$TARGET" "$REMOTE_CMD" > "$TMP"
mv "$TMP" "$OUT"
trap - EXIT

echo "Guardado: $OUT"
echo "Verificar: jq -e . '$OUT' >/dev/null"
