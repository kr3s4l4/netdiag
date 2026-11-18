# EULA — NetDiag (reforzado)

Plantilla orientativa. Revisar con asesoría jurídica antes de firmar.

**Licenciante:** `[razón social]`.  
**Licenciatario:** `[organización]`, NIF `[...]`.  
**SKU:** `[...]`. **Fecha:** `[AAAA-MM-DD]`.

## 1. Licencia

Se concede una licencia anual, intransferible y no exclusiva para usar NetDiag en los hosts gestionados y sedes del SKU contratado. La licencia es offline: no exige conexión con un servidor de licencias. Caduca el `[AAAA-MM-DD]`. Al caducar, los datos permanecen y el producto sigue funcionando como Community.

## 2. Uso autorizado

El licenciatario **declara que está autorizado** a analizar las redes, sistemas y dispositivos donde despliegue NetDiag. La autorización debe provenir del titular o de una norma aplicable.

Queda **prohibido**:

- Escanear redes de terceros sin autorización.
- Usar el producto para obtener acceso no autorizado.
- Ocultar, eliminar o alterar la auditoría generada por NetDiag.
- Revender el producto como servicio propio sin acuerdo escrito.

El licenciatario es responsable de la configuración del escaneo, las exclusiones y la ventana de mantenimiento.

## 3. Datos del licenciatario

El licenciatario conserva la propiedad de sus datos de red, configuraciones, informes y copias de seguridad. En despliegues self-hosted, el licenciante no recibe esos datos. NetDiag no envía telemetría. `secret.key` protege los secretos de integraciones; SQLite no cifra toda la base por defecto. Corresponde al licenciatario cifrar el soporte y restringir permisos.

## 4. Soporte y SLA

- **Pro:** soporte por correo, objetivo 48 h hábiles. Sin SLA contractual.
- **Enterprise:** P1 respuesta 1 h / workaround 8 h / objetivo de solución 24 h; P2 4 h hábiles / 2 días; P3 1 día hábil / 5 días. Si no hay cobertura real 24 h, el presupuesto fija 8 h hábiles como respuesta P1 y esa es la condición aplicable.

El soporte de seguridad de la versión contratada se mantiene **durante la suscripción y 12 meses adicionales**. Después se ofrece renovación; no se abandona sin aviso.

## 5. Seguridad, actualizaciones y CRA

El licenciante publica la política de vulnerabilidades en `SECURITY.md`. Las correcciones se entregan como releases nuevas firmadas con SBOM y manifiesto. Los clientes de pago reciben aviso por correo, notificación en la UI y, cuando proceda, `REVOCATION.md` firmado. El manifiesto F23 usa `min_version` para saltar versiones inseguras.

Ante vulnerabilidad explotada o incidente grave, el licenciante reporta según el CRA: aviso 24 h, notificación 72 h e informe final en 14 días tras el parche si hay explotación, o 1 mes si es incidente grave. El licenciatario designará un contacto de seguridad para recibir estos avisos.

## 6. Limitación de responsabilidad

En el máximo permitido por la ley, el licenciante no responde por daños indirectos, lucro cesante, pérdida de oportunidad ni por uso del producto fuera de las condiciones autorizadas.

Esta limitación **no se aplica** a las excepciones legales obligatorias, incluidas dolo, fraude, culpa grave, daños a personas, incumplimiento de obligaciones legales no limitables y aquellas responsabilidades que el CRA u otra normativa imperativa no permita limitar.

Nada en este acuerdo limita los derechos irrenunciables del consumidor ni los que la ley reconozca al licenciatario.

## 7. Propiedad intelectual

El código de NetDiag es del licenciante o de terceros y se distribuye bajo AGPL-3.0 para Community o bajo esta licencia comercial para el SKU contratado. Las marcas y la documentación conservan su titularidad. `LICENSE`, `NOTICE` y `THIRD_PARTY_LICENSES.md` forman parte de este acuerdo.

## 8. Resolución

Cualquiera de las partes puede resolver por incumplimiento grave no subsanado en 30 días. La resolución no elimina obligaciones de pago devengadas ni obligaciones legales de seguridad ya comunicadas.

## 9. Firma

`[licenciante]` — `[nombre, cargo]` — `[AAAA-MM-DD]`

`[licenciatario]` — `[nombre, cargo]` — `[AAAA-MM-DD]`
