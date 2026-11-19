# Publicación en Reddit — v1.5.2

**Estado: pendiente. No publicado.** Esto es un borrador para copiar y pegar desde tu cuenta. Yo no tengo sesión en Reddit y no debo hablar en tu nombre. La publicación sigue sujeta a tu OK explícito (F48).

## Antes de publicar

- Hacker News fue el día 0 (18/11). Reddit corresponde al día 1 (19/11). No publiques los dos el mismo día.
- Subreddit recomendado: `r/selfhosted`. Alternativa, solo si el primero no encaja: `r/homelab`. No cruces el mismo texto en varios sitios el mismo día.
- Verifica el correo de la cuenta y revisa la barra lateral y las reglas **actuales** del subreddit. Las reglas de autopromoción cambian y no las he podido consultar en este entorno. Si hay duda, pregunta a los mods por modmail antes de publicar.
- Si hay flair de proyecto, `Project` o `Self-Hosted`, úsalo. Si no sabes cuál, déjalo vacío antes que elegir uno mal.
- No pidas upvotes, estrellas ni votos. Pide historias de uso.

## Opción A: enlace (recomendada)

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

## Opción B: texto

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
