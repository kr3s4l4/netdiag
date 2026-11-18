# FAQ de seguridad — NetDiag

Texto para el comprador. Respuestas cortas. Sin letra pequeña escondida.

**1. ¿NetDiag envía telemetría?**  
No. No hay “teléfono a casa”. La licencia se verifica localmente. El espejo de actualizaciones es opcional y puede ser interno.

**2. ¿Se cifra la base de datos?**  
SQLite no cifra toda la base por defecto. `secret.key` (0600) cifra los secretos de integraciones con AES-256-GCM. Recomendamos cifrado de disco o volumen. La copia ZIP incluye `secret.key`: protégela igual.

**3. ¿Hay subprocesadores?**  
No en el despliegue self-hosted. El cliente es el controlador de sus datos y NetDiag no los recibe. Si se contrata el addon hosted, se firma un DPA aparte.

**4. ¿Necesita Internet para funcionar?**  
No. Funciona offline. Las actualizaciones se pueden importar como `.ndc` firmado en redes aisladas.

**5. ¿La edición portable captura 802.11 crudo?**  
No. La edición portable no captura tramas 802.11 crudas. La variante con `-tags pcap` es la que tiene esa capacidad, sujeta a permisos y autorización.

**6. ¿Cómo sabemos que el binario es auténtico?**  
Se verifica la firma minisign, los SHA256, la procedencia y el SBOM de cada release. Los pasos están en `docs/publicacion-segura.md`. Si algo falla, no se ejecuta.

**7. ¿Qué pasa con un CVE en una dependencia?**  
Es responsabilidad del fabricante del producto terminado. Se monitoriza con el SBOM de cada release. La corrección llega en una release nueva firmada, con manifiesto y `min_version`. Nunca se firma un binario de terceros sin reconstruirlo o verificarlo.

**8. ¿Qué obligaciones CRA asumís?**  
Aviso 24 h, notificación 72 h e informe final en 14 días tras parche si hay explotación, o 1 mes si es incidente grave. Responsable único y cuenta ENISA SRP descritos en `SECURITY.md`.

**9. ¿Cuánto dura el soporte de seguridad?**  
La versión contratada durante la suscripción y 12 meses adicionales. Aparece en `SECURITY.md`, la orden de compra y el EULA.

**10. ¿Qué pasa si dejamos de pagar?**  
Los datos siguen ahí. El producto funciona como Community con retención de 7 días. No hay bloqueo de lectura.

**11. ¿Podemos escanear redes de terceros?**  
No sin autorización. El EULA exige declarar que se está autorizado a analizar esas redes y prohíbe el escaneo de terceros.

**12. ¿Por qué no hay SLA en Pro?**  
Porque no hay cobertura 24 h prometida. Pro es correo con objetivo 48 h hábiles, sin SLA contractual. Enterprise ofrece SLA solo con cobertura real; si no la hay, se dice en el presupuesto.
