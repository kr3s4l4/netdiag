# Registro de evidencia — F42

Solo fuentes públicas y respuestas voluntarias. Nunca datos sacados del binario.

## Fuentes permitidas

- Descargas de GitHub Release.
- Estrellas y forks.
- Issues y Discussions.
- Encuesta voluntaria de 5 preguntas.
- Llamadas y escritos del piloto.

## Cola de manifiestos recibidos

Hoja de trabajo del [plan de seguimiento a 7 días](../plan-seguimiento-f42.md). El correo completo se queda en el buzón; aquí solo iniciales o alias.

| Recibido (UTC) | Canal | Contacto | Despliegue | Instalado | Fecha D+7 | Sigue D+7 | Caso de uso | Próxima acción | Estado |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `[AAAA-MM-DD HH:MM]` | `[correo / Discussion / HN / Megathread]` | `[alias]` | `[tipo de red, sin IPs]` | `[SÍ / NO]` | `[AAAA-MM-DD]` | `[SÍ / NO / PENDIENTE]` | `[frase concreta]` | `[qué y cuándo]` | `[ABIERTO / EVIDENCIA / CERRADO]` |

Solo `Sigue D+7 = SÍ` mueve la fila a «Evidencia de uso real». Sin caso concreto, no hay segunda casilla.

## Evidencia de uso real

| Organización | Fecha | Fuente | ¿Instalación real? | ¿7 días de uso? | Caso de uso concreto | Cita textual | Enlace | Validado por |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `[nombre]` | `[AAAA-MM-DD]` | `[Issue / Discussion / encuesta / piloto]` | `[SÍ / NO]` | `[SÍ / NO]` | `[frase concreta]` | `"[...]"` | `[URL]` | `[nombre]` |

## Señales de interés (no cuentan para el umbral)

| Fecha | Fuente | Cita textual | Acción derivada | Enlace |
| --- | --- | --- | --- | --- |
| 2026-11-19 | HN, comentario de `bradknowles` | «I didn't see any obvious way to switch the language to English» | F48: `README.en.md` y texto de respuesta | <https://news.ycombinator.com/item?id=50545732> |

## Umbral de salida

- [ ] **3** instalaciones reales con **7 días** de uso.
- [ ] **2** casos de uso concretos explicados por usuarios.


## Puerta de gasto

Hasta cumplir los dos puntos anteriores:

- [ ] No contratar seguro caro.
- [ ] No contratar espejo privado.
- [ ] No prometer ni implementar SLA.
- [ ] No construir la demo completa.
- [ ] No añadir telemetría “para medir mejor”.

Revisar el registro cada dos semanas. Si tras 6–8 semanas solo hay descargas y silencio, revisar onboarding y mensaje antes de escribir más código.

Cierre de F42 por `[nombre]` — `[AAAA-MM-DD]`
