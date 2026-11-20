# Registro de responsabilidad — NetDiag

Finalidad: demostrar quién supo qué, cuándo y por qué se decidió reportar. Aplica también a vulnerabilidades de dependencias de terceros.

**ID:** `[RESP-AAAA-NNN]` — **Incidente relacionado:** `[ID-INC-AAAA-NNN]`

| Campo | Valor |
| --- | --- |
| Fecha de conocimiento | `[AAAA-MM-DDTHH:MM:SSZ]` |
| Canal de entrada | `[Security Advisory / correo / aviso de proveedor / monitor ]` |
| Responsable de registro | `[nombre]` |
| Producto y versión/es afectadas | `[NetDiag x.y.z, ...]` |
| Componente | `[código propio dependencia]` |
| Dependencia y versión | `[nombre, versión, SBOM ref.]` |
| CVE o identificador | `[...]` |
| ¿Explotación activa? | `[SÍ / NO / en investigación]` |
| ¿Incidente grave? | `[SÍ / NO / en investigación]` |

## Decisión de reportar

**Decisión:** `[REPORTAR / NO REPORTAR / PENDIENTE]`  
**Motivo:** `[criterio CRA aplicado, una o dos frases]`  
**Aprobada por:** `[responsable único o suplente]`  
**Hora de decisión:** `[AAAA-MM-DDTHH:MM:SSZ]`

Si la decisión es NO REPORTAR, revisar de nuevo si cambia la explotación o la gravedad. Anotar la revisión.

| Revisión | Hora UTC | Decisión | Motivo |
| --- | --- | --- | --- |
| Inicial | `[...]` | `[...]` | `[...]` |
| Seguimiento | `[...]` | `[...]` | `[...]` |

## Responsabilidad y cadena de custodia

- **Responsable interno del caso:** `[nombre, cargo]`.
- **Responsable de la comunicación CRA:** `[nombre, cargo, cuenta ENISA SRP]`.
- **Responsable de la release correctiva:** `[nombre, cargo]`.
- **Proveedor aguas arriba:** `[nombre, contacto, caso/ID]`. Se reclama después; no sustituye nuestra gestión.
- **Dónde se conservan las pruebas:** `[ruta del gestor de incidencias]`.
- **Binarios terceros:** `[reconstruido y verificado | verificado sin reconstruir | no incluido]`. Nunca se firma un binario de terceros sin reconstruirlo o verificarlo.

## Parche y comunicación

| Hito | Fecha UTC | Referencia |
| --- | --- | --- |
| Parche preparado | `[...]` | `[commit / PR]` |
| Release firmada publicada | `[...]` | `[versión, SHA256]` |
| `min_version` actualizado en manifiesto F23 | `[...]` | `[manifest.json]` |
| Aviso público | `[...]` | `[ID]` |
| Notificación en UI | `[...]` | `[...]` |
| `REVOCATION.md` firmado | `[...]` | `[enlace o N/A]` |
| Informe final CRA | `[...]` | `[14 días / 1 mes]` |

## Cierre

**Fecha de cierre:** `[AAAA-MM-DDTHH:MM:SSZ]`  
**Cerrado por:** `[nombre]`  
**Resumen de responsabilidad asumida:** `[una frase: el fabricante gestionó aviso, parche y comunicación del producto terminado]`

Firmado, `[nombre]` — `[cargo]`
