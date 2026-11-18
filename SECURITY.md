# Política de seguridad de NetDiag

## Versiones soportadas

| Versión | Soporte de seguridad |
| --- | --- |
| 1.5.x | Sí |
| 1.4.x | Hasta la siguiente minor, solo correcciones críticas |
| < 1.4 | No |

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
| Triaje y clasificación | 10 días hábiles |
| Parche para severidad alta/crítica | 30 días naturales o coordinado con el reportante |
| Publicación | Notas firmadas y aviso en este archivo |

Si el problema afecta a la clave de firma o al pipeline de publicación, se sigue el procedimiento de revocación de `docs/publicacion-segura.md` de inmediato.

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
