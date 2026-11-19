package security

// Draft templates use positional verbs in this order: incident, T0, deadline
// 24 h, deadline 72 h, version, SBOM sha256, component count, provenance
// sha256, manifest sha256. No wall-clock value is baked into the text.
const draft24 = `# Aviso CRA 24 h — BORRADOR DE ENSAYO — NO ENVIADO

**NO enviar a ENISA, CSIRT, clientes ni terceros. No crea obligación legal.**
**F41 real sigue bloqueado:** faltan credenciales SRP y release correctiva.

- ID del incidente: %s
- T0 (conocimiento, simulado): %s
- Aviso temprano antes de: %s
- Notificación 72 h antes de: %s
- Producto: NetDiag %s
- SBOM firmado: sha256 %s
- Componentes SBOM únicos (nombre@versión): %d
- Procedencia firmada: sha256 %s
- Manifiesto firmado: sha256 %s

## Contenido preparado

Presentamos un aviso temprano conforme al Reglamento UE 2024/2847 (CRA),
generado solo como ensayo local. La vulnerabilidad, la explotación activa y el
impacto son hipotéticos. No se espera a tener parche.

- Vulnerabilidad: [CVE ficticia o pendiente]
- Dependencia afectada: [nombre y versión del SBOM]
- ¿Explotación activa conocida?: [SÍ / NO / en investigación]
- ¿Incidente grave?: [SÍ / NO / en investigación]
- Impacto inicial estimado: [una o dos frases]
- Medidas inmediatas: [aislar, retirar versión, avisar a clientes]
- Versiones que se recomienda no usar: [...]

## Comprobación interna

- [ ] Se ha usado reloj UTC.
- [ ] El responsable único o su suplente aprueba el envío real.
- [ ] Existe cuenta SRP y responsable identificado (hoy: NO).
- [ ] Existe release correctiva firmada (hoy: NO).
- [ ] Este fichero NO se ha enviado a ningún destinatario.
`

const draft72 = `# Notificación CRA 72 h — BORRADOR DE ENSAYO — NO ENVIADO

**NO enviar a ENISA, CSIRT, clientes ni terceros. No crea obligación legal.**
**F41 real sigue bloqueado:** faltan credenciales SRP y release correctiva.

- ID del incidente: %s
- T0 (conocimiento, simulado): %s
- Referencia al aviso 24 h: el mismo ID, preparado antes de %s
- Notificación completa antes de: %s
- Producto: NetDiag %s
- SBOM firmado: sha256 %s
- Componentes SBOM únicos (nombre@versión): %d
- Procedencia firmada: sha256 %s
- Manifiesto firmado: sha256 %s

## Evaluación inicial simulada

- Categoría: [vulnerabilidad de producto / dependencia de terceros / incidente]
- Descripción técnica: [párrafo breve, sin detalles explotables]
- CVSS estimado: [puntuación y vector]
- ¿Hay explotación activa?: [SÍ / NO / en investigación]
- Consecuencias: [confidencialidad / integridad / disponibilidad]
- Usuarios afectados: [en determinación]

## Medidas

- Contención: [...]
- Mitigación temporal recomendada al cliente: [...]
- Parche previsto: [versión correctiva], fecha objetivo [AAAA-MM-DD]
- Versión segura mínima (min_version): [versión]
- Comunicación a clientes de pago: [prevista]
- Notificación en UI y REVOCATION.md: [prevista]

## Informe final

Se enviaría 14 días tras el parche si hay explotación activa, o 1 mes desde
esta notificación si es un incidente grave sin explotación confirmada.

## Comprobación interna

- [ ] Se ha usado reloj UTC.
- [ ] El responsable único o su suplente aprueba el envío real.
- [ ] Existe cuenta SRP y responsable identificado (hoy: NO).
- [ ] Existe release correctiva firmada (hoy: NO).
- [ ] Este fichero NO se ha enviado a ningún destinatario.
`
