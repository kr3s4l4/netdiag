# Modelo comercial de NetDiag

Este documento es la fuente de verdad de precios, métricas de facturación y flujo de compra. Las decisiones están alineadas con `docs/roadmap.md` (F33–F38).

## Principios

1. **No se cobra por informe, alerta, escaneo, usuario ni host detectado.** Generar más evidencia nunca debe aumentar la factura.
2. **Se cobra por host gestionado**, con bandas anuales cerradas y predecibles.
3. **El producto no se degrada artificialmente.** Community es completo y útil; la edición de pago aporta retención, actualizaciones firmadas automáticas, informes ejecutivos, soporte, SLA y cumplimiento.
4. **Sin teléfono a casa.** La licencia se verifica localmente; el cliente decide si conecta el espejo de actualizaciones.
5. **Sin sobrecostes sorpresa.** Superar una banda avisa, no factura ni borra datos.

## Qué cuenta como host gestionado

Un host gestionado es el identificador único `s<siteID>:<identity>` que ha sido observado en los últimos **30 días**, excluyendo hosts con etiqueta `guest` o `excluded`.

- La identidad es estable por MAC (o fallback IP) y está namespaced por sede.
- Un mismo dispositivo visto en dos sedes cuenta como dos hosts gestionados.
- El descubrimiento y el escaneo son ilimitados: un host detectado una vez no genera cargo.
- El recuento se calcula localmente y se muestra en **Ajustes → Licencia** cuando F35 esté implementado.

## Planes y precios

Precios en EUR, **sin IVA**, facturación anual por adelantado.

| SKU | Precio anual | Hosts gestionados | Sedes | Retención | Actualizaciones | Soporte |
| --- | ---: | ---: | ---: | --- | --- | --- |
| `community` | 0 € | Descubrimiento ilimitado | 1 | 7 días | Manuales, firmadas | Comunidad / GitHub Issues |
| `pro-25` | 990 € | 25 | 3 | 1 mes | Firmadas automáticas | Correo, objetivo 48 h hábiles |
| `pro-100` | 2.490 € | 100 | 10 | 1 mes | Firmadas automáticas | Correo, objetivo 48 h hábiles |
| `pro-500` | 5.900 € | 500 | Ilimitadas | 1 año | Firmadas automáticas | Correo, objetivo 48 h hábiles |
| `enterprise-500` | 9.900 € | 500 | Ilimitadas | 7 años | Firmadas / espejo privado | SLA + gestor dedicado |
| `enterprise-2000` | 19.900 € | 2.000 | Ilimitadas | 7 años | Firmadas / espejo privado | SLA + gestor dedicado |
| `enterprise-custom` | A medida | Ilimitados | Ilimitadas | Según contrato | Firmadas / espejo privado | SLA a medida |

**Coste efectivo orientativo por host/año:** 39,60 € (`pro-25`), 24,90 € (`pro-100`), 11,80 € (`pro-500`). El precio baja al crecer la banda: se premia la previsibilidad, no se penaliza el crecimiento.

Las bandas de retención son **7 días / 1 mes / 1 año**, con 7 años en Enterprise. La retención es un diferenciador del plan, no un producto de duración de acceso: el acceso al producto es siempre la suscripción anual.

### Incluido en todos los planes de pago

- Informe ejecutivo de una página (F32).
- Filtros y vistas guardadas (F40).
- Acceso a artefactos firmados, SBOM y notas de versión verificables.
- Licencia offline firmada; sin dependencia de servidor de licencias.

### Solo Enterprise

- Licencia con retención de 7 años y capacidades `air_gapped`, `private_mirror` y `sla`.
- Paquete de confianza: whitepaper, threat model, flujo de datos, DPA/EULA, matriz SLA y guía de verificación.
- Onboarding guiado y revisión de arquitectura.
- Espejo de actualizaciones alojado por el cliente o entregado como `.ndc` firmado.

### Servicios opcionales

| Servicio | Precio | Cuándo tiene sentido |
| --- | ---: | --- |
| Onboarding asistido | 1.500 € (una vez) | Instalación, endurecimiento, roles y primer informe |
| Espejo privado de actualizaciones | 990 €/año | Flota aislada o política de salida controlada |
| Revisión de seguridad anual | 2.500 €/año | Sector regulado o auditoría interna |

## Prueba y renovación

- **Prueba de 30 días:** licencia `trial` con capacidades Pro. No pide tarjeta. No envía telemetría.
- Al caducar: la interfaz muestra aviso, se mantienen todos los datos y se aplican capacidades Community. No hay bloqueo de lectura.
- **Renovación:** aviso 30 días antes. No hay renovación automática sin confirmación por escrito.
- **Cambio de banda:** se evalúa en la renovación. Si el cliente crece a mitad de año, se le avisa y se le ofrece upgrade prorrateado, pero nunca se le cobra retroactivamente.
- **Downgrade:** disponible en la renovación; la retención superior se conserva solo mientras dure el pago.

## Flujo de compra

1. **Descarga:** el cliente baja Community o solicita licencia de prueba de 30 días.
2. **PoC:** usa el modo demo (F39) o su propia red con el checklist de `docs/confianza-empresarial.md`.
3. **Presupuesto:** se emite un documento con SKU, banda, número de sedes, retención, SLA y alcance de soporte.
4. **Compra:** orden de compra + factura anual. Se aceptan transferencia y SEPA. El IVA se aplica según jurisdicción.
5. **Entrega:** licencia firmada, clave pública de release, SBOM, notas firmadas y acceso a soporte.
6. **Verificación:** el cliente ejecuta los comandos de `docs/publicacion-segura.md` antes de desplegar.
7. **Renovación:** recordatorio 30 días antes; el mismo SKU se renueva con un clic o una orden.

## Qué le decimos al comprador

> NetDiag no cobra por mirar más ni por generar más pruebas. Paga por un sistema predecible que mantiene su inventario y su evidencia durante el tiempo que su auditoría exige, con actualizaciones verificadas, soporte con SLA y documentación que su equipo de seguridad puede revisar. Si deja de pagar, sus datos siguen ahí y el producto sigue funcionando como Community.

## Objeciones frecuentes

| Objeción | Respuesta |
| --- | --- |
| «AGPL nos obliga a publicar cambios» | Solo si distribuye un programa modificado; si necesita mantener cambios privados, compra la licencia comercial. Ver `docs/licenciamiento.md`. |
| «No queremos depender de internet» | La licencia se activa offline y el producto no llama a casa. El espejo de actualizaciones es opcional y puede ser interno. |
| «¿Qué pasa si crecemos?» | Cambias de banda en la renovación. No hay sobrecoste automático ni factura por picos. |
| «¿Por qué pagar si el código es abierto?» | Por retención larga, actualizaciones firmadas operadas por nosotros, SLA, soporte, paquete legal/seguridad y previsibilidad presupuestaria. |
| «¿Necesitamos un agente en cada host?» | No. NetDiag observa la red desde puntos de monitorsaje; el cobro es por hosts gestionados, no por agentes desplegados. |
