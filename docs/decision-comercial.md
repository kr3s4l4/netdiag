# Expediente de decisión comercial — F64 — CERRADO SIN VIGOR

**Cerrado el 20/11/2026.** El propietario decidió retirar la venta: NetDiag es gratuito, sin precios ni checkout. F55, F65 y F66 quedan sin vigor sin firma. El registro se conserva como historial; no es oferta ni modelo activo. Estado vigente: `docs/licenciamiento.md`.

Este expediente deja constancia de lo que ya está decidido y de lo que sigue pendiente. No fija precios, no abre cobro y no modifica `docs/comercializacion.md`. Las cifras de ese documento y del roadmap son **propuestas sin aprobar** mientras F55, F65 y F66 no tengan decisión escrita.

## Estado a la fecha

| Hecho | Estado |
| --- | --- |
| F42: instalaciones reales con 7 días | 0 de 3 |
| F42: casos de uso escritos por usuarios | 0 de 2 |
| F42: disposición a pagar o carta de intención | 0 de 1 |
| F55: precios aprobados | No |
| F65: decisión escrita | No |
| F66: decisión escrita | No |
| Cobro activo | No |
| Tag o release comercial | No |

F42 es el bloqueo previo. Abrir estructura comercial antes de esa evidencia contradice el orden aprobado en `docs/roadmap.md`.

## Métrica de facturación, ya decidida

Un **host gestionado** es la identidad `s<siteID>:<identity>` observada en los últimos **30 días**, excluyendo las etiquetas `guest` y `excluded`.

- La identidad es estable por MAC, con fallback a IP, y está separada por sede.
- El mismo dispositivo en dos sedes cuenta como dos hosts gestionados.
- El descubrimiento y el escaneo son ilimitados.
- El recuento se calcula en local. `CountManagedHostsBySiteSince` devuelve solo `map[siteID]int`, sin identidades.

**Nunca se cobra** por informe, alerta, escaneo, usuario, host detectado, sonda ni sede. Tampoco por generar más evidencia.

## Bandas propuestas, sin aprobar

De `docs/comercializacion.md`:

| SKU | Hosts gestionados | Uso previsto |
| --- | ---: | --- |
| `community` | Descubrimiento ilimitado | 1 sede, retención 7 días |
| `pro-25` | 25 | 3 sedes, retención 1 mes |
| `pro-100` | 100 | 10 sedes, retención 1 mes |
| `pro-500` | 500 | Sedes ilimitadas, retención 1 año |
| `enterprise-500` | 500 | Sedes ilimitadas, retención 7 años |
| `enterprise-2000` | 2.000 | Sedes ilimitadas, retención 7 años |
| `enterprise-custom` | Ilimitados | Según contrato |

De F55, alternativa todavía no reconciliada: `solo-5`, `solo-10` y `pro-50`. Los importes de ambas tablas están sin aprobar y no deben publicarse ni enviarse a clientes.

La evidencia de uso voluntaria de F62 reporta bandas agregadas `1-5`, `6-10`, `11-50`, `51-200` y `201+`. Son solo contadores de señal; no son las bandas de precio.

## Criterio de piloto

Antes de activar cobro:

1. Tres instalaciones reales con al menos siete días de uso cada una.
2. Dos casos de uso concretos escritos por sus usuarios.
3. Una disposición a pagar o carta de intención.
4. Piloto de 30 días con tres a cinco organizaciones, sin SLA y con una llamada semanal de 20 minutos.
5. Comprobación de que el recuento de hosts gestionados coincide con lo que el cliente entiende por «gestionado».

La señal se recoge con plantillas públicas y formularios voluntarios. Nunca con instrumentación del binario ni envío automático.

## Decisiones pendientes de firma

Cumplimentar a mano, en local. No pegar credenciales ni datos personales en el repositorio.

### F55 — Precios

- Decisión: `[vacío]`
- Bandas definitivas: `[vacío]`
- Importes aprobados: `[vacío]`
- Fecha: `[vacío]`
- Firmado por: `[vacío]`

### F65 — Decisión sin descripción pública en el roadmap

- Alcance: `[vacío]`
- Decisión: `[vacío]`
- Fecha: `[vacío]`
- Firmado por: `[vacío]`

### F66 — Decisión sin descripción pública en el roadmap

- Alcance: `[vacío]`
- Decisión: `[vacío]`
- Fecha: `[vacío]`
- Firmado por: `[vacío]`

## Restricciones que no cambian con la firma

- Licencia AGPL-3.0 del código publicado. Cualquier cambio de modelo exige decisión escrita explícita.
- Sin telemetría, sin nube de NetDiag y sin licencia online obligatoria.
- La licencia se verifica en local; una clave emitida por tercero no puede exigir validación remota.
- Sin envíos automáticos a ENISA, CSIRT ni gestores de pago.

## Checklist de activación

- [ ] F42 cerrado con evidencia escrita.
- [ ] F55, F65 y F66 firmados con fecha y responsable.
- [ ] Tabla de precios reconciliada y publicada de forma deliberada.
- [ ] Facturación, EULA y DPA revisados.
- [ ] Responsable CRA y credenciales SRP reales disponibles.
- [ ] Release correctiva firmada y verificada desde checkout limpio.

Mientras quede alguna casilla sin marcar, este documento es solo un borrador.
