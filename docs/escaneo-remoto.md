# Escaneo remoto por SSH

NetDiag no necesita un agente remoto. Copias el binario a la sede y lo ejecutas allí.
SSH transporta el comando y el JSON resultante. **El escaneo sigue siendo local a la red remota.**

Esto no es telemetría. Los datos van de la sede a tu consola, nunca a los servidores de NetDiag.

## Requisitos

```text
Acceso SSH por clave en el host de la sede.
Binario de NetDiag ya instalado en ese host.
Red escaneada propia o con autorización escrita.
```

No uses contraseñas. El script fuerza `BatchMode=yes`.

## Uso

```bash
scripts/remote-scan.sh <usuario@host> <ruta-netdiag-remoto> <red> [perfil] [salida.json]
```

La `red` puede ser un ID de red ya configurada o un CIDR.

```bash
# Red configurada con id 3 en la sede remota
scripts/remote-scan.sh operator@172.20.50.10 /opt/netdiag/netdiag \
  3 common sede-a.json

# CIDR directo, perfil quick
scripts/remote-scan.sh operator@172.20.50.10 /opt/netdiag/netdiag \
  192.168.10.0/24 quick sede-b.json
```

Sin archivo de salida, el nombre se genera solo:

```text
scan-<host>-<red>-<fecha>.json
```

## Verificar el resultado

```bash
jq -e . sede-a.json >/dev/null && echo JSON válido
jq '.hosts | length' sede-a.json
```

Si el JSON está vacío, revisa el `stderr` de SSH. El script no mezcla errores en el archivo.

## Equivalente manual

```bash
ssh -o BatchMode=yes operator@172.20.50.10 \
  '/opt/netdiag/netdiag scan 192.168.10.0/24 --profile quick --json' > sede-b.json

ssh -o BatchMode=yes operator@172.20.50.10 \
  '/opt/netdiag/netdiag scan --network 3 --json' > sede-a.json
```

## Sonda portable (F49)

Cuando interesa observar ARP, LLDP/CDP, WiFi o VPN desde otro punto de red, el mismo binario incluye `netdiag probe`. La sonda es un JSON local con esquema `netdiag.probe/v1`; **no se envía a ningún servicio**.

```bash
# En el equipo remoto, sin privilegios especiales
/opt/netdiag/netdiag probe --site-id 1 --out sonda.json

# Firma opcional con una clave Ed25519 local (misma idea que la evidencia)
/opt/netdiag/netdiag probe --site-id 1 \
  --seed-file "$HOME/.netdiag-evidence/evidence-seed.json" \
  --out sonda-firmada.json
```

En la consola, tras copiar el archivo por SSH, USB o el mecanismo que uses:

```bash
./bin/netdiag probe import --data-dir /var/lib/netdiag sonda.json
./bin/netdiag probe import --data-dir /var/lib/netdiag \
  --key "$HOME/.netdiag-evidence/evidence.pub" sonda-firmada.json
```

Si la sonda está firmada, `import` exige `--key`; sin clave, la rechaza. La firma cubre el JSON canónico, incluido `site_id`; no cifra el contenido. `--site-id` en `import` permite reasignar la sede de destino.

Fuentes: ARP y tablas del kernel siempre que estén disponibles; SNMP/LLDP/CDP best-effort contra el gateway local; WiFi muestra `available:false` salvo compilación con `-tags pcap` e interfaz en modo monitor; VPN se lee pasivamente del kernel sin privilegios. La sonda no tiene coste aparte: NetDiag es gratuito y no hay factura ni bandas de pago desde el 20/11/2026.

## Red aislada

Si no hay conectividad entre sedes:

```bash
scp /opt/netdiag/netdiag operator@host:/tmp/netdiag
ssh operator@host '/tmp/netdiag scan 192.168.10.0/24 --json' > sede.json
# Trasladar sede.json en USB a la consola central
```

## Límites

```text
ARP, LLDP y CDP no cruzan dominios de broadcast.
Un escaneo remoto ve la red donde se ejecuta, no las demás.
No es un escáner centralizado ni un túnel SOCKS.
Para un mapa de muchas redes, hace falta un punto de observación por red: usa `netdiag probe` en cada red e importa los JSON en local.
```

## Seguridad

```text
Claves SSH dedicadas y restringidas.
Sin contraseñas y sin reenvío de agente.
Solo lectura: el escaneo no modifica la red.
Sin envío automático a terceros.
Sin cobro por sede, sonda ni escaneo.
```
