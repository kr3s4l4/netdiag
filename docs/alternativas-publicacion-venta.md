# Alternativas para publicar y vender NetDiag

Estado: 18/11/2026. Los commits siguen **en local**. Esto es un plan listo para ejecutar, no una publicación hecha.

## Principio

Separar cuatro cosas que suelen mezclarse:

1. **Descubrimiento**: dónde alguien conoce NetDiag.
2. **Descarga**: dónde obtiene el binario.
3. **Cobro**: quién factura y recibe el dinero.
4. **Licencia**: cómo recibe una licencia offline firmada.

NetDiag no necesita servidor de licencias. La licencia es un JSON firmado con Ed25519 (F35). El binario se verifica con minisign y SBOM (F37). Nada de esto exige telemetría.

**Importante F35:** en el JSON de licencia, `edition` solo admite `community`, `pro` o `enterprise`. La banda comercial (`pro-25`, `pro-100`, etc.) se representa con `max_managed_hosts`, no en `edition`.

## Matriz rápida

| Ruta | Cobro | Descarga | Coste inicial | Cuándo | Riesgo |
| --- | --- | --- | --- | --- | --- |
| A. Tienda directa | Stripe Payment Link o Payhip | Tu web estática o GitHub Release | 0 € | Ahora, primer piloto de pago | Hay que emitir licencia a mano |
| B. Merchant of Record global | Paddle o Gumroad | Enlace del producto | 0 € | Si vendes fuera de UE/UK desde el primer día | Comisión por venta |
| C. Forge alternativo | Igual que A | Codeberg / Forgejo / SourceHut | 0 € | Si quieres redundancia frente a GitHub | Menos visitas que GitHub |
| D. Partners / MSP | Factura del partner | Entrega directa o enlace privado | 0 € | B2B, consultores, integradores | Ciclo de venta más lento |
| E. Marketplaces | Plataforma | Plataforma | 0 € | Solo descubrimiento | No es el canal principal |

**Descartada:** Lemon Squeezy. Ya no acepta altas y cierra el 31/03/2027.

## Ruta A — Tienda directa mínima (recomendada ahora)

No hace falta web propia ni backend. Un enlace de pago y un correo bastan.

### Pasos

1. Elegir cobro:
   - **Payhip**: 0 €/mes + 5% por venta + comisión de Stripe/PayPal. Gestiona VAT digital de UE/UK y sales tax de US/Canadá. No es Merchant of Record global. <https://payhip.com/pricing>
   - **Stripe Payment Link**: sin cuota mensual; PayPal/Stripe aparte. Si necesitas que Stripe sea Merchant of Record, usar **Stripe Managed Payments**. <https://stripe.com/payments/payment-links>
2. Crear producto por banda, no por informe: `NetDiag Pro 25 — 990 €/año`, `NetDiag Pro 100 — 2.490 €/año`, `NetDiag Pro 500 — 5.900 €/año`.
3. Adjuntar al producto o enviar por correo:
   - `netdiag-linux-amd64`, `netdiag-linux-arm64`.
   - `SHA256SUMS`, `SHA256SUMS.minisig`.
   - `RELEASE_NOTES.md`, `manifest.json`.
   - `docs/public_keys.md`, `docs/instalacion.md`, `LICENSE`, `NOTICE`.
4. Generar la licencia **en el emisor, nunca en el servidor**:
   ```bash
   netdiag license sign \
     --seed-file "$HOME/.config/netdiag/license-signing.seed" \
     --out license.json license-unsigned.json
   ```
   Plantilla `license-unsigned.json` para la banda de 25 hosts:
   ```json
   {
     "schema": "netdiag-license/v1",
     "license_id": "ND-2026-0001",
     "customer": "Nombre de la organización",
     "edition": "pro",
     "max_managed_hosts": 25,
     "max_sites": 10,
     "retention_days": 30,
     "expires_at": "2027-11-18T00:00:00Z",
     "capabilities": ["alerts", "reports"],
     "support_level": "business_8x5"
   }
   ```
5. Enviar por correo el binario, la licencia y el método de verificación. **No enviar la semilla de firma**.
6. Cobrar solo por banda de hosts gestionados. Nunca por informe, alerta, escaneo, usuario o host detectado.

### Texto de producto (copiar/pegar)

> NetDiag — inventario y monitorización de red self-hosted.
>
> Un binario. SQLite local. Sin telemetría. Sin servidor de licencias.
>
> La licencia se firma offline y se activa con `netdiag license activate license.json`. El pago cubre hasta 25 hosts gestionados, 10 sedes y 30 días de retención. Soporte por correo en horario laboral.
>
> Verifica antes de ejecutar: `minisign -V -P 'RWTJixPYyPcyWf0VEHdzn6lRYxgrPS3z+EbRBKlI9E+hX0lj1227yrZY' -m SHA256SUMS -x SHA256SUMS.minisig`.

### Correo de entrega (copiar/pegar)

> Asunto: NetDiag — licencia ND-2026-0001

Hola,

Adjunto:

- `netdiag-linux-amd64` y `netdiag-linux-arm64`.
- `SHA256SUMS` y `SHA256SUMS.minisig`.
- `license.json`, firmada para tu organización.
- Guía de instalación y claves públicas.

Verificación sin ejecutar el binario:

```bash
minisign -V -P 'RWTJixPYyPcyWf0VEHdzn6lRYxgrPS3z+EbRBKlI9E+hX0lj1227yrZY' -m SHA256SUMS -x SHA256SUMS.minisig
sha256sum -c SHA256SUMS
```

Activación:

```bash
chmod +x netdiag-linux-amd64
./netdiag-linux-amd64 license activate --data-dir ./netdiag-data license.json
```

La licencia no contacta con ningún servidor. Si caduca o se excede la banda, NetDiag avisa pero no borra datos ni bloquea lecturas.

Soporte: responde a este correo.

Un saludo,
kr3s4l4

## Ruta B — Merchant of Record global

Cuando haya compradores fuera de UE/UK y no quieras gestionar impuestos por país.

| Opción | Tarifa vista 18/11/2026 | Comentario |
| --- | --- | --- |
| Paddle | 5% + 0,50 $ por Checkout; sin cuota mensual | MoR completo; facturación B2B y cumplimiento global. <https://www.paddle.com/pricing> |
| Gumroad | 10% + 0,50 $ venta directa; 30% vía Discover; sin cuota mensual | MoR desde 01/01/2025; útil para descubrimiento, cara para tickets altos. <https://gumroad.com/pricing> |
| Stripe Managed Payments | Comisión de Stripe + recargo de Managed Payments | Revisar condiciones antes de activar. <https://stripe.com/pricing> |

Recomendación: **Paddle** si el comprador es empresa; **Gumroad** solo para venta directa pequeña o descubrimiento. Evitar Discover para tickets de 990 € por el 30%.

Flujo idéntico a la Ruta A: el producto incluye binarios y checksums; la licencia firmada se envía tras el pago.

## Ruta C — Forge alternativo como espejo

Mantener GitHub para descubrimiento y añadir un espejo sin telemetría.

- Codeberg: <https://codeberg.org>. Releases con tag; revisar si el contador de descargas está activo en la instancia.
- Forgejo propio: control total, sin dependencia de terceros; requiere servidor.
- SourceHut: <https://sourcehut.org>. Buena opción para fuente y builds, menos para audiencia general.

Pasos:

1. Crear repo `netdiag` con el mismo README y `docs/public_keys.md`.
2. Publicar el tag `v1.5.1` o el que se decida.
3. Subir los mismos artefactos que en GitHub.
4. Añadir en el README: `Espejo sin telemetría: Codeberg ...`.
5. No hacer push todavía: preparar el repo y el comando, ejecutar solo con autorización.

Comando cuando se autorice (token de alcance mínimo, borrado con `shred` después):

```bash
git remote add codeberg https://codeberg.org/kr3s4l4/netdiag.git
git push codeberg master
git push codeberg v1.5.1
```

## Ruta D — Partners / MSP / consultores

Mejor encaje B2B que un marketplace. El partner ya tiene la relación con el cliente.

- Ofrecer evaluación de 30 días en una sede real.
- Precio de lista intacto; sin descuentos permanentes.
- El partner factura soporte o despliegue; NetDiag factura la licencia.
- No prometer SLA hasta tener cobertura.

Correo corto (copiar/pegar):

> Asunto: NetDiag para vuestros clientes self-hosted

Hola,

NetDiag es un escáner e inventario de red en binario único, SQLite local y sin telemetría. Encaja cuando un cliente quiere ver qué hay en su red sin enviar datos a un SaaS.

Trabajamos por banda de hosts gestionados: 25, 100, 500 o Enterprise. La licencia es offline y firmada; no hay servidor de activación.

Si os interesa, preparo una evaluación de 30 días en una sede real y una llamada de 20 minutos para revisar el resultado.

Un saludo,
kr3s4l4

## Ruta E — Descubrimiento sin instrumentar

Canales ya probados o preparados:

- GitHub Release y Discussion fijado. Queda fijarlo a mano: ⋯ → *Pin discussion*.
- Hacker News: <https://news.ycombinator.com/item?id=50545732>. 1 punto y comentario marcado `[flagged]`; no darlo por válido.
- Reddit: falta el enlace exacto y el subreddit; desde esta máquina da HTTP 403.
- Newsletters y listas de self-hosting / sysadmin: texto corto, sin superlativos.
- Plantillas listas en `docs/divulgacion.md` y `docs/validacion.md`.

Regla: una respuesta real con caso de uso vale más que cien descargas. Registrar cada señal en `docs/plantillas/registro-evidencia-f42.md`.

## Qué no hacer

- No publicar Lemon Squeezy como alternativa: cierra en marzo de 2027.
- No meter telemetría para medir conversión.
- No cobrar por informes, alertas, escaneos, usuarios ni hosts detectados.
- No vender Enterprise ni SLA antes de tener cobertura.
- No confundir `v1.5.0` con release oficial firmada: es pre-release de validación.
- No compartir `license-signing.seed` ni `signing.json` fuera del emisor.
- No hacer push ni subir claves sin instrucción explícita.

## Decisión recomendada

1. **Ahora**: Ruta A con Payhip o Stripe Payment Link. Preparar textos, producto y proceso manual de licencia.
2. **Si aparece comprador internacional**: añadir Paddle.
3. **Si el comprador es empresa**: Ruta D con partner y licencia Enterprise.
4. **Después de F42**: release firmada oficial, espejo en Codeberg/Forgejo y SLA con cobertura real.

Siguiente paso concreto: elegir entre **Payhip**, **Stripe Payment Link** o **Paddle**; yo dejo preparados el producto, el correo y el JSON de licencia para la banda elegida.
