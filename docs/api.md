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

La cookie es `HttpOnly`, `SameSite=Lax` y `Secure` cuando la petición llega por HTTPS o `server.public_url` lo indica. Los roles son `admin` (todo) y `viewer` (solo lectura).

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
| POST | `/auth/users` | Admin; rol `admin` o `viewer` |

### Panel e inventario

| Método | Ruta | Notas |
| --- | --- | --- |
| GET | `/dashboard` | KPIs, tipos, estado, último escaneo |
| GET | `/hosts` | `search, status, type, network_id, tag, favorite, sort, desc, page, limit` |
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
| GET | `/networks` | Redes con contadores |
| POST | `/networks` | `name`, `cidr`, `interface`, `enabled` |
| GET | `/networks/suggest` | Subredes privadas detectadas en el servidor |
| PUT | `/networks/{id}` | |
| DELETE | `/networks/{id}` | |

### Escaneos

| Método | Ruta | Notas |
| --- | --- | --- |
| GET | `/scans?limit=60` | Historial y escaneos activos |
| POST | `/scans` | Ver cuerpo abajo |
| GET | `/scans/{id}` | Estado y progreso |
| POST | `/scans/{id}/cancel` | Cancelación cooperativa |

```json
{
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
| GET | `/alerts` | `status, severity, host_id, search, from, to, page, limit` |
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
| GET | `/reports` | Últimos 200 |
| POST | `/reports` | `kind` (`inventory\|executive\|alerts`), `format` (`pdf\|csv\|json\|html`), `title` |
| GET | `/reports/{id}/download` | Descarga autenticada |
| DELETE | `/reports/{id}` | Elimina registro y archivo |

### Topología y sistema

| Método | Ruta | Notas |
| --- | --- | --- |
| GET | `/topology` | Nodos, aristas y estadísticas |
| GET | `/system/info` | Versión, capacidades, interfaces, tamaño de la base |
| GET | `/system/interfaces` | Interfaces locales |
| GET | `/system/config` | Admin; secretos enmascarados |
| PUT | `/system/config` | Admin; valida, guarda y aplica en caliente |
| GET | `/system/audit` | Admin; `actor, action, from, to, page, limit` |
| GET | `/system/health` | Comprobación interna |
| GET | `/ws` | WebSocket de eventos |

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
