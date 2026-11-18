# Licenciamiento de NetDiag

NetDiag se distribuye bajo un modelo de **doble licencia**. Esta página explica qué significa para usuarios, contribuidores y compradores.

## Community: AGPL-3.0

La edición Community es software libre bajo **GNU Affero General Public License v3.0** (`LICENSE`). Puedes usarla, estudiarla, modificarla y redistribuirla cumpliendo la AGPL.

Puntos importantes:

- Si distribyes NetDiag (o una versión modificada), debes conservar avisos de copyright y entregar la AGPL junto con el programa.
- Si ejecutas una versión modificada y la ofreces a usuarios a través de una red, la sección 13 de AGPL exige ofrecerles el **código fuente correspondiente**.
- No hay cláusula de telemetría ni restricción de uso competitivo en Community.
- Los avisos de dependencias están en `NOTICE` y `THIRD_PARTY_LICENSES.md`.

La AGPL protege que las mejoras a NetDiag sigan siendo libres cuando se ofrecen como servicio. No impide usar NetDiag dentro de una organización sin distribuirlo.

## Pro y Enterprise: licencia comercial

Las ediciones Pro y Enterprise se entregan bajo **licencia comercial anual**, no bajo AGPL. La licencia concede:

- Uso del binario y capacidades de la edición contratada por el plazo de la suscripción.
- Derecho a mantener modificaciones privadas sin obligación de publicarlas.
- Acceso a actualizaciones firmadas, soporte, SLA (Enterprise) y paquete de confianza.
- Uso de la licencia offline firmada en las sedes y banda de hosts contratadas.

La licencia comercial **no** transfiere propiedad del código, **no** incluye fuente cerrada por defecto (la base sigue siendo AGPL salvo acuerdo específico) y **no** limita el número de informes o escaneos.

La diferencia de derechos es contractual; el binario funciona sin llamar a un servidor. Ver `docs/comercializacion.md` para SKU, precios y bandas.

## Activación offline (F35)

La licencia es un fichero JSON firmado con **Ed25519** que se guarda en `<data_dir>/license.json`. La verificación es **local**: NetDiag no contacta con ningún servidor de licencias, no envía el ID de licencia ni consulta caducidades en remoto. El derecho real se apoya en el contrato, las actualizaciones firmadas y el soporte.

Formato (`schema` = `netdiag-license/v1`):

```json
{
  "schema": "netdiag-license/v1",
  "license_id": "ND-2026-0001",
  "customer": "Nombre del cliente",
  "edition": "pro",
  "max_managed_hosts": 25,
  "max_sites": 3,
  "retention_days": 30,
  "expires_at": "2027-12-31T00:00:00Z",
  "capabilities": ["reports", "alerts"],
  "support_level": "business",
  "signature": "base64(Ed25519)..."
}
```

La firma cubre todos los campos **excepto** `signature`, serializados en un orden fijo. Cambiar cualquier valor (incluido el nombre del cliente) invalida la firma. `0` en `max_managed_hosts` o `max_sites` significa «sin límite».

Clave pública del emisor (base64, 32 bytes), separada de la clave minisign de los artefactos de release:

```
pNc3VNWpOKBid1TQsm6rF52MqvBxyNdvPJxvqSNha/s=
```

La semilla privada del emisor vive **fuera del repositorio**, en `~/.config/netdiag/license-signing.seed` con permisos `0600`. No se sube a Git, no se incluye en el binario y no se comparte. Si se pierde, se genera un par nuevo y se documenta la rotación en `docs/public_keys.md`.

### Comandos

```sh
# Emitir (solo quien tiene la semilla): rellena el JSON y guárdalo con 0600
netdiag license sign licencia.json --out licencia-firmada.json

# Instalar en el equipo del cliente
netdiag license activate licencia-firmada.json

# Consultar estado (texto o JSON)
netdiag license show
netdiag license show --json

# Volver a Community sin borrar datos
netdiag license clear
```

`--seed-file RUTA` permite indicar otra semilla; por defecto se usa la ruta indicada arriba. Todos los comandos aceptan `--data-dir` y `--config`.

### API y avisos

`GET /api/v1/system/license` está disponible para **cualquier rol autenticado** y devuelve siempre HTTP 200:

```json
{
  "status": "active",
  "edition": "pro",
  "license_id": "ND-2026-0001",
  "expires_at": "2027-12-31T00:00:00Z",
  "limits": { "max_managed_hosts": 25, "max_sites": 3, "retention_days": 30 },
  "capabilities": ["reports", "alerts"],
  "support_level": "business",
  "managed_hosts": 12,
  "sites": 2,
  "warnings": []
}
```

La respuesta **nunca** incluye `signature`, el nombre del cliente ni datos de otro cliente. `managed_hosts` cuenta las identidades `s<siteID>:<identity>` vistas en los últimos 30 días, excluyendo etiquetas `guest` y `excluded`.

Caducidad, banda excedida o firma inválida se traducen en entradas de `warnings`; el estado pasa a `expired`, `invalid` o mantiene `active`. En ningún caso se bloquean lecturas, escaneos ni informes, y jamás se borran datos. Ante una licencia inválida se aplican los límites Community (`retention_days: 7`, hosts y sedes sin límite) y se sigue usando el producto.

El estado también aparece en **Ajustes → Sistema → Licencia** de la interfaz web, con los avisos en un recuadro no bloqueante.

### Copias de seguridad y restauración

`license.json` se incluye en el ZIP de copia junto a `netdiag.db`, `config.yaml` y `secret.key`, y se registra en `manifest.json`. Para restaurar:

1. Restaurar la base de datos y la configuración según `docs/api.md`.
2. Extraer `license.json` del ZIP y copiarlo en `<data_dir>/license.json` con permisos `0600`, **o** volver a ejecutar `netdiag license activate licencia-firmada.json`.

Como la licencia está firmada, una copia manipulada se detecta al arrancar: se degrade a Community con un aviso, sin impedir el acceso a los datos.

## ¿Por qué AGPL y no MIT/BSD?

Porque el riesgo comercial de NetDiag no es que alguien lo use, sino que un tercero lo reempaquete como servicio propietario sin devolver mejoras. AGPL cierra esa vía; la licencia comercial ofrece una alternativa clara a quien necesita mantener cambios privados o exige garantías contractuales.

## Contribuciones

- Las contribuciones a Community se aceptan bajo **DCO** (`Signed-off-by`). No exigimos CLA.
- Al enviar un parche declaras que tienes derecho a aportarlo bajo AGPL-3.0.
- No se incorporan dependencias con licencias copyleft incompatibles con distribución de binario sin revisión legal.
- El inventario `THIRD_PARTY_LICENSES.md` se regenera cuando cambia `go.mod`.

## Terceros

El binario incluye módulos Go bajo licencias MIT, BSD-2-Clause, BSD-3-Clause, Apache-2.0, MPL-2.0 y dominio público (SQLite), entre otras. Los avisos completos se incluyen en `THIRD_PARTY_LICENSES.md` y en el SBOM de cada release.

Si una dependencia cambia de licencia o aparece una incompatibilidad, se bloquea el release hasta sustituirla o documentar la excepción.

## Marcas

NetDiag es una marca del proyecto. El uso del nombre para forks o productos derivados debe evitar sugerir respaldo oficial. La licencia AGPL no concede permiso para usar el nombre en publicidad del fork.

## Contacto

- Errores y preguntas técnicas: GitHub Issues del repositorio.
- Vulnerabilidades: canal privado descrito en `SECURITY.md`.
- Licencias comerciales y presupuestos: flujo descrito en `docs/comercializacion.md`.
