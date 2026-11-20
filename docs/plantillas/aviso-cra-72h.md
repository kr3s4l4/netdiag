# Notificación CRA 72 h — evaluación inicial

**PLANTILLA. No enviar sin completar. No publicar tal cual.**

Enviar en menos de 72 h desde el momento de conocimiento al CSIRT correspondiente y a la Single Reporting Platform de ENISA. Referenciar el aviso de 24 h.

**Asunto:** `[CRA NOTIFICATION] NetDiag [versión] — [ID-INC-AAAA-NNN]`

---

Estimados/as:

Completamos la notificación inicial del incidente `[ID-INC-AAAA-NNN]`, avisado el `[AAAA-MM-DDTHH:MM:SSZ]`.

## 1. Identificación

- **Fabricante:** `[razón social, NIF, dirección]`
- **Producto:** NetDiag `[versión/es]`
- **Responsable único:** `[nombre, cargo, correo, teléfono]`
- **Fecha de conocimiento:** `[AAAA-MM-DDTHH:MM:SSZ]`
- **Referencia del aviso 24 h:** `[ID o justificante]`

## 2. Vulnerabilidad o incidente

- **CVE:** `[CVE o pendiente]`
- **Descripción técnica:** `[párrafo breve, sin detalles explotables en canales públicos]`
- **Categoría:** `[vulnerabilidad de producto / dependencia de terceros / incidente de seguridad]`
- **Dependencia:** `[nombre, versión, referencia SBOM]`
- **CVSS estimado:** `[puntuación y vector]`
- **Vector de ataque:** `[red local / autenticado / requiere interacción / otro]`

## 3. Criterios CRA

- **¿Hay explotación activa?:** `[SÍ / NO / en investigación]` — `[evidencia]`
- **¿Es incidente grave?:** `[SÍ / NO]` — `[criterio aplicado]`
- **Consecuencias:** `[pérdida de confidencialidad / integridad / disponibilidad / control del producto]`
- **Usuarios afectados:** `[número estimado o "en determinación"]`
- **Ámbito geográfico:** `[UE / otros]`

## 4. Evaluación inicial

`[ Qué ha ocurrido, qué se sabe y qué se ignora. Una frase por línea. ]`

- `[hecho 1]`
- `[hecho 2]`
- `[incertidumbre 1]`

## 5. Medidas

- **Contención:** `[...]`
- **Mitigación temporal recomendada al cliente:** `[...]`
- **Parche previsto:** `[versión correctiva]`, fecha objetivo `[AAAA-MM-DD]`
- **Versión segura mínima (`min_version`):** `[versión]`
- **Comunicación pública:** `[enviada el ... / prevista el ...]`
- **Notificación en UI y `REVOCATION.md`:** `[estado]`

## 6. Informe final

Se enviará el informe final:

- **14 días tras el parche** si hay explotación activa.
- **1 mes desde esta notificación** si es un incidente grave sin explotación confirmada.

Fecha objetivo: `[AAAA-MM-DD]`.

## 7. Contacto

`[nombre, cargo, correo, teléfono]`

Firmado,  
`[nombre]` — `[cargo]` — `[razón social]`  
`[AAAA-MM-DDTHH:MM:SSZ]`

---

**Comprobación interna:**

- [ ] Coherente con `registro-incidentes.md` y `registro-responsabilidad.md`.
- [ ] No incluye secretos ni pasos de explotación en canales públicos.
- [ ] Guardado el justificante de envío.
