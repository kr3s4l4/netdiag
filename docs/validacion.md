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

## Material de apoyo para la validación

Estas mejoras facilitan una prueba real, pero **no cuentan como evidencia F42**: la aceptación sigue siendo uso repetido en una red real y disposición a pagar o piloto.

- **Explicación sin IA (F50, F56):** cada hallazgo muestra causa, impacto, pasos siguientes y límites con plantillas deterministas. Las hipótesis están ordenadas por confianza y nunca superan la evidencia. Sirve para que un piloto entienda qué comprobar, no para presumir una causa.
- **Resumen del histórico (F57):** `narrative` convierte los cambios estructurados en frases cortas visibles en la UI y el PDF ejecutivo. Útil para explicar al piloto qué cambió entre dos ventanas sin redactar a mano.
- **Aviso TLS (F60):** `tls_expiring` avisa a 30 y 14 días y marca crítico el último día solo con la fecha del certificado ofrecido. Es una señal concreta que un piloto puede validar en su red.
- **Evidencia firmada (F58):** `scripts/sign-evidence.sh` empaqueta informes, diffs y bundles de sede en `.ndc` con una clave distinta de la de release. Procedimiento en [evidencia-firmada.md](evidencia-firmada.md).
- **Escaneo remoto (F51):** [escaneo-remoto.md](escaneo-remoto.md) describe cómo orquestar escaneos por SSH sin agente, sin nube y sin telemetría.

Ninguna de estas funciones envía datos a NetDiag ni a terceros. La encuesta y el registro de evidencia siguen siendo los únicos canales de recogida voluntaria.

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
| Captura pasiva y `AnomalyDetector` (ICMP redirect, GARP, IP→MAC, MAC→muchas IPs, TTL anómalo) | Parcial (F43 desde escaneos; F47 añade GARP, ICMP redirect, DHCP, SYN y DNS con `-tags pcap`; sin TTL) |
| Detección de MITM por cambio de MAC del gateway y duplicados IP/MAC | Sí (F43: histórico entre escaneos; F47 añade GARP y ARP gratuito capturado cuando hay pcap) |
| DHCP rogue (OFFERs, sonda DISCOVER, regla `!IsGateway && >1 servidor`) | Parcial (F44 sonda UDP/68 + F47 captura de OFFER/ACK; sin respuesta real en esta red) |
| Escaneos entrantes, DNS tunneling y protocolos en claro | Parcial (F47: SYN y dominios DNS de alta entropía; metadata-only, sin payload) |
| Contexto DHCP y reducción de falsos positivos | Sí (F44 autorizados y ARP; F47 reutiliza la lista y el gateway del histórico) |
| Reglas de alerta `anomaly` y `dhcp_rogue` | Sí (F45 dedicadas con precedencia; F44/F46/F47 emiten sus propias familias) |
| SNMP profundo: sweep de communities, IP-MIB, Q-BRIDGE, correlación ARP↔FDB↔LLDP | Parcial (F46 verificado con filas sintéticas; sin switch real) |
| Escaneo de puertos y banners | Sí (parcial: no distingue `closed`/`filtered`) |
| WiFi y VPN | Sí (en el original solo eran roadmap) |

Nota: el «DNS spoofing» histórico era un contador de consultas UDP/53, no validación de respuestas. Prioridad: ~~anomalías ARP/MITM~~ (F43) → ~~alertas de seguridad dedicadas~~ (F45) → ~~DHCP rogue~~ (F44) → ~~SNMP profundo~~ (F46) → ~~captura pasiva~~ (F47).

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

### F46 — SNMP profundo y correlación L2 (19/11/2026)

- `snmp.Config.Deep` y `diagnostics.snmp_deep` (activo por defecto) habilitan walks adicionales solo cuando el dispositivo responde SNMP. `Collect` conserva la comunidad que respondió en forma enmascarada (`public` → `pu***ic`); la credencial en claro no se guarda.
- Walks nuevos: `ipAddrTable` `.1.3.6.1.2.1.4.20`, ARP `ipNetToMediaTable` `.4.22` e `ipNetToPhysicalTable` `.4.35`, Q-BRIDGE `.1.3.6.1.2.1.17.7.*` (VLAN y FDB) y BRIDGE-MIB `.1.3.6.1.2.1.17.4.*`. Límites: 512 direcciones, 2000 filas ARP, 5000 FDB y 256 VLAN. Los decodificadores son funciones puras con tests.
- Migración SQLite v1.5.9: `snmp_ip_addresses`, `snmp_arp_rows`, `snmp_vlans` y `snmp_fdb_rows`, con índices por sede, IP/MAC y fecha. Retención `diagnostics.snmp_history_days` (30 días).
- `DetectSNMP` emite `snmp_gateway_arp_conflict` (90, crítico), `snmp_arp_conflict` (85), `snmp_fdb_mac_moved` (88), `snmp_fdb_port_conflict` (82), `snmp_fdb_vlan_conflict` (85), `snmp_lldp_fdb_disagree` (88) y `snmp_vlan_unknown` (60), todos con `category: "snmp"` para alimentar la regla `snmp_anomaly`.
- Hook en `runScan`, endpoint `GET /api/v1/diagnostics/snmp`, OpenAPI, etiquetas i18n y sección «SNMP profundo» en `#/diagnostics` con comunidades, direcciones, ARP del switch, VLAN y FDB. La tarjeta SNMP del host muestra la comunidad enmascarada.

Verificación: `gofmt`, `go vet`, `go test ./...` y `go build ./...`. Pruebas unitarias de parseo de OID, enmascaramiento, detector (ARP del switch, FDB, LLDP), store (guardado/lectura/purga), config y endpoint. Sin dispositivo SNMP real en esta red: la validación funcional se hace con filas sintéticas inyectadas en la base de prueba; el comportamiento frente a un switch real queda pendiente.

**Smoke con binario real (`127.0.0.1:18780`).** Migración v1.5.9 aplicada (`user_version=15`, cuatro tablas `snmp_*`). Tras inyectar dos lotes sintéticos en sede 1, `GET /api/v1/diagnostics/snmp?site_id=1` devolvió 2 direcciones IP, 2 filas ARP, 2 VLAN, 3 FDB, comunidad `pu***ic` y cuatro hallazgos: `snmp_gateway_arp_conflict` (crítico, 90), `snmp_fdb_mac_moved` (88), `snmp_fdb_vlan_conflict` (85) y `snmp_fdb_port_conflict` (82). Con la regla `snmp_anomaly` (id 9) creada, un escaneo real generó cuatro alertas críticas con `site_id=1`; un segundo escaneo no añadió duplicados. Chromium headless confirmó en el DOM «SNMP profundo», «Comunidades usadas», «Direcciones IP (IP-MIB)», «ARP del switch (IP-MIB)», «VLAN (Q-BRIDGE)», «FDB (Q-BRIDGE / BRIDGE)» y la tarjeta de host con «Comunidad» y `pu***ic`. La credencial en claro no aparece ni en SQLite ni en las respuestas de la API.

### F47 — Captura pasiva opcional y metadata-only (19/11/2026)

- `internal/capture` con build-tags `pcap`/`!pcap`: `Available()`, `Capture()` y un `DefaultFilter` estrecho. En builds portables `Capture` devuelve `ErrUnavailable` y el escaneo continúa sin abrir raw sockets. `netdiag doctor` muestra «Captura pasiva (pcap)» con el estado del binario.
- Opt-in con `diagnostics.passive_capture: false` por defecto. Interfaz, ventana (10–300 s), límite de paquetes (100–100000), modo promiscuo y retención (7 días) configurables por YAML o `NETDIAG_PASSIVE_*`. El hook tras `runScan` solo actúa si hay binario `pcap`, interfaz y opt-in.
- Detectores puros sobre `gopacket`: GARP repetido, ICMP redirect, DHCP OFFER/ACK no autorizado (confían `authorized_dhcp_servers` y el gateway del histórico ARP), SYN entrantes a 10 o más puertos y DNS de alta entropía o volumen. Cinco tipos `passive_*` con `category: "capture"` para la regla `passive_anomaly`. Ningún hallazgo afirma un ataque sin validación humana.
- Migración SQLite v1.5.10: `passive_observations` con índice por sede/tipo y por fecha. Solo se guardan protocolo, IP/MAC, interfaz, detalle acotado a 253 caracteres y marca de tiempo: ni payload, ni credenciales, ni respuestas DNS.
- API `GET /api/v1/diagnostics/passive` con `available`, `enabled`, `observations`, `anomalies` y `limitations`; OpenAPI, sección «Captura pasiva» en `#/diagnostics`, etiquetas ES/EN y tarjeta de estado en la vista de diagnósticos.

Verificación: `gofmt`, `go vet`, `go test ./...`, `go build ./...`, `go build -tags pcap ./...` y `go test -tags pcap ./...`. Pruebas unitarias de los cinco parsers, normalización acotada de la configuración, detector (cinco tipos, servidor DHCP autorizado y gateway confiable), store (guardado por sede, purga y límite de detalle) y endpoint. Sin `CAP_NET_RAW` en este entorno (`CapEff: 0000000000000000`, `ip4:icmp` → `operation not permitted`), la captura real queda pendiente de un laboratorio con privilegios; aquí solo se verifica compilación, degradación limpia y el recorrido completo con observaciones sintéticas.

**Smoke con binario real (`127.0.0.1:18780`).** Migración v1.5.10 aplicada (`user_version=16`, tabla `passive_observations`). Con `diagnostics.passive_capture: true` en un binario portable, `GET /api/v1/diagnostics/passive?site_id=1` devolvió `available:false`, `enabled:true`, 0 observaciones, 0 hallazgos y 8 limitaciones. Tras inyectar 35 observaciones sintéticas en sede 1, el endpoint devolvió 35 observaciones y 6 hallazgos `capture`: los cinco tipos `passive_*` (`passive_dhcp_server` 88, `passive_icmp_redirect` 85, `passive_dns_tunnel` 82 y 80, `passive_inbound_scan` 80 y `passive_gratuitous_arp` 75). El DHCP OFFER del gateway no generó aviso. Con la regla `passive_anomaly` (id 10) creada, un escaneo real generó seis alertas críticas con `site_id=1`; un segundo escaneo no añadió duplicados. Chromium headless confirmó en el DOM «Captura pasiva», «No compilada (usa -tags pcap)», «Solo metadatos», la MAC `de:ad:be:ef:00:51`, `SYN:20` y los cinco títulos de hallazgo. No se persistió payload ni credencial alguna.

### F48 — Documentación en inglés (19/11/2026)

Nace del primer comentario público real en HN: el repo estaba en español y no había una forma evidente de leerlo en inglés. La interfaz web ya era bilingüe, pero la documentación no. Se añade `README.en.md` con visión general, inicio rápido, multi-sede, diagnósticos de seguridad, integraciones y licencias; `README.md` enlaza la versión inglesa en la primera línea. `docs/respuesta-hn.md` guarda el texto listo para publicar en el hilo. Sin telemetría ni cambio de binario.

### Validación real en laboratorio Docker (19/11/2026)

Cierre de la parte pendiente de F44, F46 y F47: hasta ahora había degradación limpia, filas sintéticas y compilación con `pcap`, pero faltaba evidencia contra servicios y paquetes reales. Se levantó un bridge Docker aislado `172.20.50.0/24` con `dnsmasq` rogue (`172.20.50.53`), un agente SNMP v2c propio (`172.20.50.161`) y un generador Scapy (`172.20.50.77`). NetDiag se compiló con `-tags pcap` y corrió en `172.20.50.10` con `CAP_NET_RAW` y `CAP_NET_ADMIN`. Todo el procedimiento y los archivos están en `scripts/lab/`.

- **F44 real.** `dnsmasq` respondió `DHCPINFORM` desde `172.20.50.53` con MAC `de:ad:be:ef:00:53`, gateway `172.20.50.1` y máscara `255.255.255.0`. La sonda activa UDP/68 guardó la observación sin raw sockets y emitió `dhcp_unauthorized_server` (85). Con la regla `dhcp_rogue` creada se generó la alerta crítica «Servidor DHCP no autorizado» con `site_id=2`. Al cambiar el equipo entre escaneos apareció también `dhcp_server_mac_changed`.
- **F46 real.** El agente SNMP sirvió 52 OIDs de system, interfaces, `ipAddrTable`, ARP, VLAN y FDB. La API devolvió la comunidad como `pu***ic`, 1 dirección IP, 5 filas ARP, 2 VLAN y 3 FDB. En el primer poll aparecieron `snmp_fdb_vlan_conflict` (85) y `snmp_fdb_port_conflict` (82). Tras cambiar el estado L2 entre escaneos aparecieron `snmp_fdb_mac_moved` (88) y `snmp_gateway_arp_conflict` (90), con tres alertas críticas de la regla `snmp_anomaly`. La cadena `public` no aparece como credencial en SQLite ni en las respuestas de la API: las únicas coincidencias en el WAL corresponden a `"allow_public":false` de las opciones de escaneo.
- **F47 real.** `netdiag doctor` dentro del contenedor confirmó «Captura pasiva (pcap) compilado; requiere CAP_NET_RAW» y «privilegios root». El tráfico generado produjo los cinco tipos de observación (`garp`, `icmp_redirect`, `dhcp_server`, `inbound_scan`, `dns_tunnel`) y los cinco hallazgos `passive_*`: `passive_gratuitous_arp` (75), `passive_icmp_redirect` (85), `passive_dhcp_server` (88), `passive_inbound_scan` (80) y `passive_dns_tunnel` (82). En el escaneo final se registraron 1.103 observaciones y los cinco hallazgos. No se persiste payload: el detalle DNS guardado es solo el nombre de dominio, con longitud acotada.
- **Reglas y dedupe.** Con las cuatro reglas dedicadas creadas antes del escaneo, un barrido limpio generó 7 alertas críticas: `passive_anomaly` 1, `snmp_anomaly` 3, `dhcp_rogue` 2 y `arp_spoofing` 1. Un cuarto escaneo sin cambios no añadió ninguna alerta. Las reglas dedicadas tienen precedencia sobre `anomaly`; si la regla genérica creó la alerta antes, la dedupe por hallazgo impide que la dedicada la vuelva a emitir.

Conclusión: F44, F46 y F47 pasan de «validado con datos sintéticos» a «validado con servicios y tráfico reales». Quedan pendientes el ensayo humano del runbook CRA (F41), la evidencia de uso externo (F42), la repetición con SPAN/tap en un switch físico, el DHCP controlado fuera del laboratorio y SNMP contra un switch real. El bridge Docker cubre los tres mecanismos con servicios reales, pero no sustituye al equipo físico. El detalle está en `scripts/lab/README.md`.

### Release `v1.5.2` publicada (19/11/2026)

`v1.5.2` está publicada y verificada. Tag `v1.5.2` = `1038c6d`; commit de build `4fb0968`. El push a `master` llevó `996affc..1038c6d`. La GitHub Release tiene 14 assets, sin borrador ni prerelease.

Verificación local:

```bash
scripts/verify-release.sh --dir dist/release/1.5.2 --version 1.5.2   # VÁLIDO
minisign -Vm SHA256SUMS                                             # firma válida
minisign -Vm manifest.json                                          # firma válida
```

```text
SHA256SUMS  4c8c1513a10abab794143be627048359115dcea6986945fb28e0e7cc31b90a39
amd64       516ccd5310b3d0b9c73347bbc8ed9846a2e28872130925584399e20ee21d1626
arm64       1e8a25ac24ba06bc0cd28e9edc582ec749f0b649cc9f00a5157544cd0cab1214
manifest    bdb27c7c92ae8aeef962ba053268aa546a80191bc5f1321028c7138d53a7d417
```

Clave pública release: `RWTJixPYyPcyWf0VEHdzn6lRYxgrPS3z+EbRBKlI9E+hX0lj1227yrZY`; `key_id=c98b13d8c8f73259`.

Las notas firmadas de `v1.5.2` no se modifican. Su texto «pendiente de publicación» es el snapshot congelado al construir los artefactos.

### Correcciones de UI validadas en DOM real (19/11/2026)

Dos defectos visuales reportados en la SPA embebida quedaron corregidos y comprobados con Chromium headless sobre un binario real y una copia de la base del laboratorio:

- **Trazas.** El aviso «Detección automática activa» usaba un `<svg>` de `icon('network')` sin dimensiones, que el navegador interpretaba como ~300×150 px y desplazaba el formulario fuera de la vista. Se añadió `.notice > svg { width:18px; height:18px; flex:none; }` y se reordenó el DOM para que «Nueva traza» aparezca antes de «Redes vecinas detectadas». Medición en viewport 1280×800: icono 18×18, `#trace-target` en `(263, 259)`, botón «Trazar ruta» en `(765, 363)`, `scrollHeight=800` e `formBeforeNeighbors=true`. El formulario cabe completo sin scroll.
- **Topología.** Las aristas se dibujaban como `<line>` de centro a centro y atravesaban nodos intermedios. Ahora se recortan en el borde de origen y destino según el radio real de cada nodo (14 para la red, 11 para gateway y 6–10 para hosts), y se arquean con una curva cuadrática cuando un tercer nodo cae sobre la recta. El punto de control usa el doble del desplazamiento porque la curva alcanza la mitad en su punto medio. Medición real: 4 `<path>`, 0 `<line>`, 10 nodos, 0 muestras dentro de un nodo y separación mínima de 1,95 px sobre el borde. Se conservan `local`, `traced`, `gateway`, `subnet`, LLDP/CDP/ARP, el guion de `subnet`, los `<title>` y el orden de capas.

Comprobaciones: `node --check` no disponible en este equipo; `go test ./internal/webui ./internal/topology ./internal/api` y `make build` pasan. La validación visual se hizo con CDP de Chromium sin dependencias nuevas.

### Señales públicas reales a 19/11/2026

- HN: <https://news.ycombinator.com/item?id=50545732>. Ahora **2 puntos** y **1 comentario real** de `bradknowles`: «This repo appears to be in Spanish. I didn't see any obvious way to switch the language to English. I don't trust online translators...». Es una queja concreta y accionable, no un caso de uso. El comentario inicial `50545733` sigue marcado `[flagged]` y no cuenta.
- GitHub: 120 visitas (58 únicas) y 28 clones (18 únicos) en la ventana de 14 días de la API, concentrados el 18/11. 0 stars, 0 forks, 0 issues y 0 watchers. Discussion #1 abierto el 18/11: 1 comentario, publicado por el propio autor con la encuesta; **0 respuestas de terceros**. Sigue sin fijar (hay que hacerlo a mano: ⋯ → *Pin discussion*).
- Descargas: 51 descargas de assets de GitHub Release (`v1.5.0`: 8, `v1.5.1`: 16, `v1.5.2`: 27). De ellas, 5 binarios amd64 y 5 arm64. Se alcanza el umbral orientativo de 50 assets, pero en circa 1 día y sin evidencia de ejecución: es curiosidad, no aceptación.
- Reddit: la publicación no ha podido localizarse. Falta el enlace exacto y el subreddit; desde esta máquina da HTTP 403.
- Conclusión: **más tráfico, todavía sin evidencia de uso**. El único comentario de la Discussion es nuestro. No abre la puerta de gasto F42: siguen faltando 3 instalaciones con 7 días, 2 casos reales y 1 disposición a pagar.

## Evidencia escrita y puerta de gasto F42

Volcar cada señal en [docs/plantillas/registro-evidencia-f42.md](plantillas/registro-evidencia-f42.md). Una fila por organización, con cita textual y enlace público cuando exista.

Antes de contratar seguro, espejo privado, SLA o demo completa:

- [ ] 3 instalaciones reales con 7 días de uso.
- [ ] 2 casos de uso concretos escritos por usuarios.
- [ ] 1 disposición a pagar o carta de intención.

Sin esos tres puntos no se activa estructura comercial. Si faltan, se sigue observando y se revisa el mensaje y el primer escaneo.

**Criterio F42:** existe evidencia escrita de uso real y de disposición a pagar antes de asumir esos costes. Plantillas en [docs/plantillas/README.md](plantillas/README.md).

### Topología estable y anclada en DHCP real (19/11/2026)

El encuadre se movía porque `renderTopology()` llamaba sin condición a `layoutTopology()` sobre un grafo nuevo sin coordenadas: la normalización final re-escalaba y re-centraba todos los nodos aunque `state.topoView` se conservara. `host.updated` no redibuja; sí lo hacen `scan.finished`, `host.new` y `host.deleted` a través de `scheduleRefresh()` → `route(true)`.

Cambios aplicados:

- `topoSignature(graph)` resume nodos, `network_id`, rol, gateway, DHCP y aristas. Si la firma no cambia, se reutilizan las coordenadas del render anterior; si cambia o se pulsa **Reordenar**, se recalcula el layout. El botón «Centrar» sigue controlando solo pan/zoom.
- `GraphNode.is_dhcp_server` es un campo aditivo rellenado en `handleTopology` desde `dhcp_observations` de los últimos 30 días (o `diagnostics.dhcp_history_days`). Marca evidencia, no confianza: un DHCP rogue también aparece marcado. El ancla de cada red prioriza DHCP → gateway → red → router → mayor grado.
- El nodo DHCP lleva un anillo punteado y `DHCP` en el `<title>`; el layout coloca los nodos con enlaces hacia otras redes en el hueco más cercano a esa dirección, en lugar de asignarlos por orden de ID.
- Orden determinista de clústers y nodos, y `i18n.js` añade *Rearrange*, *Center*, *Zoom in* y *Zoom out*.

Validación real sobre el laboratorio Docker, binario `-tags pcap` recién reconstruido y Chromium CDP tras recarga forzada:

- `GET /api/v1/topology?site_id=2` devuelve `is_dhcp_server: true` en `172.20.50.53` (`lab-dhcp.netdiag-lab`).
- SVG: 12 nodos, 11 `<path>`, 0 `<line>`, 1 anillo DHCP, 0 cruces entre aristas que no comparten extremo.
- Tras fijar `topoView={x:-137,y:64,k:1.42}` y llamar a `route(true)`: posiciones idénticas, firma idéntica, zoom conservado y 0 cruces.
- Tras mover un nodo a `(9999,9999)` y pulsar «Reordenar»: el nodo vuelve a `(594.0,424.7)` y sigue con 0 cruces.
- `go test ./...` y `make build` pasan. Se prefiere esta comprobación porque `node --check` no está instalado en el equipo.

### Publicación de v1.5.1 (19/11/2026)

Se publicó `master` en `1514af8` y el tag anotado `v1.5.1`. La release <https://github.com/kr3s4l4/netdiag/releases/tag/v1.5.1> contiene 14 artefactos: binarios linux/amd64 y linux/arm64, `SHA256SUMS` y su firma, manifiesto F23, notas, SBOM, procedencia, `.ndc` y clave pública.

La clave privada `netdiag-release.key` sigue fuera del repositorio. Se descifró solo una copia local temporal, se generó material de firma con permisos `0600` y ambos archivos se borraron con `shred` después de publicar.

Verificación real:

- `scripts/release.sh`: 10/10 pasos, firma de `SHA256SUMS` con comentario `version:1.5.1 sha256:e9c634a49cf21244396fbbd3abf8c7a9a0edaa91062ca4bf1a33aecedf9a6ee3`.
- `scripts/verify-release.sh --version 1.5.1`: `VÁLIDO`, sin red y sin ejecutar el binario.
- Descarga remota de `SHA256SUMS` y `SHA256SUMS.minisig` desde la release: `minisign -V` devolvió `Signature and comment signature verified`; `sha256sum SHA256SUMS` coincide con el comentario firmado.
- `docs/public_keys.md` añade la huella concreta y el comando de verificación de esta versión.

GitHub no expone API para fijar Discussions: la Discussion #1 sigue necesitando el paso manual ⋯ → *Pin discussion*.

### F29 y F30: buscador global y centro de notificaciones (19/11/2026)

Se validó con Chromium headless sobre CDP, contra un binario `1.5.1` construido desde `master` y un servicio real en `127.0.0.1:18082` con la base del laboratorio Docker. `go test ./...` y `make build` pasan.

**F29 — buscador global**

- `Ctrl+K` abre la paleta y deja el foco en el campo de búsqueda.
- `172.20` devuelve dispositivos con IP y MAC; por ejemplo `kali · 172.20.50.1 · 76:7b:19:45:ba:64`.
- Con la sede `Laboratorio`, el texto de una alerta real devuelve `Servidor DHCP no autorizado capturado` como resultado de tipo `Alerta`.
- La consulta numérica `27` encuentra la alerta con ID 27, aunque el buscador del servidor no indexa IDs.
- `172.20.50` devuelve la red `LabLAN · 172.20.50.0/24`. Al abrir ese resultado, la URL pasa a `#/hosts?network_id=1` y el filtro queda aplicado (`network_id=1`).
- No se registraron errores de JavaScript durante las pruebas.

**F30 — centro de notificaciones**

- La campana aparece sin contador cuando no hay eventos sin leer.
- `alert.new` incrementa el contador a `1` y abre un panel con el evento y su fecha relativa.
- Tras recargar la página, el contador sigue en `1`: el historial persiste en `localStorage`, por sede.
- Al abrir un evento se navega al objeto relacionado (`#/hosts/2`), se marca como leído y el contador vuelve a 0.
- El panel ofrece «Marcar todo como leído» y «Borrar historial».

Ambas funciones están en `master` y no forman parte de la release `v1.5.1`; requieren `v1.5.2`.

### F28: panel de salud de red (19/11/2026)

Se validó con Chromium headless sobre CDP, contra un binario `1.5.1` construido desde `master` y un servicio real en `127.0.0.1:18083` con la base del laboratorio Docker y la sede `Laboratorio` activa. `go test ./...` y `make build` pasan.

- `/dashboard` muestra la tarjeta «Salud de la red» con puntuación `0–100`, semáforo y etiqueta. En el laboratorio：`63 / 100`, tono ámbar, «Aceptable».
- El desglose muestra las cuatro contribuciones con su peso：`Disponibilidad 40% → 40,0 / 40` (raw `100,0%`), `Latencia 20% → Sin datos` (sin muestras de RTT), `Cambios no esperados 20% → 10,0 / 20` (5 cambios) y `Severidad de hallazgos 20% → 0,0 / 20` (3 críticas y 129 avisos).
- El cálculo se reproducó de forma independiente desde los tres endpoints: `computeHealthScore` devolvió `63`, igual que el número pintado. Los componentes sin muestras se excluyeron y el peso restante se renormalizó.
- Sin muestras en la comparativa a 7 días, la tarjeta muestra «Sin datos de tendencia» en lugar de un número inventado.
- Las tres incidencias principales aparecen ordenadas por severidad y confianza. El primer enlace `#/diagnostics?finding=dhcp%3Adhcp_server_mac_changed%3A172.20.50.53` abrió exactamente el `<details>` con ese `data-finding` (`openDetails` coincide); el panel de diagnósticos tenía 132 hallazgos.
- Con `NetDiagI18n.setLang('en')` la tarjeta pasó a `Network health`, `Fair`, `No trend data`, `Availability 40%`, `Latency 20%`, `Unexpected changes 20%`, `Finding severity 20%` y `3 critical · 129 warnings`.
- No se registraron errores de JavaScript.

F28 está en `master` y no forma parte de la release `v1.5.1`; requiere `v1.5.2`.

### F31: diagnósticos accionables (19/11/2026)

Se validó con Chromium headless sobre CDP, contra un binario `1.5.1` construido desde `master` y un servicio real en `127.0.0.1:18083` con la base del laboratorio Docker y la sede `Laboratorio` activa. `go test ./...` y `make build` pasan.

- El panel de diagnósticos mostró 132 hallazgos con acciones. 130 tenían «Ver host»: los hallazgos del laboratorio no incluían `host_ids`, así que el enlace se resolvió buscando su IP en el inventario (`/hosts?limit=500`). El primero apuntaba a `#/hosts/2`.
- Los 132 hallazgos ofrecieron «Copiar comando». Al pulsarlo se escribió en el portapapeles `netdiag scan 172.20.50.53/32`, verificado interceptando `navigator.clipboard.writeText`.
- Al pulsar «Crear regla» en el hallazgo «El servidor DHCP cambió de MAC», el modal se abrió rellenado con nombre `Desde hallazgo: El servidor DHCP cambió de MAC`, tipo `dhcp_rogue`, severidad `critical`, cooldown `900` y confianza mínima `90`, sin salir de `#/diagnostics`.
- Al guardar, apareció el aviso «Regla creada», el número de reglas pasó de 11 a 13 en dos pruebas y la URL quedó coherente en `#/alerts`. No hubo promesas rechazadas ni errores de JavaScript.

F31 está en `master` y no forma parte de la release `v1.5.1`; requiere `v1.5.2`.

### F32: informe ejecutivo de una página (19/11/2026)

Se validó con Chromium headless sobre CDP y descarga real, contra un binario `1.5.1` construido desde `master` y un servicio en `127.0.0.1:18083` con la base del laboratorio Docker y la sede `Laboratorio` activa. `go test ./...` y `make build` pasan.

- `/reports` muestra el botón «Informe ejecutivo (1 página)». Al pulsarlo se creó `netdiag-executive-20261119-102816.pdf` (8.686 bytes) y apareció el aviso «Informe listo»; no hubo errores ni promesas rechazadas.
- El archivo descargado por la sesión autenticada empieza por `%PDF-1.3` y tiene exactamente **1 página** (`/Type /Page`). Contiene «Resumen del inventario», «Disponibilidad y latencia · últimos 7 días», «Cambios principales», «Hallazgos de mayor severidad», «Página 1 de 1» y «datos agregados sin secretos».
- El pie indica `NetDiag 1.5.1 · edición community · retención 7 días · generado 19/11/2026 10:28`. No aparece el pie personalizado que se pasó al generador en la prueba unitaria.
- En el laboratorio se mostraron 5 dispositivos, 4 en línea, 1 caído, 8 alertas críticas, disponibilidad `100,0 %`, latencia «Sin muestras» y «Basado en 20 muestras de los últimos 7 días». Los cinco cambios son altas de hosts y los cuatro hallazgos son los de mayor severidad y confianza.
- `internal/report/executive_test.go` comprueba que una comunidad SNMP (`public`) y un secreto (`secretpass`) no aparecen en el PDF y se sustituyen por `[redactada]`; también comprueba el pie y la página única.
- Con `NetDiagI18n.setLang('en')` el botón pasó a «Executive report (1 page)».

F32 está en `master` y no forma parte de la release `v1.5.1`; requiere `v1.5.2`.

### F27: asistente de primer arranque (19/11/2026)

Se validó con Chromium headless sobre CDP, contra un binario construido desde `master` y un servicio fresco en `127.0.0.1:18084` con directorio de datos nuevo. `go test ./...` pasa y los tres ficheros JavaScript implicados se analizaron también en Chromium sin errores de sintaxis.

- `POST /auth/setup` entregó la sesión y la SPA mostró el asistente en lugar de entrar directamente al panel. El auto-tour de 900 ms quedó inhibido.
- `/networks/suggest` propuso `192.168.0.105/24`. El asistente reutilizó la red sugerida, lanzó el escaneo con `profile=common` y mostró progreso mientras duró.
- Al terminar el escaneo se creó exactamente un snapshot de auditoría llamado `Línea base inicial`. El panel se abrió con 2 hosts reales en el inventario.
- Al pulsar «Ver panel sin recorrido», apareció la shell y el centro de notificaciones quedó conectado. No se inició el tour automáticamente; el recorrido queda como acción explícita al final del asistente.
- No se registraron errores de JavaScript ni promesas rechazadas.

F27 está en `master` y no forma parte de la release `v1.5.1`; requiere `v1.5.2`.

### Artefactos `1.5.2` y dependencias (19/11/2026)

- `go test ./...` pasa; `gofmt -l` no lista cambios; `git diff --check` limpio.
- `govulncheck` con Go 1.27.3 detectó `GO-2026-6859` en `net/http` (corregida en 1.27.4). Con `GOTOOLCHAIN=go1.27.4` devolvió `No vulnerabilities found`. La toolchain mínima para publicar es 1.27.4.
- `scripts/release.sh` generó 14 artefactos en `dist/release/1.5.2` sin variante `pcap`, con `min_version=1.5.2` y base URL de GitHub. `scripts/verify-release.sh --dir dist/release/1.5.2 --version 1.5.2` devolvió `VÁLIDO`: firmas, hashes, manifiesto F23, contenedor `.ndc`, checksum del binario y versión.
- Dos generaciones con la misma época reproducible produjeron hashes idénticos para binarios, SBOM, procedencia y manifiesto. Solo cambian las notas, el contenedor que las incluye y `SHA256SUMS`.
- La clave oficial `netdiag-release` estaba cifrada con contraseña. La contraseña se introdujo en local y permitió derivar `signing.json` con `key_id=c98b13d8c8f73259`, sin guardar secretos en el repo ni enviarlos a ningún servicio. Los artefactos finales están firmados con esa clave oficial. `minisign -Vm SHA256SUMS` y `minisign -Vm manifest.json` contra `~/.minisign/netdiag-release.pub` devolvieron `Signature and comment signature verified`.
- No queda ningún artefacto con clave de prueba en `dist/release/1.5.2`; las notas ya no llevan el aviso **NO PUBLICABLE**.

### Lote 3: F49, F59 y F68 (19/11/2026)

Validación en `master` sobre un directorio de datos temporal, sin tocar la release `v1.5.2` ni la clave de release. `gofmt`, `go build ./...` y `go test ./...` pasan. No hay `node` en la máquina, así que los cambios JS no tienen `node --check`; la sintaxis se revisó por lectura y los casos de uso se comprueban en el diff.

- **F49:** `netdiag probe --data-dir "$DIR" --site-id 1 --out "$DIR/sonda.json"` terminó sin firma y avisó de `--seed-file`. `netdiag probe import` cargó `ARP=2 ... VPN=7`. Con una semilla local de prueba se generó una sonda firmada; `probe import --key` la aceptó y la misma sonda sin `--key` fue rechazada. Tests `TestSignVerifyRoundTrip` y `TestImportWritesObservations` cubren firma, manipulación y persistencia.
- **F59:** migración `v1.5.11` y `FindingReview` con clave `(site_id, finding_type, host_key)`. El test de API comprueba que `viewer` puede listar pero no revisar ni borrar, que `operator` guarda y elimina, que `status` y `note` se validan, que el listado sin `site_id` incluye todas las sedes y que quedan las entradas `finding_review.upsert` y `finding_review.deleted` con el revisor autenticado.
- **F68:** sin semilla, `netdiag evidence dossier` falló con `evidence.dossier.seed_file no configurada y no se indicó --seed-file`, como se esperaba. Con semilla y clave pública locales generó `dossier.ndc` (4102 bytes) y `container verify` devolvió `Resultado: VÁLIDO`, con `Firma: válida` y `Contenido: correcto`.
- **Config:** nuevos tests de defaults, validación con `enabled=true`, rechazo de permisos distintos de `0600` en la semilla y variables `NETDIAG_EVIDENCE_DOSSIER_*`.
- **Límites comprobados:** la sonda no envía datos; WiFi queda `available:false` en el binario portable; la firma da integridad y procedencia, no confidencialidad; el triaje no cierra alertas y expira a los 30 días; el dossier conserva solo los últimos `keep`.

Estos cambios siguen en `master` y **no son parte de la release `v1.5.2`**. No se han creado tag ni release para este lote.

### Lote 4: F61 — versión única (19/11/2026)

`internal/buildinfo` es la única fuente de `Version`, `Commit` y `BuildDate`. `Makefile` y `scripts/release.sh` inyectan esos valores sobre el paquete `internal/buildinfo`; `cmd/netdiag` solo mantiene alias para no tocar el resto del CLI. Se eliminaron los literales `1.5.1` y `1.5.0` que causaban la versión desfasada en el login.

Comprobaciones:

- `go test ./internal/buildinfo ./cmd/netdiag ./internal/webui` pasa. Los tests comparan el valor por defecto con `VERSION` del Makefile, los alias del CLI y el uso de `st.version` en el footer.
- `make build && ./bin/netdiag --version` devuelve `netdiag 1.5.2-dev (commit ..., built ...)`.
- `bash scripts/check-version-stamp.sh` construye un binario temporal, arranca un servidor con datos temporales y valida `--version`, `/api/v1/auth/status` y el footer del login. El servidor se detiene por PID y el directorio temporal se borra.
- También se reprodujo el flujo de descarga con el artefacto firmado `v1.5.2`: `sha256sum -c SHA256SUMS` y `scripts/verify-release.sh` devuelven `VÁLIDO`, y el login del binario descargado muestra `NetDiag 1.5.2 · kr3s4l4 · servidor local`.

Sin cambios en `v1.5.2`, sus hashes ni su clave. Sin tag ni release.

### Lote 4: F62 — evidencia de uso voluntaria (19/11/2026)

Nuevo subcomando `netdiag evidence usage`. Genera `netdiag.usage/v1` con días desde el primer usuario, escaneos, sedes y bandas de hosts gestionados de los últimos 30 días. Se suma `CountManagedHostsBySiteSince` en el store; devuelve solo `map[siteID]int`, sin identidades. Las bandas son `1-5`, `6-10`, `11-50`, `51-200` y `201+`.

Comprobaciones:

- `go test ./internal/store ./internal/evidence ./cmd/netdiag` pasa. El test firma un contenedor con una clave de prueba, verifica la firma y comprueba que el payload no contiene nombres de sede, IPs ni usuarios.
- Smoke con datos sintéticos: `netdiag demo --seed 11 --sites 2 --hosts 12` y `netdiag evidence usage --seed-file ... --out usage.ndc`. El payload mostró `sites: 2`, `scans: 2` y `6-10: 2`.
- `netdiag container verify --key release.pub usage.ndc` devolvió `Firma: válida`, `Contenido: correcto`, `Resultado: VÁLIDO`.

Sin red, sin envío automático y sin sustituir la evidencia escrita de F42. La clave de prueba es local y no se usa para releases oficiales. Sin tag ni release.

### Aviso legal Beta (19/11/2026)

Descargo «tal cual» y responsabilidad de autorización de escaneo en `README.md` y `README.en.md`. La pantalla de acceso muestra el aviso en español o inglés según el idioma de la UI. El asistente de primer arranque exige marcar una casilla antes de lanzar el primer escaneo y guarda esa marca solo en `localStorage` del navegador. `netdiag serve --accept-terms` o `terms.accepted: true` en la configuración registran la confirmación del operador; sin ellos se imprime un aviso en stderr, pero el servidor arranca igual.

No hay petición de red, ni telemetría, ni cambio de licencia. El aviso no traslada la responsabilidad legal: quien escanea sigue necesitando autorización. F42 sigue con 0/3 instalaciones reales, 0/2 casos y 0/1 disposición a pagar.

### Lote 4: F63 — ensayo local del runbook CRA (19/11/2026)

Nuevo subcomando `netdiag security runbook --dry-run`. Verifica con `release.pub` las firmas detached de `manifest.json`, `sbom.cdx.json` y `provenance.intoto.jsonl`, y escribe cuatro ficheros en el directorio elegido: dos borradores 24/72 h, un `runbook-dryrun.json` y un marcador `NO-ENVIADO.txt`. Sin red, sin correo y sin SRP. El comando se niega a funcionar sin `--dry-run`.

Comprobaciones:

- `go test ./internal/security ./cmd/netdiag` pasa. Los tests usan una clave de prueba, comprueban un SBOM manipulado y exigen salida byte a byte determinista.
- Smoke contra `dist/release/1.5.2` en modo solo lectura: versión `1.5.2`, 46 componentes SBOM únicos, T0 `2026-01-01T00:00:00Z`, plazos `2026-01-02T00:00:00Z` y `2026-01-04T00:00:00Z`.
- Los tres hashes del JSON coinciden con `sha256sum` de los ficheros de la release.
- Dos ejecuciones con `--out` distinto producen árboles idénticos (`diff -r` sin salida).

Sin envíos reales. F41 sigue bloqueado por credenciales SRP y release correctiva; este borrador no crea obligaciones legales ni cierra el ensayo humano de 7 días. Sin tag ni release.

### Lote 4: F64 — expediente de decisión comercial (19/11/2026)

Nuevo `docs/decision-comercial.md`, borrador para firma. Recoge la métrica ya decidida (`s<siteID>:<identity>` en 30 días sin `guest`/`excluded`), las bandas propuestas sin aprobar, la regla que prohíbe cobrar por informe, alerta, escaneo, usuario, host detectado, sonda o sede, y el criterio de piloto de 30 días. F55, F65 y F66 quedan como campos vacíos con fecha y firmante pendientes.

El documento registra F42 en 0/3, 0/2 y 0/1, declara que no hay cobro activo ni release comercial, y deja constancia de que AGPL, funcionamiento offline y ausencia de telemetría no cambian por una firma. No modifica `docs/comercializacion.md` ni publica importes. Sin tag ni release.

Con F64 queda cerrado el Lote 4: F61, F62, F63 y F64 están en `master`. F42, F41 real y las decisiones escritas siguen bloqueadas por motivos ajenos al código.
