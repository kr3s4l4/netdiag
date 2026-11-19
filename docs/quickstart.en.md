# NetDiag quick start (English)

> Spanish version: [README.md](../README.md). The web UI switches between Spanish and English in **Settings → Language**.

## 1. Build and run

Requirements: Go 1.27+. Active ARP needs `libpcap-dev` and gcc only if you build the `pcap` variant.

```bash
git clone https://github.com/kr3s4l4/netdiag.git
cd netdiag
make build                 # portable single binary, no cgo
./bin/netdiag serve        # http://127.0.0.1:8080
```

The first visit asks you to create the administrator account. Use a strong password; it is stored with Argon2id.

## 2. First useful scan

1. In the top bar, select the site. `Principal` (id `1`) already exists.
2. Go to **Settings → Networks** and add your LAN CIDR, e.g. `192.168.1.0/24`.
3. Click **Scan** and wait for completion.
4. Open **Inventory**, **Topology** and **Traces** to see the result.

Equivalent from the CLI:

```bash
./bin/netdiag scan 192.168.1.0/24 --snmp --deep --wait --json
./bin/netdiag doctor
```

`doctor` checks ICMP, ARP, UDP/68 for the DHCP probe, passive capture support and privileges. It is the fastest way to tell a missing capability from a configuration mistake.

## 3. Enable passive capture (optional)

The portable build degrades cleanly: diagnostics still work from scan history, but no raw socket is opened. To capture real frames:

```bash
sudo apt install libpcap-dev
make build-pcap
sudo setcap cap_net_raw,cap_net_admin+eip bin/netdiag-pcap
```

Then enable it in `~/.config/netdiag/config.yaml`:

```yaml
diagnostics:
  passive_capture: true
  passive_interface: eth0
  passive_seconds: 20
  passive_max_packets: 5000
```

Only metadata is stored: protocol, IP/MAC, interface and a truncated detail. No payload, credentials or DNS answers are persisted.

## 4. Docker

```bash
docker run -d --name netdiag \
  --network host \
  --cap-add NET_RAW --cap-add NET_ADMIN \
  -e NETDIAG_LISTEN=0.0.0.0:8080 \
  -v netdiag-data:/var/lib/netdiag \
  netdiag:local
```

Use `--network host` to see the real LAN. With a bridge network you will only see other containers. Mount your own config if you need to change `server.listen` or the diagnostics options.

## 5. Paths and configuration

| Item | Path |
| --- | --- |
| Configuration | `~/.config/netdiag/config.yaml` (`NETDIAG_CONFIG`) |
| Data | `~/.local/share/netdiag` (`NETDIAG_DATA_DIR`) |
| Database | `<data>/netdiag.db` |
| Encryption key | `<data>/secret.key` |
| Reports | `<data>/reports/` |

Environment overrides include `NETDIAG_SNMP`, `NETDIAG_SNMP_COMMUNITIES`, `NETDIAG_DHCP_PROBE`, `NETDIAG_SNMP_DEEP`, `NETDIAG_PASSIVE_CAPTURE` and `NETDIAG_PASSIVE_IFACE`.

## 6. Where to go next

- [README.en.md](../README.en.md): features, multi-site, integrations and licensing.
- [Security policy](../SECURITY.en.md): supported versions, responsible disclosure and CRA runbook.
- [API reference](api.md): REST endpoints and authentication.
- [Roadmap](roadmap.md): planned work (Spanish).
- [Reproducible lab](../scripts/lab/README.md): Docker lab that exercises DHCP rogue, SNMP and passive capture with real traffic.

## 7. Troubleshooting

| Symptom | Check |
| --- | --- |
| `setup_required` | Create the admin with `POST /api/v1/auth/setup`. |
| CSRF error | Send `X-CSRF-Token` from the `/auth/setup` or `/auth/login` response. |
| No hosts found | Confirm the CIDR and that ICMP/ARP are allowed. Run `netdiag doctor`. |
| Passive capture unavailable | Build with `-tags pcap` and grant `CAP_NET_RAW`. |
| DHCP probe inactive | Needs a local segment and `diagnostics.dhcp_probe: true`; it binds UDP/68 without raw sockets. |
| UI language is Spanish | **Settings → Language → English**. No translator or external service is used. |
