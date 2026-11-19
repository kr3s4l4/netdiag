# Validación de mercado sin telemetría

Objetivo: comprobar si NetDiag resuelve un problema real antes de invertir en licencias, SLA, espejo privado, demo completa o estructura comercial.

**Regla de oro:** las descargas son curiosidad; la aceptación es **uso repetido en una red real**; la viabilidad comercial es **alguien dispuesto a pagar o a firmar un piloto**.

No se añade telemetría al binario. La promesa de “sin telemetría” es parte del producto y no se negocia para medir vanidad.

## Antes de publicar: que no parezca un script

La dirección ya es visible: el README, el [roadmap](roadmap.md) y los documentos de licenciamiento, publicación segura y confianza empresarial. Lo que falta es **prueba visual**. Un visitante decide en segundos si esto es un producto con rumbo o un experimento suelto.

Esto es presentación, **no código**. No hace falta implementar F27–F40 para hacerlo:

- [ ] 3–5 capturas reales de la interfaz: inventario, topología, diagnósticos y ajustes. Usa datos de tu laboratorio; no montes un entorno ficticio.
- [ ] Un GIF corto (10–20 s) del primer valor: añadir red → escanear → ver hosts identificados.
- [ ] Firma discreta `kr3s4l4` en barra lateral, pie de acceso e informes PDF, sin sustituir el nombre del producto (ya implementada).
- [ ] El bloque «Hacia dónde va NetDiag» visible cerca del inicio del README, con enlace al roadmap.
- [ ] En la nota de Release: el mismo bloque de dirección y las tres preguntas del Discussion fijado.
- [ ] **El repo tiene `remote` configurado y los commits están pusheados.** Sin esto, todo el trabajo sigue solo en local y nadie puede encontrarlo.
- [ ] Repo llamado **`netdiag`**, sin sufijo: el usuario ya lo hace único (`kr3s4l4/netdiag`) y coincide con el path del módulo en `go.mod`. Renombrarlo obligaría a cambiar 195 referencias en 97 archivos y rompería `go install github.com/kr3s4l4/netdiag@latest`.
- [ ] Descripción: «NetDiag — escáner de red self-hosted, binario único, sin telemetría, por kr3s4l4». `topics`: `go`, `network-scanner`, `network-monitoring`, `self-hosted`, `snmp`, `network-inventory`, `ciberseguridad`, `offline`, `no-telemetry`.
- [ ] `SECURITY.md` y plantilla **Caso de uso real** en `.github/ISSUE_TEMPLATE/`; enlace desde `kr3s4l4.github.io` y desde el README de perfil de GitHub.
- [ ] Texto de la Release marcado como **preview de validación**, no como release oficial firmada.

Sin esas capturas, la gente lee 487 líneas de README y aun así se pregunta si hay interfaz. Con ellas, entiende de un vistazo que es un producto self-hosted serio.

## Qué medir

| Señal | Cómo se mide sin telemetría | Umbral orientativo |
| --- | --- | --- |
| Descargas | Contador de assets de GitHub Release | 50 descargas en 4–6 semanas |
| Interés cualificado | Issues y Discussions con caso de uso real, no solo “no arranca” | 5 personas contando dónde y para qué lo usan |
| Uso repetido | Encuesta voluntaria: “¿sigue instalado tras 7 días?” | 3 respuestas afirmativas |
| Valor percibido | Pregunta abierta: “¿qué ha descubierto NetDiag que no veía?” | 2 respuestas concretas, no genéricas |
| Profundidad | Solicitudes de systemd, TLS, backup, multi-sede o informes | 2 peticiones de despliegue serio |
| Disposición a pagar | Pregunta directa en encuesta o llamada | 1 persona acepte un piloto de pago |

Los umbrales son orientativos. Sirven para decidir el siguiente paso, no para declarar éxito.

## Fuentes públicas permitidas

Solo estas fuentes. Nunca instrumentar el binario.

- Descargas de assets de GitHub Release.
- Estrellas y forks.
- Issues y Discussions.
- Encuesta voluntaria y escritos del piloto.

Cualquier métrica que exija código dentro del producto queda fuera. La promesa «sin telemetría» no se negocia para medir vanidad.

## Discussion fijado: texto exacto

Fijar el Discussion desde ⋯ → *Pin discussion* (la API de GitHub no expone esa acción).

```text
Título: ¿Para qué estás usando NetDiag?

1. ¿Dónde lo has desplegado?
2. ¿Qué fue lo primero que intentaste hacer?
3. ¿Qué te faltó o te confundió?

Responde con tres líneas. No compartas datos de tu red ni credenciales.
Leemos todas las respuestas.
```

Plantillas listas para copiar:

- [Encuesta de 5 preguntas](plantillas/encuesta-5-preguntas.md).
- [Correo de piloto](plantillas/correo-piloto.md).
- [Registro de evidencia F42](plantillas/registro-evidencia-f42.md).

## Fases

### 1. Observar

Publicar el tag y la documentación. No hace falta espejo oficial ni SLA.

- Textos adaptados a cada canal en [docs/divulgacion.md](divulgacion.md).
- Usar contadores de GitHub Releases.
- Fijar un Discussion: “¿Para qué estás usando NetDiag?”.
- Añadir una plantilla de issue “Caso de uso real”.
- Si se adjuntan binarios, que estén claramente marcados como **preview** o publicar solo fuente con instrucciones de build. No mezclar “beta de validación” con “release oficial firmado”; para lo segundo hay que generar primero la clave real de `docs/public_keys.md`.

### 2. Escuchar

Formulario voluntario de 5 preguntas:

1. ¿Dónde lo has desplegado: casa, laboratorio, oficina, planta o datacenter?
2. ¿Qué fue lo primero que intentaste hacer?
3. ¿Qué fue lo que más te sirvió?
4. ¿Qué te faltó o te confundió?
5. ¿Lo seguirás usando? ¿Te interesaría una prueba de 30 días con soporte?

Leer las respuestas una por una. No promediar: una frase como “me sirvió para encontrar el switch que causaba los cortes” vale más que veinte estrellas.

### 3. Piloto

Con 3–5 personas u organizaciones:

- Duración: 30 días.
- Alcance: una sede real, una red, un escaneo `common`.
- Sin SLA. Soporte por correo con respuesta honesta.
- Compromiso del piloto: una llamada de 20 min por semana y un escrito corto al final.
- Entregar: checklist de `docs/confianza-empresarial.md` adaptado a “evaluación rápida”.

Si el piloto funciona, pedir permiso para usar el nombre o el testimonio. Si no funciona, preguntar qué falló antes de añadir funcionalidades.

### 4. Pago

Solo después de piloto:

- Preguntar por un piloto de pago reducido o por la suscripción `pro-25`.
- No inventar descuentos permanentes; el precio de lista ya está en `docs/comercializacion.md`.
- Si alguien acepta pagar, ahí sí activar lo mínimo: alta fiscal, factura, EULA, clave de release y runbook CRA.

## Cuándo avanzar

Avanzar a F35/F37/F38 cuando:

- Hay al menos 3 instalaciones reales con 7 días de uso.
- Hay 2 casos de uso concretos explicados por usuarios.
- Hay 1 persona u organización dispuesta a piloto de pago o carta de intención.

Mientras tanto, no bloquearse esperando perfección. El producto ya es funcional; lo que falta es evidencia.

## Cuándo no avanzar

Si tras 6–8 semanas solo hay descargas y silencio:

- No contratar espejo privado ni seguro caro todavía.
- No implementar SLA ni modo demo completo.
- Revisar onboarding, README y primer escaneo: probablemente el usuario no llega al “primer valor”.
- Considerar que el problema puede ser distribución o mensaje, no código.

## Qué NO hacer durante la validación

- No meter telemetría encubierta.
- No prometer SLA 1 h.
- No vender Enterprise antes de tener cobertura.
- No cobrar por informes ni por hosts detectados.
- No construir SaaS, multi-tenant ni plugins.
- No confundir “me gusta el proyecto” con “lo pondré en mi red”.

## Estado de publicación (18/11/2026)

- Repo público: <https://github.com/kr3s4l4/netdiag>, rama `master` por defecto, descripción, `topics` y homepage configurados.
- Release preview: <https://github.com/kr3s4l4/netdiag/releases/tag/v1.5.0>, con `netdiag-linux-amd64`, `netdiag-linux-arm64`, `SHA256SUMS` y `SHA256SUMS.minisig`. Marcada como *pre-release*; el resumen está firmado con la clave `netdiag-release` publicada en `docs/public_keys.md`.
- Discussion: <https://github.com/kr3s4l4/netdiag/discussions/1>. Creado; **queda fijarlo a mano** desde el menú ⋯ → *Pin discussion* porque la API de GitHub no expone esa acción.
- El repo antiguo `kr3s4l4/Netdiag` (programa anterior) se borró. Su último commit `11fdf866bb9c` sigue intacto en la copia local de referencia.
- Decisión actual: **los commits se mantienen locales**. `master` está por delante de `origin/master`; no se hace push sin instrucción explícita y sin token.
- Alternativas de pago, espejo y partners preparadas en [docs/alternativas-publicacion-venta.md](alternativas-publicacion-venta.md).

### Verificación técnica final (18/11/2026)

Ejecutada desde un worktree limpio en `1cafe70`, con binarios compilados desde la revision final:

- F35: licencia `pro` firmada, activada y mostrada; una copia con `customer` modificado fue rechazada.
- F36: `.ndc` construido, inspeccionado y verificado; alterar `payload/` fue rechazado.
- F37: dos releases con el mismo `SOURCE_DATE_EPOCH` produjeron `SHA256SUMS` y `.ndc` idénticos; `verify-release.sh` validó firma, checksum, contenedor y `versión mínima segura: 1.5.9`; alterar `RELEASE_NOTES.md` devolvió `NO VÁLIDO` y falló.
- F39: `demo --seed 3 --sites 2 --hosts 30` generó 2 sedes, 30 hosts y 26 hosts gestionados.
- Se usaron **claves de prueba**; `signing.json` se destruyó con `shred` y el worktree se eliminó. No quedan materiales de firma de prueba en `/tmp/opencode`.
- Comprobaciones de cierre: `make test`, `gofmt`, `go build`, `go vet`, parseo JS y `git status --short` limpio.

### Trazas y redes vecinas (18/11/2026)

- `netdiag trace 192.168.1.1 --hops 5 --probes 2` devolvió dos saltos: `192.168.0.1` y `192.168.1.1 livebox.home. [destino]`, con RTT ~5,3 ms.
- `POST /api/v1/trace` devolvió la misma ruta en JSON; un objetivo IPv6 fue rechazado con `400`.
- La primera llamada a `GET /api/v1/topology` ejecutó el descubrimiento automático y añadió `192.168.1.0/24` con router `192.168.1.1` (`livebox.home`), confianza 90 y evidencia de traza.
- `POST /api/v1/discover/neighbors` confirmó 9 candidatos privados y ninguna dirección pública.
- La vista «Trazas» mostró la red detectada y el formulario de traza en Chromium forzado a `en-US`, con la interfaz en español y sin errores JS.
- Se corrigió la topología para reutilizar nodos por IP/CIDR: ya no aparece dos veces `192.168.0.1` ni dos veces `192.168.1.0/24`. La arista de traza une directamente `192.168.0.1 → 192.168.1.1`.
- Motor implementado con UDP + `IP_RECVERR`: funciona sin raw sockets y sin binarios externos.

### Equipo local visible (18/11/2026)

- El descubrimiento excluye la IP propia. Ahora la topología añade el nodo `self:192.168.0.105` con etiqueta `Este equipo · kali` (MAC `a0:a4:c5:c9:b2:ba`), color propio, anillo discontinuo y arista `local` desde `192.168.0.1`.
- El nodo se marca con `is_self`, por lo que siempre muestra su etiqueta aunque caiga en un índice que `labelEvery` ocultaría. Verificado en el DOM: la etiqueta aparece en el SVG.
- El equipo también entra en `GET /api/v1/hosts` con tags `self` y `excluded`. `GET /api/v1/system/license` sigue informando **6 hosts gestionados**: el equipo local no cuenta para la licencia.
- Comprobación con Chromium: etiqueta en topología y chips `self`/`excluded` en inventario. `go test ./...`, `go vet` y `go build` OK.

### Brecha frente al NetDiag original (18/11/2026)

Inventario de solo lectura del repo histórico. No se borró código: el activo rediseñó el análisis pasivo por descubrimiento activo + diagnósticos por snapshot.

| Capacidad original | Estado actual |
| --- | --- |
| Captura pasiva y `AnomalyDetector` (ICMP redirect, GARP, IP→MAC, MAC→muchas IPs, TTL anómalo) | Parcial (F43: IP→MAC y MAC→muchas IPs desde escaneos; sin GARP, ICMP redirect ni TTL) |
| Detección de MITM por cambio de MAC del gateway y duplicados IP/MAC | Sí (F43: histórico entre escaneos; sin captura ARP en vivo) |
| DHCP rogue (OFFERs, sonda DISCOVER, regla `!IsGateway && >1 servidor`) | Parcial (F44: sonda UDP/68 y detectores; sin captura de OFFERs de terceros ni respuesta real en esta red) |
| Escaneos entrantes, DNS tunneling y protocolos en claro | No |
| Contexto DHCP y reducción de falsos positivos | Parcial (F44: lista de autorizados y correlación con ARP) |
| Reglas de alerta `anomaly` y `dhcp_rogue` | Sí (F45: dedicadas con precedencia; F44 ya emite hallazgos `dhcp_*`) |
| SNMP profundo: sweep de communities, IP-MIB, Q-BRIDGE, correlación ARP↔FDB↔LLDP | Parcial |
| Escaneo de puertos y banners | Sí (parcial: no distingue `closed`/`filtered`) |
| WiFi y VPN | Sí (en el original solo eran roadmap) |

Nota: el «DNS spoofing» histórico era un contador de consultas UDP/53, no validación de respuestas. Prioridad: ~~anomalías ARP/MITM~~ (F43) → ~~alertas de seguridad dedicadas~~ (F45) → ~~DHCP rogue~~ (F44) → SNMP profundo → captura pasiva.

### F43 — Histórico IP→MAC y detección de MITM (18/11/2026)

Primera fase de seguridad añadida al activo. Sin captura de paquetes, sin raw sockets y sin binarios externos: toda la evidencia sale de los escaneos ya realizados.

- Migración SQLite v1.5.6: tabla `arp_observations` append-only con índices por IP y MAC. Retención `diagnostics.arp_history_days` (30 días por defecto).
- Hook en `runScan`: cada escaneo guarda el par IP→MAC de los hosts descubiertos, incluido `is_gateway` y `scan_id`.
- Detector `DetectARP`: exige al menos dos escaneos y emite `arp_gateway_mac_changed` (crítico, confianza 95), `arp_ip_mac_changed` (75), `arp_mac_multi_ip` (80) y `arp_ip_multi_mac` (85). Descarta MACs vacías, broadcast, multicast y de todos ceros.
- Regla de alerta `anomaly` con `min_confidence` y `category`; dedupe por hallazgo y resolución del host del inventario para que la alerta aparezca en listas filtradas por sede. Tipo `dhcp_rogue` admitido pero aún sin detector.
- Endpoint `GET /api/v1/diagnostics/arp`: `{anomalies, observations, limitations}`. Documentado en OpenAPI y `docs/api.md`.
- UI: `#/diagnostics` muestra las anomalías ARP y las últimas 12 observaciones IP→MAC con equipo, gateway, escaneo y fecha.

Verificación real en `127.0.0.1:18780` con datos de la red `192.168.0.0/24`:

- Escaneo real guardó 4 observaciones IP→MAC (gateway `d4:6e:0e:57:68:8a`).
- Se insertó una observación antigua sintética del gateway con otra MAC para simular suplantación; el endpoint devolvió `arp_gateway_mac_changed` crítico con confianza 95 y su evidencia.
- Tras otro escaneo se creó la alerta `Posible MITM: cambió la MAC del gateway`, severidad crítica, regla `Anomalía de red`, host `192.168.0.1` (ID 4) y visible con `site_id=1`.
- Chromium headless confirmó en el DOM `Histórico IP→MAC`, la anomalía y las filas de observaciones.

Límite importante: es una pista basada en el histórico, no una confirmación de spoofing activo. Sin captura L2 no se distinguen GARP, ICMP redirect ni respuestas ARP duplicadas en vivo.

### F45 — Reglas de alerta de seguridad dedicadas (19/11/2026)

- `EvaluateAnomalies` acepta `anomaly`, `arp_spoofing`, `dhcp_rogue`, `snmp_anomaly` y `passive_anomaly`.
- Las reglas dedicadas fijan categoría y tienen precedencia sobre `anomaly`; un hallazgo no genera dos alertas.
- UI de reglas con los nuevos tipos y campos `min_confidence`/`category`, e i18n correspondiente.
- `TestDedicatedSecurityRuleTakesPrecedence` verifica que ARP use la regla dedicada y VLAN siga usando la genérica.

### F44 — DHCP rogue sin raw sockets (19/11/2026)

- Sonda activa `ProbeDHCP`: DHCPINFORM unicast al gateway y por broadcast, más DHCPDISCOVER por broadcast. Enlaza la IP local en UDP/68 con `SO_REUSEADDR` y `SO_BROADCAST`; si no puede enlazar, el escaneo continúa y se registra la limitación.
- Decodificación propia de BOOTP/DHCP: tipo de mensaje, server ID (54), máscara, router, DNS y lease. No se añadió ninguna dependencia.
- Migración SQLite v1.5.7: tabla `dhcp_observations` con índices por servidor, IP y fecha, y retención `diagnostics.dhcp_history_days` (30 días).
- Detector `DetectDHCP`: `dhcp_multiple_servers` (90), `dhcp_unauthorized_server` (80/85, 90 si NAK), `dhcp_gateway_spoof` (95) y `dhcp_server_mac_changed` (90/92). Un servidor es legítimo si está en `diagnostics.authorized_dhcp_servers` o si IP y MAC coinciden con el gateway del histórico ARP.
- Hook en `runScan`, endpoint `GET /api/v1/diagnostics/dhcp`, OpenAPI, documentación y tabla `Servidores DHCP` en `#/diagnostics`.
- Config: `diagnostics.dhcp_probe` (activo por defecto solo en segmentos locales), `dhcp_probe_timeout` (700 ms), `dhcp_history_days` y `authorized_dhcp_servers`.
- `DHCPAvailable()` publica la capacidad `dhcp_probe` en `/api/v1/system/info` y el check `DHCP activo (UDP/68)` en `netdiag doctor`.
- Migración SQLite v1.5.8: `alerts.site_id`. Los hallazgos de seguridad sobre IPs sin inventario (un servidor rogue no suele responder al ping) conservan la sede y aparecen en listados y contadores por sede.

Verificación: `gofmt`, `go vet`, `go test ./...` y `go build ./...`. Pruebas unitarias de construcción/parseo DHCP, detector, store, API, config y de alerta sin host con sede asignada. Prueba real en esta red: `192.168.0.105:68` enlazó con `SO_REUSEADDR`, pero ni el gateway ni el broadcast respondieron en 3 s; el código degradó sin romper el escaneo. Con una observación sintética (`192.168.0.51`, `de:ad:be:ef:00:51`) y una regla `dhcp_rogue` crítica, el escaneo 3 generó la alerta «Servidor DHCP no autorizado» con `site_id=1` y `critical:1` en las estadísticas. Chromium mostró la tabla «Servidores DHCP» y la anomalía. No hay evidencia real de un OFFER rogue en este entorno.

### Señales públicas reales a 18/11/2026

- HN: <https://news.ycombinator.com/item?id=50545732>. 1 punto. Comentario inicial `50545733` marcado `[flagged]`; no cuenta como respuesta pública.
- GitHub: 0 visitas, 0 clones, 0 stars, 0 forks, 0 issues y 0 watchers.
- Descargas: 2 por binario en la release `v1.5.0`. No sabemos si se ejecutaron.
- Reddit: la publicación no ha podido verificarse desde esta máquina (HTTP 403). Falta el enlace exacto y el subreddit.
- Conclusión: **no hay aún evidencia de uso real**. Seguir con F39/F40 y preparar canales alternativos sin instrumentar el binario.

## Evidencia escrita y puerta de gasto F42

Volcar cada señal en [docs/plantillas/registro-evidencia-f42.md](plantillas/registro-evidencia-f42.md). Una fila por organización, con cita textual y enlace público cuando exista.

Antes de contratar seguro, espejo privado, SLA o demo completa:

- [ ] 3 instalaciones reales con 7 días de uso.
- [ ] 2 casos de uso concretos escritos por usuarios.
- [ ] 1 disposición a pagar o carta de intención.

Sin esos tres puntos no se activa estructura comercial. Si faltan, se sigue observando y se revisa el mensaje y el primer escaneo.

**Criterio F42:** existe evidencia escrita de uso real y de disposición a pagar antes de asumir esos costes. Plantillas en [docs/plantillas/README.md](plantillas/README.md).
