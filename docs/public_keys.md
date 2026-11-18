# Claves públicas de release

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

## Cómo verificar la huella por un canal independiente

1. Descargar este archivo desde el repositorio oficial.
2. Comparar la huella con la mostrada en las notas de la Release.
3. Si las huellas no coinciden, detener la descarga y reportar por `SECURITY.md`.

## Rotación

Toda rotación se anuncia en `KEY_ROTATION.md`, firmado por la clave raíz previamente confiada. El cliente acepta la nueva clave solo después de verificar esa cadena de confianza. Mientras `netdiag-root` no exista, la rotación se anunciará además en el perfil de GitHub y en el Discussion fijado.
