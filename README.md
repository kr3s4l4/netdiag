# NetDiag

[🇬🇧 English](README.en.md) · [🇪🇸 Español](README.md) — La interfaz web cambia de idioma en **Ajustes → Idioma**.

**Escáner de red y monitor de inventario self-hosted.** Un único binario Go con interfaz web bilingüe (español/inglés), descubrimiento activo/pasivo, SNMP, topología L2/L3, alertas cifradas, copias de seguridad y generación de informes. Sin dependencias externas en tiempo de ejecución, sin telemetría y sin servicios en la nube.

```
┌──────────────────────────────────────────────────────────────┐
│  Descubre → identifica → almacena → vigila → alerta → informa │
└──────────────────────────────────────────────────────────────┘
```

## Características

- **Descubrimiento en tres fases**: vecinos ARP/ICMP, barrido TCP y enriquecimiento (DNS inverso, banners, TLS, SNMP, LLDP/CDP/FDB).
- **Identidad estable**: los hosts se rekeyean a su MAC, conservando notas, etiquetas y favoritos aunque cambie la IP. La identidad está namespaceada por sede, de modo que la misma IP o MAC puede existir en varias sedes sin colisionar.
- **Multi-sede**: oficinas, plantas o datacenters conviven en una sola base SQLite. Cada sede tiene sus propias redes, hosts, escaneos, informes y métricas; el mismo CIDR puede repetirse en sedes distintas. El selector de sede de la barra superior cambia el alcance de toda la interfaz.
- **Portabilidad por sede**: exporta una sede a un paquete JSON e impórtala en otra sede o en otra instancia, en modo `merge` (conservar) o `replace` (reemplazar).
- **Inventario rico**: fabricante por OUI, tipo de dispositivo con puntuación y evidencia, sistema operativo probable, puertos, servicios, certificados y modelo SNMP.
- **Topología interactiva**: SVG con zoom, arrastre y agrupación por subred; enlaces por LLDP/CDP/bridge/ARP con confianza.
- **Diagnósticos empresariales**: detección heurística de lazos STP, fugas de VLAN, desajustes de dúplex, problemas de MTU, tormentas de broadcast, AP no autorizados e incumplimientos de SLA, con evidencia, confianza y limitaciones explícitas.
- **Integraciones externas**: exporta eventos (alertas y escaneos) a Elasticsearch, syslog, webhooks firmados, Slack, Discord, MS Teams, Telegram, PagerDuty, Kafka REST Proxy y Redis Streams. Cola acotada, límite global de velocidad, guardia SSRF y secretos cifrados AES-256-GCM.
- **Detección de VPN (F12)**: observación pasiva de túneles WireGuard, OpenVPN, IPsec y PPTP a partir de sockets e interfaces del kernel, con evidencia, confianza y contadores acumulados.
- **Análisis WiFi (F11)**: captura pasiva 802.11 (binario `-tags pcap`) con APs (BSSID, SSID, canal, banda, cifrado, señal), clientes por AP, interferencia por canal y detección heurística de AP no autorizado.
- **Dashboards personalizables (F18)**: vistas por sede con hasta 16 widgets reordenables en una rejilla de 12 columnas, fuentes limitadas a un catálogo seguro de endpoints ya autorizados.
- **Comparativa temporal (F21)**: diff de dos ventanas equivalentes (por defecto hoy frente a hace 7 días) con cambios por dispositivo, topología y flota, evidencia para cada hallazgo y limitaciones explícitas cuando falta información.
- **Modo auditoría (F22)**: líneas base manuales comprimidas por sede, diff detallado entre snapshots y exportación JSON/GZIP, sin secretos en los datos guardados.
- **Monitor periódico**: escaneos programados, detección de hosts caídos y nuevos, histórico de latencia y disponibilidad.
- **Alertas**: reglas integradas y personalizadas, cooldown, deduplicación y siete canales (webhook HMAC, Slack, Discord, Telegram, PagerDuty, SMTP y syslog).
- **Informes**: PDF, CSV, JSON y HTML con portada, resumen ejecutivo, inventario y alertas.
- **Roles y multiusuario**: `admin` (todo), `operator` (operación) y `viewer` (solo lectura), con creación, cambio de rol, restablecimiento de contraseña y borrado protegido del último administrador.
- **Bilingüe y guiado**: español por defecto e inglés con un clic, guía interactiva con búsqueda y recorrido guiado por la interfaz en el primer acceso.
- **Integraciones**: OpenAPI 3.1 en `/api/v1/openapi.json` y métricas Prometheus en `/metrics`.
- **Copias de seguridad**: ZIP autocontenido (base SQLite consistente, `config.yaml` y `secret.key`), automático por intervalo con retención, desde la UI, la API o `netdiag backup`.
- **Seguridad por defecto**: Argon2id, sesiones en servidor, CSRF, límite de intentos, cabeceras CSP/HSTS, auditoría, secretos cifrados con AES-256-GCM y token opcional para `/metrics`.
- **Un solo binario**: SPA embebida, SQLite embebido (modernc) y configuración YAML. Funciona offline.

## Hacia dónde va NetDiag

La versión 1.5 ya cubre el núcleo: descubrimiento, inventario, topología, diagnósticos, alertas, informes, multi-sede, auditoría y actualización firmada. El [roadmap](docs/roadmap.md) describe el siguiente tramo, **todavía por implementar**:

| Bloque | Qué aporta | Estado |
| --- | --- | --- |
| F27–F32 | Arranque asistido, panel de salud, buscador global, centro de notificaciones, diagnósticos accionables e informe ejecutivo de una página | Planificado |
| F35–F38 | Activación de licencia offline, contenedor firmado `.ndc`, publicación reproducible y paquete de confianza empresarial | Planificado |
| F39–F40 | Modo demo con datos sintéticos y vistas guardadas | Planificado |
| F41–F42 | Cumplimiento CRA por dependencias de terceros y validación de mercado sin telemetría | Documentado, en ejecución |

Las decisiones ya cerradas —AGPL para Community, licencia comercial para Pro/Enterprise, cobro por banda de hosts gestionados, sin telemetría y sin guardia 24/7 que no se pueda cumplir—están en el propio roadmap. Si quieres influir en el orden, cuenta tu caso de uso en el Discussion fijado o abre un issue con la plantilla **Caso de uso real**.

## Inicio rápido

Requisitos: Go 1.27 o superior. Para ARP activo, además `libpcap-dev` y gcc.

```bash
git clone <repo> netdiag && cd netdiag
make build                 # bin/netdiag portable (sin cgo)
./bin/netdiag serve        # http://127.0.0.1:8080
```

En el primer acceso la interfaz pide crear la cuenta de administrador. Después:

1. Usa el **selector de sede** (barra superior) para elegir el alcance; por defecto existe la sede `Principal`.
2. **Ajustes → Sedes** para crear, renombrar, activar, exportar o importar sedes (solo administradores).
3. **Ajustes → Redes** para añadir la subred a monitorizar en la sede activa (se sugieren las locales).
4. **Escanear** para lanzar un descubrimiento manual en la sede activa.
5. **Ajustes → Escaneo** para activar el monitor periódico.

Datos y configuración por defecto:

| Elemento | Ruta |
| --- | --- |
| Configuración | `~/.config/netdiag/config.yaml` (`NETDIAG_CONFIG`) |
| Datos | `~/.local/share/netdiag` (`NETDIAG_DATA_DIR`) |
| Base de datos | `<datos>/netdiag.db` |
| Clave de cifrado | `<datos>/secret.key` |
| Informes | `<datos>/reports/` |

## Multi-sede

Todas las sedes conviven en la misma base SQLite: no hay proyectos ni directorios separados. Al arrancar se crea la sede `Principal` (id `1`). El selector de la barra superior guarda la sede activa en `localStorage` (`netdiag-site`) y la interfaz añade automáticamente `site_id` a las peticiones operativas; al cambiar de sede se recargan inventario, alertas, topología, informes, panel y métricas.

- **Datos por sede**: redes, hosts, escaneos, alertas, informes y métricas. El mismo CIDR puede configurarse en sedes distintas; la unicidad se garantiza por `(site_id, cidr)`.
- **Identidad por sede**: la clave estable de un host se guarda como `s<site_id>:<mac|ip:...>`. Exportar un paquete elimina el prefijo y al importarlo se antepone la sede de destino, por lo que el paquete es portable entre sedes e instancias.
- **Sedes**: `admin`/`owner` pueden crear, renombrar, exportar, importar y eliminar sedes. No se puede eliminar la última sede; al borrar una sede sus datos se trasladan a otra (resolviendo colisiones de CIDR).
- **Importación**: modo `merge` (conserva lo existente y actualiza metadatos) o `replace` (sustituye los datos de la sede destino). Acepta JSON o un formulario multipart con el campo `file`.
- **Informes**: `site_id=0` en el panel, métricas e informes significa «todas las sedes».

Ejemplo con la API:

```bash
curl -b cookies.txt -H 'X-CSRF-Token: <csrf>' -H 'Content-Type: application/json' \
  -d '{"name":"Sucursal Norte","description":"Planta 2"}' \
  http://127.0.0.1:8080/api/v1/sites

curl -b cookies.txt http://127.0.0.1:8080/api/v1/hosts?site_id=2
curl -b cookies.txt http://127.0.0.1:8080/api/v1/sites/2/export -o sucursal.json
curl -b cookies.txt -H 'X-CSRF-Token: <csrf>' -F 'file=@sucursal.json' \
  'http://127.0.0.1:8080/api/v1/sites/3/import?mode=merge'
```

## Diagnósticos empresariales

La vista **Diagnósticos** y `GET /api/v1/diagnostics/enterprise` correlacionan los datos ya recogidos por NetDiag para señalar problemas típicos de redes corporativas. Cada hallazgo incluye tipo, severidad, hosts implicados, evidencia, confianza (1–100) y una recomendación. Los detectores exigen varias señales simultáneas para evitar falsos positivos y respetan `site_id` y la ventana `from`/`to`.

| Categoría | Señales utilizadas |
| --- | --- |
| Lazos STP | Ciclos en el grafo L2 de vecindades LLDP/CDP/FDB con confianza suficiente |
| Fugas de VLAN | Un mismo puerto L2 que alcanza hosts de redes distintas, o adyacencia bridge entre dispositivos L2 de subredes distintas |
| Dúplex | Velocidades IF-MIB incompatibles en ambos extremos de un enlace LLDP/CDP |
| MTU | Interfaz local con MTU bajo correlacionada con pérdida en su red |
| Tormentas | Pérdida y latencia simultáneas en varios hosts de una misma subred |
| AP no autorizados | Puntos de acceso fuera de `diagnostics.authorized_aps` o sin identidad SNMP suficiente |
| SLA | Disponibilidad y latencia media por host frente a los umbrales configurados |
| ARP / MITM | Cambio de MAC del gateway o de cualquier IP entre escaneos, una MAC con muchas IPs y una IP con varias MAC |
| DHCP rogue | Varios servidores DHCP que responden, servidor no autorizado, suplantación del gateway y cambio de MAC del servidor |
| SNMP profundo | ARP del switch frente al histórico, MAC en varios puertos/VLAN o movida entre escaneos, y desacuerdo entre FDB y LLDP/CDP |
| Captura pasiva | ARP gratuito repetido, ICMP redirect, DHCP OFFER/ACK de otro servidor, SYN entrantes y consultas DNS de alta entropía |

**Limitaciones:** por defecto NetDiag no captura tramas ni BPDUs, no lee OID de dúplex ni contadores broadcast o de fragmentación, y no ve el tráfico ARP o DHCP de otros clientes. Los sondeos son activos (ICMP/ARP, UDP/68 y SNMP) y sus hallazgos se infieren de tablas e históricos: son pistas para investigar, no confirmación protocolaria de un ataque. La captura pasiva es opt-in, metadata-only y también requiere validación humana. El informe incluye siempre esas limitaciones en `limitations`. Con `enterprise_mode: false` solo se evalúan los umbrales SLA; los detectores de red permanecen desactivados.

**DHCP rogue.** Con `diagnostics.dhcp_probe: true` (por defecto) se envían DHCPINFORM y DHCPDISCOVER por UDP/68 sin raw sockets; las respuestas se guardan en `dhcp_observations` durante `diagnostics.dhcp_history_days` (30 días). `NETDIAG_DHCP_PROBE`, `NETDIAG_DHCP_PROBE_TIMEOUT` y `NETDIAG_DHCP_AUTHORIZED` permiten ajustar el sondeo y la lista de servidores de confianza. Histórico y anomalías en `GET /api/v1/diagnostics/dhcp`.

**SNMP profundo.** Con `diagnostics.snmp_deep: true` (por defecto) el recolector consulta `ipAddrTable` (`.1.3.6.1.2.1.4.20`), las tablas ARP `.4.22`/`.4.35`, Q-BRIDGE `.1.3.6.1.2.1.17.7.*` y BRIDGE-MIB `.1.3.6.1.2.1.17.4.*`, con límites de 512 direcciones, 2000 ARP, 5000 FDB y 256 VLAN. Las filas se guardan por sede durante `diagnostics.snmp_history_days` (30 días). La comunidad que respondió se guarda **enmascarada** (`public` → `pu***ic`); la credencial en claro nunca se persiste ni se expone. Variables: `NETDIAG_SNMP_DEEP` y `NETDIAG_SNMP_HISTORY_DAYS`. Hallazgos y tablas en `GET /api/v1/diagnostics/snmp`. Los tipos `snmp_*` usan la regla de alerta `snmp_anomaly`.

**Captura pasiva.** Desactivada por defecto. Requiere binario compilado con `-tags pcap`, `CAP_NET_RAW` y `diagnostics.passive_capture: true`. Abre una ventana acotada tras cada escaneo con BPF estrecho (`arp`, `icmp`, UDP/67-68-53 y SYN TCP) y guarda **solo metadatos** en `passive_observations` durante `diagnostics.passive_history_days` (7 días): nunca payload, credenciales ni respuestas DNS. Límites: `passive_seconds` (10), `passive_max_packets` (2000) y `passive_promiscuous` (false). Variables: `NETDIAG_PASSIVE_CAPTURE`, `NETDIAG_PASSIVE_IFACE`, `NETDIAG_PASSIVE_SECONDS`, `NETDIAG_PASSIVE_MAX_PACKETS`, `NETDIAG_PASSIVE_PROMISC` y `NETDIAG_PASSIVE_HISTORY_DAYS`. En builds portables el endpoint devuelve `available:false` y el escaneo continúa. Hallazgos y observaciones en `GET /api/v1/diagnostics/passive`; los tipos `passive_*` usan la regla `passive_anomaly`.

```bash
curl -b cookies.txt 'http://127.0.0.1:8080/api/v1/diagnostics/enterprise?site_id=1&from=24h'
curl -b cookies.txt 'http://127.0.0.1:8080/api/v1/diagnostics/stp?site_id=1'
curl -b cookies.txt 'http://127.0.0.1:8080/api/v1/diagnostics/vlan?site_id=1'
curl -b cookies.txt 'http://127.0.0.1:8080/api/v1/diagnostics/arp?site_id=1'
curl -b cookies.txt 'http://127.0.0.1:8080/api/v1/diagnostics/dhcp?site_id=1'
curl -b cookies.txt 'http://127.0.0.1:8080/api/v1/diagnostics/snmp?site_id=1'
curl -b cookies.txt 'http://127.0.0.1:8080/api/v1/diagnostics/passive?site_id=1'
```

## Integraciones externas

Cada integración es un destino independiente (`Integration {type, config, enabled}`) que se gestiona desde **Configuración → Integraciones** o mediante la API. Los cambios se aplican en caliente; los secretos se cifran con AES-256-GCM y la API solo devuelve `********`. Los eventos se entregan en segundo plano con una cola acotada y un límite global de eventos por minuto; un destino lento nunca bloquea un escaneo.

| Tipo | Configuración esencial |
| --- | --- |
| `elasticsearch` | `url`, `index`, y `api_key` o `username`+`password` |
| `syslog` | `host`, `port`, `protocol` (`udp`/`tcp`), `app_name` |
| `webhook` | `url`, `secret` opcional (firma HMAC `X-NetDiag-Signature`) |
| `slack` / `discord` / `msteams` | `url` del webhook entrante |
| `telegram` | `token`, `chat_id`, `api_url` opcional |
| `pagerduty` | `routing_key`, `url` opcional |
| `kafka` | `url` del REST Proxy, `topic`, `token` opcional |
| `redis` | `addr`, `stream`, `max_len`, credenciales ACL opcionales |

**Enviar logs a un Elasticsearch local.** El guardia SSRF bloquea destinos privados por defecto, así que hay que autorizarlos explícitamente:

```yaml
integrations:
  enabled: true
  allow_private_targets: true
  timeout: 10s
```

```bash
CSRF=$(curl -sb cookies.txt http://127.0.0.1:8080/api/v1/auth/status | python3 -c 'import json,sys;print(json.load(sys.stdin)["csrf"])')
curl -b cookies.txt -H "X-CSRF-Token: $CSRF" -H 'Content-Type: application/json' \
  -d '{"name":"Elasticsearch local","type":"elasticsearch","enabled":true,"config":{"url":"http://127.0.0.1:9200","index":"netdiag-events"}}' \
  http://127.0.0.1:8080/api/v1/integrations
curl -b cookies.txt -H "X-CSRF-Token: $CSRF" -X POST http://127.0.0.1:8080/api/v1/integrations/1/test
```

Cada evento se indexa como una operación `index` en `_bulk` (NDJSON) con `kind`, `site_id`, `severity`, `title`, `message`, `host_ip`, `data` y `timestamp`. La prueba envía un evento sintético y guarda `last_test_at`/`last_error`.

## Detección de VPN

La vista **VPN** y `GET /api/v1/vpn/tunnels` muestran los túneles observados en el host donde corre NetDiag. La detección es pasiva y heurística, igual que los diagnósticos F4: lee `/proc/net/{udp,udp6,tcp,tcp6}`, `/proc/net/dev` y `operstate` de sysfs; nunca inyecta ni descifra tráfico. Cada observación guarda tipo, endpoints, contadores acumulados, evidencia y confianza (1–100).

```yaml
vpn:
  enabled: true
  interval: 5m
  min_confidence: 60
  stale_after: 30m
```

Las variables de entorno `NETDIAG_VPN` y `NETDIAG_VPN_INTERVAL` sustituyen a `enabled` e `interval`. El bucle periódico se ejecuta cada `interval` (mínimo 30 s); las observaciones con confianza inferior al umbral se descartan y las no vistas durante `stale_after` pasan a estado `stale`.

```bash
curl -sb cookies.txt -H "X-CSRF-Token: $CSRF" -X POST \
  http://127.0.0.1:8080/api/v1/vpn/scan
curl -sb cookies.txt http://127.0.0.1:8080/api/v1/vpn/tunnels | python3 -m json.tool
```

`POST /api/v1/vpn/scan` permite a `admin`, `owner` y `operator` forzar una observación; borrar una observación requiere `admin`/`owner`. Los puertos conocidos (`51820` WireGuard, `1194` OpenVPN, `500`/`4500` IPsec, `1723` PPTP) también se reconocen en el inventario de escaneo. Limitaciones: TCP/443 es ambiguo (confianza baja), los contadores son acumulativos desde el arranque y un túnel inactivo o cifrado puede no dejar rastro.

## Análisis WiFi

La vista **WiFi** y `GET /api/v1/wifi/aps` muestran los puntos de acceso detectados con su canal, banda, cifrado y nivel de señal; `GET /api/v1/wifi/clients` lista las estaciones asociadas y `GET /api/v1/wifi/channels` agrega APs, clientes, solape y sugerencia de canal por banda.

La captura es pasiva y requiere un binario compilado con `-tags pcap` y una interfaz **ya puesta en modo monitor** por el operador (`iw dev wlan0 set monitor ...`); NetDiag no cambia el modo del dispositivo. El bucle periódico se configura con `wifi.interval` y una ventana de `wifi.scan_seconds`. Si se definen `wifi.channels`, NetDiag intenta saltar entre ellos con `iw` de forma best effort (si `iw` no está disponible, captura en el canal actual).

```bash
make build-pcap   # o: go build -tags pcap -o bin/netdiag-pcap ./cmd/netdiag
./bin/netdiag-pcap serve -c config.yaml --wifi-monitor-iface wlan0mon --wifi-scan-interval-sec 120
```

`wifi.authorized_bssids` y `wifi.known_ssids` forman el inventario de referencia para marcar APs no autorizados: un BSSID ausente del inventario tiene confianza 90; si solo coincide el SSID conocido, la confianza baja a 65 porque el SSID puede suplantarse. Sin inventario no se emite ningún veredicto. `GET /api/v1/wifi/status` expone la capacidad de captura y la configuración efectiva sin secretos. `POST /api/v1/wifi/scan` (escritor) fuerza una ventana; `DELETE /api/v1/wifi/aps` (admin) borra las observaciones de una sede.

Limitaciones: el binario portable no puede capturar 802.11 y devuelve `503`; los canales 6 GHz comparten numeración con 2,4 GHz y se distinguen por la frecuencia de radiotap; el solape en 2,4 GHz es una heurística de separación menor a cinco canales; un AP oculto puede no revelar su SSID en beacons.

## Dashboards personalizados

La vista **Dashboards** permite crear, renombrar y eliminar vistas por sede. Cada dashboard admite hasta 16 widgets en una rejilla fija de 12 columnas; se arrastran con HTML5 drag-and-drop nativo y los cambios se guardan mediante `PUT /api/v1/dashboards/{id}`. El botón **Ejemplo "Mi red"** crea una vista con cuatro widgets (resumen, dispositivos, alertas y topología), cumpliendo el caso de cierre de F18.

Los tipos disponibles se consultan en `GET /api/v1/dashboards/catalog`: resumen, dispositivos, puertos frecuentes, topología, alertas, escaneos recientes, APs WiFi y túneles VPN. El backend deriva la URL de datos del tipo validado: un cliente no puede inyectar una URL arbitraria ni evade el RBAC. Escribir dashboards requiere `admin`, `owner` u `operator`; leerlos está disponible para cualquier rol autenticado.

```bash
curl -sb cookies.txt -H "X-CSRF-Token: $CSRF" -H 'Content-Type: application/json' \
  -d '{"site_id":1}' http://127.0.0.1:8080/api/v1/dashboards/example
curl -sb cookies.txt http://127.0.0.1:8080/api/v1/dashboards | python3 -m json.tool
```

## Comparativa temporal

La vista **Comparativa** responde al caso de cierre de F21: «diff entre hoy y hace 7 días». Compara dos ventanas de igual duración; por defecto A son las últimas 24 h (`hoy (24h)`) y B las 24 h que terminan hace siete días (`hace 7 días`). Se pueden elegir separaciones de 1, 7 o 30 días y ventanas de 6 h, 24 h, 48 h o 7 días.

Cada lado del diff incluye muestras, disponibilidad, RTT medio, series de latencia y disponibilidad, el detalle por dispositivo y, cuando existe, un snapshot compacto de topología. Los cambios se clasifican en `host.*`, `topology.*` y `fleet.*`, van ordenados por tipo y llevan evidencia (número de muestras, umbral aplicado o nodo/enlace comparado). Los umbrales son 1,0 puntos porcentuales para disponibilidad y `max(2 ms, 10 % del RTT anterior)` para latencia; se ignoran diferencias por debajo de esos valores. El reporte se trunca en 500 cambios e incluye `limitations` cuando un periodo no tiene métricas o snapshots: **un dato ausente nunca se interpreta como «sin cambios»**.

Los snapshots se capturan automáticamente tras cada escaneo correcto y cada hora para todas las sedes, y se purgan con `storage.metrics_days` (o `storage.retention_days` como respaldo). Las identidades de hosts y topología se comparan por MAC, IP o ID, por lo que un cambio de IP con la misma MAC no genera un falso «dispositivo nuevo».

```bash
curl -sb cookies.txt -H "X-CSRF-Token: $CSRF" -H 'Content-Type: application/json' \
  -d '{}' http://127.0.0.1:8080/api/v1/history/compare | python3 -m json.tool
```

## Modo auditoría

La vista **Auditoría** cierra F22 («guardar snapshot, hacer cambios, ver diff»). Guarda una línea base completa de la sede activa, permite cambiar la red o la configuración y compara dos snapshots para mostrar qué cambió. Cada captura incluye hosts, puertos, redes, vecinos, topología, escaneos recientes, reglas, canales, integraciones, resumen de alertas y valores no sensibles de configuración. Se comprime con gzip en SQLite (máximo 16 MiB almacenados por snapshot) y se conservan como máximo 100 líneas base por sede.

**Los secretos nunca se guardan**: de canales, integraciones y reglas solo se conservan los *nombres* de las claves de configuración (`config_keys`), nunca sus valores. Las notas de hosts sí forman parte del inventario auditado. Cada creación, borrado y diff queda registrado en la auditoría de seguridad.

El diff excluye campos volátiles (`last_seen` y muestras de latencia) y compara exclusivamente hechos estables. Incluye cambios en hosts (`host.added`, `host.changed`, `host.port_added`, `host.port_changed`, `host.port_removed`), redes, vecinos, reglas, canales, integraciones, escaneos, topología (`topology.node_*`, `topology.edge_*`), alertas y configuración. Cada entrada lleva evidencia legible; el informe se ordena por tipo, se trunca en 500 cambios e indica `truncated: true` si se supera el límite.

```bash
# 1) línea base antes de los cambios
curl -sb cookies.txt -H "X-CSRF-Token: $CSRF" -H 'Content-Type: application/json' \
  -d '{"site_id":1,"name":"Antes del cambio"}' \
  http://127.0.0.1:8080/api/v1/audits/snapshot

# 2) ... hacer cambios en inventario, reglas o configuración ...

# 3) segunda línea base y diff
curl -sb cookies.txt -H "X-CSRF-Token: $CSRF" -H 'Content-Type: application/json' \
  -d '{"site_id":1,"name":"Después"}' \
  http://127.0.0.1:8080/api/v1/audits/snapshot
curl -sb cookies.txt -H "X-CSRF-Token: $CSRF" -X POST \
  http://127.0.0.1:8080/api/v1/audits/1/diff/2 | python3 -m json.tool

# 4) exportar para archivo o análisis externo
curl -sb cookies.txt -o auditoria.json.gz \
  'http://127.0.0.1:8080/api/v1/audits/1/export?format=gzip'
```

Crear y borrar snapshots requiere `admin`, `owner` u `operator`; listar, consultar, comparar y exportar está disponible para cualquier rol autenticado. El detalle (`GET /api/v1/audits/{id}`) devuelve solo metadatos salvo que se pida `?data=1`.

## Auto-actualización

La tarjeta **Actualizaciones** de *Ajustes → Sistema* cierra F23: comprueba una versión firmada, descarga el binario, lo verifica, lo instala de forma atómica y permite reiniciar o revertir. La comprobación está desactivada por defecto; para habilitarla hacen falta `update.enabled: true`, una URL de manifiesto y la clave pública del firmante:

```yaml
update:
  enabled: true
  manifest_url: https://updates.example.com/netdiag/manifest.json
  public_key: RWQf6...   # base64 de la clave pública minisign, o el archivo de dos líneas
  check_interval_hours: 24
  download_timeout: 120s
  max_download_mb: 512
  restart_command: ""    # ej. systemctl; vacío usa syscall.Exec sobre el propio binario
  restart_args: []       # ej. ["restart", "netdiag"]
```

El manifiesto está firmado con **minisign** (Ed25519 sobre BLAKE2b-512, algoritmo `ED`) y cubre exactamente `schema`, `version`, `url`, `checksum`, `released_at` y `notes`. El comentario de confianza liga además `version:` y `sha256:` para impedir sustituciones. La URL del manifiesto y del binario deben ser HTTPS; HTTP solo se acepta en `localhost`/loopback para espejos internos o pruebas. Las URL con credenciales se rechazan. Antes de instalar se valida el SHA256 del binario y, en Linux, la cabecera ELF; el tamaño está limitado por `update.max_download_mb`. El manifiesto está limitado a 1 MiB.

La instalación renombra el ejecutable a `<binario>.netdiag-old`, coloca el nuevo binario con el mismo modo de permisos y escribe `<data_dir>/update-pending.json`. Al arrancar, NetDiag confirma la actualización si el binario coincide con el checksum firmado; si está corrupto, restaura automáticamente la copia. `netdiag update rollback` hace lo mismo de forma manual. `update.restart_command`/`restart_args` permite reiniciar mediante systemd u otro supervisor; sin comando se usa `syscall.Exec`. El reinicio de la API se difiere 500 ms para que la respuesta HTTP llegue completa.

```bash
# Comprobar, instalar sin reiniciar y confirmar tras verificar el arranque
curl -sb cookies.txt -H "X-CSRF-Token: $CSRF" \
  http://127.0.0.1:8080/api/v1/update/check | python3 -m json.tool
curl -sb cookies.txt -H "X-CSRF-Token: $CSRF" -H 'Content-Type: application/json' \
  -d '{"version":"1.6.0","restart":false}' \
  http://127.0.0.1:8080/api/v1/update/apply | python3 -m json.tool
netdiag update commit

# Si la nueva versión no arranca correctamente
netdiag update rollback
```

También están disponibles `netdiag update check|apply|rollback|commit` y el flag `-update-check-interval-hours` (24 por defecto; solo sobrescribe la configuración cuando se pasa explícitamente). Las variables de entorno `NETDIAG_UPDATE_ENABLED`, `NETDIAG_UPDATE_MANIFEST_URL`, `NETDIAG_UPDATE_PUBLIC_KEY`, `NETDIAG_UPDATE_INTERVAL_HOURS` y `NETDIAG_UPDATE_RESTART_COMMAND` permiten organizaciones sin archivo de configuración writable. Los eventos WebSocket `update.available`, `update.applied`, `update.installed`, `update.restarting`, `update.restart_failed` y `update.rolled_back` alimentan la notificación de la interfaz.

Consultar el estado y comprobar versiones es posible para cualquier rol autenticado; aplicar o revertir exige `admin`/`owner`. El manejo del gestor usa solo la biblioteca estándar más `x/crypto/blake2b` (ya presente); no se incorporó la dependencia externa sugerida en el roadmap para mantener el binario portable y el control total de la verificación.

## Licencia offline y contenedores firmados

`netdiag license show|activate|clear|sign` gestiona una licencia Ed25519 local en `<data_dir>/license.json`, sin consultas a servidores externos. `GET /api/v1/system/license` expone edición, límites y avisos a cualquier rol autenticado; caducidad o exceso de banda nunca bloquean la lectura ni borran datos. El formato y la clave pública están en `docs/licenciamiento.md` y `docs/public_keys.md`.

`netdiag container inspect|verify|build` trabaja con contenedores `.ndc`: ZIP con manifiesto canónico, `payload/`, firma minisign y SBOM opcional. `verify` devuelve código `1` si la firma o el contenido no coinciden; `inspect` permite ver el contenido aunque sea un archivo manipulado. El formato y los comandos están en `docs/publicacion-segura.md`.

## Fases opcionales diferidas

El roadmap clasifica F13, F14, F15, F16, F17 y F26 como opcionales/de baja prioridad. Se documenta aquí por qué no se incluyen en esta entrega:

| Fase | Motivo de diferimiento | Alternativa cubierta hoy |
| --- | --- | --- |
| F13 — Multi-tenant | Solo tiene sentido si NetDiag se convierte en SaaS; introduces `tenant_id` en todo el modelo, cambia la autenticación y multiplica el riesgo de aislamiento. Requiere F25. | Multi-sede real con una sola base, sede `Principal` por defecto, aislamiento por `site_id` y exportación/importación por sede. |
| F14 — Plugins | Cargar `.so` o JS externo ejecuta código de terceros dentro del proceso y rompe el modelo «binario único y seguro por defecto». | F10 exporta eventos a Elasticsearch, syslog, webhook, Kafka y Redis mediante configuración cifrada, sin ejecutar código externo. |
| F15 — Forense pcap | Guardar capturas completas tiene coste de disco, privacidad y retención elevado; la demanda es inferior a la del inventario continuo. El binario portable tampoco captura tráfico crudo. | Informes JSON/CSV/PDF, snapshots comprimidos de auditoría (F22) y comparativa temporal (F21) como evidencia histórica; `-tags pcap` habilita ARP activo y WiFi. |
| F16 — DPI | TLS 1.3 cifra casi todo el payload; la identificación por SNI/HTTP Host es parcial y depende de tráfico observable, con falsos positivos. | Clasificación por banners, puertos, SNMP y heurísticas con evidencia y confianza en diagnósticos e inventario. |
| F17 — Threat Intel | Requiere servicios externos, claves, límites de tasa y listas que generan falsos positivos; contradice el funcionamiento offline y autohospedado. | Reglas y canales de alertas, integraciones salientes y detección de AP no autorizado en F11, todo local y auditable. |
| F26 — Cloud sync | Igual que F13: solo aplica a un servicio SaaS centralizado y envía inventario de red a terceros. Requiere F25. | Copias ZIP cifradas localmente, exportación por sede y despliegue autocontenido sin dependencia de nube. |

Estas decisiones no borran las ideas del roadmap: quedan como extensiones condicionadas a un modelo SaaS o a una necesidad operativa explícita, y no bloquean el cierre de la versión 1.5. Las mejoras propuestas para 1.5.1, las decisiones cerradas F33–F34 y la implementación pendiente F35–F40 están en [docs/roadmap.md](docs/roadmap.md).

## Licenciamiento, compra y publicación

- Antes de montar estructura comercial, el plan de [validación sin telemetría](docs/validacion.md) mide uso real y disposición a pagar.
- **Community** es software libre bajo [AGPL-3.0](LICENSE), con avisos de terceros en [NOTICE](NOTICE) y [THIRD_PARTY_LICENSES.md](THIRD_PARTY_LICENSES.md).
- **Pro y Enterprise** se venden con licencia comercial anual. El cobro es por banda de **hosts gestionados**, nunca por informe, alerta o host detectado.
- La licencia se activa localmente, sin telemetría ni servidor online; el espejo de actualizaciones es opcional o puede ser interno.
- Los artefactos oficiales se construyen de forma reproducible, se firman con minisign y se publican con SBOM y atestación: [docs/publicacion-segura.md](docs/publicacion-segura.md).
- Los precios, SKU y flujo de compra están en [docs/comercializacion.md](docs/comercializacion.md); el paquete que recibe un comprador corporativo, en [docs/confianza-empresarial.md](docs/confianza-empresarial.md).
- Las dudas legales se resuelven en [docs/licenciamiento.md](docs/licenciamiento.md); las vulnerabilidades se reportan por [SECURITY.md](SECURITY.md).

## Compilación

```bash
make build       # portable, CGO_ENABLED=0
make build-pcap  # ARP activo con gopacket + libpcap
make test        # pruebas unitarias
make test-race   # detector de carreras
make release     # Linux amd64 + arm64
```

El ARP activo permite descubrir dispositivos que no responden a ICMP ni TCP. El binario portable usa la tabla de vecinos del kernel y `NudgeARP`, suficiente en la mayoría de redes domésticas y de oficina.

```bash
sudo apt install libpcap-dev
make build-pcap
sudo setcap cap_net_raw,cap_net_admin+eip bin/netdiag-pcap
```

## CLI

```bash
netdiag serve [--listen 0.0.0.0:8080] [--no-monitor]
netdiag scan 192.168.1.0/24 [--profile quick|common|full|custom]
                            [--ports 22,80,8000-8100] [--snmp] [--deep]
                            [--timeout 900] [--progress] [--json]
netdiag scan --network 3    # una red ya configurada
netdiag hosts [--search texto] [--status up] [--type server] [--json|--csv]
netdiag report [--kind inventory|executive|alerts] [--format pdf|csv|json|html]
               [--from 2026-01-01] [--to 2026-01-31] [--out informe.pdf]
netdiag config [show|path|init|validate]
netdiag doctor
netdiag user [list|add|passwd|role|delete]
netdiag backup [--out RUTA] [--keep N]
```

Escaneo distribuido en sedes remotas, sin agente ni telemetría:
[docs/escaneo-remoto.md](docs/escaneo-remoto.md).

Ejemplos de gestión:

```bash
netdiag user add ana --role operator
netdiag user role ana viewer
netdiag user passwd ana            # por stdin, sin dejar la clave en el historial
netdiag backup --out /var/backups/netdiag/$(date +%F).zip
```

Las opciones se aceptan antes o después de los argumentos posicionales. Precedencia de configuración:

```
flags de línea de comandos  >  variables NETDIAG_*  >  config.yaml  >  valores por defecto
```

Variables admitidas: `NETDIAG_CONFIG`, `NETDIAG_DATA_DIR`, `NETDIAG_DB`, `NETDIAG_LISTEN`, `NETDIAG_PUBLIC_URL`, `NETDIAG_LOG_LEVEL`, `NETDIAG_LOG_FORMAT`, `NETDIAG_PROFILE`, `NETDIAG_SCAN_TIMEOUT`, `NETDIAG_SCAN_CONCURRENCY`, `NETDIAG_AUTH`, `NETDIAG_SESSION_HOURS`, `NETDIAG_MONITOR`, `NETDIAG_MONITOR_INTERVAL`, `NETDIAG_SNMP`, entre otras.

### Recuperar el acceso

```bash
sudo -u netdiag netdiag user passwd admin
# o crear otro administrador
sudo -u netdiag netdiag user add operador --role admin
# promocionar una cuenta existente sin perder su contraseña
sudo -u netdiag netdiag user role operador admin
```

La contraseña se puede pasar por `--password`, por `NETDIAG_PASSWORD` o por stdin para no dejarla en el historial.

## Configuración

`netdiag config init` escribe una configuración completa lista para editar. Los bloques principales:

```yaml
server:
  listen: 127.0.0.1:8080
  public_url: https://netdiag.example.com
  tls_cert: /etc/netdiag/fullchain.pem
  tls_key: /etc/netdiag/privkey.pem
scan:
  profile: common
  timeout: 900ms
  concurrency: 128
  port_concurrency: 256
  max_hosts: 4096
  resolve_dns: true
  banner_grab: true
  snmp: true
  snmp_version: 2c
  snmp_communities: [public]
monitor:
  enabled: true
  interval: 5m
alerts:
  enabled: true
  max_per_minute: 30
  allow_private_targets: false
storage:
  retention_days: 30
  metrics_days: 30
  backup_enabled: true
  backup_dir: ""            # por defecto <data_dir>/backups
  backup_interval: 24h      # mínimo 15m
  backup_keep: 7
security:
  auth: true
  session_hours: 12
  login_rate: 5
  trust_proxy: false
  metrics_token: ""         # si se define, /metrics exige Bearer
diagnostics:
  enterprise_mode: true     # activa todos los detectores; false solo evalúa SLA
  min_confidence: 60        # confianza mínima para publicar un hallazgo
  min_availability: 99      # SLA de disponibilidad por host
  max_rtt_ms: 200           # SLA de latencia media por host
  max_anomalies: 200        # tope de hallazgos por informe
  storm_hosts: 3            # hosts simultáneos para sospechar tormenta
  authorized_aps: []        # MAC/BSSID de puntos de acceso autorizados
  arp_history_days: 30      # retención del histórico IP→MAC entre escaneos
  mac_ip_limit: 5           # IPs a partir de las cuales una MAC se marca sospechosa
  dhcp_probe: true          # sondeo UDP/68 solo en segmentos locales
  dhcp_probe_timeout: 700ms # ventana de respuesta; sin respuesta no rompe el escaneo
  dhcp_history_days: 30     # retención del histórico de respuestas DHCP
  authorized_dhcp_servers: []  # IPs o MACs de servidores DHCP propios
integrations:
  enabled: true             # exportación de eventos
  timeout: 10s              # por entrega (1s–1m)
  max_per_minute: 60        # límite global compartido
  queue_size: 512           # eventos en memoria
  allow_private_targets: false  # necesario para Elasticsearch/syslog/Kafka/Redis locales
vpn:
  enabled: true             # observación pasiva de túneles
  interval: 5m
  min_confidence: 60
  stale_after: 30m
wifi:
  enabled: false            # requiere -tags pcap e iface en modo monitor
  interface: ""             # ej. wlan0mon; NetDiag no cambia el modo
  interval: 5m
  scan_seconds: 10
  channels: []              # ej. [1, 6, 11]; salto de canal best effort con iw
  authorized_bssids: []     # inventario de APs propios
  known_ssids: []           # inventario de SSID permitidos
update:
  enabled: false            # comprobaciones firmadas; requiere manifest_url y public_key
  manifest_url: ""          # HTTPS salvo localhost
  public_key: ""            # clave pública minisign (base64 o archivo de dos líneas)
  check_interval_hours: 24
  download_timeout: 120s
  max_download_mb: 512
  restart_command: ""       # vacío usa syscall.Exec sobre el ejecutable actual
  restart_args: []
```

La mayoría de cambios se aplican en caliente desde **Ajustes**; los de escucha, TLS, rutas de datos y nivel de registro requieren reiniciar.

## Seguridad

- Contraseñas con **Argon2id** (64 MiB, 3 iteraciones); se aceptan hashes bcrypt heredados.
- Sesiones opacas en servidor, cookie `HttpOnly`/`SameSite=Lax`/`Secure` tras TLS, y token **CSRF** en cada escritura.
- Límite de intentos de login por IP y límite global de peticiones API.
- **CSP**, `X-Frame-Options: DENY`, `nosniff`, `Referrer-Policy: no-referrer` y HSTS opcional.
- Configuraciones de canales de alerta cifradas con **AES-256-GCM**; la clave `secret.key` se crea con permisos `0600`. Los secretos nunca se devuelven al navegador.
- Protección **SSRF**: los webhooks a rangos privados, loopback y metadatos cloud se bloquean salvo `alerts.allow_private_targets: true`.
- Roles de menor privilegio: `viewer` no puede escribir y `operator` no toca configuración ni usuarios; el backend rechaza la operación aunque el frontend la ofrezca.
- `/metrics` es público por defecto; define `security.metrics_token` para exigir `Authorization: Bearer <token>`. `/api/v1/openapi.json` permanece público para el descubrimiento de la API.
- Registro de auditoría de accesos, cambios de configuración, usuarios y copias, con retención configurable.

Para exponer NetDiag fuera de localhost, usa un proxy inverso con TLS (Caddy, nginx, Traefik) o configura `server.tls_cert`/`server.tls_key`. No publiques la instancia directamente en Internet sin autenticación y HTTPS.

## Arquitectura

```
cmd/netdiag            CLI y arranque
internal/app           ciclo de vida, escaneos, planificador, mantenimiento
internal/api           REST /api/v1, middleware, WebSocket
internal/auth          Argon2id, sesiones, CSRF, auditoría
internal/store         SQLite, migraciones, consultas
internal/discover      motor de descubrimiento en tres fases
internal/fingerprint   OUI, clasificación, banners, puertos
internal/snmp          SNMP v1/v2c/v3, IF-MIB, LLDP/CDP/FDB
internal/topology      grafo de red y confianza de enlaces
internal/alerts        reglas, deduplicación y canales
internal/report        PDF, CSV, JSON, HTML
internal/backup        ZIP de base, configuración y clave; retención
internal/webui         SPA embebida bilingüe (HTML/CSS/JS, sin dependencias)
```

La API está versionada en `/api/v1` y documentada en `/api/v1/openapi.json`; el WebSocket (`/api/v1/ws`) emite eventos `scan.progress`, `scan.finished`, `host.new`, `host.updated`, `alert.new`, `dashboard.updated`. Las métricas Prometheus se exponen en `/metrics`. Consulta [docs/arquitectura.md](docs/arquitectura.md) y [docs/api.md](docs/api.md).

## Despliegue

```bash
sudo ./scripts/install.sh
sudo systemctl enable --now netdiag
```

El instalador crea el usuario `netdiag`, instala el binario en `/usr/local/bin`, los datos en `/var/lib/netdiag`, la configuración en `/etc/netdiag/config.yaml`, aplica capabilities y registra la unidad systemd. Consulta [docs/despliegue.md](docs/despliegue.md) para TLS, proxy inverso, copias de seguridad y actualizaciones.

## Estado

Versión **1.5**: inventario, escaneos, topología, diagnósticos empresariales heurísticos, integraciones externas, alertas, informes, multi-sede con exportación/importación, RBAC de tres roles, i18n es/en, guía interactiva, OpenAPI 3.1, Prometheus, copias de seguridad, comparativa temporal, modo auditoría y auto-actualización firmada con rollback. La suite cubre configuración, persistencia, identidad de hosts por sede, cifrado de secretos, clasificación, utilidades de red, topología, diagnósticos, proveedores de integración, política de contraseñas, roles, copias, endpoints públicos, aislamiento entre sedes, diffs con evidencia, líneas base auditables, manifiestos manipulados, instalación atómica, confirmación y reversión. F13, F14, F15, F16, F17 y F26 quedan documentados como opcionales diferidos. La licencia AGPL, los avisos de terceros, el modelo comercial, el pipeline de publicación firmada y el paquete de confianza empresarial están documentados y enlazados desde esta página. Las mejoras F27–F40 están priorizadas en [docs/roadmap.md](docs/roadmap.md). Las contribuciones y los informes de errores son bienvenidos.
