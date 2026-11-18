# Roadmap de NetDiag

Este documento registra el estado del producto a partir de la versión **1.5.0**. El roadmap original F0–F26 se conserva como referencia histórica en `notes/14-roadmap.md`; aquí se añaden las mejoras posteriores y las decisiones comerciales pendientes.

## Estado actual

| Bloque | Estado |
| --- | --- |
| F0–F12, F18–F25 | Completados e integrados en `1.5.0` |
| F13, F14, F15, F16, F17, F26 | Diferidos y justificados en `README.md` |
| F23 | Cerrado: notificación + descarga verificada + instalación con rollback + reinicio |
| F24 | Backup automático programado |
| Pruebas | `go build`, `go vet`, `make test`, chequeo JS y smoke con binario real |

## Mejoras propuestas para 1.5.1

Prioridad alta, bajo riesgo: son cambios principalmente de interfaz, no alteran SQLite ni la seguridad.

### F27 — Asistente de primer arranque

**Objetivo:** que un usuario nuevo llegue a ver datos útiles en menos de dos minutos.

Al crear la cuenta de administrador:

1. Detectar la red local a partir de interfaces y gateway.
2. Proponer un CIDR inicial con perfil `common`.
3. Lanzar el primer escaneo y mostrar progreso.
4. Guardar automáticamente un snapshot de auditoría como línea base.
5. Ofrecer el tour guiado existente al terminar.

**Criterio de cierre:** tras el setup, el panel muestra hosts reales y una línea base sin que el usuario tenga que buscar el botón de escaneo.

**Esfuerzo:** 2–4 h.

### F28 — Panel de salud de red

**Objetivo:** resumir el estado de la red en una sola pantalla con lenguaje ejecutivo.

Añadir al panel una tarjeta con:

- Puntuación `0–100` con semáforo.
- Tendencia de los últimos 7 datos usando `/history/compare`.
- Las tres incidencias más importantes de `/diagnostics`.
- Acceso directo a la evidencia de cada incidencia.

**Criterio de cierre:** la tarjeta explica de dónde sale la puntuación y permite abrir el hallazgo relacionado.

**Esfuerzo:** 2–3 h.

### F29 — Buscador global

**Objetivo:** localizar cualquier objeto sin recorrer menús.

- Atajo `Ctrl+K` / `Cmd+K`.
- Búsqueda por IP, MAC, hostname, identificador de alerta y red.
- Navegación directa al detalle correspondiente.

**Criterio de cierre:** desde cualquier vista se puede buscar un host por IP y abrir su ficha con el teclado.

**Esfuerzo:** 2–3 h.

### F30 — Centro de notificaciones

**Objetivo:** conservar los eventos que hoy se muestran solo como toasts.

- Campana con contador en la cabecera.
- Historial de `scan.finished`, `alert.new`, `update.available`, `update.applied` y `update.rolled_back`.
- Marcar como leído y abrir el objeto relacionado.

**Criterio de cierre:** un evento importante sigue accesible después de recargar la página.

**Esfuerzo:** 3–4 h.

### F31 — Diagnósticos accionables

**Objetivo:** convertir cada hallazgo en una gestión concreta.

Para cada hallazgo de `/diagnostics`:

- Mantener evidencia y confianza.
- Botón **Ver host**.
- Botón **Copiar comando** cuando exista una comprobación útil.
- Botón **Crear regla de alerta** cuando el hallazgo sea monitorizable.

**Criterio de cierre:** desde un hallazgo se puede llegar al host y crear una regla sin salir de la vista.

**Esfuerzo:** 3–5 h.

### F32 — Informe ejecutivo de una página

**Objetivo:** entregar un artefacto listo para clientes o dirección.

Generar un PDF de una página con:

- Inventario resumido.
- Disponibilidad y latencia de los últimos 7 días.
- Cambios principales de `/history/compare`.
- Hallazgos de mayor severidad.
- Pie con versión de NetDiag y fecha de generación.

**Criterio de cierre:** el informe se genera desde un botón y no expone secretos ni datos de configuración.

**Esfuerzo:** 4–6 h.

## Decisiones pendientes de producto

Estas tareas no están implementadas; requieren una decisión de negocio antes de codificar.

### F33 — Licenciamiento

- Elegir licencia de código abierto: recomendación **`AGPL-3.0`** para la edición comunidad, con licencia comercial aparte para empresa.
- Añadir `LICENSE`, `NOTICE` y un inventario de licencias de terceros.
- Definir una edición Enterprise con derechos de uso cerrados.
- No condicionar el funcionamiento básico a un servidor de licencias en línea.

### F34 — Modelo comercial

- Definir ediciones y límites por sede/activo.
- Evaluar cobro por suscripción anual frente a cobro por informe.
- Usar ventanas de retención (7 días, 1 mes, 1 año) como diferenciador de plan, no como duración del acceso.
- Diseñar un periodo de prueba limitado.

### F35 — Activación offline

- Clave de licencia firmada con Ed25519 y guardada en el directorio de datos.
- Campos: identificador de cliente, edición, límites, caducidad y capacidades.
- Comandos `netdiag license show` y `netdiag license activate`.
- Verificación local sin telemetría; el derecho real se apoya en soporte y acceso a actualizaciones firmadas.
