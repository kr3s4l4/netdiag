# API HTTP

Base: `http://127.0.0.1:8080/api/v1`. Todas las respuestas son JSON salvo descargas y el WebSocket.

## Autenticación

La autenticación local está activa por defecto (`security.auth: true`). El flujo es:

1. `GET /auth/status` indica si hay que crear la cuenta (`setup_required`) o iniciar sesión.
2. `POST /auth/setup` crea el primer administrador y devuelve la cookie de sesión.
3. `POST /auth/login` inicia sesión.
4. La respuesta incluye `csrf`, que debe enviarse en la cabecera `X-CSRF-Token` de toda escritura.
5. `POST /auth/logout` invalida la sesión.

```bash
# Crear administrador
curl -c cookies.txt -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"NetDiag2026"}' \
  http://127.0.0.1:8080/api/v1/auth/setup

# Iniciar sesión
curl -c cookies.txt -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"NetDiag2026"}' \
  http://127.0.0.1:8080/api/v1/auth/login
# → {"user":{...},"csrf":"...","expires_at":"..."}

# Escritura autenticada
curl -b cookies.txt -H 'Content-Type: application/json' \
  -H 'X-CSRF-Token: <csrf>' \
  -d '{"name":"LAN","cidr":"192.168.1.0/24"}' \
  http://127.0.0.1:8080/api/v1/networks
```

La cookie es `HttpOnly`, `SameSite=Lax` y `Secure` cuando la petición llega por HTTPS o `server.public_url` lo indica.

Roles:

| Rol | Permisos |
| --- | --- |
| `admin` (o `owner`) | Todo: configuración, usuarios, copias de seguridad, actualizaciones y operación |
| `operator` | Escaneos, inventario, redes, alertas, informes e importación de sedes; sin configuración, usuarios, gestión de sedes ni actualizaciones |
| `viewer` | Solo lectura de inventario, alertas, informes, sedes, estado de actualizaciones y métricas |

### Alcance por sede

Todas las sedes comparten una única base SQLite. El parámetro `site_id` está disponible en las operaciones operativas:

| Operación | Dónde se envía `site_id` |
| --- | --- |
| `GET /dashboard`, `/hosts`, `/networks`, `/scans`, `/alerts`, `/integrations`, `/topology`, `/diagnostics/*`, `/reports`, `/metrics` | Query string |
| `POST /scans`, `/networks`, `/reports` | Cuerpo JSON |
| `GET /hosts/{id}`, `/scans/{id}`, `/reports/{id}/download` | Se deriva del registro, no se envía |

`site_id=0` (o ausente) en agregados y listados significa «todas las sedes». En escrituras, si se omite, se aplica la sede por defecto (`1`, `Principal`). El mismo CIDR puede repetirse en sedes distintas. La identidad de un host se namespacea como `s<site_id>:<mac|ip:...>`; los paquetes de exportación contienen la identidad sin el prefijo y la importación antepone la sede de destino.

### Errores

```json
{"error":"mensaje en español","code":"opcional"}
```

| Código | Significado |
| --- | --- |
| `400` | Cuerpo o parámetros inválidos |
| `401` | Sin sesión |
| `403` | Sin permisos o CSRF inválido |
| `404` | Recurso inexistente |
| `409` | Conflicto (duplicado, escaneo en curso) |
| `422` | Validación de negocio |
| `428` | Se requiere crear el administrador |
| `429` | Límite de intentos o de peticiones |
| `500` | Error interno |

## Endpoints

### Autenticación

| Método | Ruta | Notas |
| --- | --- | --- |
| GET | `/auth/status` | Estado público, usuario y CSRF si hay sesión |
| POST | `/auth/login` | Rate limited por IP |
| POST | `/auth/setup` | Solo si no hay usuarios |
| POST | `/auth/logout` | |
| POST | `/auth/password` | Cierra todas las sesiones |
| GET | `/auth/users` | Admin |
| POST | `/auth/users` | Admin; rol `admin`, `operator` o `viewer` |
| PATCH | `/auth/users/{id}` | Admin; `{"role":"operator"}`. No permite degradar al último admin |
| DELETE | `/auth/users/{id}` | Admin; no permite eliminar al último admin |
| POST | `/auth/users/{id}/password` | Admin; `{"password":"..."}`; cierra las sesiones del usuario |

### Sedes

Gestión y portabilidad de sedes. La creación, renombrado, borrado, exportación e importación son operaciones de administrador; la importación también la puede realizar `operator` por ser una escritura operativa. No se puede eliminar la última sede. Al borrar una sede, sus redes, hosts, escaneos, alertas e informes se trasladan a `move_to` y se resuelven las colisiones de CIDR.

| Método | Ruta | Notas |
| --- | --- | --- |
| GET | `/sites` | Lista `{items,total}` con contadores por sede; cualquier rol autenticado |
| POST | `/sites` | Admin; `{name, description?}` → `201` |
| GET | `/sites/{id}` | Detalle de una sede |
| PUT | `/sites/{id}` | Admin; `{name, description?}` |
| DELETE | `/sites/{id}?move_to=<id>` | Admin; mueve los datos a otra sede. `409` si es la última |

Exportación e importación (`schema: "netdiag.site/v1"`, paquete JSON autocontenido con redes, hosts y metadatos de identidad):

| Método | Ruta | Notas |
| --- | --- | --- |
| GET | `/sites/{id}/export` | Descarga el paquete JSON de la sede |
| POST | `/sites/{id}/import?mode=merge\|replace` | JSON crudo o multipart con campo `file`; devuelve `{summary:{networks_created,hosts_created,hosts_updated,hosts_skipped}}` |

```bash
# Exportar y reimportar en otra sede
curl -b cookies.txt http://127.0.0.1:8080/api/v1/sites/2/export -o sede.json
curl -b cookies.txt -H 'X-CSRF-Token: <csrf>' -F 'file=@sede.json' \
  'http://127.0.0.1:8080/api/v1/sites/3/import?mode=merge'
```

### Panel e inventario

| Método | Ruta | Notas |
| --- | --- | --- |
| GET | `/dashboard?site_id=<id>` | KPIs, tipos, estado, último escaneo |
| GET | `/hosts?site_id=<id>` | `search, status, type, network_id, tag, favorite, sort, desc, page, limit` |
| GET | `/hosts/export?format=csv\|json` | Descarga del inventario |
| GET | `/hosts/{id}` | Host, puertos, SNMP, vecinos y eventos recientes |
| PATCH | `/hosts/{id}` | `notes`, `tags`, `favorite` |
| DELETE | `/hosts/{id}` | Elimina del inventario |
| GET | `/hosts/{id}/events` | Ciclo de vida |
| GET | `/hosts/{id}/history?range=24h\|7d\|30d` | Series de latencia/disponibilidad |
| GET | `/hosts/{id}/neighbors` | Vecinos L2/L3 |
| POST | `/hosts/{id}/rescan` | Encola un escaneo del host |
| GET | `/ports/summary` | Puertos más frecuentes |

### Redes

| Método | Ruta | Notas |
| --- | --- | --- |
| GET | `/networks?site_id=<id>` | Redes con contadores |
| POST | `/networks` | `site_id`, `name`, `cidr`, `interface`, `enabled` |
| GET | `/networks/suggest` | Subredes privadas detectadas en el servidor |
| PUT | `/networks/{id}` | |
| DELETE | `/networks/{id}` | |

### Escaneos

| Método | Ruta | Notas |
| --- | --- | --- |
| GET | `/scans?site_id=<id>&limit=60` | Historial y escaneos activos |
| POST | `/scans` | Ver cuerpo abajo |
| GET | `/scans/{id}` | Estado y progreso |
| POST | `/scans/{id}/cancel` | Cancelación cooperativa |

```json
{
  "site_id": 1,
  "cidr": "192.168.1.0/24",
  "network_id": 0,
  "profile": "common",
  "ports": [22, 80, 443],
  "snmp": true,
  "deep": false,
  "timeout_ms": 900,
  "allow_public": false
}
```

`profile` puede ser `quick`, `common`, `full` o `custom`. En `custom` es obligatorio `ports`. Un CIDR público se rechaza salvo `allow_public`.

### Alertas

| Método | Ruta | Notas |
| --- | --- | --- |
| GET | `/alerts?site_id=<id>` | `status, severity, host_id, search, from, to, page, limit` |
| GET | `/alerts/stats` | Activas, reconocidas, resueltas, críticas |
| POST | `/alerts/{id}/ack` | Reconocer |
| POST | `/alerts/{id}/resolve` | Resolver |
| GET/POST | `/alerts/rules` | Listar/crear regla |
| PUT/DELETE | `/alerts/rules/{id}` | Las integradas no se eliminan |
| GET/POST | `/alerts/channels` | Listar/crear canal |
| PUT/DELETE | `/alerts/channels/{id}` | |
| POST | `/alerts/channels/{id}/test` | Envía una notificación de prueba |

Reglas: `host_down`, `host_new`, `latency_high`, `port_opened`, `port_closed`, `service_changed`, `device_changed`, `subnet_usage`, `scan_failed`, más las de seguridad `anomaly`, `arp_spoofing`, `dhcp_rogue`, `snmp_anomaly` y `passive_anomaly`.

Las reglas de seguridad disparan alertas para los hallazgos de diagnóstico. Todas admiten `min_confidence` (75 por defecto) y `category` (vacía = todas). Las reglas dedicadas fijan su categoría (`arp_spoofing`→`arp`, `dhcp_rogue`→`dhcp`, `snmp_anomaly`→`snmp`, `passive_anomaly`→`capture`) y tienen precedencia sobre la genérica `anomaly`, que actúa como comodín para las demás familias. El dedupe es por hallazgo, así que un mismo hallazgo nunca genera dos alertas. Cuando el dispositivo señalado no está en el inventario, la alerta guarda `site_id` directamente (v1.5.8) para seguir apareciendo en los filtros y estadísticas por sede. Los históricos IP→MAC y DHCP se consultan en `GET /diagnostics/arp` y `GET /diagnostics/dhcp` (ver "Diagnósticos empresariales").

Canales: `webhook`, `slack`, `discord`, `telegram`, `pagerduty`, `smtp`, `syslog`. Los campos secretos se devuelven como `********`; enviar ese valor conserva el secreto.

### Informes

| Método | Ruta | Notas |
| --- | --- | --- |
| GET | `/reports?site_id=<id>` | Últimos 200; `0` (o ausente) incluye todas las sedes |
| POST | `/reports` | `site_id`, `kind` (`inventory\|executive\|alerts`), `format` (`pdf\|csv\|json\|html`), `title` |
| GET | `/reports/{id}/download` | Descarga autenticada |
| DELETE | `/reports/{id}` | Elimina registro y archivo |

### Diagnósticos empresariales

Análisis heurístico sin captura de paquetes. Todas las rutas aceptan `site_id` (0 = todas las sedes) y `from`/`to` (RFC 3339 o ventana relativa como `24h`, `7d`, `30d`; por defecto 24 h). Las respuestas son de solo lectura y están disponibles para cualquier rol autenticado. Los hallazgos son pistas para investigar: incluyen `evidence`, `confidence` (1–100) y `recommendation`, y el informe enumera sus `limitations`.

| Método | Ruta | Notas |
| --- | --- | --- |
| GET | `/diagnostics/enterprise` | Informe completo: `anomalies`, `counts`, `stp`, `vlan`, `duplex`, `mtu`, `limitations` |
| GET | `/diagnostics/stp` | `{loops, anomalies, limitations}` con los ciclos L2 detectados |
| GET | `/diagnostics/vlan` | `{leaks, anomalies, limitations}` con las posibles fugas |
| GET | `/diagnostics/arp` | `{anomalies, observations, limitations}` con cambios IP→MAC y su evidencia |
| GET | `/diagnostics/dhcp` | `{anomalies, observations, limitations}` con respuestas DHCP y su evidencia |

Tipos de anomalía: `stp_loop`, `vlan_leak`, `duplex_mismatch`, `mtu_problem`, `broadcast_storm`, `rogue_ap`, `sla_breach`, `arp_gateway_mac_changed`, `arp_ip_mac_changed`, `arp_mac_multi_ip`, `arp_ip_multi_mac`, `dhcp_multiple_servers`, `dhcp_unauthorized_server`, `dhcp_gateway_spoof`, `dhcp_server_mac_changed`. Con `diagnostics.enterprise_mode: false` solo se emiten `sla_breach`. El parámetro `diagnostics.min_confidence` filtra los detectores y `max_anomalies` limita el tamaño del informe.

**Señales y límites.** Los ciclos se deducen del grafo LLDP/CDP/FDB; las fugas de VLAN de puertos L2 con hosts en subredes distintas; el dúplex de velocidades IF-MIB incompatibles; el MTU de interfaces locales bajas con pérdida; las tormentas de pérdida y latencia simultáneas en una subred; los AP de `diagnostics.authorized_aps` o de baja confianza de fingerprint; y el SLA de muestras `up`/`rtt_ms`. Las anomalías `arp_*` comparan los pares IP→MAC guardados en cada escaneo: un cambio de MAC en el gateway se marca crítico como posible MITM; una MAC con más de `diagnostics.mac_ip_limit` IPs (5 por defecto) y una IP con varias MAC se marcan como avisos. El histórico usa `diagnostics.arp_history_days` (30 por defecto). En cada escaneo de un segmento local se envían DHCPINFORM al gateway y DHCPINFORM/DHCPDISCOVER por broadcast desde UDP/68, sin raw sockets; las respuestas se guardan en `dhcp_observations` durante `diagnostics.dhcp_history_days` (30 días). Un servidor es legítimo si figura en `diagnostics.authorized_dhcp_servers` o si su IP y MAC coinciden con el gateway del histórico ARP. Varios servidores en un mismo escaneo, un NAK no autorizado, un gateway con MAC DHCP distinta o un servidor que cambia de MAC entre escaneos generan hallazgos `dhcp_*`. Si el puerto 68 está ocupado o no hay respuesta, el escaneo continúa y la limitación queda registrada. No hay BPDUs, VLAN IDs, OID de dúplex, contadores broadcast/fragmentación ni captura ARP o DHCP de otros clientes, por lo que ningún hallazgo constituye una confirmación protocolaria.

```bash
curl -b cookies.txt 'http://127.0.0.1:8080/api/v1/diagnostics/enterprise?site_id=1&from=24h'
curl -b cookies.txt 'http://127.0.0.1:8080/api/v1/diagnostics/vlan?site_id=1'
```

### Integraciones externas

Exportación de eventos (`alert`, `scan.finished`, `integration.test`) a sistemas externos. Las escrituras requieren rol `admin`; la lectura está disponible para cualquier rol autenticado. Los campos sensibles se cifran con AES-256-GCM en la base de datos y la API devuelve `********` en su lugar; enviar ese valor o una cadena vacía al actualizar conserva el secreto almacenado.

| Método | Ruta | Notas |
| --- | --- | --- |
| GET | `/integrations` | `{items, types, secret_fields}`; secretos enmascarados |
| POST | `/integrations` | Crear; cuerpo `{name, type, enabled?, config}` |
| PUT | `/integrations/{id}` | Editar; los secretos enmascarados se conservan |
| DELETE | `/integrations/{id}` | Eliminar |
| POST | `/integrations/{id}/test` | Envía un evento sintético; responde `{ok:true}` o `{ok:false,error}` |

Tipos: `elasticsearch`, `syslog`, `webhook`, `slack`, `discord`, `msteams`, `telegram`, `pagerduty`, `kafka`, `redis`. La configuración del motor (`integrations.enabled`, `timeout`, `max_per_minute`, `queue_size`, `allow_private_targets`) se edita en `PUT /system/config`. El guardia SSRF bloquea destinos privados y de metadatos salvo `allow_private_targets: true`; es obligatorio para apuntar a un Elasticsearch, syslog, Kafka REST Proxy o Redis local.

```bash
curl -b cookies.txt -H "X-CSRF-Token: $CSRF" -H 'Content-Type: application/json' \
  -d '{"name":"Elastic local","type":"elasticsearch","config":{"url":"http://127.0.0.1:9200","index":"netdiag-events"}}' \
  http://127.0.0.1:8080/api/v1/integrations
curl -b cookies.txt -H "X-CSRF-Token: $CSRF" -X POST \
  http://127.0.0.1:8080/api/v1/integrations/1/test
```

### Dashboards personalizados (F18)

Vistas por sede con widgets en una rejilla fija de 12 columnas (máximo 16). Las escrituras requieren `canWrite`; la lectura está disponible para cualquier rol autenticado.

| Método | Ruta | Notas |
| --- | --- | --- |
| GET | `/dashboards` | `{items, total}`; filtrable por `site_id` |
| GET | `/dashboards/catalog` | `{items:[{type,label,source}], max_widgets:16}` |
| POST | `/dashboards` | `{name, site_id?, widgets?}`; responde `201` con el dashboard creado |
| POST | `/dashboards/example` | Crea "Mi red" con cuatro widgets; `site_id?` en el cuerpo |
| GET | `/dashboards/{id}` | Detalle con `widgets` |
| PUT | `/dashboards/{id}` | Reemplaza `name`, `site_id` y `widgets` |
| DELETE | `/dashboards/{id}` | `204` |

Cada widget tiene `{id, type, position:{x,y}, size:{w,h}, config}`. Los tipos válidos son `dashboard`, `hosts`, `ports`, `topology`, `alerts`, `scans`, `wifi` y `vpn`. El campo `config.source` se deriva siempre del catálogo en el servidor: los valores enviados por el cliente se ignoran, de modo que no se pueden consultar URL arbitrarias. `position.x` va de 0 a 11, `position.y` de 0 a 23, `size.w` ∈ {2,3,4,6,12} y `size.h` ∈ {1,2,3}; los valores fuera de rango se normalizan o rechazan.

### Filtros y vistas guardadas (F40)

Filtros personales para el inventario (`hosts`) y la lista de alertas (`alerts`). Pertenecen a la cuenta autenticada: **cualquier rol autenticado**, incluido `viewer`, puede crear, editar y borrar sus propias vistas, pero nunca las de otro usuario. Las escrituras requieren la cabecera `X-CSRF-Token`.

La primera llamada a `GET /views` siembra una sola vez cuatro vistas integradas por usuario (`Críticas`, `Nuevos hosts`, `Caídos`, `Cambios de la semana`). Borrar una integrada no la recrea. `site_id=0` significa «todas las sedes»; un `site_id` positivo restringe la vista a esa sede. Máximo 100 vistas por usuario, con nombres únicos sin distinguir usuario.

| Método | Ruta | Notas |
| --- | --- | --- |
| GET | `/views` | `{items, total, max_views}`; admite `kind` y `site_id` en la query |
| GET | `/views/catalog` | Catálogo de `kinds`, `columns`, `sorts`, `statuses`, `severities`, `windows`, `defaults`, `max_views` y `max_columns` |
| POST | `/views` | Crea una vista; responde `201` con el objeto creado |
| GET | `/views/{id}` | Detalle de una vista propia; `404` si pertenece a otro usuario |
| PUT | `/views/{id}` | Reemplaza los filtros de una vista propia |
| DELETE | `/views/{id}` | `204`; `404` si no es propia |
| GET | `/views/{id}/share` | `{id, name, path, url, params}` con un hash `#/<kind>?...` listo para compartir |
| GET | `/views/resolve` | Valida los parámetros de un enlace compartido y devuelve la vista resultante sin guardarla |

Cuerpo de `POST`/`PUT` (todos los campos opcionales salvo `name` y `kind`):

```json
{
  "name": "Críticas de LAN",
  "kind": "alerts",
  "site_id": 1,
  "query": "srv-web",
  "severity": "critical",
  "status": "active",
  "window": "7d",
  "columns": ["severity", "title", "created_at"],
  "sort": "created_at",
  "desc": true
}
```

Los valores se normalizan en servidor: el nombre (≤80 caracteres) y la consulta (≤200) se sanea escapando HTML y eliminando caracteres de control; `kind`, `severity`, `status`, `window`, `columns` (≤16) y `sort` se validan contra el catálogo. `columns`/`sort` nunca se interpolan en SQL. La interfaz aplica `query`, `status`, `severity`, `sort`, `desc` y la visibilidad de `columns` al abrir una vista o un enlace compartido. Los errores usan `400` con `code: invalid_view`, `invalid_body`, `invalid_id` o `unauthorized`; `404` con `not_found`; y `409` con `duplicate_view` o `view_limit`.

El enlace de `/views/{id}/share` serializa **todos los filtros**, no el identificador interno. Así, `#/alerts?severity=critical&site_id=1&...` funciona para otro operador sin exponer la fila privada de su autor. El SPA vuelve a validar y acota esos parámetros al aplicarlos desde la URL.

```bash
# Crear y compartir
curl -b cookies.txt -H "X-CSRF-Token: $CSRF" -H 'Content-Type: application/json' \
  -d '{"name":"Caídas","kind":"hosts","status":"down","sort":"last_seen","desc":true}' \
  http://127.0.0.1:8080/api/v1/views

curl -b cookies.txt http://127.0.0.1:8080/api/v1/views/7/share
# → {"path":"#/hosts?status=down&sort=last_seen&desc=1&name=Ca%C3%ADdas", ...}
```

### WiFi (F11)

Captura pasiva 802.11 y análisis de APs/clientes. Las lecturas están disponibles para cualquier rol autenticado; `POST /wifi/scan` requiere `canWrite` y `DELETE /wifi/aps` requiere `admin`/`owner`.

| Método | Ruta | Notas |
| --- | --- | --- |
| GET | `/wifi/status` | `{capture_available, enabled, interface, interval, scan_seconds, channels, authorized_bssids, known_ssids, note}` |
| GET | `/wifi/aps` | `{items, total}`; cada AP incluye `bssid`, `ssid`, `channel`, `band`, `encryption`, `signal_dbm`, `beacons`, `clients`, `rogue`, `rogue_info`, `rogue_confidence`, `first_seen`, `last_seen` |
| GET | `/wifi/clients` | `{items, total}`; `mac`, `ap_bssid`, `signal_dbm`, `last_seen` |
| GET | `/wifi/channels` | `{items, total}`; agregado por canal con `aps`, `clients`, `overlap`, `load` (`baja`/`media`/`alta`) y `suggest` |
| POST | `/wifi/scan` | Captura una ventana; responde `{ok:true, result:{aps, clients, frames, duration_ms}}`; `503` si no hay binario `-tags pcap` o interfaz configurada |
| DELETE | `/wifi/aps` | Borra APs y clientes de `site_id`; `200` con `{ok:true}` |

La interfaz debe estar previamente en modo monitor: NetDiag no cambia el modo del dispositivo. El salto de canales definido en `wifi.channels` usa `iw` de forma best effort. La detección de AP no autorizado compara `bssid` con `wifi.authorized_bssids` y `ssid` con `wifi.known_ssids`; cada veredicto incluye una explicación y confianza (65 si solo coincide el SSID, 90 si el BSSID no está autorizado). Sin inventario no se emite veredicto. Los parámetros se editan en `PUT /system/config` (bloque `wifi`).

### VPN (F12)

Observación pasiva de túneles. La lectura está disponible para cualquier rol autenticado; forzar una observación requiere `canWrite` (`admin`, `owner`, `operator`) y borrar una observación requiere `admin`/`owner`.

| Método | Ruta | Notas |
| --- | --- | --- |
| GET | `/vpn/tunnels` | `{items, total, config}`; cada túnel incluye `type`, endpoints, contadores, `evidence`, `confidence` y `state` (`active`/`observed`/`stale`) |
| POST | `/vpn/scan` | Fuerza una observación; responde `{ok:true, stored:N}` |
| DELETE | `/vpn/tunnels/{id}` | Elimina una observación; `204` |

La detección lee tablas del kernel (`/proc/net/{udp,udp6,tcp,tcp6}`, `/proc/net/dev`, sysfs `operstate`) y puertos conocidos de la fase de descubrimiento. Es heurística: cada hallazgo incluye evidencia y confianza. La configuración (`vpn.enabled`, `interval`, `min_confidence`, `stale_after`) se edita en `PUT /system/config`.

### Comparativa temporal (F21)

| Método | Ruta | Notas |
| --- | --- | --- |
| POST | `/history/compare` | Diff entre dos momentos. Disponible para cualquier rol autenticado; la consulta queda registrada en auditoría |

Cuerpo (todo opcional; `{}` aplica los valores por defecto):

```json
{
  "site_id": 1,
  "days": 7,
  "window": "24h",
  "period_a": { "label": "hoy", "from": "2026-11-17T12:00:00Z", "to": "2026-11-18T12:00:00Z" },
  "period_b": { "label": "hace 7 días", "from": "2026-11-10T12:00:00Z", "to": "2026-11-11T12:00:00Z" }
}
```

`window` admite duraciones Go y los sufijos `d`/`w` (`24h`, `7d`, `2w`), entre `1h` y `30d`. `days` va de 1 a 365. En `period_a`/`period_b`, `from` y `to` aceptan RFC3339 o una duración relativa al momento de la petición (`24h`, `7d`). Si se omite un lado, se usa la ventana equivalente por defecto. El parámetro `site_id` también puede ir en la query string.

Respuesta (`DiffReport`): `period_a` y `period_b` con `{label, from, to, metrics, hosts, rtt, uptime, topology}`; `changes` con `{type, entity, id, label, from, to, evidence}`; `summary` por tipo y `total`; `limitations` y `truncated`. `metrics` contiene `samples`, `availability`, `avg_rtt`, `min_rtt`, `max_rtt`, `hosts_up` y `hosts_down`. Las series `rtt`/`uptime` están agrupadas en hasta 100 buckets por ventana.

Los tipos de cambio son `host.added`, `host.removed`, `host.status_changed`, `host.availability_changed`, `host.rtt_changed`, `topology.node_added`, `topology.node_removed`, `topology.edge_added`, `topology.edge_removed`, `fleet.availability_changed` y `fleet.rtt_changed`. Umbrales: 1,0 pp de disponibilidad y `max(2 ms, 10 % del RTT anterior)` de latencia. Máximo 500 cambios por respuesta; si se supera, `truncated: true`.

Los snapshots de topología se capturan tras cada escaneo correcto y cada hora. Si faltan en una ventana, el campo `topology.snapshot_at` es nulo y se añade una entrada a `limitations`; la ausencia de datos nunca se reporta como «sin cambios».

### Modo auditoría (F22)

| Método | Ruta | Notas |
| --- | --- | --- |
| POST | `/audits/snapshot` | Guarda una línea base de una sede. Requiere `admin`, `owner` u `operator` |
| GET | `/audits?site_id=<id>&limit=<n>` | Lista metadatos; `limit` entre 1 y 200, por defecto 50 |
| GET | `/audits/{id}?data=1` | Devuelve el snapshot. Sin `data=1`, omite el payload para respuestas ligeras |
| DELETE | `/audits/{id}` | Elimina una línea base. Requiere `admin`, `owner` u `operator` |
| POST | `/audits/{id}/diff/{id2}` | Compara `{id}` (antes) con `{id2}` (después) |
| GET | `/audits/{id}/export?format=json\|gzip` | Descarga portable; por defecto JSON |

Cuerpo de creación (todo opcional):

```json
{
  "site_id": 1,
  "name": "Antes del cambio",
  "notes": "Auditoría trimestral"
}
```

Si se omite `site_id`, se usa el parámetro de query string o la sede `1` (`Principal`). `name` admite hasta 120 caracteres y `notes` hasta 1000; si se omite `name`, se genera `Auditoría YYYY-MM-DD HH:MM`. La respuesta de creación es un `AuditSnapshot` ligero con `{id, site_id, name, notes, ts, size, raw_size}`; el campo `data` se omite y se recupera con `GET /audits/{id}?data=1` o mediante exportación. `size` son los bytes gzip almacenados y `raw_size` los bytes JSON antes de comprimir.

El payload (`AuditSnapshotData`) incluye `schema`, `site_id`, `site_name`, `captured_at`, `hosts`, `networks`, `neighbors`, `rules`, `channels`, `integrations`, `topology`, `scans`, `alerts` y `config`. Los hosts incluyen identidad, IP, MAC, nombre, fabricante, modelo, SO, tipo, estado, confianza, puertos, etiquetas y notas. Los campos volátiles (`last_seen` y muestras de latencia) se conservan en el snapshot pero **no** se comparan en el diff.

**Redacción de secretos:** de reglas, canales e integraciones solo se guardan sus identificadores, metadatos y `config_keys` (nombres ordenados de las claves). Los valores de `password`, `token`, `api_key`, `url`, `secret` y cualquier otra clave sensible nunca se escriben en la base ni aparecen en exportaciones. Las operaciones de creación, borrado y diff generan entradas `audits.snapshot.*` en la auditoría de seguridad.

La respuesta del diff (`AuditDiff`) contiene `from_id`, `to_id`, `from_name`, `to_name`, `from_ts`, `to_ts`, `changes`, `summary`, `truncated` y `limitations`. Cada cambio es `{type, entity, id, label, field, from, to, evidence}` con al menos una cadena de evidencia. Los tipos incluyen `host.*`, `network.*`, `neighbor.*`, `rule.*`, `channel.*`, `integration.*`, `scan.*`, `topology.node_*`, `topology.edge_*`, `alerts.changed` y `config.changed`. Los resultados se ordenan por tipo, entidad, identificador y campo; el máximo es 500 cambios por respuesta y `truncated: true` indica que se alcanzó el límite. Si alguno de los snapshots no tiene datos, `changes` queda vacío y se añade una entrada a `limitations`: nunca se reporta un diff incompleto como «sin cambios».

Los snapshots se comprimen con gzip antes de persistirse en SQLite. Los límites son 64 MiB de JSON antes de comprimir y 16 MiB comprimidos por snapshot; se conservan como máximo 100 capturas por sede, descartando las más antiguas. La exportación `format=gzip` devuelve `application/gzip` con el mismo documento JSON comprimido y un nombre de archivo `netdiag-audit-<id>-<fecha>.json.gz`.

### Actualizaciones (F23)

| Método | Ruta | Notas |
| --- | --- | --- |
| GET | `/update/status` | Estado local sin contactar con el servidor de manifiestos; cualquier rol autenticado |
| GET | `/update/check` | Descarga y verifica el manifiesto firmado; cualquier rol autenticado |
| POST | `/update/apply` | Descarga, verifica e instala; requiere `admin`/`owner` |
| POST | `/update/rollback` | Restaura el binario anterior; requiere `admin`/`owner` |

`GET /update/status` responde `{current_version, enabled, check_interval_hours, pending}`; `pending` es `null` o un objeto `{schema, version, previous_version, checksum, backup_path, applied_at}`.

`GET /update/check` responde `{current_version, latest_version, available, release, checked_at, message, pending}`. `release` contiene el manifiesto verificado (`schema`, `version`, `url`, `checksum`, `released_at`, `notes`, `signature`) tal como se recibió y validó. Si las actualizaciones están desactivadas, `available: false` y `message` lo explica. Errores de red o firma se devuelven como `502`.

Cuerpo de `/update/apply`:

```json
{
  "version": "1.6.0",
  "restart": false
}
```

`version` es opcional; si se envía, debe coincidir exactamente con la versión del manifiesto firmado. `restart` vale `true` por defecto: el reinicio se programa 500 ms después de la respuesta para no cortar la petición; con `false` el cliente debe reiniciar manualmente (`netdiag update commit` tras comprobar el arranque). La respuesta es `{from_version, to_version, installed_at, backup_path, message}`. Cada aplicación y reversión genera una entrada `update.applied` / `update.rolled_back` en la auditoría de seguridad.

El manifiesto firmado es JSON con estos campos exactos; cualquier propiedad adicional se rechaza:

```json
{
  "schema": "netdiag/update/v1",
  "version": "1.6.0",
  "url": "https://updates.example.com/netdiag-1.6.0",
  "checksum": "64 hexadecimales del SHA256 del binario",
  "released_at": "2026-11-18T00:00:00Z",
  "notes": "Notas de la versión",
  "signature": "untrusted comment: ...\n<base64>\ntrusted comment: version:1.6.0\tsha256:<checksum>\n<base64>"
}
```

La firma es minisign prehashed (`ED`): Ed25519 sobre BLAKE2b-512, con el comentario de confianza firmado por separado. El comentario debe incluir `version:` y `sha256:` coherentes con el resto del manifiesto. La configuración `update.public_key` acepta la clave en base64 o el archivo completo de dos líneas. URL HTTPS obligatorias salvo `localhost`/loopback; sin credenciales de usuario. Límites: 1 MiB de manifiesto y `update.max_download_mb` (512 por defecto) de binario; en Linux el binario debe empezar por `ELF`.

La instalación guarda `<ejecutable>.netdiag-old` y `<data_dir>/update-pending.json`. En el arranque, si el binario coincide con el checksum del marcador se borran marcador y copia; si no coincide se restaura la copia automáticamente. `POST /update/rollback` fuerza la restauración manual. El reinicio usa `update.restart_command`/`update.restart_args` cuando están definidos y, en caso contrario, `syscall.Exec` sobre el propio ejecutable.

Ejemplo de aplicación controlada:

```bash
curl -sb cookies.txt -H "X-CSRF-Token: $CSRF" -H 'Content-Type: application/json' \
  -d '{"version":"1.6.0","restart":false}' \
  http://127.0.0.1:8080/api/v1/update/apply | python3 -m json.tool
```

### Topología y sistema

| Método | Ruta | Notas |
| --- | --- | --- |
| GET | `/topology?site_id=<id>` | Nodos, aristas y estadísticas; incluye redes vecinas detectadas |
| POST | `/trace` | Traza IPv4 con `{target, hops, probes, timeout_ms, resolve}` |
| POST | `/discover/neighbors` | Fuerza la detección de subredes privadas vecinas |
| GET | `/discover/neighbors` | Último resultado cacheado (TTL de 15 minutos) |
| GET | `/system/info` | Versión, capacidades, interfaces, tamaño de la base |
| GET | `/system/license` | Edición, límites, caducidad y avisos; cualquier rol autenticado |
| GET | `/system/interfaces` | Interfaces locales |
| GET | `/system/config` | Admin; secretos enmascarados |
| PUT | `/system/config` | Admin; valida, guarda y aplica en caliente |
| GET | `/system/audit` | Admin; `actor, action, from, to, page, limit` |
| GET | `/system/health` | Comprobación interna |
| GET | `/system/backups` | Admin; lista `{items:[{name,size,created_at}],dir}` |
| POST | `/system/backups` | Admin; crea un ZIP y aplica la retención |
| GET | `/system/backups/{name}/download` | Admin; descarga el ZIP |
| DELETE | `/system/backups/{name}` | Admin; elimina una copia |
| GET | `/ws` | WebSocket de eventos |

La detección de vecinos se ejecuta automáticamente la primera vez que se consulta la topología. Solo prueba direcciones RFC1918 adyacentes a las redes locales y semillas del inventario; nunca sondea direcciones públicas. Cada red detectada aparece en el grafo como un nodo `neighbor:<cidr>` y su router como `neighbor-router:<ip>`, con una arista `routed` etiquetada como «detectado por traza».

Ejemplo de traza:

```bash
curl -s -X POST http://127.0.0.1:8080/api/v1/trace \
  -H 'Content-Type: application/json' \
  -d '{"target":"192.168.1.1","hops":5,"probes":2}' | python3 -m json.tool
```

También está disponible en terminal: `netdiag trace 192.168.1.1 --hops 5 --probes 2 --json`.

### Endpoints públicos

| Método | Ruta | Notas |
| --- | --- | --- |
| GET | `/api/v1/openapi.json` | Especificación OpenAPI 3.1 generada por el servidor |
| GET | `/metrics` | Métricas Prometheus en formato texto |

Ambos son públicos por defecto. Si se define `security.metrics_token`, `/metrics` exige `Authorization: Bearer <token>` (responde `401` sin él). OpenAPI permanece público para que clientes y herramientas puedan descubrir la API.

```bash
curl -s http://127.0.0.1:8080/metrics | head
curl -s http://127.0.0.1:8080/api/v1/openapi.json | jq '.info'
curl -s -H 'Authorization: Bearer <token>' http://127.0.0.1:8080/metrics
```

Series principales: `netdiag_info`, `netdiag_up`, `netdiag_uptime_seconds`, `netdiag_db_size_bytes`, `netdiag_hosts{status}`, `netdiag_hosts_by_type{type}`, `netdiag_hosts_total`, `netdiag_hosts_up`, `netdiag_hosts_down`, `netdiag_hosts_new_24h`, `netdiag_networks`, `netdiag_scans_24h`, `netdiag_alerts_active`, `netdiag_alerts_critical`, `netdiag_avg_rtt_ms` y `netdiag_availability_ratio`. No se consulta la base de datos para cada scrape más allá de las estadísticas agregadas del panel.

## WebSocket

```js
const ws = new WebSocket('ws://127.0.0.1:8080/api/v1/ws');
ws.onmessage = (e) => {
  const ev = JSON.parse(e.data); // {"type":"scan.progress","data":{...}}
};
```

Eventos: `hello`, `scan.started`, `scan.progress`, `scan.finished`, `scan.canceled`, `host.new`, `host.updated`, `host.deleted`, `alert.new`, `alert.updated`, `dashboard.updated`, `sites.updated`, `report.created`, `update.checked`, `update.available`, `update.applied`, `update.installed`, `update.restarting`, `update.restart_failed`, `update.rolled_back`.

El servidor hace ping periódico y cierra clientes que no responden. Si la cola de un cliente se llena, los eventos se descartan para ese cliente; la UI siempre puede recargar por REST.

## Límites

- Cuerpo máximo: `server.max_body_mb` (2 MB por defecto).
- API: `security.api_rate` por minuto; login: `security.login_rate` por minuto e IP.
- Escaneos simultáneos: 3; uno por CIDR.
- Exportaciones e informes: el inventario se limita a 10 000 hosts por archivo.
