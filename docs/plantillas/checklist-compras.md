# Checklist de compras — NetDiag

Copiar en el correo al comprador. Adjuntar como aceptación antes de firmar.

## Alcance

- [ ] SKU confirmado: `[community | pro-25 | pro-100 | pro-500 | enterprise-500 | enterprise-2000 | enterprise-custom]`.
- [ ] Banda de hosts gestionados: `[número]`.
- [ ] Sedes: `[número]`.
- [ ] Retención requerida: `[7 días | 1 mes | 1 año | 7 años | según contrato]`.
- [ ] Definición aceptada: host gestionado = `s<siteID>:<identity>` visto en los últimos 30 días, sin etiquetas `guest`/`excluded`.
- [ ] Confirmado por escrito: no se cobra por informe, alerta, escaneo, usuario ni host detectado.

## Legal y facturación

- [ ] NIF/CIF y datos fiscales recibidos.
- [ ] Presupuesto con SKU, precio anual, IVA y periodo aceptado.
- [ ] Orden de compra recibida.
- [ ] Factura anual emitida por adelantado.
- [ ] EULA firmado, con declaración de autorización para analizar esas redes.
- [ ] DPA firmado si hay addon hosted; si no, declaración de no subprocesamiento adjunta.
- [ ] Soporte de seguridad aceptado: versión contratada durante la suscripción + 12 meses.

## Soporte y SLA

- [ ] Canal de soporte y correo de contacto acordados.
- [ ] Pro: correo, objetivo 48 h hábiles, **sin SLA contractual**.
- [ ] Enterprise: matriz P1/P2/P3 revisada.
- [ ] Cobertura P1 de 1 h: [ ] SÍ, con responsable alterno o retainer. [ ] NO: se ofrece 8 h hábiles y consta en el presupuesto.
- [ ] Responsable CRA y cuenta en la Single Reporting Platform de ENISA documentados en `SECURITY.md`.

## Seguridad y verificación

- [ ] Huella de la clave pública de release comparada con `docs/public_keys.md`.
- [ ] `SHA256SUMS.minisig` verificado con `minisign -Vm SHA256SUMS -p netdiag-release.pub`.
- [ ] `sha256sum -c SHA256SUMS` correcto.
- [ ] `netdiag container verify --key netdiag-release.pub <archivo>.ndc` correcto si se entrega contenedor.
- [ ] SBOM CycloneDX y atestación de procedencia archivados.
- [ ] Declaraciones leídas: SQLite no cifra toda la base por defecto; `secret.key` protege secretos de integraciones; la edición portable no captura 802.11 crudo.
- [ ] Guía de endurecimiento aplicada: `docs/plantillas/guia-endurecimiento.md`.
- [ ] Plan de actualizaciones acordado: espejo oficial o interno; `min_version` en el manifiesto F23.
- [ ] Procedimiento de revocación conocido: `REVOCATION.md` firmado.

## Cierre

- [ ] Licencia firmada recibida y comprobada con `netdiag license show`.
- [ ] Alta en soporte confirmada por correo.
- [ ] Paquete completo archivado: licencia, factura, SBOM, documentos legales y pasos de verificación.

Firmado por `[cliente]` — Fecha `[AAAA-MM-DD]`
