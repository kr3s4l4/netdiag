# Registro de incidentes — NetDiag

Un registro por incidente. No borrar. Completa en UTC.

**ID:** `[ID-INC-AAAA-NNN]`  
**Responsable único:** `[nombre]` — **Suplente:** `[nombre]`

## Cronología

| Hora UTC | Hito | Quién | Evidencia |
| --- | --- | --- | --- |
| `[AAAA-MM-DDTHH:MM:SSZ]` | Detección / reporte recibido | `[...]` | `[issue privada, correo, SRP]` |
| `[...]` | Conocimiento por el fabricante | `[...]` | `[...]` |
| `[...]` | Triaje y clasificación | `[...]` | `[...]` |
| `[...]` | Decisión de reportar: SÍ / NO | `[...]` | `[motivo en registro-responsabilidad]` |
| `[...]` | Aviso 24 h enviado | `[...]` | `[justificante SRP]` |
| `[...]` | Notificación 72 h enviada | `[...]` | `[justificante SRP]` |
| `[...]` | Release correctiva firmada | `[...]` | `[versión + SHA256]` |
| `[...]` | Correo a clientes de pago | `[...]` | `[ID del correo]` |
| `[...]` | Notificación en UI / `REVOCATION.md` | `[...]` | `[enlace]` |
| `[...]` | Informe final enviado | `[...]` | `[14 días / 1 mes]` |
| `[...]` | Cierre | `[...]` | `[acta]` |

## Datos del hallazgo

- **Versión/es afectadas:** `[...]`
- **CVE:** `[...]`
- **Dependencia:** `[nombre y versión, referencia SBOM]`
- **Explotación activa:** `[SÍ / NO / en investigación]`
- **Incidente grave:** `[SÍ / NO]`
- **Criterio aplicado:** `[una frase]`

## Árbol de decisión

```text
¿Hay explotación activa O incidente grave?
├─ SÍ → aviso <24 h + notificación <72 h. No esperar al parche.
│       informe final: 14 días tras parche si explotación; 1 mes si incidente grave.
└─ NO → triaje, corregir en release firmada y vigilar.
        Si cambia el criterio, entrar en la rama SÍ de inmediato.
```

## Comunicación

- [ ] Clientes de pago: correo con versión afectada, mitigación y versión segura.
- [ ] UI: notificación `update.available` / entrada en el centro de notificaciones.
- [ ] Air-gapped: `.ndc` firmado con el aviso.
- [ ] Manifiesto F23: `min_version` apunta a la primera versión segura.
- [ ] `REVOCATION.md` firmado si hay compromiso de clave o versión insegura.

## Cierre

- [ ] Causa raíz documentada.
- [ ] Parche verificado desde checkout limpio.
- [ ] Plazos CRA cumplidos o incidencia justificada.
- [ ] Lección aprendida y cambio de proceso asignado a `[responsable, fecha]`.

Cerrado por `[nombre]` — `[AAAA-MM-DDTHH:MM:SSZ]`
