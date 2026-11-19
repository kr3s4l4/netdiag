#!/usr/bin/env bash
# Laboratorio aislado para validar F44, F46 y F47 con tráfico real.
# No toca la red del host: usa un bridge Docker propio 172.20.50.0/24.
set -euo pipefail

LAB_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$LAB_DIR"

command -v docker >/dev/null || { echo "Necesitas Docker." >&2; exit 1; }
command -v go >/dev/null || { echo "Necesitas Go." >&2; exit 1; }

mkdir -p data config state

echo "[1/5] Compilando NetDiag con captura pasiva..."
go build -tags pcap -trimpath -o netdiag-pcap ../../cmd/netdiag

echo "[2/5] Construyendo imágenes..."
docker build -q -t netdiag-lab:local -f Dockerfile.netdiag . >/dev/null
docker build -q -t dhcp-rogue:local -f Dockerfile.dhcp . >/dev/null
docker build -q -t snmp-agent:local -f Dockerfile.agent . >/dev/null
docker build -q -t scapy-lab:local -f Dockerfile.generator . >/dev/null

echo "[3/5] Creando red 172.20.50.0/24..."
docker network inspect netdiag-lab >/dev/null 2>&1 || \
  docker network create --driver bridge --subnet 172.20.50.0/24 --gateway 172.20.50.1 netdiag-lab >/dev/null

echo "[4/5] Limpiando contenedores anteriores..."
for name in lab-netdiag lab-dhcp lab-snmp lab-scapy; do
  docker rm -f "$name" >/dev/null 2>&1 || true
done

echo "[5/5] Arrancando laboratorio..."
docker run -d --name lab-netdiag --network netdiag-lab --ip 172.20.50.10 \
  --cap-add NET_RAW --cap-add NET_ADMIN -p 18080:18080 \
  -v "$LAB_DIR/config:/config:ro" -v "$LAB_DIR/data:/data" \
  netdiag-lab:local serve --config /config/config.yaml >/dev/null

docker run -d --name lab-dhcp --network netdiag-lab --ip 172.20.50.53 \
  --cap-add NET_ADMIN dhcp-rogue:local >/dev/null

docker run -d --name lab-snmp --network netdiag-lab --ip 172.20.50.161 \
  -v "$LAB_DIR/state:/state" snmp-agent:local >/dev/null

docker run -d --name lab-scapy --network netdiag-lab --ip 172.20.50.77 \
  scapy-lab:local >/dev/null

cat <<'TXT'

Laboratorio listo en http://127.0.0.1:18080

Siguiente paso (copiar/pegar):

  curl -s -c cookies.txt -H 'Content-Type: application/json' \
    -d '{"username":"admin","password":"LabPass2026"}' \
    http://127.0.0.1:18080/api/v1/auth/setup

Guarda el `csrf` de la respuesta y crea sede, red y reglas:

  CSRF='<pega-aqui-el-csrf>'
  curl -s -b cookies.txt -H 'Content-Type: application/json' -H "X-CSRF-Token: $CSRF" \
    -d '{"name":"Laboratorio"}' http://127.0.0.1:18080/api/v1/sites
  curl -s -b cookies.txt -H 'Content-Type: application/json' -H "X-CSRF-Token: $CSRF" \
    -d '{"site_id":2,"name":"LabLAN","cidr":"172.20.50.0/24","interface":"eth0","enabled":true}' \
    http://127.0.0.1:18080/api/v1/networks

  for T in dhcp_rogue snmp_anomaly passive_anomaly arp_spoofing; do
    curl -s -b cookies.txt -H 'Content-Type: application/json' -H "X-CSRF-Token: $CSRF" \
      -d "{\"name\":\"$T\",\"type\":\"$T\",\"enabled\":true,\"severity\":\"critical\",\"config\":{\"min_confidence\":75},\"channel_ids\":[],\"cooldown_sec\":60}" \
      http://127.0.0.1:18080/api/v1/alerts/rules
  done

Tráfico y escaneo:

  docker exec -d lab-scapy python3 /app/inject.py --loop
  docker exec -d lab-scapy python3 /app/dhcp_offer.py
  curl -s -b cookies.txt -H 'Content-Type: application/json' -H "X-CSRF-Token: $CSRF" \
    -d '{"site_id":2,"network_id":1,"cidr":"172.20.50.0/24","profile":"common","snmp":true,"deep":true,"timeout_ms":900}' \
    http://127.0.0.1:18080/api/v1/scans

Para simular un movimiento L2 entre escaneos:

  docker exec lab-snmp touch /state/moved
  # ...nuevo escaneo...
  docker exec lab-snmp rm -f /state/moved

Parar:

  docker rm -f lab-netdiag lab-dhcp lab-snmp lab-scapy
  docker network rm netdiag-lab
TXT
