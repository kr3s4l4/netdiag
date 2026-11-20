# Guía de ejecución de F42 — validación sin telemetría — HISTÓRICA, SIN VIGOR

**Aviso (20/11/2026):** el propietario retiró la venta. Hoy no hay seguro, espejo, SLA ni estructura comercial que activar. El umbral vigente es **3 instalaciones reales con 7 días** y **2 casos de uso concretos**. Este documento se conserva como registro.

Objetivo original: reunir evidencia escrita de uso real antes de invertir más tiempo.

Umbral original: **3 instalaciones reales con 7 días**, **2 casos de uso concretos** y **1 disposición a pagar**, este último retirado el 20/11/2026. Sin telemetría. Nada de datos de red, credenciales ni identificadores de host.

Tiempo estimado: 1 h de preparación + 7 días de espera + 1 h de cierre.

## 0. Preparación local

```bash
cd /run/media/kr3s4l4/importante/Github_Proyectos/proyectos/scaner_Red/kr3s4l4-Netdiag
mkdir -p /tmp/opencode/f42
cp docs/plantillas/registro-evidencia-f42.md /tmp/opencode/f42/registro.md
cp docs/plantillas/encuesta-5-preguntas.md /tmp/opencode/f42/encuesta.md
cp docs/plantillas/correo-piloto.md /tmp/opencode/f42/correo.md
printf 'organizacion,fecha,fuente,instalacion_real,dias7,caso_uso,cita,enlace,validado_por\n' > /tmp/opencode/f42/seguimiento.csv
```

Elige 3 candidatos reales. Personas que conozcas, un laboratorio y una pequeña organización funcionan mejor que el software libre en general. No envíes correos en frío.

## 1. Invitación

Abre `/tmp/opencode/f42/correo.md`. Rellena `[nombre]`, `[organización]`, `[contacto]` y `[AAAA-MM-DD]`. No prometas SLA ni coste. La plantilla ya lo dice.

Envíalo tú desde tu correo. Guarda el correo enviado como evidencia de la fecha.

## 2. Instalación guiada, sin telemetría

Pega este bloque a cada participante. No pide permisos especiales y verifica la firma antes de ejecutar.

```bash
BASE=https://github.com/kr3s4l4/netdiag/releases/download/v1.5.1
ARCH=$(dpkg --print-architecture)
BIN="netdiag-linux-${ARCH}"
mkdir -p ~/netdiag && cd ~/netdiag
curl -fsSLO "$BASE/$BIN" "$BASE/SHA256SUMS" "$BASE/SHA256SUMS.minisig"
minisign -V \
  -P 'RWTJixPYyPcyWf0VEHdzn6lRYxgrPS3z+EbRBKlI9E+hX0lj1227yrZY' \
  -m SHA256SUMS -x SHA256SUMS.minisig
sha256sum -c SHA256SUMS --ignore-missing
chmod +x "$BIN"
./"$BIN" serve --data-dir ./data
```

Después abre `http://127.0.0.1:8080` o la dirección que indique el arranque. Si no tiene `minisign`, instálalo con `sudo apt install -y minisign`.

Anota en `seguimiento.csv` la fecha real de instalación. La cuenta de los 7 días empieza ahí.

## 3. Día 1: primera respuesta

Pega el texto de `/tmp/opencode/f42/encuesta.md`. Es voluntario y no pide datos de red. Copia las respuestas **palabra por palabra** en `registro.md`. No las resumas: el valor está en la frase concreta.

Si alguien contesta en una Issue o Discussion, guarda también la URL.

## 4. Día 7: comprobación de uso real

Pregunta solo esto:

```text
¿Sigue instalado NetDiag? ¿Qué ha hecho con él esta semana? ¿Qué le ha fallado?
```

Para que cuente como instalación real con 7 días, necesitas las dos cosas:

- confirmación de que sigue instalado;
- una acción concreta, no «lo probé un rato».

Añade una fila por persona en `registro.md` con: organización, fecha, fuente, instalación real SÍ/NO, 7 días SÍ/NO, caso de uso, cita textual, enlace y tu nombre como validador.

## 5. Disposición a pagar

Pregunta directa, con el precio público encima de la mesa:

```text
Si existiera una edición Pro con soporte y garantías a 990 €/año para hasta
25 hosts gestionados, ¿la contratarías? ¿Con qué condición?
```

No descuentes, no prometas SLA y no cobres nada todavía. Cualquiera de estas respuestas cuenta:

- acepta un piloto de pago;
- firma una carta de intención;
- responde «sí» y explica la condición.

Guarda el correo o la carta y pega la frase exacta en la sección «Disposición a pagar». Un «me interesa» sin condición no basta.

## 6. Cierre de la puerta

Antes de gastar dinero, comprueba en `registro.md`:

- [ ] 3 instalaciones reales con 7 días.
- [ ] 2 casos de uso explicados por usuarios.
- [ ] 1 disposición a pagar documentada.

Mientras falte algo, siguen prohibidos: seguro caro, espejo privado, SLA, demo completa y telemetría «para medir mejor».

Revisa el registro cada dos semanas. Si a las 6–8 semanas solo hay descargas y silencio, no escribas más código de producto: revisa el onboarding y el mensaje. Las descargas, estrellas y comentarios son señales de interés; no cuentan para el umbral.

Cuando se cumpla, escribe la fecha y tu nombre al final de `registro.md` y actualiza el estado de F42 en `docs/roadmap.md`.
