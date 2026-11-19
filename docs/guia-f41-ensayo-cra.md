# Guía de ensayo del runbook CRA — F41

Objetivo: demostrar, con horas reales, que el proceso de incidentes funciona. **Es un simulacro.** No envíes avisos falsos al CSIRT ni a la SRP de ENISA. Guarda los borradores y los justificantes simulados.

Tiempo estimado: 2 h de preparación + 7 días de reloj.

## 0. Datos que debes rellenar tú, en local

No los pegues en el chat ni en el repositorio.

| Dato | Dónde se guarda |
| --- | --- |
| Nombre del responsable y del suplente | `SECURITY.md` línea 48 y `docs/plantillas/registro-responsabilidad.md` |
| ID de organización y usuario de la SRP | Gestor de contraseñas local |
| Contraseña de la clave de firma | Papel, como ya está |
| Lista de contactos de clientes de pago | Fuera del repositorio |

Si aún no tienes cuenta en la SRP, el ensayo sirve para medir el proceso, pero queda marcado como **pendiente de credenciales reales**. Crear la cuenta es un paso manual que solo puedes hacer tú.

## 1. Preparación

```bash
cd /run/media/kr3s4l4/importante/Github_Proyectos/proyectos/scaner_Red/kr3s4l4-Netdiag
mkdir -p /tmp/opencode/ensayo-cra
cp docs/plantillas/registro-incidentes.md /tmp/opencode/ensayo-cra/INC-2026-001.md
cp docs/plantillas/aviso-cra-24h.md /tmp/opencode/ensayo-cra/aviso-24h.md
cp docs/plantillas/aviso-cra-72h.md /tmp/opencode/ensayo-cra/aviso-72h.md
cp docs/plantillas/registro-responsabilidad.md /tmp/opencode/ensayo-cra/responsabilidad.md
date -u +%Y-%m-%dT%H:%M:%SZ
```

Rellena en `INC-2026-001.md`: ID, responsable, suplente y la hora UTC que imprimió `date`. Esa hora es `T0`.

Escenario sugerido, coherente con el SBOM real de `v1.5.1`:

```text
CVE ficticia en una dependencia de golang.org/x/net incluida en v1.5.1.
Explotación activa confirmada en un laboratorio de terceros.
Afecta a v1.5.0 y v1.5.1. Sin mitigación configurable.
```

Usa la rama estricta del árbol de decisión: «explotación activa» = SÍ. Así se ejercitan los plazos de 24 h y 72 h.

## 2. T0: detección y triaje

En `INC-2026-001.md`:

- Hito «Detección» = `T0`.
- Hito «Conocimiento por el fabricante» = `T0 + 15 min`.
- Hito «Triaje y clasificación» = `T0 + 1 h`.
- Decisión de reportar: **SÍ**, con el motivo escrito en una frase.

Marca en «Datos del hallazgo»: versiones, dependencia, explotación activa `SÍ`, incidente grave `NO`, y el criterio aplicado.

## 3. Antes de T0 + 24 h

Abre `aviso-24h.md` y rellena el borrador. Después simula el envío:

```bash
date -u +%Y-%m-%dT%H:%M:%SZ
cp /tmp/opencode/ensayo-cra/aviso-24h.md /tmp/opencode/ensayo-cra/aviso-24h-enviado.md
printf 'borrador guardado, NO enviado a ENISA\n' > /tmp/opencode/ensayo-cra/recibo-srp-24h.txt
```

Anota esa hora como «Aviso 24 h enviado» y guarda la ruta del recibo como evidencia. Comprueba la lista de la plantilla: el aviso se prepara **sin esperar al parche**.

## 4. Antes de T0 + 72 h

Igual con el aviso de 72 h. Debe referenciar explícitamente el aviso de 24 h.

```bash
date -u +%Y-%m-%dT%H:%M:%SZ
cp /tmp/opencode/ensayo-cra/aviso-72h.md /tmp/opencode/ensayo-cra/aviso-72h-enviado.md
printf 'borrador guardado, NO enviado a ENISA\n' > /tmp/opencode/ensayo-cra/recibo-srp-72h.txt
```

## 5. Antes de T0 + 7 días: parche firmado

Ejercita la parte técnica real con una versión de prueba. No sustituye a `v1.5.1`.

```bash
cd /run/media/kr3s4l4/importante/Github_Proyectos/proyectos/scaner_Red/kr3s4l4-Netdiag
bash scripts/release.sh --version 1.5.2-test \
  --signing-json RUTA_JSON_TEMPORAL \
  --public-key RUTA_CLAVE_PUBLICA \
  --notes-file docs/release-notes-1.5.1.md \
  --min-version 1.5.2-test
bash scripts/verify-release.sh --dir dist/release/1.5.2-test --version 1.5.2-test
```

El JSON temporal se genera solo con la clave descifrada en local y se borra con `shred` al terminar, igual que en la release real. Si no quieres repetir ese paso durante el ensayo, valida solo el manifiesto de `v1.5.1` y marca el parche como «simulado».

Comprueba además:

- El manifiesto F23 lleva `min_version` apuntando a la primera versión segura.
- Una instalación anterior recibe `security_update: true` y el evento `update.security`.
- El centro de notificaciones muestra el aviso de actualización.
- `REVOCATION.md` solo se firma si hay compromiso de clave o versión insegura.

Anota versión y SHA256 en la cronología.

## 6. Comunicación y cierre

Marca en `registro-incidentes.md` los correos a clientes, la notificación en UI y el informe final. Plazos reales del CRA:

- 24 h: aviso temprano.
- 72 h: notificación.
- 14 días tras parche si hay explotación; 1 mes si es incidente grave: informe final.

Cierra con:

```bash
date -u +%Y-%m-%dT%H:%M:%SZ
```

Calcula los tres intervalos reales desde `T0`. Rellena la sección «Cierre» con causa raíz, lección aprendida y cambio de proceso asignado.

## Criterio de éxito

| Plazo | Objetivo | Evidencia |
| --- | --- | --- |
| < 24 h | Aviso preparado | `aviso-24h-enviado.md` + hora UTC |
| < 72 h | Notificación preparada | `aviso-72h-enviado.md` + hora UTC |
| < 7 días | Release firmada y verificada | Versión + SHA256 + `verify-release.sh` |
| Siempre | Sin envíos reales | Recibos marcan «NO enviado» |

Si falta la cuenta SRP o el responsable, el ensayo no está cerrado: anótalo como bloqueo y resuélvelo antes del primer cliente Enterprise.
