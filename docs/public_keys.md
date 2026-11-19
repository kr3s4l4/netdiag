# Claves públicas del proyecto

Esta página publica las claves minisign oficiales de NetDiag. Solo las claves que aparecen aquí son válidas para verificar artefactos; cualquier clave recibida por correo o por otro canal debe ignorarse.

## `netdiag-release` — activa desde 2026-11-18

- **Uso:** firma de `SHA256SUMS`, `manifest.json` (F23), notas de release y contenedores `.ndc`.
- **Creada:** 2026-11-18.
- **Comentario minisign:** `minisign public key 5932F7C8D8138BC9`
- **Huella SHA256:** `7900c17acd0163e16e0df59a24e82e50f05a035c0cbf692d78a14480066bc06b`
  (SHA256 de los 42 bytes de la clave pública decodificada desde base64)

```text
untrusted comment: minisign public key 5932F7C8D8138BC9
RWTJixPYyPcyWf0VEHdzn6lRYxgrPS3z+EbRBKlI9E+hX0lj1227yrZY
```

La clave privada `netdiag-release.key` está cifrada con contraseña, se guarda fuera del repositorio y nunca se sube a GitHub. La contraseña se conserva por separado, en papel.

## `netdiag-license` — Ed25519, activa desde 2026-11-18

- **Uso:** firma de licencias offline (F35). **No** sirve para verificar binarios ni actualizaciones.
- **Creada:** 2026-11-18.
- **Clave pública (base64, 32 bytes):**

```text
pNc3VNWpOKBid1TQsm6rF52MqvBxyNdvPJxvqSNha/s=
```

- **Huella SHA256:** `b4ac3d75b7c93902624636aa07ad7c867983c52bc3746119a9033b3bcb9739f1`
  (SHA256 de los 32 bytes de la clave pública decodificada desde base64)

La semilla privada se guarda como `~/.config/netdiag/license-signing.seed` con permisos `0600`, fuera del repositorio y del binario. Está compilada en el programa solo como clave pública; no hay forma de emitir licencias válidas desde una instalación normal. El detalle del formato está en `docs/licenciamiento.md`.

## `netdiag-root` — pendiente de generar

La clave raíz solo servirá para firmar rotaciones de la clave de release, según `docs/publicacion-segura.md`. Hasta que se publique aquí, cualquier rotación de `netdiag-release` debe considerarse no oficial.

## Verificación de v1.5.0

La Release `v1.5.0` publica `SHA256SUMS` y su firma `SHA256SUMS.minisig`. Para comprobarla:

```bash
BASE=https://github.com/kr3s4l4/netdiag/releases/download/v1.5.0
curl -fsSLO "$BASE/SHA256SUMS"
curl -fsSLO "$BASE/SHA256SUMS.minisig"

minisign -V \
  -P 'RWTJixPYyPcyWf0VEHdzn6lRYxgrPS3z+EbRBKlI9E+hX0lj1227yrZY' \
  -m SHA256SUMS -x SHA256SUMS.minisig

sha256sum -c SHA256SUMS
```

La firma debe devolver `Signature and comment signature verified` con el comentario de confianza `NetDiag v1.5.0 SHA256SUMS`. Después, `sha256sum -c` comprueba que los binarios descargados coinciden con el resumen firmado.

## Verificación de v1.5.1

La Release `v1.5.1` publica 14 artefactos, incluidos `SHA256SUMS`, `manifest.json`, SBOM, procedencia, notas firmadas y el contenedor `.ndc`. La comprobación remota devolvió:

```text
Trusted comment: version:1.5.1 sha256:e9c634a49cf21244396fbbd3abf8c7a9a0edaa91062ca4bf1a33aecedf9a6ee3
```

Para verificarla:

```bash
BASE=https://github.com/kr3s4l4/netdiag/releases/download/v1.5.1
curl -fsSLO "$BASE/SHA256SUMS"
curl -fsSLO "$BASE/SHA256SUMS.minisig"

minisign -V \
  -P 'RWTJixPYyPcyWf0VEHdzn6lRYxgrPS3z+EbRBKlI9E+hX0lj1227yrZY' \
  -m SHA256SUMS -x SHA256SUMS.minisig

sha256sum -c SHA256SUMS
```

El hash de `SHA256SUMS` debe ser `e9c634a49cf21244396fbbd3abf8c7a9a0edaa91062ca4bf1a33aecedf9a6ee3`. La verificación completa sin red está en `scripts/verify-release.sh`.

## Cómo verificar la huella por un canal independiente

1. Descargar este archivo desde el repositorio oficial.
2. Comparar la huella con la mostrada en las notas de la Release.
3. Si las huellas no coinciden, detener la descarga y reportar por `SECURITY.md`.

## Rotación

Toda rotación se anuncia en `KEY_ROTATION.md`, firmado por la clave raíz previamente confiada. El cliente acepta la nueva clave solo después de verificar esa cadena de confianza. Mientras `netdiag-root` no exista, la rotación se anunciará además en el perfil de GitHub y en el Discussion fijado.
