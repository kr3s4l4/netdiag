# Guía de endurecimiento — NetDiag

Checklist para el despliegue. Marcar cada punto con evidencia.

## 1. Verificar antes de ejecutar

- [ ] Descargar desde el espejo oficial o el interno autorizado.
- [ ] Importar la clave pública desde `docs/public_keys.md` y comparar la huella.
- [ ] `minisign -Vm SHA256SUMS -p netdiag-release.pub`.
- [ ] `sha256sum -c SHA256SUMS`.
- [ ] `netdiag container verify --key netdiag-release.pub <archivo>.ndc` si aplica.
- [ ] No ejecutar si falla cualquier paso. Reportar por `SECURITY.md`.

## 2. Sistema y permisos

- [ ] Usuario dedicado sin login interactivo: `netdiag`.
- [ ] `<data_dir>` con permisos `0700`; ficheros con `0600`.
- [ ] `secret.key` con `0600` y copia separada y cifrada.
- [ ] Cifrado de disco o volumen: SQLite **no cifra toda la base por defecto**.
- [ ] La copia ZIP incluye `secret.key`: guardarla con el mismo nivel de secreto.
- [ ] Reloj sincronizado (NTP) para auditoría y plazos CRA.

## 3. Red y exposición

- [ ] Escuchar solo en loopback o en interfaz interna.
- [ ] Publicar detrás de proxy inverso con HTTPS.
- [ ] Cambiar la contraseña inicial.
- [ ] Roles de menor privilegio: `admin`/`owner`/`operator`/`viewer`.
- [ ] Definir `security.metrics_token` si `/metrics` no está en loopback.
- [ ] No publicar NetDiag directamente en Internet sin autenticación y TLS.
- [ ] Revisar `allow_private_targets` antes de activar webhooks a rangos privados (guardia SSRF).

## 4. Escaneo responsable

- [ ] Documentar la autorización de las redes a analizar.
- [ ] Aplicar exclusiones y perfil conservador en equipos antiguos.
- [ ] Usar ventana de mantenimiento si el escaneo puede saturar o disparar IDS.
- [ ] La edición portable **no captura 802.11 crudo**. No darla por válida para esa necesidad.

## 5. Datos y retención

- [ ] Fijar la retención del plan: 7 días / 1 mes / 1 año / 7 años.
- [ ] Configurar solo hacia abajo; no ampliar sin licencia.
- [ ] Activar copias automáticas.
- [ ] Probar una restauración completa antes de producción.
- [ ] Revisar exportaciones JSON/CSV/PDF/ZIP antes de compartirlas.

## 6. Actualizaciones y seguridad

- [ ] Elegir espejo oficial o interno; en air-gapped importar `.ndc` firmado.
- [ ] Verificar cada actualización con los pasos de la sección 1.
- [ ] Suscribir el correo de seguridad y designar contacto CRA.
- [ ] Conocer `REVOCATION.md` y el `min_version` del manifiesto F23.
- [ ] Revisar `SECURITY.md` cada trimestre.

Firmado por `[responsable]` — Fecha `[AAAA-MM-DD]`
