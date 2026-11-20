# Plan de seguimiento F42 a 7 días

**Estado:** 0/3 instalaciones con 7 días · 0/2 casos concretos.
**Preparado:** 19/11/2026, 20:27 UTC. **Publicado:** no. Documento de trabajo.

**Umbral vigente:** **0/3 instalaciones con 7 días y 0/2 casos concretos**. El formulario genera `netdiag.usecase/v1` y solo recoge el caso de uso.

## Reglas

- Sin telemetría. No instrumentar el binario.
- Sin servicios externos ni licencia online.
- Sin citas públicas sin permiso explícito de quien escribió.
- El correo completo no se copia al registro. Solo iniciales o alias.
- Cualquier mensaje en foros necesita OK explícito del propietario.

## Calendario

El día de recepción es D0. Si un manifiesto llega otro día, se recalcula desde su fecha.

| Día | Fecha si D0 = 19/11 | Acción | Canal | Salida |
| --- | --- | --- | --- | --- |
| D0 | 19/11 | Leer el manifiesto. Anotar fecha UTC en la hoja. | Correo local | Fila nueva en la hoja |
| D+1 | 20/11 | Acuse corto. Preguntar dónde se desplegó, qué probó y qué faltó. | Correo | 3 preguntas respondidas o pendientes |
| D+3 | 22/11 | Si hay respuesta, pedir el caso concreto si falta. Sin insistir si no hay respuesta. | Correo | `caso_de_uso` completo o en blanco |
| D+7 | 26/11 | Preguntar si sigue instalado. Si dice sí, pasar la fila a «Evidencia de uso real». | Correo + registro | 1/3, 2/3 o 3/3 |
| D+14 | 03/12 | Revisión quincenal. Contar instalaciones, casos e intención. Si no hay señales, revisar mensaje y onboarding. | Registro | Revisión fechada |
| D+28 | 17/12 | Puerta de gasto. Con 3/2/1, decidir por escrito abrir F35/F37/F38. Sin umbral, seguir cerrada. | `docs/roadmap.md` | Decisión escrita |

Responsable de las respuestas: kr3s4l4. El asistente prepara textos y rellena el registro. No envía correos ni publica en foros.

## Hoja de registro

La hoja única es [registro-evidencia-f42.md](plantillas/registro-evidencia-f42.md), sección «Cola de manifiestos recibidos». No crear otra.

Al llegar un manifiesto, copiar a la fila estos campos del JSON `netdiag.usecase/v1`:

1. `recibido_utc`: fecha y hora de recepción en el buzón.
2. `canal`: correo, Discussion, HN o Megathread.
3. `contacto`: iniciales o alias. El correo completo se queda en el buzón.
4. `despliegue` e `instalado`: lo que diga la persona. Nunca deducirlo de una descarga.
5. `fecha_d7`: recepción más siete días.
6. `sigue_d7`: `SÍ`, `NO` o `PENDIENTE`. Solo `SÍ` suma al contador.
7. `caso_de_uso`: frase concreta. Texto libre genérico no cuenta.
8. `proxima_accion`: qué se hará y cuándo.
9. `estado`: `ABIERTO`, `EVIDENCIA` o `CERRADO`.

El formulario recoge solo el caso de uso.

No contabilizar: visitas, descargas, estrellas, forks ni comentarios sin instalación. Esos se anotan en «Señales de interés».

## Pasar de manifiesto a evidencia

1. Llega el manifiesto por correo. Se registra el mismo día.
2. D+1: acuse y tres preguntas sobre el uso real.
3. D+7: confirmar permanencia. Si es `SÍ`, copiar la fila a «Evidencia de uso real» con cita autorizada o anónima.
4. Si hay caso concreto, añadir otra fila de evidencia con la misma fecha y fuente `manifiesto`.
5. Marcar casillas del umbral solo con filas ya cumplimentadas.

## Borrador de recordatorio para el Discussion

**NO PUBLICADO.** Requiere OK explícito del propietario. Publicarlo desde su cuenta, como comentario en Discussion #1.

```text
Actualización breve sobre NetDiag v1.5.2.

Sigo decidiendo el orden de las próximas mejoras con casos reales, no con telemetría. Si has instalado NetDiag, me ayudaría mucho responder tres cosas:

1. ¿Dónde lo desplegaste? Sin IPs ni nombres de equipos.
2. ¿Qué probaste primero?
3. ¿Qué te faltó o te confundió?

Si ya pasaron siete días, dime también si sigue instalado.

Si prefieres dejar constancia con más detalle, está el manifiesto de caso de uso:
https://kr3s4l4.github.io/netdiag/plantillas/formulario-caso-de-uso.html

Es una página estática. Genera el texto en tu navegador y tú decides si lo envías. No recoge datos de tu red. NetDiag es software libre. Gracias.
```

Opcional, solo si alguien escribe en inglés:

```text
If you prefer English: where did you deploy it, what did you try first, and what was missing? No IPs or hostnames needed. Thanks.
```

### Recordatorio a D+7

**NO PUBLICADO.** Dos variantes. La fecha D+7 se calcula desde la recepción de cada manifiesto, no desde la publicación del plan.

Para quien ya respondió, por el mismo canal:

```text
Hola, hace siete días que me contaste que usabas NetDiag. ¿Sigue instalado? Si es así, ¿qué has probado desde entonces y qué te falta todavía? No hace falta dar IPs ni nombres de equipos. Gracias.
```

Para el Discussion, solo si nadie ha respondido después de siete días:

```text
Recordatorio breve: sigo esperando casos reales de NetDiag para decidir el orden de las mejoras. Tres preguntas: ¿dónde lo desplegaste, qué probaste primero y qué te faltó? Sin IPs ni nombres de equipos.

El manifiesto de caso de uso sigue aquí:
https://kr3s4l4.github.io/netdiag/plantillas/formulario-caso-de-uso.html

Es estático, no recoge datos de tu red y NetDiag es gratuito. Gracias.
```

### Antes de publicar

- [ ] Revisar que la URL siga respondiendo con `curl -I`.
- [ ] Sin servicios externos ni promesa de SLA.
- [ ] Sin telemetría ni envío automático.
- [ ] Publicar desde la cuenta del propietario en Discussion #1.
- [ ] Anotar en `docs/validacion.md` la hora UTC y el ID del comentario.
- [ ] No contar el recordatorio como evidencia: solo cuenta la respuesta de una persona real.

## Puerta de gasto

Sin los tres umbrales cumplidos: no seguro caro, no espejo privado, no SLA, no demo completa y no telemetría «para medir mejor». Revisar cada dos semanas.
