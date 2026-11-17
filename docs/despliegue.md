# Despliegue

NetDiag se ejecuta como un único proceso: sirve la interfaz web, la API, el planificador de escaneos y escribe en SQLite. No necesita un servidor de aplicaciones, Node ni una base de datos externa, así que el despliegue se reduce a instalar el binario, preparar dos directorios y arrancar un servicio.

## Requisitos

- Linux con systemd (probado en Debian/Ubuntu y derivados; Fedora y Arch funcionan igual).
- 256 MB de RAM y 1 vCPU para redes de hasta /24. Para monitorizar varias /16 o usar el perfil `full`, reserva 1 GB y 2 vCPU.
- El binario portable es estático (`CGO_ENABLED=0`).
- Solo si compilas con `-tags pcap` (ARP activo) necesitas `libpcap` en tiempo de ejecución:
  - Debian/Ubuntu: `apt install libpcap0.8`
  - Fedora/RHEL: `dnf install libpcap`
  - Arch: `pacman -S libpcap`
- Para descubrir todos los dispositivos conviene que el servidor esté en la misma VLAN que la red monitorizada y que el switch no aísle el tráfico entre puertos.

## Instalación con el script

```bash
git clone <repo> netdiag && cd netdiag
sudo ./scripts/install.sh              # compila el binario portable si falta
sudo ./scripts/install.sh --pcap       # compila con ARP activo (requiere libpcap-dev y gcc)
sudo ./scripts/install.sh --start      # además arranca el servicio
```

El instalador es idempotente y hace lo siguiente:

1. Crea el usuario de sistema `netdiag` (sin shell, sin home real).
2. Instala el binario en `/usr/local/bin/netdiag`.
3. Crea `/etc/netdiag/config.yaml` con valores seguros si no existe.
4. Prepara `/var/lib/netdiag` para la base, la clave de cifrado y los informes.
5. Aplica `cap_net_raw`, `cap_net_admin` y `cap_net_bind_service` al binario.
6. Instala y habilita `netdiag.service`.

Después del primer arranque abre `http://127.0.0.1:8080`; la interfaz pedirá crear la cuenta de administrador. Si el servidor es remoto, publica el puerto mediante un proxy inverso con TLS (ver más abajo) antes de crear la cuenta.

Opciones del instalador:

| Opción | Efecto |
| --- | --- |
| `--pcap` | Compila con ARP activo (`make build-pcap`) |
| `--build` | Fuerza una compilación aunque exista `bin/netdiag` |
| `--binary RUTA` | Instala un binario ya compilado |
| `--listen DIR` | Escucha inicial (por defecto `127.0.0.1:8080`) |
| `--start` | Habilita y arranca el servicio al terminar |
| `--no-enable` | No habilita el servicio en el arranque |

## Instalación manual

```bash
# 1. Compilar
make build            # o make build-pcap
sudo install -Dm755 bin/netdiag /usr/local/bin/netdiag

# 2. Usuario y directorios
sudo useradd --system --home-dir /var/lib/netdiag --shell /usr/sbin/nologin netdiag
sudo install -d -o netdiag -g netdiag -m 0700 /var/lib/netdiag
sudo install -d -o netdiag -g netdiag -m 0700 /etc/netdiag

# 3. Configuración inicial
sudo -u netdiag netdiag config init --config /etc/netdiag/config.yaml --data-dir /var/lib/netdiag

# 4. Permisos de red
sudo setcap cap_net_raw,cap_net_admin,cap_net_bind_service=+ep /usr/local/bin/netdiag

# 5. Servicio
sudo install -m 0644 scripts/netdiag.service /etc/systemd/system/netdiag.service
sudo systemctl daemon-reload
sudo systemctl enable --now netdiag
sudo systemctl status netdiag
```

Comprueba el entorno con:

```bash
sudo -u netdiag netdiag doctor --config /etc/netdiag/config.yaml
```

`doctor` verifica la configuración, el directorio de datos, la base, el soporte ICMP, el ARP activo y los privilegios, y sugiere las subredes locales que puedes añadir al inventario.

## Configuración

La configuración vive en `/etc/netdiag/config.yaml` (permisos `0600`, propietario `netdiag`). Puedes editarla a mano o desde **Ajustes** en la interfaz. Los cambios de escucha, TLS, rutas y nivel de registro requieren reiniciar; el resto se aplica en caliente.

```yaml
server:
  listen: 127.0.0.1:8080
  public_url: https://netdiag.example.com
  # TLS directo (opcional; también puedes delegar en un proxy)
  tls_cert: ""
  tls_key: ""
  max_body_mb: 2
  enable_csp: true
  enable_hsts: true
scan:
  profile: common
  timeout: 900ms
  concurrency: 128
  max_hosts: 4096
  snmp: true
  snmp_version: 2c
  snmp_communities: [public]
monitor:
  enabled: true
  interval: 5m
  deep_interval: 6h
alerts:
  enabled: true
  max_per_minute: 30
  allow_private_targets: false
storage:
  data_dir: /var/lib/netdiag
  retention_days: 30
  metrics_days: 30
  audit_days: 90
  max_scans: 500
security:
  auth: true
  session_hours: 12
  login_rate: 5
  api_rate: 600
  trust_proxy: false
log:
  level: info
  format: text
```

Prioridad: flags de línea de comandos > variables `NETDIAG_*` > `config.yaml` > valores por defecto. En el servicio systemd no se usan flags adicionales salvo `--config`, así que la configuración es la fuente de verdad.

> **`trust_proxy`**: actívalo solo cuando NetDiag esté detrás de un proxy propio. Hace que la IP del cliente se lea de `X-Forwarded-For`, lo que afecta al límite de intentos de login y a la auditoría. Si lo activas sin proxy, cualquiera puede falsificar su IP.

## TLS y proxy inverso

NetDiag habla HTTP plano y no gestiona certificados salvo que configures `server.tls_cert` y `server.tls_key`. Las dos opciones válidas son:

### Proxy inverso (recomendado)

Mantén `server.listen: 127.0.0.1:8080` y termina TLS en el proxy. Activa `security.trust_proxy: true` y `server.public_url` con el esquema `https`.

**Caddy** (certificado automático):

```caddyfile
netdiag.example.com {
    encode zstd gzip
    reverse_proxy 127.0.0.1:8080
}
```

**nginx**:

```nginx
server {
    listen 443 ssl http2;
    server_name netdiag.example.com;

    ssl_certificate     /etc/letsencrypt/live/netdiag.example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/netdiag.example.com/privkey.pem;

    client_max_body_size 8m;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
        proxy_read_timeout 300s;
    }

    location /api/v1/ws {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;
        proxy_set_header Upgrade $http_upgrade;
        proxy_set_header Connection "upgrade";
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_read_timeout 1h;
    }
}

server {
    listen 80;
    server_name netdiag.example.com;
    return 301 https://$host$request_uri;
}
```

Si no necesitas WebSocket en tiempo real, el resto de la interfaz funciona con recargas REST, pero perderás el progreso de escaneo en vivo.

### TLS directo

```yaml
server:
  listen: 0.0.0.0:8443
  public_url: https://netdiag.example.com:8443
  tls_cert: /etc/netdiag/fullchain.pem
  tls_key: /etc/netdiag/privkey.pem
```

El usuario `netdiag` debe poder leer ambos archivos. Si usas Let's Encrypt, un `deploy-hook` que copie los certificados a `/etc/netdiag` con propietario `netdiag` evita reiniciar el servicio en cada renovación solo si el proceso relee los archivos; **NetDiag carga el certificado al arrancar**, así que añade `systemctl restart netdiag` al hook.

### Endurecimiento adicional

- Restringe el acceso por IP o VPN. NetDiag no está pensado para exponerse en Internet sin autenticación.
- Mantén `security.auth: true`. Si lo desactivas, cualquiera con acceso al puerto controla el inventario y los escaneos.
- Añade `security.allowed_hosts: [netdiag.example.com]` para rechazar peticiones API con un `Host` inesperado (protección básica contra DNS rebinding). La SPA estática se sigue sirviendo, pero las operaciones autenticadas quedan bloqueadas.
- El CSP estricto se activa con `server.enable_csp: true`; desactívalo solo si integras la API en otro origen.
- Revisa el registro de auditoría en **Ajustes → Auditoría** tras cambiar usuarios o canales.

## Firewall

NetDiag solo necesita salir a la red que monitoriza (ICMP, TCP, SNMP UDP/161, ARP) y escuchar en el puerto configurado.

```bash
# Servidor con proxy local
sudo ufw allow from 127.0.0.1 to any port 8080 proto tcp
sudo ufw allow 443/tcp

# Acceso directo desde la LAN (solo si confías en toda la red)
sudo ufw allow from 192.168.0.0/24 to any port 8080 proto tcp
```

No abras el puerto de administración a Internet. Si el servidor tiene varias interfaces, `listen: 127.0.0.1:8080` impide el acceso externo por diseño.

## Copias de seguridad

Todo el estado está en dos sitios:

| Ruta | Contenido | Crítico |
| --- | --- | --- |
| `/var/lib/netdiag/netdiag.db` | Inventario, alertas, escaneos, usuarios | Sí |
| `/var/lib/netdiag/secret.key` | Clave AES-256-GCM de los canales y secretos SNMP | Sí |
| `/var/lib/netdiag/reports/` | Informes generados | Opcional |
| `/etc/netdiag/config.yaml` | Configuración | Sí |

**Sin `secret.key` no se pueden descifrar las configuraciones de los canales de alerta.** NetDiag incluye la clave dentro de la copia para que el ZIP sea autocontenido: guárdalo en un destino cifrado y con permisos restringidos.

### Copias integradas

NetDiag genera un ZIP con `manifest.json`, una instantánea consistente de SQLite (`VACUUM INTO`), `config.yaml` y `secret.key`:

```bash
# Copia puntual en el directorio configurado
sudo -u netdiag netdiag backup

# Ruta explícita y retención distinta
sudo -u netdiag netdiag backup --out /var/backups/netdiag/$(date +%F).zip --keep 30
```

También desde **Ajustes → Copias** (solo administradores), que permite crear, descargar y eliminar copias. Las copias automáticas se configuran en `storage`:

```yaml
storage:
  backup_enabled: true
  backup_dir: /var/lib/netdiag/backups   # por defecto <data_dir>/backups
  backup_interval: 24h                   # mínimo 15m
  backup_keep: 7                         # copias a conservar
```

Al arrancar, si no hay ninguna copia, se crea una; después se respeta el intervalo y la retención. El ZIP se crea en un archivo temporal y se renombra al terminar, así que un corte no deja copias corruptas con nombre válido.

### Restauración

```bash
sudo systemctl stop netdiag
sudo install -d -m 0700 /var/lib/netdiag
sudo unzip -o /var/backups/netdiag/2026-11-17.zip \
  netdiag.db config.yaml secret.key -d /var/lib/netdiag
sudo chown -R netdiag:netdiag /var/lib/netdiag
sudo install -m 0600 /var/lib/netdiag/config.yaml /etc/netdiag/config.yaml
sudo systemctl start netdiag
```

Si restauras la base pero no `secret.key`, los canales existentes fallarán al descifrar. En ese caso elimina y vuelve a crear los canales desde la interfaz.

### Alternativa manual

Si prefieres tu propio esquema, la API de SQLite produce una copia consistente con WAL:

```bash
sudo install -d -m 0700 /var/backups/netdiag
sudo -u netdiag sqlite3 /var/lib/netdiag/netdiag.db \
  ".backup '/var/backups/netdiag/netdiag-$(date +%F).db'"
sudo tar -czf /var/backups/netdiag/netdiag-$(date +%F).tar.gz \
  -C /var/lib/netdiag secret.key reports \
  -C /etc/netdiag config.yaml
```

## Observabilidad

- **Prometheus**: `GET /metrics` expone contadores de hosts, alertas, escaneos, RTT medio, disponibilidad y tamaño de la base. Es público por defecto; define `security.metrics_token` para exigir `Authorization: Bearer <token>`.

  ```yaml
  scrape_configs:
    - job_name: netdiag
      authorization:
        credentials: <token>   # omitir si metrics_token está vacío
      static_configs:
        - targets: ['127.0.0.1:8080']
  ```

- **OpenAPI 3.1**: la especificación se publica en `/api/v1/openapi.json` y puede importarse en Insomnia, Postman o generadores de clientes.
- **Healthcheck**: `/healthz` responde `{"status":"ok"}` sin autenticación y sin tocar la base, apto para Sondeos de Kubernetes o `systemd` watchdog.
- **Auditoría**: las operaciones sensibles (usuarios, configuración, copias) se registran con actor, acción y objetivo.

## Actualizaciones

```bash
git pull
make build            # o make build-pcap
sudo install -m755 bin/netdiag /usr/local/bin/netdiag
sudo setcap cap_net_raw,cap_net_admin,cap_net_bind_service=+ep /usr/local/bin/netdiag
sudo systemctl restart netdiag
```

La base se migra automáticamente al arrancar. Antes de actualizar entre versiones mayores, haz una copia de seguridad. Las migraciones son idempotentes y no borran datos, pero no hay vuelta atrás automática.

El binario se puede reemplazar en caliente sin tocar `/var/lib/netdiag` ni `/etc/netdiag`. Los escaneos en curso se marcan como interrumpidos al reiniciar y el planificador los retomará en el siguiente ciclo.

## Operación

```bash
sudo systemctl status netdiag
sudo journalctl -u netdiag -f
curl -fsS http://127.0.0.1:8080/healthz
```

- **Logs**: `journalctl` recoge la salida estándar. Para logs JSON, usa `log.format: json` y envíalos a tu colector.
- **Salud**: `GET /healthz` devuelve `{"status":"ok"}` sin autenticación; úsalo en el balanceador o en tu monitorización.
- **Recuperar la contraseña**: `sudo -u netdiag netdiag user passwd admin`. También puedes crear otro administrador con `user add`.
- **Capacidades**: si el servicio no puede hacer ping o ARP, comprueba `getcap /usr/local/bin/netdiag` y que la unidad tenga `AmbientCapabilities`. En contenedores necesitas `--cap-add=NET_RAW --cap-add=NET_ADMIN`.
- **SELinux**: en Fedora/RHEL, si el servicio no arranca, revisa `ausearch -m avc -ts recent`. Puedes ejecutar en modo permisivo para diagnosticar, pero no lo dejes así.
- **Puerto ocupado**: `ss -ltnp | grep 8080`. Cambia `server.listen` o el flag `--listen`.
- **Datos en otra ruta**: edita `storage.data_dir` y `storage.db`, mueve el contenido y ajusta `ReadWritePaths` de la unidad systemd.

## Contenedores (opcional)

NetDiag no necesita contenedor, pero el binario portable funciona en cualquier imagen base (incluidas `alpine` o `distroless`). Si lo empaquetas tú:

- Monta `/var/lib/netdiag` y `/etc/netdiag` como volúmenes persistentes.
- Ejecuta como usuario no root y añade `--cap-add=NET_RAW --cap-add=NET_ADMIN`.
- Usa `--network host` para descubrir la LAN real; con la red bridge de Docker solo verás contenedores.
- Pasa `NETDIAG_LISTEN=0.0.0.0:8080` o edita `server.listen` en la configuración montada.

La configuración montada debe ser escribible si quieres guardar cambios desde la interfaz; si la montas como solo lectura, esos cambios fallarán y deberás editar el archivo a mano.

## Checklist de puesta en marcha

1. `netdiag doctor` sin avisos (ARP y privilegios incluidos).
2. Servicio activo y `healthz` respondiendo.
3. Cuenta de administrador creada y contraseña guardada en tu gestor.
4. Red local añadida en **Ajustes → Redes** y primer escaneo completado.
5. Monitor activado con el intervalo deseado.
6. Al menos un canal de alerta probado con **Enviar prueba**.
7. Copia de seguridad programada, incluyendo `secret.key`.
8. Acceso publicado por HTTPS con `trust_proxy` correctamente configurado.
