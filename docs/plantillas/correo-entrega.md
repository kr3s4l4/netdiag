# Correo de entrega de licencia

Plantilla lista para copiar y pegar. Sustituye los campos entre corchetes `[...]`.
La semilla de firma **nunca** se incluye en este correo.

---

Asunto: NetDiag [VERSION] — licencia [LICENSE_ID]

Hola, [NOMBRE]:

Gracias por adquirir NetDiag. Adjunto el paquete de entrega:

- `netdiag-entrega-[VERSION].zip`
- `license.json`, firmada para [ORGANIZACIÓN]
- Factura [NUMERO_FACTURA]

La licencia cubre hasta [HOSTS] hosts gestionados, [SITIOS] sedes y [RETENCION] días de retención. Se activa sin conexión y no consulta ningún servidor.

Verificación antes de ejecutar:

```bash
minisign -V -P 'RWTJixPYyPcyWf0VEHdzn6lRYxgrPS3z+EbRBKlI9E+hX0lj1227yrZY' \
  -m SHA256SUMS -x SHA256SUMS.minisig
sha256sum -c SHA256SUMS
```

Instalación y activación:

```bash
unzip netdiag-entrega-[VERSION].zip
cd netdiag-entrega-[VERSION]
chmod +x netdiag-linux-amd64
./netdiag-linux-amd64 license activate --data-dir ./netdiag-data license.json
./netdiag-linux-amd64 license show --data-dir ./netdiag-data
```

Si aparece un aviso de caducidad o de banda excedida, NetDiag sigue funcionando en modo Community y conserva todos los datos. Escríbeme y ajustamos la licencia.

Soporte incluido: [NIVEL_SOPORTE]. Responde a este correo.

Un saludo,
kr3s4l4
