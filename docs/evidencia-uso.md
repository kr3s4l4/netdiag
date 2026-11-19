# Evidencia de uso voluntaria (F62)

`netdiag evidence usage` crea un contenedor `.ndc` firmado con contadores agregados de la instalación local. Sirve para que una persona **decida por su cuenta** compartir una señal de uso. NetDiag no la envía a ningún sitio.

## Qué contiene

```json
{
  "schema": "netdiag.usage/v1",
  "generated_at": "...",
  "version": "1.5.2-dev",
  "installed_days": 12,
  "scans": 8,
  "sites": 2,
  "managed_host_bands": {
    "1-5": 1,
    "6-10": 1,
    "11-50": 0,
    "51-200": 0,
    "201+": 0
  }
}
```

- `installed_days`: días desde el alta del primer usuario local. Sin usuarios, `0`.
- `scans`: escaneos iniciados desde que existe la base.
- `sites`: número de sedes.
- `managed_host_bands`: cuántas sedes caen en cada banda de hosts gestionados vistos en los últimos 30 días, sin etiquetas `guest` ni `excluded`.

**Nunca contiene:** IPs, MACs, nombres de host, nombres de sede, usuarios, rutas, secretos ni metadatos de red. Solo enteros y bandas.

## Qué no es

No es telemetría. No hay red, ni envío automático, ni nube. No cuenta como evidencia de F42: F42 sigue exigiendo 3 instalaciones reales con 7 días, 2 casos escritos por usuarios y 1 disposición a pagar. La frase se imprime al terminar el comando.

## Generar y verificar

La clave de firma es la clave dedicada de evidencia F58, distinta de la clave de release. Para una prueba local:

```bash
go run ./cmd/netdiag-release keygen --out /tmp/netdiag-usage-keys
./bin/netdiag evidence usage \
  --data-dir /tmp/mi-red \
  --seed-file /tmp/netdiag-usage-keys/signing.json \
  --out /tmp/netdiag-usage-keys/usage.ndc

./bin/netdiag container verify \
  --key /tmp/netdiag-usage-keys/release.pub \
  /tmp/netdiag-usage-keys/usage.ndc
```

Salida esperada: `Firma: válida`, `Contenido: correcto`, `Resultado: VÁLIDO`.

También se puede inspeccionar el contenido sin verificar la firma:

```bash
unzip -p /tmp/netdiag-usage-keys/usage.ndc payload/usage.json
```

## Criterio de cierre

- Contenedor firmado y verificado con `container verify`.
- Payload sin identificadores de red, comprobado por tests.
- Documento que deja explícito que no sustituye la evidencia escrita de F42.
