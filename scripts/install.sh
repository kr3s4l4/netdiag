#!/usr/bin/env bash
# Instalador de NetDiag para Linux con systemd.
#
# Uso:
#   sudo ./scripts/install.sh [opciones]
#
# Opciones:
#   --pcap            compila con ARP activo (requiere libpcap-dev y gcc)
#   --build           fuerza la compilación aunque exista bin/netdiag
#   --binary RUTA     instala un binario ya compilado
#   --listen DIR      dirección de escucha inicial (por defecto 127.0.0.1:8080)
#   --start           arranca el servicio al terminar
#   --no-enable       no habilita el servicio en el arranque
#   -h, --help        muestra esta ayuda

set -euo pipefail

PREFIX="${PREFIX:-/usr/local}"
BIN_DST="$PREFIX/bin/netdiag"
CONFIG_DIR="${CONFIG_DIR:-/etc/netdiag}"
CONFIG="$CONFIG_DIR/config.yaml"
DATA_DIR="${DATA_DIR:-/var/lib/netdiag}"
UNIT_DST="/etc/systemd/system/netdiag.service"
SERVICE_USER="netdiag"
LISTEN="127.0.0.1:8080"
DO_BUILD=0
USE_PCAP=0
DO_START=0
DO_ENABLE=1
BIN_SRC=""

REPO_ROOT="$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)"

usage() {
	sed -n '2,15p' "$0" | sed 's/^# \{0,1\}//'
}

log() { printf '\033[36m→\033[0m %s\n' "$*"; }
warn() { printf '\033[33m!\033[0m %s\n' "$*" >&2; }
die() {
	printf '\033[31m✗\033[0m %s\n' "$*" >&2
	exit 1
}

while [ $# -gt 0 ]; do
	case "$1" in
	--pcap)
		USE_PCAP=1
		DO_BUILD=1
		;;
	--build) DO_BUILD=1 ;;
	--binary)
		shift
		[ $# -gt 0 ] || die "--binary requiere una ruta"
		BIN_SRC="$1"
		;;
	--listen)
		shift
		[ $# -gt 0 ] || die "--listen requiere una dirección"
		LISTEN="$1"
		;;
	--start) DO_START=1 ;;
	--no-enable) DO_ENABLE=0 ;;
	-h | --help)
		usage
		exit 0
		;;
	*) die "opción desconocida: $1 (usa --help)" ;;
	esac
	shift
done

[ "$(id -u)" -eq 0 ] || die "ejecuta el instalador como root (sudo)"
[ "$(uname -s)" = "Linux" ] || die "NetDiag solo se ha probado en Linux"
command -v systemctl >/dev/null 2>&1 || die "se requiere systemd"

# ---------------------------------------------------------------- compilación

if [ -n "$BIN_SRC" ]; then
	[ -f "$BIN_SRC" ] || die "no existe el binario $BIN_SRC"
elif [ "$DO_BUILD" -eq 0 ] && [ -x "$REPO_ROOT/bin/netdiag" ]; then
	BIN_SRC="$REPO_ROOT/bin/netdiag"
elif [ "$DO_BUILD" -eq 0 ] && [ "$USE_PCAP" -eq 0 ] && [ -x "$REPO_ROOT/bin/netdiag-pcap" ]; then
	BIN_SRC="$REPO_ROOT/bin/netdiag-pcap"
else
	command -v go >/dev/null 2>&1 || die "Go no está instalado; compila en otra máquina y usa --binary RUTA"
	if [ "$USE_PCAP" -eq 1 ]; then
		command -v gcc >/dev/null 2>&1 || die "el build con pcap requiere gcc y libpcap-dev"
		log "compilando con ARP activo (CGO + libpcap)"
		(cd "$REPO_ROOT" && make build-pcap)
		BIN_SRC="$REPO_ROOT/bin/netdiag-pcap"
	else
		log "compilando binario portable"
		(cd "$REPO_ROOT" && make build)
		BIN_SRC="$REPO_ROOT/bin/netdiag"
	fi
fi

[ -f "$BIN_SRC" ] || die "no se encontró un binario para instalar"
log "usando $BIN_SRC"

# ------------------------------------------------------------ usuario y rutas

if ! getent group "$SERVICE_USER" >/dev/null 2>&1; then
	log "creando grupo $SERVICE_USER"
	groupadd --system "$SERVICE_USER"
fi

if ! id -u "$SERVICE_USER" >/dev/null 2>&1; then
	log "creando usuario de sistema $SERVICE_USER"
	if command -v useradd >/dev/null 2>&1; then
		useradd --system --gid "$SERVICE_USER" --home-dir "$DATA_DIR" \
			--shell /usr/sbin/nologin --no-create-home "$SERVICE_USER"
	else
		adduser --system --ingroup "$SERVICE_USER" --home "$DATA_DIR" \
			--shell /usr/sbin/nologin --no-create-home "$SERVICE_USER"
	fi
fi

WAS_ACTIVE=0
if systemctl is-active --quiet netdiag 2>/dev/null; then
	WAS_ACTIVE=1
	log "deteniendo el servicio para reemplazar el binario"
	systemctl stop netdiag
fi

install -d -o root -g root -m 0755 "$PREFIX/bin"
install -Dm755 "$BIN_SRC" "$BIN_DST"
log "binario instalado en $BIN_DST"

install -d -o "$SERVICE_USER" -g "$SERVICE_USER" -m 0700 "$DATA_DIR"
install -d -o "$SERVICE_USER" -g "$SERVICE_USER" -m 0700 "$CONFIG_DIR"

# ------------------------------------------------------------- configuración

if [ ! -f "$CONFIG" ]; then
	log "creando configuración en $CONFIG"
	"$BIN_DST" config init --config "$CONFIG" --data-dir "$DATA_DIR" >/dev/null
	if [ "$LISTEN" != "127.0.0.1:8080" ]; then
		sed -i -E "s|^([[:space:]]*listen:).*|\1 $LISTEN|" "$CONFIG"
	fi
else
	log "se conserva la configuración existente en $CONFIG"
	if [ "$LISTEN" != "127.0.0.1:8080" ]; then
		warn "--listen se ignora porque $CONFIG ya existe"
	fi
fi
chown -R "$SERVICE_USER:$SERVICE_USER" "$CONFIG_DIR" "$DATA_DIR"
chmod 0600 "$CONFIG"
chmod 0700 "$DATA_DIR" "$CONFIG_DIR"

# -------------------------------------------------------------- capacidades

if command -v setcap >/dev/null 2>&1; then
	if setcap cap_net_raw,cap_net_admin,cap_net_bind_service=+ep "$BIN_DST" 2>/dev/null; then
		log "capacidades aplicadas a $BIN_DST"
	else
		warn "no se pudieron aplicar capabilities (¿sistema de archivos sin soporte?)"
		warn "la unidad systemd usa AmbientCapabilities y debería seguir funcionando"
	fi
else
	warn "setcap no está disponible; se usarán solo las capacidades de systemd"
fi

# ------------------------------------------------------------------- systemd

install -Dm644 "$REPO_ROOT/scripts/netdiag.service" "$UNIT_DST"
systemctl daemon-reload
log "unidad systemd instalada en $UNIT_DST"

if [ "$DO_ENABLE" -eq 1 ]; then
	systemctl enable netdiag >/dev/null
	log "servicio habilitado en el arranque"
fi

if [ "$DO_START" -eq 1 ] || [ "$WAS_ACTIVE" -eq 1 ]; then
	systemctl restart netdiag
	log "servicio arrancado"
	sleep 1
	systemctl --no-pager --lines=5 status netdiag || true
else
	log "arranca el servicio con: sudo systemctl start netdiag"
fi

# -------------------------------------------------------------- diagnóstico

printf '\n'
log "comprobación del entorno"
sudo -u "$SERVICE_USER" "$BIN_DST" doctor --config "$CONFIG" || true

printf '\n\033[32mNetDiag instalado.\033[0m\n\n'
cat <<EOF
  Interfaz:      http://$LISTEN
  Configuración: $CONFIG
  Datos:         $DATA_DIR
  Servicio:      systemctl status netdiag
  Logs:          journalctl -u netdiag -f

  El primer acceso pide crear la cuenta de administrador.
  Publica el puerto mediante un proxy con TLS antes de usarlo en producción:
  consulta docs/despliegue.md.
EOF
