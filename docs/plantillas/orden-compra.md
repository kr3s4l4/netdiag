# Orden de compra — NetDiag

**Número de OC:** `[OC-AAAA-NNN]`  
**Fecha:** `[AAAA-MM-DD]`  
**Presupuesto asociado:** `[PRE-AAAA-NNN]`

## Comprador

`[organización]` — NIF `[...]`  
Dirección `[...]`  
Contacto de facturación `[nombre, correo]`  
Contacto técnico `[nombre, correo, teléfono]`

## Proveedor

`[razón social]` — NIF `[...]`  
Dirección `[...]`  
Correo de soporte `[...]`

## Línea de compra

| SKU | Hosts | Sedes | Retención | Periodo | Precio sin IVA | IVA | Total |
| --- | ---: | ---: | --- | --- | ---: | ---: | ---: |
| `[SKU]` | `[n]` | `[n]` | `[...]` | `[AAAA-MM-DD] a [AAAA-MM-DD]` | `[€]` | `[€]` | `[€]` |

## Condiciones contratadas

1. Factura anual por adelantado. Pago por transferencia o SEPA en `[n]` días.
2. Licencia offline firmada. Sin servidor de licencias online. Sin telemetría.
3. Métrica: host gestionado = `s<siteID>:<identity>` en últimos 30 días, excluidas etiquetas `guest`/`excluded`. Sin cargo por informes, alertas, escaneos, usuarios ni hosts detectados.
4. Soporte contratado: `[Pro: correo, 48 h hábiles, sin SLA | Enterprise: P1 1 h / 8 h / 24 h; P2 4 h hábiles / 2 días; P3 1 día hábil / 5 días]`.
5. Cobertura P1 declarada: `[1 h con cobertura 24 h | 8 h hábiles sin cobertura 24 h, según presupuesto]`.
6. **Soporte de seguridad de la versión contratada durante la suscripción y 12 meses adicionales.**
7. Se aplica el EULA adjunto. El comprador declara que está autorizado a analizar las redes indicadas. Queda prohibido el escaneo de terceros sin autorización.
8. CRA: el proveedor mantiene responsable único, cuenta ENISA SRP y avisos de 24 h / 72 h según `SECURITY.md`.
9. Entrega: licencia, factura, SBOM firmado, documentos legales y pasos de verificación antes del despliegue.
10. No hay renovación automática sin confirmación escrita.

## Aceptación

Se acepta la OC cuando la licencia verificada, la factura y el paquete de confianza estén recibidos.

Firmado por `[comprador]` — Fecha `[AAAA-MM-DD]`  
Firmado por `[proveedor]` — Fecha `[AAAA-MM-DD]`
