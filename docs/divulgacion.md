# Divulgación de v1.5.0

Textos listos para publicar. El tono es deliberadamente sobrio: proyecto de una persona, en validación, sin promesas grandes. Eso genera más confianza que cualquier eslogan.

**Regla:** publícalos tú desde tu cuenta. Yo no tengo sesión en ningún foro y no debo hablar en tu nombre.

## Mensaje central

> NetDiag es un escáner de red self-hosted en un único binario Go, con interfaz web embebida, SQLite local y cero telemetría. Descubre, inventaría, dibuja topología, diagnostica con evidencia, alerta e informa. Sin agentes, sin nube, sin npm.

En inglés:

> NetDiag is a self-hosted network scanner in a single Go binary, with an embedded web UI, local SQLite and zero telemetry. It discovers, inventories, maps topology, diagnoses with evidence, alerts and reports. No agents, no cloud, no npm.

Enlaces:
- Repo: <https://github.com/kr3s4l4/netdiag>
- Release: <https://github.com/kr3s4l4/netdiag/releases/tag/v1.5.0>
- Clave pública: <https://github.com/kr3s4l4/netdiag/blob/master/docs/public_keys.md>

## Cómo enviarlo (paso a paso)

### Hacker News

1. Verifica el correo de la cuenta.
2. Abre <https://news.ycombinator.com/submit>.
3. **Title:** el título de la sección Show HN de abajo, tal cual, empezando por `Show HN:`.
4. **URL:** `https://github.com/kr3s4l4/netdiag`
5. **Text:** pega aquí el cuerpo de la sección Show HN de abajo. HN sí admite texto junto a la URL y lo muestra arriba del hilo; el mensaje del formulario lo confirma (*"If there is a url, text is optional"*).
6. Envía. Si dejaste el campo Text vacío, añade el cuerpo después como primer comentario.
7. Quédate respondiendo durante las primeras 2 h. Si el post tarda en aparecer, es moderación inicial; espera.

### Reddit

1. Verifica el correo y lee la barra lateral de `r/selfhosted` (reglas y flairs).
2. Abre <https://www.reddit.com/r/selfhosted/submit/>.
3. **Título:** el de la sección r/selfhosted de abajo.
4. Si te sale elegir tipo, usa **Link** con `https://github.com/kr3s4l4/netdiag`; si no, usa **Text** y pega el cuerpo directamente.
5. Elige el flair que indique proyecto si existe (por ejemplo `Project` o `Self-Hosted`). Si no sabes cuál, déjalo vacío antes que elegir uno mal.
6. Publica y, si usaste Link, añade el cuerpo como primer comentario.
7. Responde las primeras preguntas tú mismo, con datos y sin insistir.

**No publiques en los dos sitios el mismo día.** Show HN el día 0, Reddit el día 1.

## Show HN (inglés)

Título:

```text
Show HN: NetDiag – self-hosted network inventory and diagnostics, one Go binary, no telemetry
```

En HN se envía **título + URL**; este texto va en el campo **Text** del mismo formulario (o como primer comentario si lo dejaste vacío).

Cuerpo:

```text
Hi HN,

I'm kr3s4l4, a solo developer. I built NetDiag because I wanted to know what was actually on my network without installing agents, running a cloud service, or sending data anywhere.

It's a single Go binary with an embedded web UI (Spanish by default, English available) and embedded SQLite. It does active/passive discovery, SNMP, L2/L3 topology, heuristic diagnostics with evidence and confidence, alerts, PDF/CSV/JSON reports, multi-site, audit snapshots, RBAC, and signed updates with rollback. No telemetry, no cloud, no npm dependencies.

v1.5.0 is a validation preview. The SHA256SUMS file is signed with minisign and the public key is in the repo; there's no official update mirror yet.

Repo: https://github.com/kr3s4l4/netdiag
Release: https://github.com/kr3s4l4/netdiag/releases/tag/v1.5.0

I'm looking for real use cases, not stars. If you try it, tell me three things: where you deployed it, what you tried first, and what was missing. There's a discussion and a "Caso de uso real" issue template for that.

Happy to answer questions about the design tradeoffs, especially doing this without agents or cloud.
```

## r/selfhosted (inglés)

Título:

```text
NetDiag – self-hosted network inventory and diagnostics in one Go binary (no agents, no telemetry)
```

Cuerpo:

```text
I'm the author, a solo dev, and this is a validation preview rather than a finished product.

NetDiag runs on one machine on your LAN and tells you what is connected: ARP/ICMP discovery, TCP sweep, DNS/banner/SNMP/LLDP enrichment, L2/L3 topology, heuristic diagnostics (STP loops, VLAN leaks, duplex mismatches, rogue APs) with evidence and confidence, alerts, reports and multi-site. Everything lives in a local SQLite file. One Go binary, embedded UI, no cgo, no external services, no telemetry.

I deliberately avoided agents, a cloud backend and npm. The tradeoff is that visibility is limited to what one host can observe; the docs are explicit about those limitations.

Linux amd64/arm64 binaries and SHA256SUMS (minisign-signed) are in the release:
https://github.com/kr3s4l4/netdiag/releases/tag/v1.5.0

If you give it a spin, I'd love to know where you deployed it, what you tried first and what was missing. Feedback decides what I build next.
```

## Español: Mastodon, LinkedIn, grupo local

Versión larga:

```text
Llevo tiempo construyendo NetDiag y hoy lo he publicado como preview de validación.

Es un escáner de red autocontendido: un único binario Go, interfaz web embebida en español, SQLite local, sin telemetría y sin servicios en la nube. Descubre dispositivos, inventaría puertos y servicios, dibuja topología L2/L3, señala problemas con evidencia y confianza, alerta y genera informes. También permite varias sedes en una sola base y guarda líneas base para comparar cambios.

No cobro por informes, alertas ni hosts detectados. La edición Community es AGPL-3.0 y el modelo comercial, cuando llegue, será por banda de hosts gestionados.

Ahora mismo no busco ventas: busco casos de uso reales. Si lo pruebas, dime dónde lo desplegaste, qué intentaste hacer y qué te faltó. El enlace está en el primer comentario / en mi perfil.

https://github.com/kr3s4l4/netdiag
```

Versión corta (para X o un hilo):

```text
He publicado NetDiag 1.5.0: escáner de red self-hosted, un binario Go, SQLite local, cero telemetría. Inventario, topología, diagnósticos con evidencia, informes y auditoría. Preview de validación: quiero casos de uso reales, no estrellas. https://github.com/kr3s4l4/netdiag
```

## Respuestas rápidas

| Pregunta | Respuesta honesta |
| --- | --- |
| ¿Telemetría? | Ninguna. El binario no llama a ningún servicio. Puedes leer el código. |
| ¿Por qué no Docker? | Prioricé binario único y sin cgo. Hay unidad systemd e instalador. Si hay demanda, se añade. |
| ¿Necesita agentes? | No. Observa desde un host: ARP/ICMP, banners, SNMP y vecinos. No instala nada en los equipos. |
| ¿Licencia? | Community es AGPL-3.0. Habrá licencia comercial para soporte y garantías. No se cobra por informes ni por hosts detectados. |
| ¿Es «enterprise-ready»? | Tiene RBAC, auditoría, multi-sede y actualizaciones firmadas. Pero no vendo SLA de 1 h ni guardia 24/7: aún estoy validando y soy una persona sola. |
| ¿Cómo sé que el binario es tuyo? | `SHA256SUMS` está firmado con minisign; la clave pública y el comando de verificación están en `docs/public_keys.md`. |
| ¿Windows o macOS? | Ahora mismo solo Linux amd64/arm64. Dime si lo necesitas y lo valoro según demanda. |
| ¿Puedo escanear cualquier red? | Solo redes que administres o tengas autorización para analizar. El producto no tiene modo oculto ni captura credenciales. |
| ¿Es seguro escanear? | Perfil conservador por defecto, concurrencia acotada y exclusiones. Aun así, usa ventana de mantenimiento en equipos antiguos. |

## Respuestas listas en inglés

No hace falta escribir bien: escribe **frases cortas y correctas**. Un inglés sencillo y honesto genera más confianza que un texto elaborado con traductor. Si te preguntan algo que no sabes responder, pega aquí el comentario y te preparo la respuesta.

Frase de apertura si quieres avisar:

```text
Thanks for looking at it. My English is basic, so I'll keep it short. Happy to answer anything.
```

| Si te preguntan | Respuesta para copiar |
| --- | --- |
| Telemetry? | `No telemetry at all. The binary makes no outbound calls. You can read the code, or run it with no network access and see for yourself.` |
| Agents? | `No agents. It observes from one host: ARP/ICMP, banners, SNMP and LLDP/CDP neighbours. Nothing is installed on the monitored devices.` |
| Why a single binary? | `I wanted something you can copy to a machine and run. One Go binary, embedded UI and SQLite, no cgo, no npm, no external services.` |
| What are the limits? | `The tradeoff is visibility: it only sees what one host can observe. The docs say so explicitly. Diagnostics are heuristic and always carry evidence, confidence and limitations.` |
| License / pricing? | `Community is AGPL-3.0. I don't charge per report, alert or discovered host. If I sell anything later, it will be a commercial licence for support and guarantees.` |
| Is it enterprise-ready? | `It has RBAC, audit and multi-site, but I'm not selling an SLA yet. I'm one person and this is a validation preview. I'd rather say that than promise something I can't keep.` |
| How do I trust the binary? | `SHA256SUMS is signed with minisign. The public key and a one-line verification command are in docs/public_keys.md.` |
| Docker? | `No Docker image yet. I prioritised a single binary and a systemd unit. If people ask for it, I'll build it.` |
| Windows/macOS? | `Linux amd64 and arm64 for now. Tell me if you need another platform and I'll consider it.` |
| Why not just use nmap? | `It isn't replacing nmap. It keeps history, topology and evidence over time, so you can see what changed and why.` |
| Do you use it in production? | `I use it in my own lab. This is a preview: I'm looking for real deployments before calling it finished.` |
| They give you feedback | `Thanks, that's exactly the kind of detail I need. Could you tell me where you deployed it and what was missing?` |

Reglas para tus respuestas:

- una idea por frase;
- si no entiendes algo, pregunta: `Could you rephrase that? My English is limited.`
- no traduzcas párrafos largos: resume;
- si la pregunta es hostil, responde con un dato o no respondas;
- y recuerda que puedes pegarme aquí cualquier comentario y te devuelvo el texto en inglés listo para copiar.

## Qué NO decir

- No «production-ready» ni «enterprise-grade» sin matiz.
- No prometer SLA de 1 h ni soporte 24/7.
- No «gratis para siempre»: Community es AGPL y Pro será comercial.
- No pedir votos, estrellas ni upvotes. Pide historias de uso.
- No publicar el mismo texto en cinco sitios el mismo día. Espacia los mensajes y adapta el tono.
- No responder a un comentario hostil con argumentos de marketing. Responde con datos o no respondas.

## Plan de las primeras 48 horas

1. **Día 0:** Show HN por la mañana (EU). Quédate cerca y responde cada comentario en menos de 2 h.
2. **Día 1:** r/selfhosted. Si un mod te pide el título con `[Self-Hosted]`, cámbialo sin discutir.
3. **Día 2:** Mastodon/LinkedIn y un grupo de redes o ciberseguridad en español.
4. **Durante:** mira descargas, comentarios con caso concreto, issues con la plantilla y respuestas en el Discussion. Ignora las estrellas como métrica.
5. **Si alguien pregunta por precio:** di que aún no hay checkout, que estás validando y que el precio de lista está publicado en `docs/comercializacion.md`. No improvises descuentos.

El objetivo de esta semana no es vender. Es conseguir tres frases como «me sirvió para encontrar el switch que causaba los cortes». Con eso se decide qué se construye después.
