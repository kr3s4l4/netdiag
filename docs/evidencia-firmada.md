# Evidencia firmada: informes, diffs y snapshots

NetDiag reutiliza el contenedor abierto `.ndc` para convertir un informe en
prueba íntegra. No hay nube, telemetría ni licencia online. La firma se hace
**en local**, con una clave distinta de la clave de release.

## Qué se puede firmar

```text
PDF de inventario o informe ejecutivo.
JSON de diff de histórico (F21) con resumen narrativo (F57).
Snapshot de auditoría o bundle de sede (SiteBundle).
```

Un contenedor `.ndc` es un ZIP normal:

```text
manifest.json        manifiesto canónico con schema, versión, kind, fecha y sha256
payload/             ficheros originales, sin modificar
signature.minisig    firma minisign sobre manifest.json
sbom.cdx.json        opcional
```

Cualquier persona puede abrirlo con `unzip`. NetDiag solo hace falta para
verificar la firma y los digests.

## Clave dedicada

**No reutilices la clave de release `c98b13d8c8f73259`.** Esa clave firma
binarios. La evidencia usa otra clave Ed25519 creada y guardada en local.

```bash
# En local, fuera del repo. Nunca pegues la semilla en un chat.
mkdir -p "$HOME/.netdiag-evidence"
umask 077
minisign -G -p "$HOME/.netdiag-evidence/evidence.pub" \
  -s "$HOME/.netdiag-evidence/evidence.key" \
  -c "NetDiag evidence key - $(date +%F)"
```

`netdiag container build` espera un JSON con la semilla y el `key_id`:

```json
{"seed":"BASE64_DE_LA_SEMILLA","key_id":"HEX16"}
```

Guarda ese JSON con permisos `600` y fuera del repo. La clave pública sí se
puede distribuir y pegar en la documentación del cliente.

## Firmar

```bash
NETDIAG_BIN=./bin/netdiag scripts/sign-evidence.sh \
  --seed-file "$HOME/.netdiag-evidence/evidence-seed.json" \
  --key-pub "$HOME/.netdiag-evidence/evidence.pub" \
  --version 1.5.2 --out evidencia-sede-a.ndc \
  informe-ejecutivo.pdf diff.json snapshot.json
```

Para un bundle de sede:

```bash
NETDIAG_BIN=./bin/netdiag scripts/sign-evidence.sh \
  --seed-file "$HOME/.netdiag-evidence/evidence-seed.json" \
  --key-pub "$HOME/.netdiag-evidence/evidence.pub" \
  --version 1.5.2 --kind site --out sede-a.ndc \
  site-bundle.json
```

El script firma y **verifica de inmediato**. Si la verificación falla, el
comando termina con error y no hay salida «válida».

## Verificar

```bash
./bin/netdiag container inspect evidencia-sede-a.ndc
./bin/netdiag container verify \
  --key "$HOME/.netdiag-evidence/evidence.pub" evidencia-sede-a.ndc
```

`inspect` no afirma que la firma sea válida. `verify` comprueba firma,
comentario, manifiesto y sha256 de cada fichero.

Salida esperada:

```text
Firma válida
Contenido íntegro
```

Si se modifica un fichero del payload, `verify` falla aunque el ZIP siga
abriéndose con `unzip`.

## Equivalente manual

```bash
./bin/netdiag container build \
  --version 1.5.2 --kind evidence --producer netdiag-operator \
  --seed-file "$HOME/.netdiag-evidence/evidence-seed.json" \
  --out evidencia.ndc informe.pdf diff.json

./bin/netdiag container verify \
  --key "$HOME/.netdiag-evidence/evidence.pub" evidencia.ndc
```

## Dossier local (F68)

`evidence.dossier` genera el mismo tipo de `.ndc` de forma programada dentro de
`netdiag serve`, como las copias de seguridad. Incluye `diff.json`,
`informe-ejecutivo.pdf`, `auditoria.json`, `audit-snapshots.json`,
`audit-diff.json` y `dossier.json` de los últimos `days` días, conserva los
últimos `keep` archivos y no envía nada a ningún destino.

```yaml
evidence:
  dossier:
    enabled: true
    interval_hours: 24
    days: 30
    keep: 7
    seed_file: /home/netdiag/.netdiag-evidence/evidence-seed.json
    key_pub: /home/netdiag/.netdiag-evidence/evidence.pub
```

La semilla **no se escribe en `config.yaml`**: solo la ruta, con permisos
`0600`. Las variables `NETDIAG_EVIDENCE_DOSSIER_*` permiten sobreescribir cada
campo. Si falta la ruta de la semilla o la clave pública, la configuración no
valida.

```bash
# Generación manual; útil antes de activar el programador
./bin/netdiag evidence dossier --site-id 1 --days 30 \
  --seed-file "$HOME/.netdiag-evidence/evidence-seed.json" \
  --key-pub "$HOME/.netdiag-evidence/evidence.pub"

# Verificación con la misma clave pública
./bin/netdiag container verify \
  --key "$HOME/.netdiag-evidence/evidence.pub" \
  /var/lib/netdiag/dossiers/netdiag-dossier-20261119-120000.ndc
```

## Sondas firmadas (F49)

`netdiag probe` puede firmar su JSON con el mismo formato de semilla local. La
firma cubre el documento canónico con `signature: null`; `netdiag probe import`
exige `--key` cuando la sonda está firmada. No hay nube ni envío automático. El
procedimiento está en [escaneo-remoto.md](escaneo-remoto.md).

## Límites

```text
La firma prueba integridad y origen de la clave, no que el análisis sea correcto.
No cifra el contenido: quien tiene el .ndc puede leer el informe.
No comprueba que la clave pública pertenezca a una organización concreta.
No sustituye al control de acceso del servidor donde se generó el informe.
No envía nada a terceros.
```

## Generación local

La firma de evidencia se genera en local. NetDiag es software libre y no necesita
servicios externos. No recoge ni envía datos de la red. La antigua métrica de
bandas quedó sin vigor el 20/11/2026.
