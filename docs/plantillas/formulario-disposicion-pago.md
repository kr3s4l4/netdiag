# Manifiesto voluntario de interés — F42

**Plantilla desplegada en <https://kr3s4l4.github.io/netdiag/formulario-disposicion-pago.html> (19/11/2026), con autorización explícita.** El fichero
[formulario-disposicion-pago.html](formulario-disposicion-pago.html) es una
página estática: no tiene backend, no hace `fetch`, no usa cookies ni carga
recursos externos. La persona rellena el formulario, genera un JSON
`netdiag.intent/v1` y decide si lo envía por correo.

**Contacto:** kr3s4l4@protonmail.com. Es público en el CV y su propietario ha
autorizado expresamente su publicación en esta plantilla.

## Qué recoge

- Correo de la persona, para poder responderle.
- Organización y país, opcionales.
- Dónde se desplegó NetDiag y si sigue instalado tras 7 días.
- Interés en una suscripción por banda de hosts gestionados, con banda estimada opcional.
- Qué faltó o confundió, texto libre opcional de hasta 1000 caracteres.
- Consentimiento explícito y autorización opcional para citar la respuesta de forma anónima.

**No recoge:** IPs, MACs, nombres de host, nombres de red, credenciales,
identificadores de hosts ni ningún dato de escaneo. No hay telemetría ni
envío automático. El envío lo hace la persona con su propio cliente de correo.

## Datos y consentimiento

- Finalidad única: validar NetDiag y responder a quien escribió.
- Base: consentimiento explícito, marcado en una casilla obligatoria.
- Conservación: solo hasta responder y cerrar el registro de F42. Se borra si la persona lo pide.
- Destinatarios: solo el contacto indicado. Sin gestores de correo, CRM, Merchant of Record ni analítica.
- El manifiesto no es pedido, factura, contrato ni activación de cobro. No publica precios.

## Cómo se contabiliza para F42

Esta página **no cuenta como evidencia** por existir. Cuenta cuando una persona
real envía el manifiesto y se registra en
[registro-evidencia-f42.md](registro-evidencia-f42.md) con:

1. Fecha y hora UTC de recepción.
2. Respuesta a «sigue instalado tras 7 días».
3. Caso de uso concreto, si lo incluyó.
4. Intención de pago, con banda estimada si la dio.

El umbral sigue siendo 3 instalaciones reales con 7 días, 2 casos concretos y
1 disposición a pagar. Hoy está en 0/3, 0/2 y 0/1.

## Publicación

Publicado con OK explícito del propietario el 19/11/2026. GitHub Pages sirve
`master:/docs` en <https://kr3s4l4.github.io/netdiag/>; el formulario está en
`/formulario-disposicion-pago.html`. El enlace se añadió a ambos README y al
Discussion #1. No se ha tocado `docs/respuesta-hn.md` ni se han publicado textos
en Reddit sin su autorización específica.
