# Roadmap de NetDiag

Este documento registra el estado del producto a partir de la versión **1.5.0**. El roadmap original F0–F26 se conserva como referencia histórica en `notes/14-roadmap.md`; aquí se añaden las mejoras posteriores, las decisiones comerciales ya tomadas y el plan de salida al mercado.

## Estado actual

| Bloque | Estado |
| --- | --- |
| F0–F12, F18–F25 | Completados e integrados en `1.5.0` |
| F13, F14, F15, F16, F17, F26 | Diferidos y justificados en `README.md` |
| F23 | Cerrado: notificación + descarga verificada + instalación con rollback + reinicio |
| F24 | Backup automático programado |
| F33, F34 | Decisiones cerradas; documentación legal y comercial añadida en `1.5.0` |
| F35 | Implementado: licencia Ed25519 offline, CLI, API, UI y backup |
| F36 | Implementado: contenedor `.ndc` ZIP firmado, inspect/verify/build |
| F37 | Implementado y verificado: release reproducible, SBOM, procedencia, `.ndc` y verificación offline |
| F38 | Documentación y plantillas de confianza empresarial listas |
| F39 | Implementado y verificado: demo sintética aislada, marcada en UI e informes |
| F40 | Implementado y verificado: vistas guardadas por usuario y enlaces compartidos |
| F41 | Parte técnica implementada: `min_version` firmado; queda el ensayo humano del runbook CRA |
| F42 | Plantillas y plan listos; sin evidencia de uso real todavía |
| F43 | Implementado y verificado: histórico IP→MAC, anomalías ARP y regla `anomaly` |
| F45 | Implementado: reglas de alerta de seguridad dedicadas con precedencia |
| F44 | Implementado: sonda DHCP UDP/68, histórico y detectores de servidor rogue |
| F46 | Implementado: SNMP profundo IP-MIB/Q-BRIDGE, comunidad enmascarada y correlación L2 |
| F47 | Implementado: captura pasiva opt-in con `-tags pcap`, metadata-only y cinco detectores |
| Pruebas | `gofmt`, `go build`, `go vet`, `make test`, chequeo JS y smoke con binario real |

## Mejoras propuestas para 1.5.1

Prioridad alta, bajo riesgo: son cambios principalmente de interfaz, no alteran SQLite ni la seguridad.

### F27 — Asistente de primer arranque

**Objetivo:** que un usuario nuevo llegue a ver datos útiles en menos de dos minutos.

Al crear la cuenta de administrador:

1. Detectar la red local a partir de interfaces y gateway.
2. Proponer un CIDR inicial con perfil `common`.
3. Lanzar el primer escaneo y mostrar progreso.
4. Guardar automáticamente un snapshot de auditoría como línea base.
5. Ofrecer el tour guiado existente al terminar.

**Criterio de cierre:** tras el setup, el panel muestra hosts reales y una línea base sin que el usuario tenga que buscar el botón de escaneo.

**Esfuerzo:** 2–4 h.

### F28 — Panel de salud de red

**Objetivo:** resumir el estado de la red en una sola pantalla con lenguaje ejecutivo.

Añadir al panel una tarjeta con:

- Puntuación `0–100` con semáforo.
- Tendencia de los últimos 7 datos usando `/history/compare`.
- Las tres incidencias más importantes de `/diagnostics`.
- Acceso directo a la evidencia de cada incidencia.

La puntuación debe ser explicable: disponibilidad 40 %, latencia 20 %, cambios no esperados 20 % y severidad de hallazgos 20 %. Nunca se muestra un número sin su desglose.

**Criterio de cierre:** la tarjeta explica de dónde sale la puntuación y permite abrir el hallazgo relacionado.

**Esfuerzo:** 2–3 h.

### F29 — Buscador global

**Objetivo:** localizar cualquier objeto sin recorrer menús.

- Atajo `Ctrl+K` / `Cmd+K`.
- Búsqueda por IP, MAC, hostname, identificador de alerta y red.
- Navegación directa al detalle correspondiente.

**Criterio de cierre:** desde cualquier vista se puede buscar un host por IP y abrir su ficha con el teclado.

**Esfuerzo:** 2–3 h.

### F30 — Centro de notificaciones

**Objetivo:** conservar los eventos que hoy se muestran solo como toasts.

- Campana con contador en la cabecera.
- Historial de `scan.finished`, `alert.new`, `update.available`, `update.applied` y `update.rolled_back`.
- Marcar como leído y abrir el objeto relacionado.

**Criterio de cierre:** un evento importante sigue accesible después de recargar la página.

**Esfuerzo:** 3–4 h.

### F31 — Diagnósticos accionables

**Objetivo:** convertir cada hallazgo en una gestión concreta.

Para cada hallazgo de `/diagnostics`:

- Mantener evidencia y confianza.
- Botón **Ver host**.
- Botón **Copiar comando** cuando exista una comprobación útil.
- Botón **Crear regla de alerta** cuando el hallazgo sea monitorizable.

**Criterio de cierre:** desde un hallazgo se puede llegar al host y crear una regla sin salir de la vista.

**Esfuerzo:** 3–5 h.

### F32 — Informe ejecutivo de una página

**Objetivo:** entregar un artefacto listo para clientes o dirección.

Generar un PDF de una página con:

- Inventario resumido.
- Disponibilidad y latencia de los últimos 7 días.
- Cambios principales de `/history/compare`.
- Hallazgos de mayor severidad.
- Pie con versión de NetDiag, retención activa, edición y fecha de generación.

**Criterio de cierre:** el informe se genera desde un botón y no expone secretos ni datos de configuración.

**Esfuerzo:** 4–6 h.

## Decisiones de producto cerradas

Estas decisiones ya no están pendientes: definen la licencia, el cobro y el canal de publicación. Los documentos ejecutivos están en `docs/licenciamiento.md`, `docs/comercializacion.md`, `docs/publicacion-segura.md` y `docs/confianza-empresarial.md`.

### F33 — Licenciamiento

**Decisión:** doble licencia.

- **Community:** `AGPL-3.0`. Archivo `LICENSE` en la raíz; `NOTICE` y `THIRD_PARTY_LICENSES.md` con los avisos de terceros.
- **Enterprise/Pro:** licencia comercial de pago, sin obligación de publicar modificaciones, con derechos de uso, soporte y actualizaciones definidos en contrato.
- El funcionamiento básico **no depende** de un servidor de licencias en línea. La edición de pago se acredita con una licencia firmada localmente.
- El código aportado por terceros se acepta bajo DCO; no se exige CLA para Community. La licencia comercial se firma aparte con el cliente.

**Criterio de cierre:** toda versión utilizable incluye `LICENSE`, `NOTICE`, `THIRD_PARTY_LICENSES.md` y `SECURITY.md`, y la oferta comercial queda descrita sin ambigüedad.

### F34 — Modelo comercial

**Decisión:** suscripción anual por banda de hosts gestionados; no se cobra por informe ni por host detectado.

| SKU | Precio anual | Hosts gestionados | Sedes | Retención | Incluye |
| --- | ---: | ---: | ---: | --- | --- |
| `community` | 0 € | Sin límite de descubrimiento | 1 | 7 días | Binario AGPL, actualización manual, soporte comunitario |
| `pro-25` | 990 € | 25 | 3 | 1 mes | Actualizaciones firmadas automáticas, informe ejecutivo, soporte por correo |
| `pro-100` | 2.490 € | 100 | 10 | 1 mes | Igual que `pro-25` |
| `pro-500` | 5.900 € | 500 | Ilimitadas | 1 año | Igual que `pro-25` |
| `enterprise-500` | 9.900 € | 500 | Ilimitadas | 7 años | Licencia offline, SLA, paquete de confianza, onboarding |
| `enterprise-2000` | 19.900 € | 2.000 | Ilimitadas | 7 años | Igual que `enterprise-500` |
| `enterprise-custom` | A medida | Ilimitados | Ilimitadas | 7 años o según contrato | Espejo privado, condiciones a medida |

**Definición de host gestionado:** identificador único `s<siteID>:<identity>` observado en los últimos 30 días, excluyendo hosts con etiqueta `guest` o `excluded`. El descubrimiento y el escaneo son ilimitados: un host detectado una sola vez no factura.

**Reglas de facturación:**

- Factura anual por adelantado; IVA no incluido; transferencia/SEPA y orden de compra aceptadas.
- La banda se revisa en la renovación; superar la banda durante el año genera aviso, nunca una factura sorpresa ni borrado de datos.
- La retención es un diferenciador del plan (7 días / 1 mes / 1 año; Enterprise 7 años), no un producto de duración de acceso.
- Prueba de 30 días con todas las funciones Pro; al caducar se conservan los datos y se vuelve a capacidades Community.
- No hay cargo por informe, alerta, escaneo, usuario ni puerto.

**Criterio de cierre:** un cliente puede elegir SKU, entender qué paga y recibir una factura predecible sin hablar con ventas para los planes Pro.

## Implementación pendiente

### F35 — Activación offline — **implementado, pendiente de release**

Detalle de uso, claves y restauración en `docs/licenciamiento.md`.

- Clave de licencia firmada con Ed25519 y guardada en `<data_dir>/license.json`.
- Campos: `schema`, `license_id`, `customer`, `edition`, `max_managed_hosts`, `max_sites`, `retention_days`, `expires_at`, `capabilities` y `support_level`.
- Comandos `netdiag license show`, `netdiag license activate <archivo>` y `netdiag license clear`.
- Endpoint `GET /api/v1/system/license` para cualquier rol autenticado; devuelve estado, edición, límites y caducidad, nunca la firma ni datos de cliente ajenos.
- Verificación local sin telemetría; el derecho real se apoya en soporte, SLA y acceso a actualizaciones firmadas.
- Si se supera una banda o caduca la licencia: aviso en UI y API, sin bloquear lectura ni destruir datos.

**Criterio de cierre (cumplido):** una licencia firmada puede activarse en una máquina sin red y `netdiag license show` explica edición, límites y motivo de cualquier aviso. Verificado con `go vet ./...`, `go test ./...`, prueba end-to-end de `sign → activate → show → clear` y manipulación de la firma.

**Esfuerzo:** 6–8 h (realizado).

### F36 — Contenedor firmado `.ndc` — **implementado, pendiente de release**

Detalle de formato, comandos y protecciones en `docs/publicacion-segura.md`.

- Formato abierto y versionado: ZIP con `manifest.json`, `payload/`, `signature.minisig` y opcionalmente `sbom.cdx.json`.
- `manifest.json` incluye `schema`, `version`, `kind` (`release`|`evidence`|`site`), `created_at`, `files[{path,sha256,size}]` y `producer`.
- La firma cubre el manifiesto canónico; el comentario de confianza liga `version:` y `sha256:` del manifiesto.
- CLI: `netdiag container inspect <archivo>` y `netdiag container verify --key <pub> <archivo>`.
- JSON/CSV/PDF/ZIP sin firma siguen siendo formatos abiertos y prioritarios; `.ndc` se usa solo cuando se necesita integridad y procedencia.
- Un `.ndc` de evidencia puede contener el informe ejecutivo, el diff de auditoría y el snapshot de configuración sin secretos.

**Criterio de cierre (cumplido):** un cliente verifica un `.ndc` modificado y la verificación falla con un mensaje claro; el archivo puede inspeccionarse con `unzip` sin NetDiag. Verificado con `go test ./internal/container`, `go vet ./...` y prueba end-to-end: `build → inspect → verify`, manipulación de un fichero de `payload/` y rechazo con `Resultado: NO VÁLIDO`, más `unzip -l` sobre el contenedor.

**Esfuerzo:** 6–8 h (realizado).

### F37 — Publicación segura y reproducible — **implementado, verificado, pendiente de release**

Herramientas: `cmd/netdiag-release`, `scripts/release.sh`, `scripts/verify-release.sh`, `scripts/sbom.py` y `scripts/provenance.py`. Objetivos de `make release-secure`, `make verify-release` y `make release-keygen`.

- Build reproducible: Go 1.27.x fijado, `CGO_ENABLED=0`, `-trimpath`, `-buildvcs=false`, `-ldflags '-s -w -buildid='`, `SOURCE_DATE_EPOCH` derivado del commit, `go mod verify`.
- Generar por versión: binarios linux/amd64 y linux/arm64, variante `-tags pcap`, `SHA256SUMS`, `SHA256SUMS.minisig`, `manifest.json` F23, `RELEASE_NOTES.md` firmado, SBOM CycloneDX y atestación de procedencia.
- Clave minisign de release en hardware/offline; clave raíz aparte solo para rotar la de release; publicación de clave y revocación en `docs/public_keys.md`.
- Espejo HTTPS inmutable (`releases/<versión>/`) con tags de git inmutables; sin HTTP, sin credenciales en URL, HSTS y listado de checksums en la página de descarga.
- F23 consume exactamente `manifest.json` + `public_key` del espejo oficial; el cliente puede sustituir ambos por su espejo interno.
- Enterprise recibe el mismo artefacto firmado, licencia offline y manifiesto para alojarlo en su red sin conexión externa.

**Criterio de cierre (cumplido):** desde un checkout limpio, dos ejecuciones con el mismo `SOURCE_DATE_EPOCH` producen `SHA256SUMS`, manifiesto, SBOM y `.ndc` byte a byte idénticos. `scripts/verify-release.sh` valida firmas, hashes, manifiesto F23 y contenedor; al añadir un byte a `RELEASE_NOTES.md` la verificación termina con `NO VÁLIDO` y código de salida 1. Se usó una clave de prueba de `netdiag-release keygen`; la clave oficial `netdiag-release` sigue siendo la que se documentará en `docs/public_keys.md`.

**Esfuerzo:** 8–12 h (realizado).

### F38 — Paquete de confianza empresarial — **documentación y plantillas listas**

Paquete en `docs/confianza-empresarial.md`, `docs/comercializacion.md` y `docs/plantillas/`. La demo con datos sintéticos depende de F39.

- Documentos: whitepaper de seguridad, modelo de amenazas, flujo de datos, DPA/EULA, matriz SLA, guía de endurecimiento y política de vulnerabilidades.
- Proceso: prueba de 30 días, PoC guiado, presupuesto con SKU, orden de compra, factura anual, licencia firmada y alta en soporte.
- SLA Enterprise: P1 respuesta 1 h / workaround 8 h / objetivo de solución 24 h; P2 4 h hábiles / 2 días; P3 1 día hábil / 5 días. El P1 de 1 h requiere cobertura alterna; sin ella se ofrece 8 h hábiles y se indica en el presupuesto.
- Pro: soporte por correo con objetivo 48 h hábiles, sin SLA contractual.
- Kit de venta: ficha de precios, one-pager de ROI, demo con datos sintéticos (F39), preguntas frecuentes de seguridad y checklist de compras.
- Transparencia: declarar que SQLite no cifra toda la base por defecto; `secret.key` protege secretos de integraciones; la edición portable no captura 802.11 crudo.

**Criterio de cierre:** un comprador recibe un paquete único con licencia, factura, SBOM firmado, documentos legales y pasos de verificación antes de firmar.

**Esfuerzo:** 10–16 h (mayormente documentación y plantillas).

### F39 — Modo demo con datos sintéticos — **implementado y verificado con binario real**

Uso: `netdiag demo --seed 1 --sites 3 --hosts 120 [--data-dir RUTA] [--json]` y, a continuación, `netdiag serve --data-dir <data_dir>/demo`. El marcador `<data_dir>/demo/demo.json` activa la marca `demo: true` en `/auth/status`, `/system/info`, la barra superior, la tarjeta de acceso y los informes (`netdiag-demo-*.json/pdf/csv/html` con `demo: true`).

- `netdiag demo --seed 1 [--sites 3] [--hosts 120]` genera sedes, hosts, puertos, alertas, métricas, topología y snapshots de auditoría ficticios.
- No ejecuta escaneos reales ni abre sockets hacia la red; los datos se marcan con `demo: true` en la UI y en exportaciones.
- Se usa para demostraciones, documentación, pruebas de UI y evaluaciones sin exponer inventario real.
- Opción de reiniciar el dataset demo sin borrar datos reales: base separada bajo `<data_dir>/demo/netdiag.db`.

**Criterio de cierre (cumplido):** e2e con binario real: `demo --seed 11 --sites 3 --hosts 90` generó 3 sedes, 90 hosts, 1080 métricas, 6 alertas y 79 hosts gestionados tras excluir `guest`/`excluded`. Al servir `<data_dir>/demo`, `/auth/status` y `/system/info` devolvieron `demo: true`, la licencia Community mostró esos 79 hosts y el informe ejecutivo JSON se guardó como `netdiag-demo-executive-*.json` con `demo: true` y 90 hosts. La base real en `<data_dir>/netdiag.db` quedó intacta. Los tests cubren marcador, determinismo por semilla y aislamiento.

**Esfuerzo:** 6–10 h (realizado).

### F40 — Filtros y vistas guardadas — **implementado y verificado**

API en `/api/v1/views`, nueva página en el SPA, barra de vistas en inventario y alertas, editor con catálogo validado en servidor y paleta `Ctrl+K`.

- Guardar filtros de inventario y alertas por usuario: nombre, `site_id`, consulta, severidad/estado, columnas y orden.
- Vistas por defecto: `Críticas`, `Nuevos hosts`, `Caídos`, `Cambios de la semana`.
- Compartir una vista mediante URL con parámetros; el nombre y la consulta se sanean antes de persistir.
- Integración con F29: `Ctrl+K` puede abrir una vista guardada.

**Criterio de cierre (cumplido):** las cuatro vistas integradas se siembran por usuario; `POST /views` crea una vista sanada y `GET /views/{id}/share` devuelve un hash `#/<kind>?...` que `GET /views/resolve` vuelve a validar sin persistirla. Un operador recupera sus vistas tras recargar y otro operador abre el mismo filtro sin acceder a su fila privada. `gofmt`, build, vet, tests y parseo JS pasan.

**Esfuerzo:** 4–6 h (realizado).

### F41 — Cumplimiento CRA y responsabilidad por terceros — **implementado, pendiente de ensayo y ejecución continua**

Runbook y plantillas en `SECURITY.md` y `docs/plantillas/`. `min_version` ya viaja firmado en F23: `Release.MinVersion`, `CheckResult.SecurityUpdate` y evento `update.security`. Se genera con `--min-version` en el pipeline F37.

- Política de soporte de seguridad publicada: versión contratada durante la suscripción + 12 meses. Debe aparecer en `SECURITY.md`, en la orden de compra y en el EULA.
- Runbook de incidentes: responsable único, cuenta en la Single Reporting Platform de ENISA, árbol de decisión «¿explotación activa o incidente grave?» y plantillas de aviso 24 h / 72 h.
- Monitorización de dependencias: SBOM por release, revisión periódica de avisos, parche entregado como release nueva firmada. Nunca firmar un binario de terceros sin reconstruirlo o verificarlo.
- Registro de responsabilidad: fecha de conocimiento, versiones afectadas, decisión de reportar, parche, comunicación al cliente y cierre.
- EULA reforzado: el cliente declara que está autorizado a analizar esas redes; se prohíbe escaneo de terceros; se limita responsabilidad con las excepciones legales obligatorias.
- Comunicación: correo a clientes de pago, notificación en la UI y `REVOCATION.md` firmado; `min_version` en el manifiesto F23 para saltar versiones inseguras.

**Criterio de cierre (parte técnica cumplida):** un manifiesto con `min_version` manipulado falla la verificación; una instalación anterior a esa cifra recibe `security_update: true` y el evento `update.security`. Queda pendiente el ensayo humano del runbook con responsable y credenciales SRP reales: aviso preparado en < 24 h, notificación en < 72 h y release firmada en < 7 días.

**Esfuerzo:** 8–12 h de proceso y plantillas, más la gestión continua.

### F42 — Validación de mercado sin telemetría — **plantillas listas; ejecución en curso**

Plan, fuentes públicas y umbral 3/2/1 en `docs/validacion.md` y `docs/plantillas/`.

- Publicar Community y medir solo fuentes públicas: descargas de GitHub, estrellas, forks, issues y Discussions.
- Discussion fijado con tres preguntas: dónde se desplegó, qué se intentó hacer, qué faltó.
- Encuesta voluntaria de 5 preguntas; **nunca instrumentar el binario**.
- Piloto de 30 días con 3–5 organizaciones, sin SLA y con una llamada semanal de 20 minutos.
- Umbral para pasar a F35/F37/F38: 3 instalaciones reales con 7 días de uso, 2 casos de uso concretos y 1 disposición a pagar o carta de intención.
- Plan completo y plantillas en `docs/validacion.md`.

**Criterio de cierre:** existe evidencia escrita de uso real y disposición a pagar antes de contratar seguros, espejo privado, SLA o demo completa.

**Esfuerzo:** 4–6 h de plantillas y seguimiento; la gestión es continua pero ligera.

### F43 — Histórico ARP y detección de MITM — **implementado y verificado con binario real**

Primera fase de seguridad del activo, sin captura de paquetes ni raw sockets. Cada escaneo guarda su par IP→MAC en `arp_observations` (v1.5.6, retención 30 días) y el detector compara lotes para emitir `arp_gateway_mac_changed`, `arp_ip_mac_changed`, `arp_mac_multi_ip` y `arp_ip_multi_mac`. La regla de alerta `anomaly` dispara avisos con `min_confidence` y `category`, con dedupe y host del inventario resuelto. Endpoint `GET /diagnostics/arp` y tabla `Histórico IP→MAC` en `#/diagnostics`.

Verificado en `127.0.0.1:18780`: escaneo real de `192.168.0.0/24`, observación sintética antigua del gateway con otra MAC, anomalía crítica confianza 95 y alerta `Posible MITM` visible con `site_id=1`. Detalle en `docs/validacion.md`. Siguieron F45 (alertas dedicadas), F44 (DHCP rogue), F46 (SNMP profundo) y F47 (captura pasiva metadata-only).

**Criterio de cierre:** dos escaneos con MAC distinta para una misma IP generan un hallazgo con evidencia, y la alerta aparece filtrando por sede. Sin captura L2 no se confirma spoofing activo: es una pista para investigar.

**Esfuerzo:** 6–10 h de implementación y verificación.

### F45 — Alertas de seguridad dedicadas — **implementado**

`EvaluateAnomalies` enruta los hallazgos a reglas `arp_spoofing`, `dhcp_rogue`, `snmp_anomaly` y `passive_anomaly`, con precedencia sobre `anomaly` para no duplicar avisos. La UI permite crearlas con `min_confidence` y `category`. Cubre las cinco familias que faltaban antes de tener detectores para todas ellas.

### F44 — DHCP rogue con UDP/68 — **implementado y verificado solo por tests**

Sonda DHCPINFORM/DHCPDISCOVER sin raw sockets, tabla `dhcp_observations` (v1.5.7) y detectores `dhcp_multiple_servers`, `dhcp_unauthorized_server`, `dhcp_gateway_spoof` y `dhcp_server_mac_changed`. La confianza depende de la lista `diagnostics.authorized_dhcp_servers` y de la correlación con el MAC del gateway en el histórico ARP. Endpoint `GET /diagnostics/dhcp` y tabla en `#/diagnostics`.

En esta red `192.168.0.105:68` enlazó con `SO_REUSEADDR`, pero el gateway y el broadcast no respondieron: no hay evidencia real de rogue. El escaneo degradó limpiamente. Con una respuesta sintética se validó el recorrido completo: anomalía `dhcp_unauthorized_server` → regla `dhcp_rogue` → alerta crítica visible en `site_id=1`. Queda pendiente validar contra un laboratorio con un servidor DHCP controlado.

**Criterio de cierre:** un OFFER de un servidor distinto al gateway o al autorizado genera una alerta `dhcp_rogue` con evidencia, y un cambio de MAC del servidor entre escaneos genera `dhcp_server_mac_changed`. Ambos se comprueban con tests; el primero además con un escaneo real y datos sintéticos.

### F46 — SNMP profundo — **implementado**

Amplía el recolector con `ipAddrTable` (`.1.3.6.1.2.1.4.20`), las tablas ARP `.4.22`/`.4.35`, Q-BRIDGE `.1.3.6.1.2.1.17.7.*` (VLAN y FDB) y BRIDGE-MIB `.1.3.6.1.2.1.17.4.*`. Guarda la comunidad usada **enmascarada** y las filas crudas en cuatro tablas SQLite (v1.5.9) con retención de 30 días. `DetectSNMP` correlaciona ARP del switch, FDB y LLDP/CDP y emite hallazgos `snmp_*` con `category: "snmp"`, listos para la regla `snmp_anomaly`. Endpoint `GET /diagnostics/snmp` y sección «SNMP profundo» en `#/diagnostics`.

No hay switch SNMP real en este entorno, así que la verificación funcional usa filas sintéticas. Los walks tienen límites y degradan limpiamente si el dispositivo no expone las tablas.

**Criterio de cierre:** un cambio de MAC del gateway en la tabla ARP del switch y una MAC en dos puertos o VLAN generan hallazgos con evidencia; FDB y LLDP en puertos distintos generan `snmp_lldp_fdb_disagree`. Se comprueba con tests y con el endpoint sobre datos sintéticos.

### F47 — Captura pasiva — **implementada**

Añade `internal/capture` con build-tags `pcap`/`!pcap`. El binario portable sigue funcionando sin raw sockets y `doctor` indica que la captura no está compilada. Con `-tags pcap`, `CAP_NET_RAW` y `diagnostics.passive_capture: true`, cada escaneo abre una ventana corta con BPF estrecho y guarda solo metadatos en `passive_observations` (v1.5.10), sin payload, credenciales ni respuestas DNS. `DetectPassive` emite cinco tipos `passive_*` con `category: "capture"` para la regla `passive_anomaly`: GARP repetido, ICMP redirect, DHCP no autorizado, SYN entrantes y túnel DNS. Endpoint `GET /diagnostics/passive` y sección «Captura pasiva» en `#/diagnostics`.

No hay `CAP_NET_RAW` en este entorno, así que no se puede capturar tráfico real. Se verifica compilación en ambos modos, degradación limpia, parsers con paquetes construidos en memoria y el recorrido detector→alerta con observaciones sintéticas. La captura frente a un switch con SPAN o un tap queda pendiente.

**Criterio de cierre:** una ventana con GARP, ICMP redirect, DHCP OFFER de un servidor no autorizado, escaneo SYN y dominio de alta entropía genera los cinco hallazgos con evidencia; un servidor autorizado o el gateway del histórico no genera `passive_dhcp_server`. Se comprueba con tests y con el endpoint sobre datos sintéticos.

## Dependencias y orden recomendado

| Fase | Depende de | Prioridad |
| --- | --- | --- |
| F27, F28, F29 | Ninguna | Alta, entrega 1.5.1 |
| F30, F31, F32 | F28 para el panel y F21/F22 para datos | Alta, entrega 1.5.1 |
| F33, F34 | Ninguna | Cerradas en 1.5.0 |
| F35 | F33, F34 | Alta: habilita prueba y venta |
| F36 | F23 | Media: reutiliza minisign y canonicalización |
| F37 | F23, F36 | Alta: es el canal de distribución |
| F38 | F33–F37 | Alta: es el paquete que se entrega al comprador |
| F39 | F27, F28 para lucir la demo | Media: ventas y documentación |
| F40 | F29 | Media: retención de usuarios |
| F41 | F33, F37, F38 | Alta: obligación legal y contractual |
| F42 | Ninguna | Alta: se ejecuta antes de F35/F37/F38 |
| F43 | Ninguna | Media: valor de seguridad sin permisos ni captura |
| F45 | F43 | Alta: evita alertas duplicadas al añadir detectores |
| F44 | F43, F45 | Alta: aprovecha el histórico ARP y la regla dedicada |
| F46 | F43, F45 | Alta: completa seguridad con SNMP y reutiliza `snmp_anomaly` |
| F47 | F43, F45, F46 | Media: captura opcional con privilegios y reutiliza `passive_anomaly` |

## Cierre de salida al mercado

NetDiag se considera listo para vender cuando:

1. F42 ha mostrado uso real y al menos una disposición a pagar antes de invertir en estructura comercial.
2. Los documentos legales y de precios están publicados y revisados.
3. El espejo oficial publica artefactos firmados y verificables desde un checkout limpio.
4. Una licencia de prueba de 30 días puede emitirse y activarse sin red.
5. Existe una reproducción completa del flujo: demo → PoC → presupuesto → factura → licencia → soporte → renovación.
6. El paquete Enterprise incluye SBOM, atestación, DPA/EULA, SLA y guía de verificación.
7. `SECURITY.md` define canal privado de reporte, plazos de respuesta, obligaciones CRA 24/72 h y proceso de rotación de claves.
8. Existe un responsable identificado para la Single Reporting Platform y se ha ensayado el runbook de CVE en dependencia de terceros.
