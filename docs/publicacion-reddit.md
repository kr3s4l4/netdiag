# Publicación en Reddit — v1.5.2

**Estado: pendiente. No publicado.** Esto es un borrador para copiar y pegar desde tu cuenta. Yo no tengo sesión en Reddit y no debo hablar en tu nombre. La publicación sigue sujeta a tu OK explícito (F48).

## Antes de publicar

- Hacker News fue el día 0 (18/11). Reddit corresponde al día 1 (19/11). No publiques los dos el mismo día.
- Subreddit recomendado: `r/selfhosted`. Alternativa, solo si el primero no encaja: `r/homelab`. No cruces el mismo texto en varios sitios el mismo día.
- Verifica el correo de la cuenta y revisa la barra lateral y las reglas **actuales** del subreddit. Las reglas de autopromoción cambian y no las he podido consultar en este entorno. Si hay duda, pregunta a los mods por modmail antes de publicar.
- Si hay flair de proyecto, `Project` o `Self-Hosted`, úsalo. Si no sabes cuál, déjalo vacío antes que elegir uno mal.
- No pidas upvotes, estrellas ni votos. Pide historias de uso.
- Reddit no traduce los posts de forma fiable. La versión elegida va en castellano. El resumen en inglés va como primer comentario opcional, no sustituye al texto principal.

## Versión en castellano (elegida)

**Título**

```text
NetDiag v1.5.2 — inventario y diagnósticos de red autocontendidos en un único binario Go (sin agentes ni telemetría)
```

**URL**

```text
https://github.com/kr3s4l4/netdiag
```

**Cuerpo** (si eliges tipo Text; si eliges Link, va como primer comentario)

```text
Soy el autor, desarrollo en solitario. Esto es una preview de validación, no un producto terminado.

NetDiag corre en una máquina de tu LAN y te dice qué hay conectado: descubrimiento ARP/ICMP, barrido TCP, enriquecimiento DNS, banners, SNMP y LLDP, topología L2/L3, diagnósticos heurísticos (lazos STP, fugas de VLAN, desajustes de dúplex, AP no autorizados) con evidencia y confianza, alertas, informes y multisede. Todo vive en un SQLite local. Un único binario Go, interfaz web embebida (español/inglés), sin cgo, sin agentes, sin nube, sin npm y sin telemetría.

La contrapartida es la visibilidad: solo ve lo que un host puede observar. La documentación lo dice explícitamente. Los diagnósticos son heurísticos y siempre llevan evidencia, confianza y limitaciones.

v1.5.2 es una release firmada: SHA256SUMS está firmado con minisign y la clave pública está en el repo. Los binarios Linux amd64 y arm64 están aquí:
https://github.com/kr3s4l4/netdiag/releases/tag/v1.5.2

Si lo pruebas, me encantaría saber dónde lo desplegaste, qué intentaste primero y qué te faltó. Eso decide qué construyo después.
```

**Primer comentario**

```text
Soy el autor. Busco casos de uso reales, no estrellas.

Si lo pruebas, dime tres cosas:
1. ¿Dónde lo desplegaste (casa, laboratorio, oficina, planta, datacenter)?
2. ¿Qué intentaste hacer primero?
3. ¿Qué te faltó o te confundió?

Hay un Discussion fijado en el repo para eso. Leo todas las respuestas. Gracias por mirarlo.
```

**Primer comentario opcional en inglés** (para ampliar alcance; añadir tras el castellano)

```text
English summary: one Go binary, local SQLite, no agents, no cloud, no telemetry. Signed v1.5.2 release. I'm looking for real deployments: where you deployed it, what you tried first, and what was missing.
```

Las secciones «Opción A», «Opción B» y «Respuestas listas» están en inglés y se conservan como material de referencia. **La versión elegida es la de castellano de arriba.** Si necesitas responder en inglés a un comentario, usa la tabla y dime si quieres adaptarla.

## Opción A: enlace (recomendada, en inglés)

**Título**

```text
NetDiag v1.5.2 – self-hosted network inventory and diagnostics in one Go binary (no agents, no telemetry)
```

**URL**

```text
https://github.com/kr3s4l4/netdiag
```

**Primer comentario** (lo pegas tú justo después de publicar)

```text
I'm the author, a solo dev. This is a validation preview, not a finished product.

If you give it a try, I'd love to know three things:
1. Where did you deploy it (home, lab, office, plant, datacenter)?
2. What did you try first?
3. What was missing or confusing?

There's a pinned discussion in the repo for that. Feedback decides what I build next. Thanks for looking.
```

## Opción B: texto (en inglés)

Si el subreddit no permite enlaces directos o te sale elegir tipo **Text**, pega esto:

```text
I'm the author, a solo dev, and this is a validation preview rather than a finished product.

NetDiag runs on one machine on your LAN and tells you what is connected: ARP/ICMP discovery, TCP sweep, DNS/banner/SNMP/LLDP enrichment, L2/L3 topology, heuristic diagnostics (STP loops, VLAN leaks, duplex mismatches, rogue APs) with evidence and confidence, alerts, reports and multi-site. Everything lives in a local SQLite file. One Go binary, embedded web UI (ES/EN), no cgo, no agents, no cloud, no npm and no telemetry.

The tradeoff is visibility: it only sees what one host can observe. The docs say so explicitly. Diagnostics are heuristic and always carry evidence, confidence and limitations.

v1.5.2 is a signed release: SHA256SUMS is signed with minisign and the public key is in the repo. Linux amd64 and arm64 binaries are here:
https://github.com/kr3s4l4/netdiag/releases/tag/v1.5.2

If you give it a spin, I'd love to know where you deployed it, what you tried first and what was missing. Feedback decides what I build next.
```

## Respuestas listas

| Pregunta | Respuesta para copiar |
| --- | --- |
| Telemetry? | `No telemetry at all. The binary makes no outbound calls. You can read the code, or run it with no network access and see for yourself.` |
| Agents? | `No agents. It observes from one host: ARP/ICMP, banners, SNMP and LLDP/CDP neighbours. Nothing is installed on the monitored devices.` |
| Why a single binary? | `I wanted something you can copy to a machine and run. One Go binary, embedded UI and SQLite, no cgo, no npm, no external services.` |
| What are the limits? | `Visibility: one host only sees its own network vantage point. Diagnostics are heuristic, with evidence, confidence and explicit limitations.` |
| License / pricing? | `Community is AGPL-3.0. There's no checkout or active pricing yet. I'm validating with real deployments first. If I charge later, it will be per managed-host band, never per report, alert or discovered host.` |
| Is it enterprise-ready? | `It has RBAC, audit and multi-site, but I'm not selling an SLA yet. I'm one person and this is a preview. I'd rather say that than promise something I can't keep.` |
| How do I trust the binary? | `SHA256SUMS is minisign-signed. The key and a verification command are in docs/public_keys.md. The v1.5.2 SHA256SUMS hash is 4c8c1513a10abab794143be627048359115dcea6986945fb28e0e7cc31b90a39.` |
| Docker? | `No official Docker image yet. I prioritised a single binary and a systemd unit. If people ask for it, I'll build it.` |
| Windows/macOS? | `Linux amd64 and arm64 for now. Tell me if you need another platform and I'll consider it.` |
| Why not just nmap? | `It isn't replacing nmap. It keeps history, topology and evidence over time, so you can see what changed and why.` |
| Do you use it in production? | `I use it in my own lab. This is a preview: I'm looking for real deployments before calling it finished.` |
| Can I scan any network? | `Only networks you own or are authorised to assess. The Beta notice now says so at first run.` |
| Where is the feedback form? | `There's a pinned discussion in the repo. That's enough for now; I'm reading every reply.` |

## Resultado del intento en r/selfhosted (19/11/2026)

Un mod eliminó el post con este mensaje:

> Gracias por publicar en r/selfhosted. Tu publicación se eliminó. En su lugar, comparte tu proyecto en el Megathread de New Project actual o mira la regla 6 para más información.

No responder con quejas ni republicar el mismo post. La ruta correcta es el Megathread de New Project.

**Respuesta al mod, para copiar:**

```text
Entendido, gracias por el aviso. Lo llevo al Megathread de New Project. Perdón por no haber revisado la regla 6 antes.
```

**Entrada corta para el Megathread:**

```text
NetDiag v1.5.2 — inventario y diagnósticos de red autocontendidos.

Un único binario Go, SQLite local, interfaz web en español/inglés. Sin agentes, sin nube, sin telemetría y sin npm. Descubrimiento ARP/ICMP, topología L2/L3, diagnósticos heurísticos con evidencia, alertas, informes y multisede.

Release firmada con minisign; clave pública y verificación en el repo. Beta: úsalo solo en redes propias o con autorización.

Busco que gente real lo pruebe y me diga dónde lo desplegó y qué le faltó.

Repo: https://github.com/kr3s4l4/netdiag
Releases: https://github.com/kr3s4l4/netdiag/releases/tag/v1.5.2
```

En el Megathread se publica una entrada breve: no pegues el cuerpo completo ni abras otro post. Después responde a quien comente con las respuestas listas, en castellano o en inglés. Esto cuenta como señal pública, no como evidencia F42.

## Error «Your repost includes a link that isn't working»

Aparece al usar **Crosspost / Repost** desde un post de tipo **Text** que contiene URLs en el cuerpo. Reddit intenta validar ese enlace como destino del repost y lo rechaza. No significa que el enlace esté roto: ambas URLs del borrador responden correctamente.

Comprobado el 19/11/2026:

- `https://github.com/kr3s4l4/netdiag` → carga el repo público.
- `https://github.com/kr3s4l4/netdiag/releases/tag/v1.5.2` → carga la release con sus 16 assets.

### Solución recomendada

1. Publica en `r/selfhosted` como **Link**, no como Text.
2. URL: `https://github.com/kr3s4l4/netdiag`.
3. Título: el de la versión en castellano.
4. Pega el cuerpo en castellano como **primer comentario**, sin la línea de la URL de release.
5. Después usa Crosspost sobre ese post de tipo Link. Ahí Reddit ya tiene una URL válida.

### Cuerpo adaptado para el primer comentario

```text
Soy el autor, desarrollo en solitario. Esto es una preview de validación, no un producto terminado.

NetDiag corre en una máquina de tu LAN y te dice qué hay conectado: descubrimiento ARP/ICMP, barrido TCP, enriquecimiento DNS, banners, SNMP y LLDP, topología L2/L3, diagnósticos heurísticos (lazos STP, fugas de VLAN, desajustes de dúplex, AP no autorizados) con evidencia y confianza, alertas, informes y multisede. Todo vive en un SQLite local. Un único binario Go, interfaz web embebida (español/inglés), sin cgo, sin agentes, sin nube, sin npm y sin telemetría.

La contrapartida es la visibilidad: solo ve lo que un host puede observar. La documentación lo dice explícitamente. Los diagnósticos son heurísticos y siempre llevan evidencia, confianza y limitaciones.

La release v1.5.2 está firmada con minisign; los binarios Linux amd64 y arm64 están en la página de releases del repo y la clave pública está en docs/public_keys.md.

Si lo pruebas, me encantaría saber dónde lo desplegaste, qué intentaste primero y qué te faltó. Eso decide qué construyo después.
```

### Alternativa sin Crosspost

Publicar directamente en cada comunidad evita el validador. No cruces el mismo texto el mismo día: espacia publicaciones y cambia el título según el sitio. Si un mod objeta, retira el post sin discutir.

## Qué NO decir

- No «production-ready» ni «enterprise-grade» sin matiz.
- No prometer SLA, soporte 24/7 ni precios.
- No decir «gratis para siempre»: Community es AGPL.
- No pedir votos ni estrellas. Pide historias de uso.
- No publicar `docs/respuesta-hn.md` ni el mismo texto en otros foros sin autorización específica.

## Después de publicar

1. Quédate cerca las primeras 2 h y responde con frases cortas.
2. Si no entiendes un comentario, pega: `Could you rephrase that? My English is limited.`
3. Pega aquí cualquier respuesta interesante y te preparo el texto en inglés.
4. Registra el enlace en `docs/validacion.md`, pero **no** lo cuentes como evidencia F42: son señales públicas, no instalaciones reales.
5. Si un mod objeta el título o el flair, cámbialo sin discutir.
