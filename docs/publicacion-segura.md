# Publicación segura de NetDiag

Este documento define cómo se construye, firma y distribuye una versión oficial. Es el complemento operativo de F23 (actualización firmada), F36 (contenedor `.ndc`) y F37 (pipeline de publicación).

## Principios

- Un binario oficial debe ser **reproducible** desde un commit público.
- Todo artefacto descargable lleva **SHA256 verificado y firma minisign**.
- La clave de firma vive **offline**, preferiblemente en hardware; nunca en el servidor de publicación.
- El canal es **HTTPS inmutable**; nunca se publica por HTTP ni con credenciales en la URL.
- El cliente puede verificar antes de ejecutar, y puede sustituir el espejo oficial por uno interno.
- No se publica una versión hasta que sus notas, SBOM y manifiesto estén firmados.

## Jerarquía de claves

| Clave | Uso | Almacenamiento | Rotación |
| --- | --- | --- | --- |
| `netdiag-root` | FirmaOnly el certificado de rotación de la clave de release | Air-gapped, 2 personas, HSM/YubiKey | 3 años o incidente |
| `netdiag-release` | Firma de `SHA256SUMS`, `manifest.json`, notas y `.ndc` | Air-gapped/HSM, nunca en CI | 12 meses o incidente |
| Clave de prueba | Solo para smoke tests en loopback | Generada por test, efímera | Cada prueba |

La clave pública oficial se publicará en `docs/public_keys.md`. **Mientras ese archivo no contenga una clave real, no existe una versión oficial verificable**: las builds locales son solo de desarrollo.

La rotación se hace con un fichero `KEY_ROTATION.md` firmado por la clave raíz que indica `old_key`, `new_key`, `valid_from` y motivo. El cliente actualiza `update.public_key` solo después de verificar esa rotación con la raíz previamente confiada.

## Artefactos por versión

Para `v1.5.0`, el directorio inmutable del espejo contiene:

```text
releases/v1.5.0/
  netdiag_1.5.0_linux_amd64
  netdiag_1.5.0_linux_arm64
  netdiag_1.5.0_linux_amd64_pcap
  netdiag_1.5.0_linux_arm64_pcap
  netdiag_1.5.0_linux_amd64.ndc
  SHA256SUMS
  SHA256SUMS.minisig
  manifest.json
  manifest.json.minisig
  RELEASE_NOTES.md
  RELEASE_NOTES.md.minisig
  sbom.cdx.json
  provenance.intoto.jsonl
```

`manifest.json` sigue exactamente el formato consumido por F23:

```json
{
  "schema": 1,
  "version": "1.5.0",
  "url": "https://updates.netdiag.example/releases/v1.5.0/netdiag_1.5.0_linux_amd64",
  "checksum": "<sha256 del binario>",
  "released_at": "2026-11-18T00:00:00Z",
  "notes": "https://updates.netdiag.example/releases/v1.5.0/RELEASE_NOTES.md"
}
```

El comentario de confianza de la firma debe ligar `version:1.5.0` y `sha256:<checksum del manifiesto>`. Ver `internal/update/` para la canonicalización exacta; no se cambia el orden de campos ni se añaden espacios.

## Build reproducible

```bash
export VERSION=1.5.0
export COMMIT=$(git rev-parse HEAD)
export SOURCE_DATE_EPOCH=$(git show -s --format=%ct HEAD)

git status --porcelain          # debe estar limpio
go mod verify
go mod download

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
  go build -trimpath -buildvcs=false -ldflags='-s -w -buildid=' \
  -o dist/netdiag_${VERSION}_linux_amd64 ./cmd/netdiag

CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
  go build -trimpath -buildvcs=false -ldflags='-s -w -buildid=' \
  -o dist/netdiag_${VERSION}_linux_arm64 ./cmd/netdiag
```

La variante `pcap` necesita `libpcap-dev` y CGO; por no ser reproducible bit a bit entre distribuciones, se publica con el **digest del contenedor de build** en las notas. La variante portable es la referencia para el manifiesto F23.

**Prueba de reproducibilidad:** repetir el build desde el mismo commit en otra máquina y comparar `sha256sum`. Si difiere, la versión no se publica.

## Firma

Con la clave de release en hardware/offline:

```bash
cd dist

sha256sum netdiag_${VERSION}_* manifest.json RELEASE_NOTES.md sbom.cdx.json > SHA256SUMS
minisign -S -s "$RELEASE_KEY" -m SHA256SUMS \
  -x SHA256SUMS.minisig -t "NetDiag ${VERSION} SHA256SUMS"

minisign -S -s "$RELEASE_KEY" -m manifest.json \
  -x manifest.json.minisig -t "version:${VERSION} sha256:$(sha256sum manifest.json | cut -d' ' -f1)"

minisign -S -s "$RELEASE_KEY" -m RELEASE_NOTES.md \
  -x RELEASE_NOTES.md.minisig -t "version:${VERSION} notes"
```

El contenedor `.ndc` se firma con el mismo esquema que F36: la firma cubre `manifest.json` interno, que a su vez enumera el SHA256 de cada fichero del `payload/`. Nunca se firma un ZIP directamente sin manifiesto interno.

## Formato `.ndc` (F36)

Un `.ndc` es un **ZIP normal**. Cualquiera puede abrirlo con `unzip -l`; NetDiag solo hace falta para verificar la firma y los digests. Es un formato abierto y versionado (`schema: netdiag/container/v1`).

| Entrada del ZIP | Contenido |
| --- | --- |
| `manifest.json` | Manifiesto canónico firmado (JSON compacto, ficheros ordenados por ruta) |
| `payload/<nombre>` | Un fichero por entrada de `manifest.files` |
| `sbom.cdx.json` | Opcional; CycloneDX |
| `signature.minisig` | Firma minisign prehashed (`ED`) sobre los bytes exactos de `manifest.json` |

Ejemplo de manifiesto:

```json
{
  "schema": "netdiag/container/v1",
  "version": "1.5.0",
  "kind": "release",
  "created_at": "2026-11-18T12:00:00Z",
  "files": [
    { "path": "netdiag", "sha256": "…", "size": 12345678 },
    { "path": "RELEASE_NOTES.md", "sha256": "…", "size": 2048 }
  ],
  "producer": "netdiag"
}
```

`kind` vale `release`, `evidence` o `site`. Un contenedor de **evidencia** puede llevar el informe ejecutivo, el diff de auditoría y el snapshot de configuración **sin secretos**. El comentario de confianza de la firma liga `version:<versión>`, `sha256:<digest del contenido>` y, si existe SBOM, `sbom:<sha256 del SBOM>`. El digest del contenido es SHA256 de las líneas `<sha256>  <ruta>` ordenadas por ruta.

### Comandos

```sh
# Inspeccionar sin confiar en el contenido: muestra manifiesto y comprueba digests
netdiag container inspect netdiag_1.5.0_linux_amd64.ndc
netdiag container inspect --json evidencia.ndc

# Verificar firma, comentario firmado, SBOM y todos los ficheros
netdiag container verify --key netdiag-release.pub netdiag_1.5.0_linux_amd64.ndc
netdiag container verify --key 'RWTJixPYyPcyWf0VEHdzn6lRYxgrPS3z+EbRBKlI9E+hX0lj1227yrZY' evidencia.ndc

# Crear un contenedor (solo el emisor, con la clave de release)
netdiag container build --version 1.5.0 --kind evidence \
  --sbom sbom.cdx.json --seed-file signing.json \
  --out evidencia.ndc informe.md diff.json
```

`--key` admite un archivo `.pub` de dos líneas o la clave base64 desnuda. `verify` devuelve código de salida `1` si la firma no es válida, si el contenido no coincide o si falta la firma; `inspect` siempre muestra lo que hay, incluso en un contenedor manipulado.

El `--seed-file` de `build` es un JSON local con la semilla Ed25519 y el id de clave minisign:

```json
{ "seed": "base64 de 32 bytes", "key_id": "hex de 8 bytes" }
```

Ese archivo se guarda fuera del repositorio con permisos `0600` y no se distribuye. `--key-id HEX16` lo sobrescribe si hace falta.

### Protecciones

- Rutas relativas, sin `..`, sin barras iniciales ni contrabarras: un `.ndc` malicioso no puede escribir fuera de `payload/`.
- Entradas ZIP duplicadas o desconocidas se rechazan.
- Límites: 1 MiB de manifiesto, 8 KiB de firma, 1 GiB por fichero y 10 000 ficheros.
- `manifest.json` debe estar en forma canónica byte a byte; cualquier reordenación o campo extra se rechaza antes de verificar.
- La fecha de modificación de las entradas ZIP es fija (1980-01-01) para que el mismo contenido produzca el mismo archivo, base del trabajo reproducible de F37.

## SBOM y procedencia

- Generar `sbom.cdx.json` en CycloneDX con `syft packages ./... -o cyclonedx-json=sbom.cdx.json` (o equivalente).
- Incluir `go version -m <binario>` y `go list -m all` como anexo de `provenance.intoto.jsonl`.
- La atestación registra: commit, tag, Go version, `SOURCE_DATE_EPOCH`, comando de build, autor de la firma y OAuth/OIDC del runner si aplica.
- `THIRD_PARTY_LICENSES.md` debe corresponder al mismo grafo de módulos que el SBOM.

## Verificación del cliente

```bash
# 1) Descargar clave pública oficial desde docs/public_keys.md
curl -fsSLO https://updates.netdiag.example/releases/v1.5.0/SHA256SUMS
curl -fsSLO https://updates.netdiag.example/releases/v1.5.0/SHA256SUMS.minisig

# 2) Verificar la firma antes de mirar los hashes
minisign -Vm SHA256SUMS -p netdiag-release.pub

# 3) Verificar los archivos descargados
sha256sum -c SHA256SUMS

# 4) Verificar el contenedor firmado (F36)
netdiag container verify --key netdiag-release.pub netdiag_1.5.0_linux_amd64.ndc

# 5) Verificar el manifiesto que consumirá F23
minisign -Vm manifest.json -p netdiag-release.pub
```

Si cualquiera de estos pasos falla, el cliente no ejecuta el binario y reporta el evento por `SECURITY.md`.

## Publicación

- El tag de git (`v1.5.0`) es inmutable y se firma con GPG del mantenedor.
- GitHub Release contiene código fuente, binarios, firmas, SBOM y notas; el espejo propio es la URL canónica del manifiesto.
- El servidor de espejo sirve solo objetos versionados; un `PUT` sobre `v1.5.0` está prohibido por política del bucket.
- Cabeceras: `Strict-Transport-Security`, `Content-Type` correcto y `Cache-Control: public, max-age=31536000, immutable`.
- La página de descarga muestra versión, commit, fecha y huella de la clave pública.
- No se usan URLs con `user:pass@host`; F23 las rechaza y el pipeline también debe hacerlo.

## Espejo Enterprise / air-gapped

1. Se entrega un `.ndc` firmado con binario, manifiesto, notas y SBOM.
2. El cliente lo importa en su red y lo verifica con la clave pública documentada.
3. Publica `manifest.json` y el binario en un servidor HTTPS interno (o loopback para pruebas).
4. Configura `update.manifest_url` y `update.public_key` en `config.yaml`.
5. NetDiag aplica la actualización sin salir a Internet.

La licencia Enterprise no exige conectividad; el espejo es una decisión operativa del cliente.

## Dependencias de terceros

El productor responde ante el cliente y ante el regulador por el producto terminado, incluso cuando el defecto esté en una dependencia incluida. Eso no significa pagar todo de tu bolsillo en exclusiva: significa que **tú gestionas el aviso, el parche y la comunicación**, y después reclamas aguas arriba si tienes contrato con el proveedor.

Reglas operativas:

- `go.sum`, SBOM y `go version -m` identifican exactamente qué versión de cada dependencia se entregó.
- Monitorizar avisos de seguridad de dependencias de forma periódica; un CVE relevante se trata como incidente interno aunque aún no haya explotación conocida.
- **Nunca firmar un binario de terceros tal cual.** Si un componente se distribuye junto a NetDiag, se reconstruye o se verifica y se documenta su procedencia antes de incluirlo en un artefacto firmado.
- La corrección se entrega como **release nueva de NetDiag** con su manifiesto firmado. El cliente no debe tener que parchear dependencias por su cuenta.
- Si el proveedor no corrige, se sustituye la dependencia o se documenta la excepción con fecha de revisión. No se silencia el hallazgo.

## Incidentes y revocación

- Si se sospecha compromiso de la clave de release: revocar el artefacto, publicar `REVOCATION.md` firmado por la raíz y rotar la clave de release.
- Marcar versiones afectadas con `min_version` en el manifiesto para forzar salto a la siguiente segura.
- Retirar el release del espejo: no borrar el objeto, sino dejar de servirlo como oficial y publicar la versión correctiva.
- Comunicar a clientes de pago por correo, notificación `update.available`/`update.applied` y entrada en el centro de notificaciones. En air-gapped se entrega un `.ndc` firmado con el aviso.
- Si hay explotación activa o incidente grave, reportar en **24 h / 72 h** según `SECURITY.md` y el CRA; no esperar a tener parche.
- `SECURITY.md` define el canal privado, los plazos de respuesta y el responsable de la notificación.
- Nunca se reemplaza un archivo publicado: se publica una versión nueva. Eso preserva la cadena de custodia.
