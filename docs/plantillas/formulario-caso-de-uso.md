# Manifiesto de caso de uso — F42

**Plantilla desplegada en <https://kr3s4l4.github.io/netdiag/plantillas/formulario-caso-de-uso.html> (19/11/2026), con autorización explícita. Actualizada el 20/11/2026.** El fichero [formulario-caso-de-uso.html](formulario-caso-de-uso.html) es una página estática: no tiene backend, no hace `fetch`, no usa cookies ni carga recursos externos. La persona rellena el formulario, genera un JSON `netdiag.usecase/v1` y decide si lo envía por correo.

El nombre del fichero se conserva para no romper los enlaces ya publicados.

**Contacto:** kr3s4l4@protonmail.com. Es público en el CV y su propietario ha autorizado expresamente su publicación en esta plantilla.

## Qué recoge

- Correo de la persona, para poder responderle.
- Organización y país, opcionales.
- Dónde se desplegó NetDiag y si sigue instalado tras 7 días.
- Para qué usa NetDiag, texto libre opcional de hasta 1000 caracteres. Es el campo que alimenta los 2 casos concretos de F42.
- Qué faltó o confundió, texto libre opcional de hasta 1000 caracteres.
- Consentimiento explícito y autorización opcional para citar la respuesta de forma anónima.

**Recoge:** el caso de uso y el despliegue, sin datos de la red.

**No recoge:** IPs, MACs, nombres de host, nombres de red, credenciales, identificadores de hosts ni ningún dato de escaneo. No hay telemetría ni envío automático. El envío lo hace la persona con su propio cliente de correo.

## Datos y consentimiento

- Finalidad única: conocer usos reales y responder a quien escribió.
- Base: consentimiento explícito, marcado en una casilla obligatoria.
- Conservación: solo hasta responder y cerrar el registro de F42. Se borra si la persona lo pide.
- Destinatarios: solo el contacto indicado. Sin gestores de correo, CRM, Merchant of Record ni analítica.
- El manifiesto es solo un caso de uso. No es un pedido ni un contrato.

## Cómo se contabiliza para F42

Esta página **no cuenta como evidencia** por existir. Cuenta cuando una persona real envía el manifiesto y se registra en [registro-evidencia-f42.md](registro-evidencia-f42.md) con:

1. Fecha y hora UTC de recepción.
2. Respuesta a «sigue instalado tras 7 días».
3. Caso de uso concreto, del campo `caso_de_uso` del manifiesto si lo rellenó.

El umbral vigente es 3 instalaciones reales con 7 días y 2 casos concretos. Hoy está en 0/3 y 0/2.

## Publicación

Publicado con OK explícito del propietario el 19/11/2026. GitHub Pages sirve `master:/docs` en <https://kr3s4l4.github.io/netdiag/>; el formulario está en `/plantillas/formulario-caso-de-uso.html`. La URL antigua mantiene una redirección al nuevo nombre. No se ha tocado `docs/respuesta-hn.md` ni se han publicado textos en Reddit sin su autorización específica.

El seguimiento de cada manifiesto está en el [plan de seguimiento a 7 días](../plan-seguimiento-f42.md) y en la cola de [registro-evidencia-f42.md](registro-evidencia-f42.md). El recordatorio para el Discussion es un borrador: no se publica sin OK explícito.
