# NetDiag

**Escáner de red y monitor de inventario self-hosted.** Un único binario Go con interfaz web en español, descubrimiento activo/pasivo, SNMP, topología L2/L3, alertas cifradas y generación de informes. Sin dependencias externas en tiempo de ejecución, sin telemetría y sin servicios en la nube.

```
┌──────────────────────────────────────────────────────────────┐
│  Descubre → identifica → almacena → vigila → alerta → informa │
└──────────────────────────────────────────────────────────────┘
```

## Características

- **Descubrimiento en tres fases**: vecinos ARP/ICMP, barrido TCP y enriquecimiento (DNS inverso, banners, TLS, SNMP, LLDP/CDP/FDB).
- **Identidad estable**: los hosts se rekeyean a su MAC, conservando notas, etiquetas y favoritos aunque cambie la IP.
- **Inventario rico**: fabricante por OUI, tipo de dispositivo con puntuación y evidencia, sistema operativo probable, puertos, servicios, certificados y modelo SNMP.
- **Topología interactiva**: SVG con zoom, arrastre y agrupación por subred; enlaces por LLDP/CDP/bridge/ARP con confianza.
- **Monitor periódico**: escaneos programados, detección de hosts caídos y nuevos, histórico de latencia y disponibilidad.
- **Alertas**: reglas integradas y personalizadas, cooldown, deduplicación y siete canales (webhook HMAC, Slack, Discord, Telegram, PagerDuty, SMTP y syslog).
- **Informes**: PDF, CSV, JSON y HTML con portada, resumen ejecutivo, inventario y alertas.
- **Seguridad por defecto**: Argon2id, sesiones en servidor, CSRF, límite de intentos, cabeceras CSP/HSTS, auditoría y secretos cifrados con AES-256-GCM.
- **Un solo binario**: SPA embebida, SQLite embebido (modernc) y configuración YAML. Funciona offline.

## Inicio rápido

Requisitos: Go 1.27 o superior. Para ARP activo, además `libpcap-dev` y gcc.

```bash
git clone <repo> netdiag && cd netdiag
make build                 # bin/netdiag portable (sin cgo)
./bin/netdiag serve        # http://127.0.0.1:8080
```

En el primer acceso la interfaz pide crear la cuenta de administrador. Después:

1. **Ajustes → Redes** para añadir la subred a monitorizar (se sugieren las locales).
2. **Escanear** para lanzar un descubrimiento manual.
3. **Ajustes → Escaneo** para activar el monitor periódico.

Datos y configuración por defecto:

| Elemento | Ruta |
| --- | --- |
| Configuración | `~/.config/netdiag/config.yaml` (`NETDIAG_CONFIG`) |
| Datos | `~/.local/share/netdiag` (`NETDIAG_DATA_DIR`) |
| Base de datos | `<datos>/netdiag.db` |
| Clave de cifrado | `<datos>/secret.key` |
| Informes | `<datos>/reports/` |

## Compilación

```bash
make build       # portable, CGO_ENABLED=0
make build-pcap  # ARP activo con gopacket + libpcap
make test        # pruebas unitarias
make test-race   # detector de carreras
make release     # Linux amd64 + arm64
```

El ARP activo permite descubrir dispositivos que no responden a ICMP ni TCP. El binario portable usa la tabla de vecinos del kernel y `NudgeARP`, suficiente en la mayoría de redes domésticas y de oficina.

```bash
sudo apt install libpcap-dev
make build-pcap
sudo setcap cap_net_raw,cap_net_admin+eip bin/netdiag-pcap
```

## CLI

```bash
netdiag serve [--listen 0.0.0.0:8080] [--no-monitor]
netdiag scan 192.168.1.0/24 [--profile quick|common|full|custom]
                            [--ports 22,80,8000-8100] [--snmp] [--deep]
                            [--timeout 900] [--progress] [--json]
netdiag scan --network 3    # una red ya configurada
netdiag hosts [--search texto] [--status up] [--type server] [--json|--csv]
netdiag report [--kind inventory|executive|alerts] [--format pdf|csv|json|html]
               [--from 2026-01-01] [--to 2026-01-31] [--out informe.pdf]
netdiag config [show|path|init|validate]
netdiag doctor
netdiag user [list|add|passwd]
```

Las opciones se aceptan antes o después de los argumentos posicionales. Precedencia de configuración:

```
flags de línea de comandos  >  variables NETDIAG_*  >  config.yaml  >  valores por defecto
```

Variables admitidas: `NETDIAG_CONFIG`, `NETDIAG_DATA_DIR`, `NETDIAG_DB`, `NETDIAG_LISTEN`, `NETDIAG_PUBLIC_URL`, `NETDIAG_LOG_LEVEL`, `NETDIAG_LOG_FORMAT`, `NETDIAG_PROFILE`, `NETDIAG_SCAN_TIMEOUT`, `NETDIAG_SCAN_CONCURRENCY`, `NETDIAG_AUTH`, `NETDIAG_SESSION_HOURS`, `NETDIAG_MONITOR`, `NETDIAG_MONITOR_INTERVAL`, `NETDIAG_SNMP`, entre otras.

### Recuperar el acceso

```bash
sudo -u netdiag netdiag user passwd admin
# o crear otro administrador
sudo -u netdiag netdiag user add operador --role admin
```

La contraseña se puede pasar por `--password`, por `NETDIAG_PASSWORD` o por stdin para no dejarla en el historial.

## Configuración

`netdiag config init` escribe una configuración completa lista para editar. Los bloques principales:

```yaml
server:
  listen: 127.0.0.1:8080
  public_url: https://netdiag.example.com
  tls_cert: /etc/netdiag/fullchain.pem
  tls_key: /etc/netdiag/privkey.pem
scan:
  profile: common
  timeout: 900ms
  concurrency: 128
  port_concurrency: 256
  max_hosts: 4096
  resolve_dns: true
  banner_grab: true
  snmp: true
  snmp_version: 2c
  snmp_communities: [public]
monitor:
  enabled: true
  interval: 5m
alerts:
  enabled: true
  max_per_minute: 30
  allow_private_targets: false
storage:
  retention_days: 30
  metrics_days: 30
security:
  auth: true
  session_hours: 12
  login_rate: 5
  trust_proxy: false
```

La mayoría de cambios se aplican en caliente desde **Ajustes**; los de escucha, TLS, rutas de datos y nivel de registro requieren reiniciar.

## Seguridad

- Contraseñas con **Argon2id** (64 MiB, 3 iteraciones); se aceptan hashes bcrypt heredados.
- Sesiones opacas en servidor, cookie `HttpOnly`/`SameSite=Lax`/`Secure` tras TLS, y token **CSRF** en cada escritura.
- Límite de intentos de login por IP y límite global de peticiones API.
- **CSP**, `X-Frame-Options: DENY`, `nosniff`, `Referrer-Policy: no-referrer` y HSTS opcional.
- Configuraciones de canales de alerta cifradas con **AES-256-GCM**; la clave `secret.key` se crea con permisos `0600`. Los secretos nunca se devuelven al navegador.
- Protección **SSRF**: los webhooks a rangos privados, loopback y metadatos cloud se bloquean salvo `alerts.allow_private_targets: true`.
- Registro de auditoría de accesos, cambios de configuración y operaciones sensibles, con retención configurable.

Para exponer NetDiag fuera de localhost, usa un proxy inverso con TLS (Caddy, nginx, Traefik) o configura `server.tls_cert`/`server.tls_key`. No publiques la instancia directamente en Internet sin autenticación y HTTPS.

## Arquitectura

```
cmd/netdiag            CLI y arranque
internal/app           ciclo de vida, escaneos, planificador, mantenimiento
internal/api           REST /api/v1, middleware, WebSocket
internal/auth          Argon2id, sesiones, CSRF, auditoría
internal/store         SQLite, migraciones, consultas
internal/discover      motor de descubrimiento en tres fases
internal/fingerprint   OUI, clasificación, banners, puertos
internal/snmp          SNMP v1/v2c/v3, IF-MIB, LLDP/CDP/FDB
internal/topology      grafo de red y confianza de enlaces
internal/alerts        reglas, deduplicación y canales
internal/report        PDF, CSV, JSON, HTML
internal/webui         SPA embebida (HTML/CSS/JS, sin dependencias)
```

La API está versionada en `/api/v1`; el WebSocket (`/api/v1/ws`) emite eventos `scan.progress`, `scan.finished`, `host.new`, `host.updated`, `alert.new`, `dashboard.updated`. Consulta [docs/arquitectura.md](docs/arquitectura.md) y [docs/api.md](docs/api.md).

## Despliegue

```bash
sudo ./scripts/install.sh
sudo systemctl enable --now netdiag
```

El instalador crea el usuario `netdiag`, instala el binario en `/usr/local/bin`, los datos en `/var/lib/netdiag`, la configuración en `/etc/netdiag/config.yaml`, aplica capabilities y registra la unidad systemd. Consulta [docs/despliegue.md](docs/despliegue.md) para TLS, proxy inverso, copias de seguridad y actualizaciones.

## Estado

Proyecto en desarrollo activo. La suite cubre configuración, persistencia, identidad de hosts, cifrado de canales, clasificación, utilidades de red, topología y política de contraseñas. Las contribuciones y los informes de errores son bienvenidos.
