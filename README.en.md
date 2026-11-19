# NetDiag

**Self-hosted network scanner and inventory monitor.** A single Go binary with an embedded web UI, active and passive discovery, SNMP, L2/L3 topology, encrypted alerts, backups and reporting. No runtime dependencies, no telemetry, no cloud accounts.

> **Beta and disclaimer.** NetDiag is provided "as is", without warranties. Use it only on networks you own or have explicit authorization to scan. Unauthorized scans may break laws and trigger corporate firewall alerts. You choose which networks are scanned and you are responsible for that. The notice is repeated on first run; nothing is sent to any server.

[🇬🇧 English](README.en.md) · [🇪🇸 Español](README.md) — The web UI switches language in **Settings → Language**.

```
Discover → identify → store → monitor → alert → report
```

## Features

- **Three-stage discovery**: ARP/ICMP neighbours, TCP sweep and enrichment (reverse DNS, banners, TLS, SNMP, LLDP/CDP/FDB).
- **Stable identity**: hosts are keyed by MAC, so notes, tags and favourites survive IP changes. Identity is scoped per site: the same IP or MAC can exist in several sites without collisions.
- **Multi-site**: offices, plants and datacenters share one SQLite database. Each site has its own networks, hosts, scans, reports and metrics; the same CIDR may be reused.
- **Portable site packages**: export a site to JSON and import it into another site or instance, in `merge` or `replace` mode.
- **Rich inventory**: OUI vendor, scored device type with evidence, probable OS, ports, services, certificates and SNMP model.
- **Interactive topology**: zoomable, draggable SVG grouped by subnet, with LLDP/CDP/bridge/ARP links and confidence.
- **Enterprise diagnostics** (see below) with evidence, confidence (1–100) and explicit limitations.
- **Security diagnostics**: ARP/IP→MAC history, rogue DHCP probe, deep SNMP correlation and optional metadata-only passive capture.
- **External integrations**: Elasticsearch, syslog, signed webhooks, Slack, Discord, MS Teams, Telegram, PagerDuty, Kafka REST Proxy and Redis Streams. Bounded queue, global rate limit, SSRF guard and AES-256-GCM encrypted secrets.
- **VPN detection (F12)**: passive, heuristic observation of WireGuard, OpenVPN, IPsec and PPTP tunnels from kernel sockets and interfaces; never injects or decrypts traffic.
- **WiFi analysis (F11)**: passive 802.11 capture in `-tags pcap` builds, with APs, clients per AP, channel interference and rogue AP heuristics.
- **Custom dashboards (F18)**: up to 16 reorderable widgets per site on a 12-column grid, sourced only from an allowlisted endpoint catalogue.
- **Time comparison (F21)**: diff of two equivalent windows (today vs. 7 days ago by default), with per-device, topology and fleet changes.
- **Audit mode (F22)**: compressed manual baselines per site, detailed snapshot diffs and JSON/GZIP export, without secrets.
- **Periodic monitor**: scheduled scans, host-down/host-new detection, latency and availability history.
- **Alerting**: built-in and custom rules, cooldown, deduplication and seven channels (HMAC webhook, Slack, Discord, Telegram, PagerDuty, SMTP, syslog).
- **Reports**: PDF, CSV, JSON and HTML with cover page, executive summary, inventory and alerts.
- **Roles**: `admin`, `operator` and `viewer`, with protected deletion of the last administrator.
- **Bilingual and guided**: Spanish by default, English in one click, searchable in-app guide and first-run tour.
- **OpenAPI 3.1** at `/api/v1/openapi.json` and Prometheus metrics at `/metrics`.
- **Backups**: self-contained ZIP (consistent SQLite database, `config.yaml` and `secret.key`), scheduled with retention, from the UI, API or `netdiag backup`.
- **Secure defaults**: Argon2id, server-side sessions, CSRF, rate limiting, CSP/HSTS headers, auditing, AES-256-GCM secrets and an optional token for `/metrics`.
- **One binary**: embedded SPA, embedded SQLite (modernc) and YAML configuration. Works offline.

## Quick start

Requirements: Go 1.27+. For active ARP, also `libpcap-dev` and gcc.

```bash
git clone <repo> netdiag && cd netdiag
make build                 # portable netdiag binary (no cgo)
./bin/netdiag serve        # http://127.0.0.1:8080
```

The first visit asks you to create the administrator account. Then:

1. Pick a site with the **site selector** in the top bar; `Principal` exists by default.
2. **Settings → Sites** to create, rename, activate, export or import sites (admins only).
3. **Settings → Networks** to add the subnet to monitor in the active site.
4. **Scan** to run a manual discovery in the active site.
5. **Settings → Scan** to enable the periodic monitor.

Default paths:

| Item | Path |
| --- | --- |
| Configuration | `~/.config/netdiag/config.yaml` (`NETDIAG_CONFIG`) |
| Data | `~/.local/share/netdiag` (`NETDIAG_DATA_DIR`) |
| Database | `<data>/netdiag.db` |
| Encryption key | `<data>/secret.key` |
| Reports | `<data>/reports/` |

## Multi-site

All sites live in the same SQLite file. The `Principal` site (id `1`) is created on first run. The top-bar selector stores the active site in `localStorage` (`netdiag-site`) and adds `site_id` to operational requests. Switching sites reloads inventory, alerts, topology, reports, dashboards and metrics.

- Host identity is stored as `s<site_id>:<mac|ip:...>`. Export strips the prefix and import prepends the destination site, so packages are portable.
- Admins can create, rename, export, import and delete sites. The last site cannot be deleted; deleting a site moves its data to another one, resolving CIDR collisions.
- Import modes: `merge` (keep existing data, update metadata) or `replace` (replace destination data). Accepts JSON or multipart `file`.
- `site_id=0` in dashboards, metrics and reports means "all sites".

```bash
curl -b cookies.txt -H 'X-CSRF-Token: <csrf>' -H 'Content-Type: application/json' \
  -d '{"name":"North Office","description":"Plant 2"}' \
  http://127.0.0.1:8080/api/v1/sites

curl -b cookies.txt http://127.0.0.1:8080/api/v1/hosts?site_id=2
curl -b cookies.txt http://127.0.0.1:8080/api/v1/sites/2/export -o office.json
curl -b cookies.txt -H 'X-CSRF-Token: <csrf>' -F 'file=@office.json' \
  'http://127.0.0.1:8080/api/v1/sites/3/import?mode=merge'
```

## Enterprise and security diagnostics

The **Diagnostics** view and `GET /api/v1/diagnostics/enterprise` correlate data already collected by NetDiag. Each finding has type, severity, hosts, evidence, confidence (1–100), `cause`, `impact`, `next_steps`, `limitations` and `hypotheses`. The `recommendation` field is kept for compatibility. Detectors require several simultaneous signals and respect `site_id` and the `from`/`to` window.

| Category | Signals |
| --- | --- |
| STP loops | Cycles in the LLDP/CDP/FDB L2 graph with sufficient confidence |
| VLAN leaks | One L2 port reaching hosts on different networks, or bridge adjacency between L2 devices on distinct subnets |
| Duplex | Incompatible IF-MIB speeds at both ends of an LLDP/CDP link |
| MTU | Local low-MTU interface correlated with loss on its network |
| Storms | Simultaneous loss and latency across several hosts in one subnet |
| Rogue APs | APs outside `diagnostics.authorized_aps` or without enough SNMP identity |
| SLA | Availability and mean latency per host against configured thresholds |
| ARP / MITM | Gateway or any IP changing MAC between scans, one MAC with many IPs, one IP with several MACs |
| Rogue DHCP | Several DHCP responders, unauthorized server, gateway spoofing, server MAC change |
| Deep SNMP | Switch ARP vs. history, MAC on several ports/VLANs or moved between scans, FDB vs. LLDP/CDP |
| Passive capture | Repeated gratuitous ARP, ICMP redirect, DHCP OFFER/ACK, inbound SYNs, high-entropy DNS names |
| TLS certificates | `TLSCert.NotAfter` at 30, 14 or 1 day to expiry; critical on the last day |

**Limits.** By default NetDiag captures no frames or BPDUs, reads no duplex OIDs or broadcast/fragment counters, and cannot see other clients' ARP/DHCP traffic. Active probes (ICMP/ARP, UDP/68, SNMP) and their findings are leads for investigation, not protocol-level proof. Passive capture is opt-in and metadata-only and also needs human review. The TLS check only reads the expiry date offered in the handshake: it queries no CA, OCSP or DNS and cannot know whether renewal is in progress. Every finding carries its own `limitations`, and the report adds the global ones. With `enterprise_mode: false`, SLA thresholds and TLS certificates are evaluated; the other network detectors stay off.

**Rogue DHCP.** With `diagnostics.dhcp_probe: true` (default), DHCPINFORM and DHCPDISCOVER are sent over UDP/68 without raw sockets; replies are stored in `dhcp_observations` for `dhcp_history_days` (30). Tune with `NETDIAG_DHCP_PROBE`, `NETDIAG_DHCP_PROBE_TIMEOUT` and `NETDIAG_DHCP_AUTHORIZED`. See `GET /api/v1/diagnostics/dhcp`.

**Deep SNMP.** With `diagnostics.snmp_deep: true` (default), the collector queries `ipAddrTable` (`.1.3.6.1.2.1.4.20`), ARP tables `.4.22`/`.4.35`, Q-BRIDGE `.1.3.6.1.2.1.17.7.*` and BRIDGE-MIB `.1.3.6.1.2.1.17.4.*`, with row caps of 512 addresses, 2000 ARP, 5000 FDB and 256 VLANs. Rows are kept per site for `snmp_history_days` (30). The community that answered is stored **masked** (`public` → `pu***ic`); the plaintext credential is never persisted or exposed. Types `snmp_*` feed the `snmp_anomaly` rule. See `GET /api/v1/diagnostics/snmp`.

**Passive capture.** Disabled by default. Requires a `-tags pcap` binary, `CAP_NET_RAW` and `diagnostics.passive_capture: true`. After each scan it opens a short, BPF-narrow window (ARP, ICMP, UDP/67-68-53 and TCP SYNs) and stores **metadata only** in `passive_observations` for `passive_history_days` (7): never payloads, credentials or DNS answers. Limits: `passive_seconds` (10), `passive_max_packets` (2000), `passive_promiscuous` (false). Environment: `NETDIAG_PASSIVE_CAPTURE`, `NETDIAG_PASSIVE_IFACE`, `NETDIAG_PASSIVE_SECONDS`, `NETDIAG_PASSIVE_MAX_PACKETS`, `NETDIAG_PASSIVE_PROMISC`, `NETDIAG_PASSIVE_HISTORY_DAYS`. Portable builds report `available:false` and keep scanning. See `GET /api/v1/diagnostics/passive`; types `passive_*` feed the `passive_anomaly` rule.

**Explanation without AI.** `cause`, `impact` and `next_steps` come from deterministic templates keyed by finding type. The same evidence always produces the same text. Hypotheses are ranked by confidence and never exceed the evidence confidence: they are leads to verify, not confirmed causes. Three fixed correlation rules cover gateway spoofing, an L2 loop and MAC movement or spoofing.

**Local triage (F59).** Each finding can be marked `pending`, `confirmed`, `false_positive` or `resolved` in **Diagnostics**. The decision is keyed by `(site_id, finding_type, host_key)`, records the authenticated reviewer, a note of up to 500 characters and an audit entry, and expires after 30 days. `operator`, `admin` and `owner` may review; `viewer` is read-only. The badge also appears in **Alerts**. A review never closes an alert or changes the evidence. Manage it with `GET/POST/DELETE /api/v1/diagnostics/reviews`.

**History summary.** `POST /api/v1/history/compare` adds `narrative`: short sentences built only from structured changes. The history view and the executive PDF show them as a summary; they infer no causes and do not replace the evidence table.

**TLS certificates.** The `tls_expiring` detector reads `TLSCert.NotAfter` already collected during the handshake. It warns at 30 and 14 days and flags one day or less as critical. The `tls_expiring` alert rule uses category `tls` and the same `min_confidence` as the other finding rules.

**Signed evidence.** `scripts/sign-evidence.sh` reuses `netdiag container build` with a local Ed25519 key distinct from the release key. It packages reports, diffs and site bundles into a verifiable `.ndc` without a cloud. Procedure in [docs/evidencia-firmada.md](docs/evidencia-firmada.md).

**Voluntary usage evidence.** `netdiag evidence usage` produces a signed `.ndc` with days installed, scans, sites and aggregated managed-host bands. It contains no IPs, MACs, names or secrets, and is never sent automatically. It does not replace the written evidence for F42. Procedure in [docs/evidencia-uso.md](docs/evidencia-uso.md).

**Portable probe (F49).** `netdiag probe --site-id 1 --out sonda.json` collects ARP and kernel tables, attempts LLDP/CDP over SNMP and reports `available:false` for unavailable WiFi or VPN sources. The JSON may carry an optional minisign signature via `--seed-file`. `netdiag probe import [--key CLAVE.pub] sonda.json` writes it to the local database; a signed probe requires the public key. No cloud, no automatic upload and no charge per probe. See [docs/escaneo-remoto.md](docs/escaneo-remoto.md).

**Local dossier (F68).** `evidence.dossier` writes a signed `.ndc` every `interval_hours` containing the executive report, diff, audits and snapshots from the last `days` days, keeps the latest `keep` files and never uploads them. `netdiag evidence dossier` generates one manually. The seed is not stored in `config.yaml`: only a `0600` path to it.

```bash
curl -b cookies.txt 'http://127.0.0.1:8080/api/v1/diagnostics/enterprise?site_id=1&from=24h'
curl -b cookies.txt 'http://127.0.0.1:8080/api/v1/diagnostics/arp?site_id=1'
curl -b cookies.txt 'http://127.0.0.1:8080/api/v1/diagnostics/dhcp?site_id=1'
curl -b cookies.txt 'http://127.0.0.1:8080/api/v1/diagnostics/snmp?site_id=1'
curl -b cookies.txt 'http://127.0.0.1:8080/api/v1/diagnostics/passive?site_id=1'
curl -b cookies.txt 'http://127.0.0.1:8080/api/v1/diagnostics/reviews?site_id=1'
```

```bash
netdiag probe --site-id 1 --out sonda.json [--seed-file RUTA]
netdiag probe import [--key CLAVE.pub] sonda.json
netdiag evidence dossier [--site-id 1] [--days 30] [--seed-file RUTA] [--key-pub CLAVE]
```

## Integrations

Each integration is an independent `Integration {type, config, enabled}` managed in **Settings → Integrations** or via the API. Changes apply hot; secrets are AES-256-GCM encrypted and the API only returns `********`. Delivery happens in the background with a bounded queue and a global event rate limit.

| Type | Essential config |
| --- | --- |
| `elasticsearch` | `url`, `index`, and `api_key` or `username`+`password` |
| `syslog` | `host`, `port`, `protocol` (`udp`/`tcp`), `app_name` |
| `webhook` | `url`, optional `secret` (`X-NetDiag-Signature` HMAC) |
| `slack` / `discord` / `msteams` | incoming webhook `url` |
| `telegram` | `token`, `chat_id`, optional `api_url` |
| `pagerduty` | `routing_key`, optional `url` |
| `kafka` | REST Proxy `url`, `topic`, optional `token` |
| `redis` | `addr`, `stream`, `max_len`, optional ACL credentials |

The SSRF guard blocks private destinations by default; set `integrations.allow_private_targets: true` to allow a local Elasticsearch.

## Licensing

Community is AGPL-3.0 and free. Pro and Enterprise are commercial, offline-licensed editions. Billing is by **managed-host band**, never per report, alert, scan, user or detected host. A managed host is `s<siteID>:<identity>` seen in the last 30 days, excluding `guest`/`excluded` tags. There is no telemetry, online licence check or intrusive DRM.

**Already using NetDiag?** Leave a [voluntary interest note](https://kr3s4l4.github.io/netdiag/plantillas/formulario-disposicion-pago.html). It is generated in your browser and you decide whether to send it. No IPs, MACs or network data are collected. No billing, prices or automatic submission.

## Documentation

- [Quick start](docs/quickstart.en.md)
- [Security policy](SECURITY.en.md)
- [Report a real use case](.github/ISSUE_TEMPLATE/real-use-case.en.md)
- [Roadmap](docs/roadmap.md) (Spanish; roadmap only, no operational docs)
- [API reference](docs/api.md)
- [Signed evidence](docs/evidencia-firmada.md) (Spanish)
- [Remote scanning over SSH](docs/escaneo-remoto.md) (Spanish)
- [Validation notes](docs/validacion.md) (Spanish)
- [Distribution and licensing alternatives](docs/alternativas-publicacion-venta.md) (Spanish)

Most design and validation notes are still in Spanish. The README, quick start and security policy are the maintained English entry points; the web UI is fully bilingual.
