# NetDiag 1.5.2 — NO PUBLICABLE

> **NO PUBLICABLE.** Estos artefactos se han firmado con una clave de prueba (`NDTEST01`), no con la clave oficial `netdiag-release`. Sirven solo para validar el proceso de empaquetado. No subir a GitHub, no publicar en la release, no distribuir. Hay que regenerarlos con la clave oficial antes de cualquier publicación.

Clave de prueba usada: `RWRORFRFU1QwMcNMfMVU641azSiiWbkjkUmhj74oUbY6CIaMn/qBGLTt`. La clave pública oficial es `RWTJixPYyPcyWf0VEHdzn6lRYxgrPS3z+EbRBKlI9E+hX0lj1227yrZY` y **no** coincide con estos artefactos.

Commit de origen: `9bc0da3`. Toolchain: Go 1.27.4. Época reproducible: `1795090003` (`2026-11-19T12:06:43Z`). Cualquier regeneración desde otro commit cambiará los hashes.

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

## Distribución y licencias

- Sin telemetría, llamadas de licencia online ni DRM.
- Descubrimiento y escaneo ilimitados.
- Cobranza por banda de hosts gestionados, nunca por informe, alerta, escaneo, usuario o host detectado.

## Verificación local

`scripts/verify-release.sh --dir dist/release/1.5.2 --version 1.5.2` devolvió **VÁLIDO**: firmas, hashes, manifiesto F23, contenedor `.ndc`, checksum del binario, versión mínima `1.5.2` y versión `1.5.2`. Sin conexión y sin ejecutar el binario. `govulncheck` con Go 1.27.4 devolvió `No vulnerabilities found`.

| Artefacto | SHA256 |
|---|---|
| `netdiag-linux-amd64` | `e0e58bce594e413e3df7a8fe5c8eeee823301f860eefb9112e73a8c8e64cc7c2` |
| `netdiag-linux-arm64` | `57d9aa92fb3ed07d9fc6411f07d6707fa03c91b85b9270219bacdeeb9a46d865` |
| `manifest.json` | `b9f3bcb59ab338d1e18cd5a4cead3750e3c7c2d17d2d5a04c801931410fa791b` |
| `sbom.cdx.json` | `765ab2b3a2c626a3a82e601293c121ccc9b2d94e5783d36337995465eee6b66d` |
| `provenance.intoto.jsonl` | `a149d7a5e1d677c7209339d6d52b05b8ecc5c121774856988b0c9c3707480295` |

El hash de `RELEASE_NOTES.md`, del contenedor `.ndc` y de `SHA256SUMS` se recalcula al regenerar el paquete. La fuente de verdad es `SHA256SUMS`, firmado con la clave que figura en `release.pub` de este directorio.

## Comando de regeneración con clave oficial

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

La ruta privada debe contener el material desencriptado de `netdiag-release.key`. La contraseña se introduce en local; nunca se pega en el chat ni se guarda en el repo.
