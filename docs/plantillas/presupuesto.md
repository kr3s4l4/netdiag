# Presupuesto anual — NetDiag

**Para:** `[organización]` — NIF `[...]`  
**De:** `[razón social del fabricante]`  
**Fecha:** `[AAAA-MM-DD]` — **Válido hasta:** `[AAAA-MM-DD]`  
**Referencia:** `[PRE-AAAA-NNN]`

## Propuesta

| Concepto | Valor |
| --- | --- |
| SKU | `[pro-25 / pro-100 / pro-500 / enterprise-500 / enterprise-2000 / enterprise-custom]` |
| Hosts gestionados incluidos | `[número]` |
| Sedes incluidas | `[número]` |
| Retención de datos | `[1 mes / 1 año / 7 años / según contrato]` |
| Plazo | 12 meses desde `[AAAA-MM-DD]` |
| Precio anual sin IVA | `[importe] €` |
| IVA | `[importe] €` |
| Total | `[importe] €` |

Precios de lista en `docs/comercializacion.md`. No se aplican descuentos permanentes no documentados.

## Qué se factura

Se factura por **host gestionado**: `s<siteID>:<identity>` observado en los últimos 30 días. Se excluyen etiquetas `guest` y `excluded`.

No se factura por:

- Informes.
- Alertas.
- Escaneos.
- Usuarios.
- Hosts detectados una sola vez.

Superar la banda avisa. No factura ni borra datos. El cambio se evalúa en la renovación.

## Soporte

- [ ] **Pro:** correo, objetivo 48 h hábiles, sin SLA contractual.
- [ ] **Enterprise:** SLA P1 respuesta 1 h / workaround 8 h / objetivo de solución 24 h; P2 4 h hábiles / 2 días; P3 1 día hábil / 5 días.

**Cobertura real (marcar una):**

- [ ] Existe cobertura 24 h: responsable alterno o retainer de seguridad. Se aplica el SLA P1 de 1 h.
- [ ] No existe cobertura 24 h: se ofrece P1 con **respuesta objetivo de 8 h hábiles**. No se promete 1 h.

Soporte de seguridad incluido: **la versión contratada durante la suscripción y 12 meses más**.

## Entrega

Tras orden de compra y factura anual:

1. Licencia offline firmada.
2. Factura anual.
3. Clave pública de release y huella verificable.
4. SBOM CycloneDX, atestación de procedencia y notas firmadas.
5. DPA/EULA y matriz SLA.
6. Pasos de verificación de `docs/publicacion-segura.md`.
7. Alta en soporte y responsable CRA contactable.

## Condiciones

- Pago anual por adelantado. Transferencia o SEPA.
- Sin renovación automática sin confirmación por escrito.
- La licencia se activa offline. El producto no envía telemetría.
- Si termina la suscripción, los datos siguen disponibles y el producto funciona como Community.
- CRA: aviso 24 h, notificación 72 h e informe final según `SECURITY.md`.

Aceptado por `[cliente]` — Fecha `[AAAA-MM-DD]`
