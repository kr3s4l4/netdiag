# Licenciamiento de NetDiag

## Estado vigente (20/11/2026)

NetDiag es **software libre bajo AGPL-3.0** (`LICENSE`) y **gratuito**. No hay ediciones de pago, precios activos ni checkout. La antigua propuesta de ediciones Pro y Enterprise queda como documento histórico, sin vigor: `docs/comercializacion.md` y `docs/decision-comercial.md`.

La release publicada `v1.5.2` se construyó el 19/11/2026 e incluye texto comercial en sus notas. Esa release no se modifica: sus tags, hashes y claves de firma siguen intactos. Esta aclaración afecta a `master` y a la documentación.

## Uso y redistribución

Puedes usar NetDiag, estudiarlo, modificarlo y redistribuirlo cumpliendo la AGPL. Puntos importantes:

- Al distribuir NetDiag o una versión modificada, conserva los avisos de copyright y entrega la AGPL junto con el programa.
- Si ejecutas una versión modificada y la ofreces a usuarios a través de una red, la sección 13 de AGPL exige ofrecerles el **código fuente correspondiente**.
- No hay cláusula de telemetría ni restricción de uso competitivo.
- Los avisos de dependencias están en `NOTICE` y `THIRD_PARTY_LICENSES.md`.

La AGPL permite usar NetDiag dentro de una organización sin distribuirlo, y no exige pago ni licencia online.

## Activación offline (F35)

El binario sabe leer un fichero JSON firmado con **Ed25519** guardado en `<data_dir>/license.json`. La verificación es **local**: NetDiag no contacta con ningún servidor, no envía el ID de licencia ni consulta caducidades en remoto. Es una capacidad técnica del programa, no una oferta comercial. No activa funciones premium ni bloquea el uso.

Formato (`schema` = `netdiag-license/v1`):

```json
{
  "schema": "netdiag-license/v1",
  "license_id": "ND-2026-0001",
  "customer": "Nombre de la organización",
  "edition": "community",
  "max_managed_hosts": 25,
  "max_sites": 3,
  "retention_days": 30,
  "expires_at": "2027-12-31T00:00:00Z",
  "capabilities": ["reports", "alerts"],
  "support_level": "community",
  "signature": "base64(Ed25519)..."
}
```

La firma cubre todos los campos **excepto** `signature`, serializados en un orden fijo. Cambiar cualquier valor invalida la firma. `0` en `max_managed_hosts` o `max_sites` significa «sin límite». Los valores `pro` y `enterprise` siguen siendo compatibles con el formato por motivos históricos; hoy no representan ningún producto.

Clave pública del emisor (base64, 32 bytes), separada de la clave minisign de los artefactos de release:

```
pNc3VNWpOKBid1TQsm6rF52MqvBxyNdvPJxvqSNha/s=
```

La semilla privada vive **fuera del repositorio**, en `~/.config/netdiag/license-signing.seed` con permisos `0600`. No se sube a Git, no se incluye en el binario y no se comparte. Si se pierde, se genera un par nuevo y se documenta la rotación en `docs/public_keys.md`.

### Comandos

```sh
# Emitir (solo quien tiene la semilla): rellena el JSON y guárdalo con 0600
netdiag license sign licencia.json --out licencia-firmada.json

# Instalar en el equipo
netdiag license activate licencia-firmada.json

# Consultar estado (texto o JSON)
netdiag license show
netdiag license show --json

# Volver al estado sin licencia, sin borrar datos
netdiag license clear
```

`--seed-file RUTA` permite indicar otra semilla; por defecto se usa la ruta indicada arriba. Todos los comandos aceptan `--data-dir` y `--config`.

### API y avisos

`GET /api/v1/system/license` está disponible para **cualquier rol autenticado** y devuelve siempre HTTP 200:

```json
{
  "status": "active",
  "edition": "community",
  "license_id": "ND-2026-0001",
  "expires_at": "2027-12-31T00:00:00Z",
  "limits": { "max_managed_hosts": 25, "max_sites": 3, "retention_days": 30 },
  "capabilities": ["reports", "alerts"],
  "support_level": "community",
  "managed_hosts": 12,
  "sites": 2,
  "warnings": []
}
```

La respuesta **nunca** incluye `signature`, el nombre del cliente ni datos de otro cliente. `managed_hosts` cuenta las identidades `s<siteID>:<identity>` vistas en los últimos 30 días, excluyendo etiquetas `guest` y `excluded`.

Caducidad, banda excedida o firma inválida se traducen en entradas de `warnings`; el estado pasa a `expired`, `invalid` o mantiene `active`. En ningún caso se bloquean lecturas, escaneos ni informes, y jamás se borran datos. Ante una licencia inválida se aplican los límites por defecto (`retention_days: 7`, hosts y sedes sin límite) y se sigue usando el producto.

El estado también aparece en **Ajustes → Sistema → Licencia** de la interfaz web, con los avisos en un recuadro no bloqueante.

### Copias de seguridad y restauración

`license.json` se incluye en el ZIP de copia junto a `netdiag.db`, `config.yaml` y `secret.key`, y se registra en `manifest.json`. Para restaurar:

1. Restaurar la base de datos y la configuración según `docs/api.md`.
2. Extraer `license.json` del ZIP y copiarlo en `<data_dir>/license.json` con permisos `0600`, **o** volver a ejecutar `netdiag license activate licencia-firmada.json`.

Como la licencia está firmada, una copia manipulada se detecta al arrancar: se degrada al estado sin licencia con un aviso, sin impedir el acceso a los datos.

## Contribuciones

- Las contribuciones se aceptan bajo **DCO** (`Signed-off-by`). No exigimos CLA.
- Al enviar un parche declaras que tienes derecho a aportarlo bajo AGPL-3.0.
- No se incorporan dependencias con licencias incompatibles con distribución de binario sin revisión legal.
- El inventario `THIRD_PARTY_LICENSES.md` se regenera cuando cambia `go.mod`.

## Terceros

El binario incluye módulos Go bajo licencias MIT, BSD-2-Clause, BSD-3-Clause, Apache-2.0, MPL-2.0 y dominio público (SQLite), entre otras. Los avisos completos se incluyen en `THIRD_PARTY_LICENSES.md` y en el SBOM de cada release.

Si una dependencia cambia de licencia o aparece una incompatibilidad, se bloquea el release hasta sustituirla o documentar la excepción.

## Marcas

NetDiag es el nombre del proyecto. El uso de ese nombre en forks o productos derivados debe evitar sugerir respaldo oficial.

## Contacto

- Errores y preguntas técnicas: GitHub Issues del repositorio.
- Preguntas generales: GitHub Discussions. No hay soporte formal ni SLA.
- Vulnerabilidades: canal privado descrito en `SECURITY.md`.
- Cualquier propuesta comercial: quedaría sin vigor salvo decisión escrita con fecha y firmante. Hoy no hay oferta comercial.
