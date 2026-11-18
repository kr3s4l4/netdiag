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
