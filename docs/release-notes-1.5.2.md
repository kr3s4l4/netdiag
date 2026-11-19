# NetDiag 1.5.2

Release pendiente de publicación. Incluye los commits locales `3d561a7`, `35377e4`, `8f15b49` y `b6accb6`, más F27 y detección de dependencias. Los tags `v1.5.0` y `v1.5.1` no se mueven.

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
- La firma se comprueba con la clave pública `netdiag-release` de `docs/public_keys.md`.

## Verificación

Los hashes y el resultado de `scripts/verify-release.sh` se completarán al generar los artefactos de `1.5.2` en local.
