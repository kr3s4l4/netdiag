# Validación de mercado sin telemetría

Objetivo: comprobar si NetDiag resuelve un problema real antes de invertir en licencias, SLA, espejo privado, demo completa o estructura comercial.

**Regla de oro:** las descargas son curiosidad; la aceptación es **uso repetido en una red real**; la viabilidad comercial es **alguien dispuesto a pagar o a firmar un piloto**.

No se añade telemetría al binario. La promesa de “sin telemetría” es parte del producto y no se negocia para medir vanidad.

## Antes de publicar: que no parezca un script

La dirección ya es visible: el README, el [roadmap](roadmap.md) y los documentos de licenciamiento, publicación segura y confianza empresarial. Lo que falta es **prueba visual**. Un visitante decide en segundos si esto es un producto con rumbo o un experimento suelto.

Esto es presentación, **no código**. No hace falta implementar F27–F40 para hacerlo:

- [ ] 3–5 capturas reales de la interfaz: inventario, topología, diagnósticos y ajustes. Usa datos de tu laboratorio; no montes un entorno ficticio.
- [ ] Un GIF corto (10–20 s) del primer valor: añadir red → escanear → ver hosts identificados.
- [ ] Firma discreta `kr3s4l4` en barra lateral, pie de acceso e informes PDF, sin sustituir el nombre del producto (ya implementada).
- [ ] El bloque «Hacia dónde va NetDiag» visible cerca del inicio del README, con enlace al roadmap.
- [ ] En la nota de Release: el mismo bloque de dirección y las tres preguntas del Discussion fijado.
- [ ] **El repo tiene `remote` configurado y los commits están pusheados.** Sin esto, todo el trabajo sigue solo en local y nadie puede encontrarlo.
- [ ] Repo llamado **`netdiag`**, sin sufijo: el usuario ya lo hace único (`kr3s4l4/netdiag`) y coincide con el path del módulo en `go.mod`. Renombrarlo obligaría a cambiar 195 referencias en 97 archivos y rompería `go install github.com/kr3s4l4/netdiag@latest`.
- [ ] Descripción: «NetDiag — escáner de red self-hosted, binario único, sin telemetría, por kr3s4l4». `topics`: `go`, `network-scanner`, `network-monitoring`, `self-hosted`, `snmp`, `network-inventory`, `ciberseguridad`, `offline`, `no-telemetry`.
- [ ] `SECURITY.md` y plantilla **Caso de uso real** en `.github/ISSUE_TEMPLATE/`; enlace desde `kr3s4l4.github.io` y desde el README de perfil de GitHub.
- [ ] Texto de la Release marcado como **preview de validación**, no como release oficial firmada.

Sin esas capturas, la gente lee 487 líneas de README y aun así se pregunta si hay interfaz. Con ellas, entiende de un vistazo que es un producto self-hosted serio.

## Qué medir

| Señal | Cómo se mide sin telemetría | Umbral orientativo |
| --- | --- | --- |
| Descargas | Contador de assets de GitHub Release | 50 descargas en 4–6 semanas |
| Interés cualificado | Issues y Discussions con caso de uso real, no solo “no arranca” | 5 personas contando dónde y para qué lo usan |
| Uso repetido | Encuesta voluntaria: “¿sigue instalado tras 7 días?” | 3 respuestas afirmativas |
| Valor percibido | Pregunta abierta: “¿qué ha descubierto NetDiag que no veía?” | 2 respuestas concretas, no genéricas |
| Profundidad | Solicitudes de systemd, TLS, backup, multi-sede o informes | 2 peticiones de despliegue serio |
| Disposición a pagar | Pregunta directa en encuesta o llamada | 1 persona acepte un piloto de pago |

Los umbrales son orientativos. Sirven para decidir el siguiente paso, no para declarar éxito.

## Fases

### 1. Observar

Publicar el tag y la documentación. No hace falta espejo oficial ni SLA.

- Usar contadores de GitHub Releases.
- Fijar un Discussion: “¿Para qué estás usando NetDiag?”.
- Añadir una plantilla de issue “Caso de uso real”.
- Si se adjuntan binarios, que estén claramente marcados como **preview** o publicar solo fuente con instrucciones de build. No mezclar “beta de validación” con “release oficial firmado”; para lo segundo hay que generar primero la clave real de `docs/public_keys.md`.

### 2. Escuchar

Formulario voluntario de 5 preguntas:

1. ¿Dónde lo has desplegado: casa, laboratorio, oficina, planta o datacenter?
2. ¿Qué fue lo primero que intentaste hacer?
3. ¿Qué fue lo que más te sirvió?
4. ¿Qué te faltó o te confundió?
5. ¿Lo seguirás usando? ¿Te interesaría una prueba de 30 días con soporte?

Leer las respuestas una por una. No promediar: una frase como “me sirvió para encontrar el switch que causaba los cortes” vale más que veinte estrellas.

### 3. Piloto

Con 3–5 personas u organizaciones:

- Duración: 30 días.
- Alcance: una sede real, una red, un escaneo `common`.
- Sin SLA. Soporte por correo con respuesta honesta.
- Compromiso del piloto: una llamada de 20 min por semana y un escrito corto al final.
- Entregar: checklist de `docs/confianza-empresarial.md` adaptado a “evaluación rápida”.

Si el piloto funciona, pedir permiso para usar el nombre o el testimonio. Si no funciona, preguntar qué falló antes de añadir funcionalidades.

### 4. Pago

Solo después de piloto:

- Preguntar por un piloto de pago reducido o por la suscripción `pro-25`.
- No inventar descuentos permanentes; el precio de lista ya está en `docs/comercializacion.md`.
- Si alguien acepta pagar, ahí sí activar lo mínimo: alta fiscal, factura, EULA, clave de release y runbook CRA.

## Cuándo avanzar

Avanzar a F35/F37/F38 cuando:

- Hay al menos 3 instalaciones reales con 7 días de uso.
- Hay 2 casos de uso concretos explicados por usuarios.
- Hay 1 persona u organización dispuesta a piloto de pago o carta de intención.

Mientras tanto, no bloquearse esperando perfección. El producto ya es funcional; lo que falta es evidencia.

## Cuándo no avanzar

Si tras 6–8 semanas solo hay descargas y silencio:

- No contratar espejo privado ni seguro caro todavía.
- No implementar SLA ni modo demo completo.
- Revisar onboarding, README y primer escaneo: probablemente el usuario no llega al “primer valor”.
- Considerar que el problema puede ser distribución o mensaje, no código.

## Qué NO hacer durante la validación

- No meter telemetría encubierta.
- No prometer SLA 1 h.
- No vender Enterprise antes de tener cobertura.
- No cobrar por informes ni por hosts detectados.
- No construir SaaS, multi-tenant ni plugins.
- No confundir “me gusta el proyecto” con “lo pondré en mi red”.
