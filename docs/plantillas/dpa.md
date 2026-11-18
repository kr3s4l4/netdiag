# DPA / declaración de no subprocesamiento — NetDiag

Plantilla orientativa. Revisar con asesoría jurídica. Elegir **una** de las dos opciones.

## Opción A — Despliegue self-hosted (caso habitual)

**Declaración de no subprocesamiento**

`[proveedor]` declara que:

1. NetDiag se entrega como binario único para despliegue en infraestructura controlada por `[cliente]`.
2. El cliente es el controlador de los datos que introduzca o descubra.
3. NetDiag no recibe esos datos. No hay nube obligatoria, telemetría ni servicio de licencias online.
4. No existen subprocesadores en el producto self-hosted.
5. No se realizan transferencias internacionales de datos por parte del proveedor.
6. La licencia se verifica localmente. El cliente decide si conecta el espejo de actualizaciones.
7. Si se contrata el addon hosted, se firma la Opción B antes de activarlo.

Firmado por `[proveedor]` — Fecha `[AAAA-MM-DD]`  
Firmado por `[cliente]` — Fecha `[AAAA-MM-DD]`

## Opción B — Addon hosted (solo si se contrata)

**Acuerdo de tratamiento de datos**

**Controlador:** `[cliente]`, NIF `[...]`.  
**Encargado:** `[proveedor]`, NIF `[...]`.  
**Finalidad:** `[alojiar el espejo / servicio opcional contratado]`.  
**Datos:** `[tipo de datos, categorías y sujetos]`. Evitar incluir datos de red sensibles salvo necesidad documentada.  
**Duración:** `[periodo de suscripción + 12 meses de soporte de seguridad]`.

El encargado se compromete a:

- Tratar los datos solo según instrucciones documentadas del controlador.
- Aplicar medidas técnicas y organizativas apropiadas: cifrado en tránsito, control de acceso, registro de accesos y copias restringidas.
- No contratar subprocesadores sin aviso previo: `[lista o "ninguno"]`.
- No transferir datos fuera de `[UE/EEE]` sin base legal y garantías documentadas.
- Ayudar al controlador con ejercicios de derechos y con la seguridad de los datos.
- Notificar al controlador sin demora indebida cualquier incidencia de seguridad conocida.
- Devolver o eliminar los datos al finalizar, salvo obligación legal de conservación.
- Aportar información y auditoría razonable para verificar este acuerdo.

El controlador declara que cuenta con base legal para los datos enviados y que ha informado a los afectados cuando procede.

Firmado por `[controlador]` — Fecha `[AAAA-MM-DD]`  
Firmado por `[encargado]` — Fecha `[AAAA-MM-DD]`

**Nota:** el addon hosted es un servicio aparte. Su alcance, precios y responsabilidades se describen en un anexo específico.
