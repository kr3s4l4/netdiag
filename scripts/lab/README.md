# Laboratorio de validación real (F44 / F46 / F47)

Entorno aislado con Docker para comprobar las capacidades de seguridad contra
servicios y tráfico reales, sin depender de la red del host ni de inventar datos
en SQLite. No forma parte del binario ni del producto; es solo evidencia.

## Qué levanta

| Contenedor | IP | Función |
| --- | --- | --- |
| `lab-netdiag` | `172.20.50.10` | NetDiag compilado con `-tags pcap`, `CAP_NET_RAW` y `CAP_NET_ADMIN` |
| `lab-dhcp` | `172.20.50.53` | `dnsmasq` rogue que responde DHCPINFORM/DISCOVER |
| `lab-snmp` | `172.20.50.161` | Agente SNMP v2c propio con IP-MIB, ARP, VLAN y FDB |
| `lab-scapy` | `172.20.50.77` | Generador de GARP, ICMP redirect, SYN sweep, DNS y OFFER DHCP |

Red: bridge Docker `netdiag-lab`, `172.20.50.0/24`, gateway `172.20.50.1`.

## Uso

```bash
cd scripts/lab
chmod +x run.sh
./run.sh
```

`run.sh` imprime los comandos exactos para crear el administrador, la sede, la
red, las cuatro reglas de alerta dedicated y lanzar el primer escaneo. El
archivo `config.yaml` ya habilita `dhcp_probe`, `snmp_deep` y
`passive_capture` con interfaz `eth0`.

La carpeta `state/` permite simular un cambio de capa 2 entre escaneos:

```bash
docker exec lab-snmp touch /state/moved      # MAC del gateway y puerto FDB cambian
docker exec lab-snmp rm -f /state/moved      # vuelve al estado inicial
```

## Qué se espera

- **F44:** el rogue responde `DHCPINFORM`/`OFFER`; aparece una observación con
  IP `172.20.50.53` y MAC `de:ad:be:ef:00:53`, el hallazgo
  `dhcp_unauthorized_server` y una alerta crítica de la regla `dhcp_rogue`.
- **F46:** el agente responde community `public`; la API muestra solo
  `pu***ic`, filas de `ipAddrTable`, ARP, VLAN y FDB. En un segundo escaneo
  aparecen `snmp_fdb_mac_moved` y conflictos de puerto/VLAN, con alertas
  `snmp_anomaly`.
- **F47:** la captura `pcap` registra observaciones de los cinco tipos
  (`garp`, `icmp_redirect`, `dhcp_server`, `inbound_scan`, `dns_tunnel`) y
  los cinco hallazgos `passive_*`, con alertas `passive_anomaly`.

La comunidad en claro no se guarda en SQLite ni se expone en la API. Las
observaciones pasivas guardan solo metadatos: protocolo, IP/MAC, interfaz y un
detalle acotado. No se persiste payload ni credencial alguna.

## Estado real de las validaciones

| Mecanismo | Evidencia actual | Alcance | Lo que falta |
| --- | --- | --- | --- |
| DHCP rogue | F44 real con `dnsmasq` en `172.20.50.53` | Servicio real en bridge aislado | DHCP controlado en una red de producción |
| SNMP v2c | F46 real con agente propio en `172.20.50.161` | 52 OIDs, ARP/VLAN/FDB reales | Un switch físico con SNMP |
| Captura pasiva | F47 real con Scapy y `-tags pcap` | Cinco tipos de observación con `CAP_NET_RAW` | SPAN/tap en switch físico |

El laboratorio usa servicios reales, pero no sustituye al equipo físico.
No se afirma detección donde no hay captura, ni L2 más allá del dominio de broadcast.

Tras `./run.sh`, comprobar los hallazgos en el puerto `18080`:

```bash
curl -s -b cookies.txt 'http://127.0.0.1:18080/api/v1/diagnostics/dhcp?site_id=2' | jq '.anomalies[].type'
curl -s -b cookies.txt 'http://127.0.0.1:18080/api/v1/diagnostics/snmp?site_id=2' | jq '.anomalies[].type'
curl -s -b cookies.txt 'http://127.0.0.1:18080/api/v1/diagnostics/passive?site_id=2' | jq '.anomalies[].type'
```

## Limpieza

```bash
docker rm -f lab-netdiag lab-dhcp lab-snmp lab-scapy
docker network rm netdiag-lab
```
