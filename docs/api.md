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
| `admin` (o `owner`) | Todo: configuración, usuarios, copias de seguridad y operación |
| `operator` | Escaneos, inventario, redes, alertas, informes e importación de sedes; sin configuración, usuarios ni gestión de sedes |
| `viewer` | Solo lectura de inventario, alertas, informes, sedes y métricas |

### Alcance por sede

Todas las sedes comparten una única base SQLite. El parámetro `site_id` está disponible en las operaciones operativas:

| Operación | Dónde se envía `site_id` |
| --- | --- |
| `GET /dashboard`, `/hosts`, `/networks`, `/scans`, `/alerts`, `/topology`, `/diagnostics/*`, `/reports`, `/metrics` | Query string |
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

Reglas: `host_down`, `host_new`, `latency_high`, `port_opened`, `port_closed`, `service_changed`, `device_changed`, `subnet_usage`, `scan_failed`.

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

Tipos de anomalía: `stp_loop`, `vlan_leak`, `duplex_mismatch`, `mtu_problem`, `broadcast_storm`, `rogue_ap`, `sla_breach`. Con `diagnostics.enterprise_mode: false` solo se emiten `sla_breach`. El parámetro `diagnostics.min_confidence` filtra los detectores y `max_anomalies` limita el tamaño del informe.

**Señales y límites.** Los ciclos se deducen del grafo LLDP/CDP/FDB; las fugas de VLAN de puertos L2 con hosts en subredes distintas; el dúplex de velocidades IF-MIB incompatibles; el MTU de interfaces locales bajas con pérdida; las tormentas de pérdida y latencia simultáneas en una subred; los AP de `diagnostics.authorized_aps` o de baja confianza de fingerprint; y el SLA de muestras `up`/`rtt_ms`. No hay BPDUs, VLAN IDs, OID de dúplex ni contadores broadcast/fragmentación, por lo que ningún hallazgo constituye una confirmación protocolaria.

```bash
curl -b cookies.txt 'http://127.0.0.1:8080/api/v1/diagnostics/enterprise?site_id=1&from=24h'
curl -b cookies.txt 'http://127.0.0.1:8080/api/v1/diagnostics/vlan?site_id=1'
```

### Topología y sistema

| Método | Ruta | Notas |
| --- | --- | --- |
| GET | `/topology?site_id=<id>` | Nodos, aristas y estadísticas |
| GET | `/system/info` | Versión, capacidades, interfaces, tamaño de la base |
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

Eventos: `hello`, `scan.started`, `scan.progress`, `scan.finished`, `scan.canceled`, `host.new`, `host.updated`, `host.deleted`, `alert.new`, `alert.updated`, `dashboard.updated`.

El servidor hace ping periódico y cierra clientes que no responden. Si la cola de un cliente se llena, los eventos se descartan para ese cliente; la UI siempre puede recargar por REST.

## Límites

- Cuerpo máximo: `server.max_body_mb` (2 MB por defecto).
- API: `security.api_rate` por minuto; login: `security.login_rate` por minuto e IP.
- Escaneos simultáneos: 3; uno por CIDR.
- Exportaciones e informes: el inventario se limita a 10 000 hosts por archivo.
