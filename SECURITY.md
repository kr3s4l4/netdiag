# Política de seguridad de NetDiag

## Versiones soportadas

| Versión | Soporte de seguridad |
| --- | --- |
| 1.5.x | Sí |
| 1.4.x | Hasta la siguiente minor, solo correcciones críticas |
| < 1.4 | No |

**Sin ventana contractual:** NetDiag es gratuito y no hay ediciones de pago, contratos ni EULA. La tabla anterior describe el soporte de seguridad ofrecido. No hay SLA ni soporte formal: los errores se atienden por GitHub Issues.

## Cómo reportar una vulnerabilidad

**No abras un issue público** con detalles explotables.

Usa el canal privado de GitHub Security Advisories del repositorio:

<https://github.com/kr3s4l4/netdiag/security/advisories/new>

Incluye, si puedes:

- Versión y plataforma afectadas.
- Pasos para reproducir o proof of concept mínimo.
- Impacto esperado (confidencialidad, integridad, disponibilidad).
- Configuración relevante (sin secretos reales).

## Compromisos de respuesta

| Fase | Plazo objetivo |
| --- | --- |
| Confirmación de recepción | 3 días hábiles |
| Triaje y clasificación | 10 días hábiles; inmediato si hay indicios de explotación activa |
| Aviso CRA cuando aplique | 24 h (early warning) y 72 h (notificación completa); **no se espera a tener parche** |
| Parche para vulnerabilidad crítica explotada | objetivo 72 h–7 días |
| Parche para severidad alta no explotada | objetivo 30 días |
| Severidad media/baja | siguiente versión, objetivo 90 días |
| Publicación | notas firmadas, aviso en este archivo y comunicación a clientes de pago |

Si hay evidencia fiable de explotación activa o de un incidente grave, se reporta aunque todavía no exista solución. El parche llega después; el aviso no puede esperar.

Si el problema afecta a la clave de firma o al pipeline de publicación, se sigue el procedimiento de revocación de `docs/publicacion-segura.md` de inmediato.

## Runbook de incidentes CRA

**Responsable único:** `[nombre, cargo, correo, teléfono]`.  
**Suplente:** `[nombre, cargo, correo, teléfono]`.  
**Cuenta en la Single Reporting Platform de ENISA:** `[ID de organización, usuario, gestor de credenciales]`. Comprobar acceso cada trimestre. Sin estos tres datos, el runbook no está cerrado.

Objetivo F41: aviso preparado en **< 24 h**, notificación oficial en **< 72 h** y release firmada correctiva en **< 7 días**.

Árbol de decisión:

```text
¿Hay explotación activa O incidente grave?
├─ SÍ → aviso 24 h + notificación 72 h. No esperar al parche.
│       Informe final: 14 días tras el parche si hay explotación;
│       1 mes desde la notificación de 72 h si es incidente grave.
└─ NO → triaje y corrección en release firmada. Vigilar el criterio.
        Si cambia a SÍ, entrar en la rama superior de inmediato.
```

Pasos:

1. Registrar la hora de conocimiento en UTC. Abrir `docs/plantillas/registro-incidentes.md` y `docs/plantillas/registro-responsabilidad.md`.
2. Decidir si se reporta. Anotar el motivo y quién aprueba la decisión.
3. Enviar el [aviso de 24 h](docs/plantillas/aviso-cra-24h.md) al CSIRT del Estado miembro del establecimiento principal y a la SRP de ENISA. Guardar el justificante.
4. Enviar la [notificación de 72 h](docs/plantillas/aviso-cra-72h.md) con la evaluación inicial.
5. Corregir en una release nueva firmada con SBOM, manifiesto y `min_version` apuntando a la primera versión segura. F23 marca esas instalaciones con `security_update: true` y emite `update.security`; el cliente ve el aviso aunque la comparación de versiones no parezca urgente.
6. Comunicar a clientes de pago por correo, notificar en la UI y publicar [REVOCATION.md](docs/plantillas/REVOCATION.md) firmado cuando proceda. En air-gapped se entrega un `.ndc` firmado con el aviso.
7. Enviar el informe final y cerrar ambos registros.

**Monitorización de dependencias:** cada release lleva SBOM CycloneDX (`sbom.cdx.json`) y atestación de procedencia. Revisar avisos de forma periódica. Un CVE relevante se trata como incidente interno aunque no haya explotación conocida. El productor responde por el producto terminado; después reclama aguas arriba si existe contrato. **Nunca firmar un binario de terceros sin reconstruirlo o verificarlo.** Ver `docs/publicacion-segura.md`.

**Autorización de escaneo:** quien ejecuta NetDiag declara que está autorizado a analizar esas redes. Se prohíbe el escaneo de terceros. No hay EULA ni contrato de pago.

## Obligaciones CRA en la UE

Desde el **11 de septiembre de 2026**, NetDiag como producto con elementos digitales está sujeto a las obligaciones de notificación del Cyber Resilience Act (Reglamento UE 2024/2847):

- **24 h:** aviso temprano al CSIRT del Estado miembro del establecimiento principal y a ENISA a través de la Single Reporting Platform.
- **72 h:** notificación completa con evaluación inicial.
- **14 días tras el parche:** informe final para vulnerabilidades explotadas activamente.
- **1 mes tras la notificación de 72 h:** informe final para incidentes graves.

Esto aplica también a versiones puestas en el mercado antes del 11 de diciembre de 2027. El resto de obligaciones del CRA —documentación técnica, evaluación de conformidad, instrucciones de uso, periodo de soporte y marcado cuando proceda— llegan el **11 de diciembre de 2027**.

La responsabilidad es del **fabricante**, es decir, de quien comercializa NetDiag bajo su nombre o marca. Que una vulnerabilidad esté en una dependencia de terceros no traslada esa responsabilidad al cliente: se gestiona en nuestro producto y se reclama aguas arriba si existe contrato.

## Divulgación

- Preferimos divulgación coordinada: el reportante recibe confirmación antes de publicar.
- Si no hay respuesta en 90 días, el reportante puede divulgar de forma responsable.
- Agradecemos el crédito público, pero respetamos el anonimato si se solicita.

## Alcance

- Código de NetDiag, SPA embebida, API y documentación.
- Pipeline de build, espejo de releases y esquema de firmas.
- Dependencias incluidas en `go.mod` cuando afecten al binario publicado.

**Fuera de alcance:** configuraciones inseguras decididas por el operador, exposición de una instancia sin TLS, credenciales filtradas por el usuario, ataques que requieran root en el host donde corre NetDiag y reportes puramente teóricos sin escenario de impacto.

## Recordatorio de despliegue seguro

- No expongas NetDiag directamente a Internet sin autenticación y HTTPS.
- Cambia la contraseña inicial y usa roles de menor privilegio.
- Define `security.metrics_token` si `/metrics` es accesible fuera de loopback.
- Verifica siempre la firma de la versión antes de actualizar: `docs/publicacion-segura.md`.
- Mantén copias automáticas activadas y comprueba las restauraciones.
