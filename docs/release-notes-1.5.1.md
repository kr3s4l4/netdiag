# NetDiag 1.5.1

Release de validación. Incluye el código publicado en `master`, 37 commits después de `v1.5.0`. El tag `v1.5.0` no se ha movido.

## Seguridad validada con servicios reales

- F43: histórico IP→MAC y detección de cambios de MAC del gateway.
- F44: sonda DHCP UDP/68 contra `dnsmasq` rogue, con `dhcp_unauthorized_server` y alerta `dhcp_rogue`.
- F45: reglas dedicadas `arp_spoofing`, `dhcp_rogue`, `snmp_anomaly` y `passive_anomaly`, con precedencia sobre `anomaly`.
- F46: SNMP profundo con IP-MIB, ARP, VLAN y FDB; comunidad guardada enmascarada.
- F47: captura pasiva opcional `-tags pcap`, metadata-only, con cinco detectores sobre tráfico real.
- Laboratorio Docker reproducible en `scripts/lab/`: DHCP rogue, agente SNMP y generador Scapy.

## Interfaz y uso

- Trazas: el formulario cabe completo y el aviso usa un icono de 18×18.
- Topología: el encuadre y el zoom se conservan entre actualizaciones; botón «Reordenar».
- Topología: el centro de cada red prioriza servidor DHCP real, gateway, red y router.
- Inglés: `README.en.md`, `docs/quickstart.en.md`, `SECURITY.en.md` e issue template.

## Distribución y licencias

- F35: licencia Ed25519 offline, sin telemetría ni servidor de licencias.
- F36: contenedor `.ndc` firmado e inspeccionable.
- F37: build reproducible, SBOM, procedencia y verificación offline.
- F39: demo sintética aislada y marcada.
- F40: vistas guardadas y enlaces compartidos.
- F41: `min_version` firmado en el manifiesto; pendiente el ensayo humano del runbook CRA.

No hay telemetría, llamadas de licencia online ni DRM. El descubrimiento y el escaneo siguen siendo ilimitados.

## Verificación

La firma debe comprobarse con la clave pública `netdiag-release` de `docs/public_keys.md`:

```bash
BASE=https://github.com/kr3s4l4/netdiag/releases/download/v1.5.1
curl -fsSLO "$BASE/SHA256SUMS"
curl -fsSLO "$BASE/SHA256SUMS.minisig"

minisign -V \
  -P 'RWTJixPYyPcyWf0VEHdzn6lRYxgrPS3z+EbRBKlI9E+hX0lj1227yrZY' \
  -m SHA256SUMS -x SHA256SUMS.minisig

sha256sum -c SHA256SUMS
```

El comentario de confianza debe indicar `version:1.5.1` y el SHA256 de `SHA256SUMS`.
