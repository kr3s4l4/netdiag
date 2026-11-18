# Paquete de confianza empresarial

Un cliente no compra un escáner: compra la capacidad de defender una decisión ante seguridad, compras y auditoría. Este documento define qué se entrega para que NetDiag sea defendible en ese proceso.

## Qué recibe el comprador

En una sola carpeta, antes de firmar el contrato:

1. `README.md`, `docs/licenciamiento.md` y `docs/comercializacion.md`.
2. `LICENSE`, `NOTICE`, `THIRD_PARTY_LICENSES.md` y `SECURITY.md`.
3. `docs/publicacion-segura.md` + clave pública oficial.
4. SBOM CycloneDX, atestación de procedencia y notas de versión firmadas.
5. Whitepaper de seguridad, threat model y flujo de datos (resumen abajo).
6. Plantillas de DPA/EULA, matriz SLA y checklist de compras.
7. Licencia de prueba de 30 días y acceso al modo demo (F39).

## Whitepaper de seguridad: contenido obligatorio

- **Arquitectura:** binario único Go, SPA embebida, SQLite local, opcional `-tags pcap`.
- **Datos que se recogen:** inventario, puertos, banners, SNMP, vecinos, topología, métricas, alertas, auditoría. No se recogen credenciales de la red monitorizada.
- **Datos que NO se recogen:** telemetría de uso, consultas del usuario, tráfico payload, credenciales de terceros. No hay servicio en la nube obligatorio.
- **Almacenamiento:** SQLite en `<data_dir>`; `secret.key` (0600) cifra secretos de integraciones con AES-256-GCM. El resto de la base **no está cifrada en reposo por defecto**: se recomienda cifrado de disco/LUKS o volumen del sistema.
- **Identidad y acceso:** Argon2id, sesiones opacas en servidor, CSRF, RBAC `admin`/`owner`/`operator`/`viewer`, auditoría de cambios.
- **Exposición:** CSP, `X-Frame-Options: DENY`, `nosniff`, HSTS opcional, `/metrics` con token opcional; recomendación de no publicar sin TLS y proxy inverso.
- **Salidas:** webhooks firmados HMAC e integraciones salientes; guardia SSRF que bloquea rangos privados salvo autorización explícita.
- **Actualizaciones:** manifiesto firmado, SHA256, verificación ELF, instalación atómica y rollback automático (F23).
- **Limitaciones honestas:** diagnósticos heurísticos con evidencia/confianza; el binario portable no captura 802.11; TLS 1.3 limita inspección de aplicaciones; no se hace threat intel externo.

## Modelo de amenazas (resumen)

| Activo | Amenaza | Mitigación | Riesgo residual |
| --- | --- | --- | --- |
| Inventario y topología | Lectura por usuario no autorizado | RBAC, sesiones en servidor, CSRF, CSP | Admin local puede exportar; auditoría lo registra |
| Secretos de integraciones | Exfiltración desde API o disco | AES-256-GCM con `secret.key` 0600, nunca se devuelven | Compromiso total del host |
| Navegador del operador | XSS / clickjacking | SPA sin dependencias, CSP, `DENY`, escape de datos | Extensiones maliciosas del navegador |
| Destinos de alertas | SSRF a metadata/loopback | Guardia SSRF y `allow_private_targets` explícito | Admin puede autorizar un destino interno |
| Binario | Sustitución por versión maliciosa | Manifiesto minisign + SHA256 + ELF + rollback | Clave de release comprometida (mitigado por rotación) |
| Base SQLite | Modificación directa | Permisos del SO, copias automáticas, auditoría | Acceso root al host |
| Datos de red | Falsa sensación de precisión | Evidencia, confianza y `limitations` en cada hallazgo | Requiere validación del operador |

El documento completo debe ampliarse con diagramas de flujo y una sesión de preguntas con el equipo de seguridad del cliente.

## Flujo de datos

```text
Navegador del operador
        | HTTPS / TLS (o proxy inverso)
        v
NetDiag (binario único, self-hosted)
        |--- SQLite local <data_dir>/netdiag.db
        |--- secret.key (cifra secretos de integraciones)
        |---backup ZIP local
        |
        +---(opcional) webhook HMAC / syslog / Slack / Teams / PagerDuty
        +---(opcional) espejo HTTPS de actualizaciones firmadas
        x--- SIN telemetría, SIN nube obligatoria, SIN license server online
```

Si se ofrece el addon hosted, ese servicio se describe como un **subprocesador** aparte con DPA específico; el producto self-hosted no tiene subprocesadores.

## Matriz de soporte y SLA

| Severidad | Definición | Enterprise: respuesta | Enterprise: workaround | Enterprise: objetivo de solución | Pro |
| --- | --- | --- | --- | --- | --- |
| P1 | Producto caído, pérdida de datos o incidente de seguridad | 1 h | 8 h | 24 h | Mejor esfuerzo, 48 h hábiles |
| P2 | Función principal inutilizable sin workaround | 4 h hábiles | 2 días hábiles | 5 días hábiles | Mejor esfuerzo |
| P3 | Error menor, pregunta o mejora | 1 día hábil | — | 5 días hábiles | Foro / correo |
| P4 | Solicitud de información comercial | 2 días hábiles | — | — | Correo |

El SLA se mide desde la confirmación de recepción en el canal de soporte definido en el contrato. Exclusiones: mantenimiento programado avisado, cambios no soportados y fallos de infraestructura del cliente.

## Contratos y cumplimiento

- **EULA:** alcance de licencia por banda de hosts, plazo anual, uso permitido, limitación de responsabilidad y propiedad de datos del cliente.
- **DPA:** aplica solo si se contrata el addon hosted. Para self-hosted se entrega una **declaración de no subprocesamiento**: el cliente es el controlador y NetDiag no recibe datos personales.
- **Política de retención:** 7 días Community, 1 mes en `pro-25`/`pro-100`, 1 año en `pro-500` y 7 años en Enterprise; configurable hacia abajo, nunca hacia arriba sin licencia.
- **Derecho de auditoría:** el cliente recibe SBOM, procedencia y registros de verificación; se ofrece sesión técnica anual.
- **Exportación:** formatos abiertos JSON/CSV/PDF/ZIP; `.ndc` opcional y verificable sin software propietario.

## Prueba de concepto guiada (45 minutos)

1. Arrancar `netdiag demo --seed 1` y recorrer panel, topología, diagnósticos e informe ejecutivo.
2. Cargar licencia de prueba de 30 días con `netdiag license activate`.
3. Configurar una sede real y una red, lanzar escaneo `common`.
4. Crear una línea base F22 y un filtro guardado F40.
5. Probar una alerta saliente al webhook interno del cliente.
6. Verificar el binario con la clave pública y el `.ndc` con `netdiag container verify`.
7. Revisar `SECURITY.md`, SBOM y flujo de datos con el equipo de seguridad.
8. Cerrar con criterios de éxito: host descubierto, evidencia explicada, informe generado, firma verificada.

## Checklist de compras

- [ ] SKU y banda de hosts gestionados confirmados.
- [ ] Número de sedes y retención requeridos por auditoría.
- [ ] Factura anual, NIF y datos de orden de compra.
- [ ] Licencia firmada recibida y verificada con `netdiag license show`.
- [ ] Clave pública de release importada y huella comparada con la documentación.
- [ ] SBOM, notas firmadas y atestación archivados.
- [ ] DPA/EULA firmados o declaración de no subprocesamiento adjunta.
- [ ] Plan de actualizaciones: espejo oficial o interno.
- [ ] Contacto de soporte y severidades acordados.
