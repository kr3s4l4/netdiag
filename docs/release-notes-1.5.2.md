# NetDiag 1.5.2

Release publicada y verificada. Incluye los 8 commits locales posteriores a `v1.5.1`. Los tags `v1.5.0` y `v1.5.1` no se mueven.

Firmada con la clave oficial `netdiag-release`: `RWTJixPYyPcyWf0VEHdzn6lRYxgrPS3z+EbRBKlI9E+hX0lj1227yrZY` (key_id `c98b13d8c8f73259`). Sin telemetría, licencia online ni DRM.

Commit de build: `4fb0968`. Toolchain: Go 1.27.4. Época reproducible: `1795090003` (`2026-11-19T12:06:43Z`). Cualquier regeneración desde otro commit cambiará los hashes.

## Interfaz y uso

- F27: asistente de primer arranque. Tras crear la cuenta de administrador se detecta la red local, se propone un CIDR con perfil `common`, se lanza el primer escaneo con progreso visible, se guarda una línea base de auditoría y se ofrece el tour guiado sobre datos reales. Se puede omitir.
- F28: panel de salud de red con puntuación 0–100 explicable y desglose 40/20/20/20 con acceso a la evidencia.
- F29: buscador global con `Ctrl+K` para hosts, alertas y redes.
- F30: centro de notificaciones con campana, historial por sede y eventos persistentes.
- F31: diagnósticos accionables desde cada hallazgo: ver host, copiar comando y crear regla.
- F32: informe ejecutivo PDF de una página desde `/reports`, con redacción de credenciales y pie controlado.

## Seguridad

- F41: `min_version` firmado en el manifiesto. Runbook ensayado y cerrado como documentación fuera del repo. Sin envíos automáticos a ENISA ni al CSIRT.
- Dependabot semanal para el módulo Go y script local `scripts/govulncheck.sh`. Sin telemetría ni reporting automático.
- Toolchain mínima Go 1.27.4: corrige `GO-2026-6859` en `net/http`.

## Distribución y licencias

- Descubrimiento y escaneo ilimitados.
- Cobranza por banda de hosts gestionados, nunca por informe, alerta, escaneo, usuario o host detectado.
- La firma se comprueba con la clave pública `netdiag-release` de `docs/public_keys.md`.

## Verificación local

`scripts/verify-release.sh --dir dist/release/1.5.2 --version 1.5.2` devolvió **VÁLIDO** con la clave oficial: firmas, hashes, manifiesto F23, contenedor `.ndc`, checksum del binario, versión mínima `1.5.2` y versión `1.5.2`. Sin conexión y sin ejecutar el binario. `govulncheck` con Go 1.27.4 devolvió `No vulnerabilities found`.

| Artefacto | SHA256 |
|---|---|
| `netdiag-linux-amd64` | `516ccd5310b3d0b9c73347bbc8ed9846a2e28872130925584399e20ee21d1626` |
| `netdiag-linux-arm64` | `1e8a25ac24ba06bc0cd28e9edc582ec749f0b649cc9f00a5157544cd0cab1214` |
| `manifest.json` | `bdb27c7c92ae8aeef962ba053268aa546a80191bc5f1321028c7138d53a7d417` |
| `sbom.cdx.json` | `765ab2b3a2c626a3a82e601293c121ccc9b2d94e5783d36337995465eee6b66d` |
| `provenance.intoto.jsonl` | `3cb5850685d4bc7f968d232ed615b424eb9dbe0a390645686cfeed9c51910511` |

El hash de `RELEASE_NOTES.md`, del contenedor `.ndc` y de `SHA256SUMS` se recalcula al regenerar el paquete. La fuente de verdad es `SHA256SUMS`, firmado con la clave oficial incluida como `release.pub` en este directorio.

## Regeneración

```bash
GOTOOLCHAIN=go1.27.4 SOURCE_DATE_EPOCH=1795090003 \
bash scripts/release.sh --version 1.5.2 \
  --signing-json RUTA_PRIVADA/signing.json \
  --public-key RUTA_PRIVADA/release.pub \
  --notes-file docs/release-notes-1.5.2.md \
  --base-url https://github.com/kr3s4l4/netdiag/releases/download/v1.5.2 \
  --min-version 1.5.2
bash scripts/verify-release.sh --dir dist/release/1.5.2 --version 1.5.2
```

La ruta privada contiene el material desencriptado de `netdiag-release.key`. La contraseña se introduce en local; nunca se pega en el chat ni se guarda en el repo.
