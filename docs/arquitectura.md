# Arquitectura

NetDiag se compila en un único binario que contiene la API HTTP, la SPA, el motor de descubrimiento y SQLite. No necesita Node, Java, un servidor de aplicaciones ni una base de datos externa.

## Capas

```
                  ┌─────────────────────────────┐
  navegador ────► │ internal/api (REST + WS)     │
                  │  middleware: auth, CSRF,     │
                  │  límites, cabeceras, logs    │
                  └──────────────┬──────────────┘
                                 │
                  ┌──────────────▼──────────────┐
                  │ internal/app                │
                  │  ciclo de vida, escaneos,   │
                  │  planificador, retención    │
                  └───┬────────┬────────┬───────┘
                      │        │        │
          ┌───────────▼──┐ ┌───▼────┐ ┌─▼────────────┐
          │ discover     │ │ alerts │ │ report       │
          │ snmp         │ │        │ │              │
          │ fingerprint  │ │        │ │              │
          │ topology     │ │        │ │              │
          │ diagnostics  │ │        │ │              │
          └───────┬──────┘ └───┬────┘ └──────┬───────┘
                        ┌──────▼────────┐
                        │ integrations  │
                        └──────┬────────┘
                  └────────────┴─────────────┘
                               │
                     ┌─────────▼─────────┐
                     │ internal/store    │
                     │ SQLite (modernc)  │
                     └───────────────────┘
```

### `internal/api`

Router `net/http` con patrones de método (`GET /api/v1/hosts/{id}`). Cada escritura exige sesión y token CSRF. Los permisos tienen tres niveles: `admin`/`owner` (todo, incluidas sedes), `operator` (operación e importación de sedes; sin configuración, usuarios ni gestión de sedes) y `viewer` (solo lectura). Las rutas de sedes (`/api/v1/sites`) son administrativas salvo la importación. La capa inyecta `site_id` en los filtros de panel, inventario, redes, escaneos, alertas, topología, diagnósticos, informes y métricas. El hub WebSocket mantiene una cola por cliente y descarta mensajes si el cliente se retrasa, para no bloquear los escaneos. La especificación OpenAPI 3.1 se genera en el propio paquete y las métricas Prometheus se sirven desde `/metrics`.

### `internal/app`

- `New` abre la base, crea la clave de cifrado si falta y conecta todas las capas.
- `StartScan` valida el CIDR, comprueba solapamientos (máximo tres escaneos y uno por CIDR), crea el registro y lanza el trabajo en segundo plano.
- El planificador recorre las redes habilitadas según `monitor.interval`; cada `monitor.deep_interval` ejecuta un perfil más profundo.
- El mantenimiento purga métricas, alertas, auditoría y escaneos según la retención.
- El gestor de copias crea un ZIP inicial si no existe ninguno y luego uno por intervalo, aplicando la retención configurada.
- Al apagar, cancela escaneos, espera a los workers y cierra el servidor con un timeout.

### `internal/discover`

El motor ejecuta tres fases y emite progreso throttled:

1. **Descubrimiento**: ICMP, TCP a puertos de alta señal y ARP activo (si se compiló con `-tags pcap`) o pasivo mediante la tabla de vecinos del kernel y `NudgeARP`.
2. **Puertos**: sondeo concurrente del perfil elegido, con detección de estado y servicio.
3. **Enriquecimiento**: DNS inverso, banners, TLS, SNMP (sistema, interfaces, LLDP/CDP/FDB) y clasificación final.

La cancelación es cooperativa: cada worker consulta el contexto y el resultado parcial se conserva. El motor nunca devuelve `nil`; los errores viajan en `Result.Err`.

### `internal/store`

SQLite en modo WAL con `busy_timeout`, claves foráneas y conexiones acotadas. Las migraciones son una lista ordenada de sentencias idempotentes. Los tipos se serializan con JSON en columnas `TEXT` (puertos, etiquetas, evidencias, configuración SNMP).

**Identidad de host.** `identity` es único y está namespaceado por sede: `s<site_id>:<MAC|ip:IP>`. Si hay MAC válida se usa la MAC normalizada; si no, `ip:<IP>`. Cuando más adelante se descubre la MAC, la fila se rekeyea en la misma transacción. Si la MAC ya pertenece a otra fila de la misma sede, se fusiona. Las notas, etiquetas y favoritos nunca se sobrescriben con datos de escaneo. La migración 4 reescribe las identidades heredadas anteponiendo `s<site_id>:`.

**Sedes.** Las migraciones 2–4 añaden la tabla `sites` y las columnas `site_id` en `networks`, `hosts`, `scans` y `reports`; la sede `Principal` (id `1`) se crea al inicializar. La unicidad de CIDR es por `(site_id, cidr)`, y todos los listados y agregados aceptan un filtro de sede (`0` = todas). La exportación genera un `SiteBundle` (`netdiag.site/v1`) con identidades sin prefijo de sede y la importación las re-prefija para la sede destino, con modos `merge` y `replace`. Borrar una sede mueve sus redes, hosts, escaneos, alertas e informes a otra sede, fusionando redes con el mismo CIDR; la última sede no se puede borrar.

**Canales de alerta.** La configuración se cifra con AES-256-GCM antes de guardar. La clave se carga o crea en `<datos>/secret.key` con permisos `0600`. La API devuelve los campos secretos enmascarados.

### `internal/backup`

Instantáneas autocontenidas en ZIP: `manifest.json` (versión, fecha, tamaño y lista de archivos), una copia consistente de SQLite obtenida con `VACUUM INTO`, `config.yaml` y `secret.key`. El archivo se escribe con nombre temporal y se renombra al final, la retención se aplica por fecha de modificación y los nombres se validan contra traversal (`../`, separadores y extensiones ajenas). Está disponible por CLI (`netdiag backup`), por REST y desde la UI.

### `internal/fingerprint`

- Base OUI embebida (gzip) y posibilidad de superponer una actualización local.
- Clasificación por evidencia ponderada: OUI, nombre, puertos, banners, SNMP, tipo de puerta de enlace y TTL. La evidencia y la confianza se guardan para que el usuario pueda auditarlas.
- Detección de plataformas virtuales (Docker, KVM, VMware, Hyper-V, VirtualBox, Xen…).

### `internal/snmp`

Cliente propio sobre `gosnmp` con soporte v1, v2c y v3 (MD5/SHA-1/SHA-2 y DES/AES). Recoge `sysDescr`, `sysObjectID`, `sysName`, uptime, tabla `ifTable`, y vecinos LLDP/CDP y FDB de bridge.

### `internal/topology`

Construye un grafo determinista a partir de hosts, redes y vecinos. Crea un nodo ancla por red (preferentemente la puerta de enlace), enlaza hosts por subred y añade aristas LLDP/CDP/bridge/ARP con confianza. Marca nodos aislados y resume por tipo.

### `internal/diagnostics`

Analizador puro (sin E/S) que recibe un snapshot por sede y ventana: hosts, vecinos resueltos, redes, muestras de métricas, interfaces locales, el histórico IP→MAC (`arp_observations`) y la configuración `diagnostics`. Produce un `DiagnosticsReport` con anomalías comunes (`DiagnosticAnomaly`: id, tipo, categoría, severidad, evidencia, confianza y recomendación) y vistas tipadas para STP, VLAN, dúplex y MTU. Los detectores exigen varias señales o confianza alta: el grafo L2 solo usa LLDP/CDP/FDB, las fugas de VLAN requieren un puerto compartido o una adyacencia bridge entre subredes, las tormentas exigen hosts simultáneos en la misma red y `DetectARP` compara al menos dos lotes de escaneo para señalar cambios de MAC. SLA se evalúa siempre; el resto solo con `enterprise_mode: true`. Las carencias de datos (sin BPDUs, VLAN IDs, OID de dúplex, contadores broadcast/fragmentación ni captura ARP en vivo) se documentan en `report.limitations`.

### `internal/integrations`

Exportadores de eventos independientes y opcionales. Un `Manager` con dos workers consume una cola acotada (`integrations.queue_size`), aplica un límite deslizante global (`integrations.max_per_minute`) y entrega cada evento a las integraciones habilitadas. Los proveedores usan HTTP directo (Elasticsearch `_bulk` en NDJSON, webhooks firmados, Slack, Discord, MS Teams, Telegram, PagerDuty y Kafka REST Proxy), RFC 5424 sobre UDP/TCP (syslog) y RESP mínimo `AUTH`+`XADD` (Redis Streams), sin dependencias de SDK. Todas las conexiones inspeccionan la IP resuelta en `DialContext` para bloquear SSRF, incluidos DNS rebinding y metadatos cloud; los destinos privados requieren `integrations.allow_private_targets: true`. Las definiciones viven en la tabla `integrations` con `config_enc` cifrado por `cryptoutil.Box` y la API expone solo secretos enmascarados. `Test` envía un evento sintético de forma síncrona y registra `last_test_at`/`last_error`. El manager se crea en `app.New`, se inicia/detiene en `App.Run`, se actualiza en `App.UpdateConfig` y recibe los eventos del notificador de alertas y del fin de escaneos.

### `internal/alerts`

Reglas integradas: host caído, host nuevo, latencia alta, puerto abierto/cerrado, servicio o dispositivo cambiado, uso de subred y escaneo fallido. Cada regla tiene severidad, configuración, canales y cooldown. La deduplicación usa una clave estable por regla/host/recurso para no repetir alertas activas.

Canales: webhook (firma `X-NetDiag-Signature: sha256=…`), Slack, Discord, Telegram, PagerDuty, SMTP y syslog. Los envíos pasan por un rate limiter global y una validación SSRF que bloquea destinos privados salvo autorización explícita.

### `internal/report`

Genera PDF con `go-pdf/fpdf`, CSV, JSON y HTML. Los informes se guardan en `<datos>/reports`, se registran en la base y se descargan por la API. El PDF incluye portada, resumen, inventario y alertas del periodo.

### `internal/webui`

`//go:embed dist` sirve la SPA, `/metrics` (delegado a la API) y `/healthz`. No hay CDN ni fuentes externas: el CSP puede ser estricto. El frontend usa delegación de eventos, un router por hash, estado en memoria y WebSocket con reconexión.

La SPA está escrita en español y un diccionario español→inglés (`i18n.js`) traduce el DOM renderizado, incluida la guía y el recorrido guiado, que mantienen su propio contenido bilingüe. El idioma se guarda en `localStorage`. La guía interactiva y el tour por selectores viven en `views.js` y `app.js`; no requieren backend. Los elementos de escritura se ocultan con `html[data-can-write="0"]`, pero la autorización real siempre la aplica la API.

## Flujo de un escaneo

```
POST /api/v1/scans
  ├─ validar CIDR, permisos y solapamientos
  ├─ crear fila en scans (queued)
  ├─ goroutine runScan
  │    ├─ discover.Run(progress) ──► scan.progress (WS)
  │    ├─ UpsertHost por hallazgo ──► métricas, eventos, vecinos
  │    ├─ MarkMissingHosts
  │    ├─ alerts.Evaluate ─────────► alert.new (WS + canales)
  │    └─ FinishScan(done|error|canceled)
  └─ respuesta 201 con el escaneo
```

## Decisiones de diseño

- **SQLite antes que un ORM**: consultas explícitas, migraciones legibles y cero dependencias nativas.
- **Un binario**: despliegues reproducibles, copias de seguridad triviales y funcionamiento offline.
- **Copias autocontenidas**: un ZIP con base, configuración y clave evita restauraciones parciales o canales indescifrables.
- **Identidad por MAC con fallback a IP**: evita duplicados cuando DHCP cambia las direcciones.
- **Seguridad conservadora**: canales cifrados, SSRF bloqueado, sesiones en servidor y auditoría desde el primer arranque.
- **Progreso por WebSocket, no polling**: la UI recibe eventos y el backend limita la frecuencia de escritura.
