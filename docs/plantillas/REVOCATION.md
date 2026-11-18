# REVOCACIÓN DE ARTEFACTO — PLANTILLA

**Plantilla. No publicar tal cual.** Copiar a la raíz del espejo oficial como `REVOCATION.md` y firmar con la clave raíz. Nunca reemplazar un archivo ya publicado: publicar una versión nueva.

---

# REVOCATION — NetDiag `[ID-REV-AAAA-NNN]`

**Fecha UTC:** `[AAAA-MM-DDTHH:MM:SSZ]`  
**Estado:** `[REVOCADO / RETIRADO DE OFICIAL / SUSTITUIDO]`  
**Motivo:** `[compromiso de clave de release / vulnerabilidad explotada / build no reproducible / error de publicación]`

## Artefactos revocados

| Archivo | Versión | SHA256 |
| --- | --- | --- |
| `[netdiag_x.y.z_linux_amd64]` | `[x.y.z]` | `[sha256]` |
| `[manifest.json]` | `[x.y.z]` | `[sha256]` |

## Versiones afectadas

- Afectadas: `[x.y.z, ...]`
- Primera versión segura: `[x.y+1.z]`
- `min_version` del manifiesto F23: `[x.y+1.z]`
- Si F23 aún no consume `min_version`: retirar el release del espejo y publicar la versión correctiva. No dejar el objeto como oficial.

## Qué deben hacer los clientes

1. **No ejecutar** la versión revocada.
2. Actualizar a `[versión segura]`.
3. Verificar antes de instalar: `docs/publicacion-segura.md`.
4. Si no hay conexión: importar el `.ndc` firmado con la versión correctiva.
5. Revisar `SECURITY.md` y el correo de seguridad recibido.

## Comunicación realizada

- [ ] Correo a clientes de pago.
- [ ] Notificación en UI.
- [ ] Entrada en el centro de notificaciones.
- [ ] Aviso CRA 24 h / 72 h si procede.
- [ ] Clave de release rotada si el motivo fue su compromiso.

## Firma

Firmar con la clave raíz, guardada offline:

```bash
minisign -Sm REVOCATION.md -s release-root.key \
  -c "revocation:[ID-REV-AAAA-NNN] version:[x.y.z] min_version:[x.y+1.z]"
```

Publicar `REVOCATION.md` y `REVOCATION.md.minisig` en la raíz del espejo. Actualizar `docs/public_keys.md` si hay rotación de clave.

Firmado, `[nombre]` — `[cargo]` — `[razón social]`
