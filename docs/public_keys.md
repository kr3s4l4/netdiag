# Claves públicas de release

Aquí se publicarán las claves minisign oficiales de NetDiag.

> **Estado:** pendiente de generación. No hay ninguna clave oficial publicada todavía. Mientras este archivo no contenga una clave real y su huella verificada, **ninguna versión debe considerarse oficial**. Las builds locales son solo de desarrollo.

## Formato esperado

Cuando se generen, cada clave se publicará con este formato:

```text
Clave: netdiag-release
Uso: firma de SHA256SUMS, manifest.json, notas y contenedores .ndc
Creada: YYYY-MM-DD
Caduca/rota: YYYY-MM-DD
Huella SHA256 de la clave pública codificada: <64 hex>
Comentario minisign: untrusted comment

RWQf6...
```

La clave raíz `netdiag-root` se publicará de la misma forma y solo servirá para firmar rotaciones de la clave de release, según `docs/publicacion-segura.md`.

## Cómo verificar la huella por un canal independiente

1. Descargar este archivo desde el repositorio oficial y desde el espejo HTTPS.
2. Comparar la huella con la mostrada en la página de descargas y en las notas firmadas.
3. Si las huellas no coinciden, detener la actualización y reportar por `SECURITY.md`.

## Rotación

Toda rotación se anuncia en `KEY_ROTATION.md`, firmado por la clave raíz previamente confiada. El cliente acepta la nueva clave solo después de verificar esa firma. No se aceptan claves nuevas recibidas por correo sin esa cadena de confianza.
