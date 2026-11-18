'use strict';

/* ============================================================
 * NetDiag — i18n ligero sin dependencias.
 *
 * El código fuente de la SPA está escrito en español. Este módulo
 * traduce el DOM ya renderizado con un diccionario español→inglés,
 * por lo que no hay que duplicar cada vista. Los datos del usuario
 * (IPs, MAC, notas, nombres de host) no se tocan: solo se traducen
 * nodos de texto cuya cadena completa está en el diccionario y que
 * no viven dentro de .mono, code, pre o SVG.
 * ============================================================ */

(function () {
  const STORAGE_KEY = 'netdiag-lang';
  const SUPPORTED = ['es', 'en'];

  const EN = {
    // ---- shell / navegación ----
    'Operación': 'Operations',
    'Historial': 'History',
    'Sistema': 'System',
    'Panel': 'Dashboard',
    'Dispositivos': 'Devices',
    'Topología': 'Topology',
    'Alertas': 'Alerts',
    'Escaneos': 'Scans',
    'Informes': 'Reports',
    'Configuración': 'Settings',
    'Guía': 'Guide',
    'Guía interactiva': 'Interactive guide',
    'Resumen operativo': 'Operational summary',
    'Inventario': 'Inventory',
    'Mapa de red': 'Network map',
    'Reglas y canales': 'Rules and channels',
    'Historial y ejecución': 'History and runs',
    'Exportación': 'Export',
    'Detalle': 'Detail',
    'Menú': 'Menu',
    'Cambiar tema': 'Toggle theme',
    'Tema oscuro': 'Dark theme',
    'Tema claro': 'Light theme',
    'Alto contraste': 'High contrast',
    'Cambiar idioma': 'Change language',
    'Escanear': 'Scan',
    'Cerrar sesión': 'Sign out',
    'Cerrar': 'Close',
    'En vivo': 'Live',
    'Reconectando…': 'Reconnecting…',
    'Conectando…': 'Connecting…',
    'network diagnostics · kr3s4l4': 'network diagnostics · kr3s4l4',
    'servidor local': 'local server',
    'Sin datos todavía': 'No data yet',
    'Sin puertos escaneados': 'No scanned ports',
    'Últimas 24 horas': 'Last 24 hours',
    '24 h': '24 h',
    'Ver todos': 'View all',
    'Ver detalle': 'View details',
    'Gestionar': 'Manage',
    'Actualizar': 'Refresh',
    'Volver a detectar': 'Detect again',
    'Copiar': 'Copy',
    'Descargar': 'Download',
    'Reintentar': 'Retry',
    'Guardar': 'Save',
    'Guardar cambios': 'Save changes',
    'Cancelar': 'Cancel',
    'Crear': 'Create',
    'Crear usuario': 'Create user',
    'Editar': 'Edit',
    'Eliminar': 'Delete',
    'Añadir': 'Add',
    'Habilitar': 'Enable',
    'Deshabilitar': 'Disable',
    'Nunca': 'Never',
    'Todos': 'All',
    'Todas': 'All',
    'Sí': 'Yes',
    'No': 'No',
    '—': '—',

    // ---- login ----
    'Configuración inicial': 'Initial setup',
    'Iniciar sesión': 'Sign in',
    'Accede al inventario y monitoreo de tu red.':
      'Access your network inventory and monitoring.',
    'Crea la cuenta de administrador. La contraseña debe tener al menos 10 caracteres, con letras y números.':
      'Create the administrator account. The password must be at least 10 characters long and contain letters and numbers.',
    'Usuario': 'Username',
    'Contraseña': 'Password',
    'Repetir contraseña': 'Repeat password',
    'Crear cuenta y entrar': 'Create account and sign in',
    'Entrar': 'Sign in',
    'Las contraseñas no coinciden': 'Passwords do not match',
    'No se pudo inicializar la interfaz.': 'The interface could not be initialized.',

    // ---- comunes ----
    'Buscar': 'Search',
    'Buscar por IP, nombre, MAC o fabricante…': 'Search by IP, name, MAC or vendor…',
    'Estado': 'Status',
    'Tipo': 'Type',
    'Nombre': 'Name',
    'Red': 'Network',
    'Redes': 'Networks',
    'Sede': 'Site',
    'Sedes': 'Sites',
    'Sede activa': 'Active site',
    'Todas las sedes': 'All sites',
    'Activar': 'Activate',
    'Activa': 'Active',
    'Nueva sede': 'New site',
    'Editar sede': 'Edit site',
    'Descripción': 'Description',
    'Último escaneo': 'Last scan',
    'Importar': 'Import',
    'Exportar': 'Export',
    'Modo': 'Mode',
    'Combinar — conservar datos existentes': 'Merge — keep existing data',
    'Reemplazar — borrar datos actuales de la sede': 'Replace — clear current site data',
    'Paquete JSON de sede': 'Site JSON bundle',
    'Sede actualizada': 'Site updated',
    'Sede creada': 'Site created',
    'Sede eliminada': 'Site deleted',
    'Interfaz': 'Interface',
    'Interfaces': 'Interfaces',
    'Puerta': 'Gateway',
    'Direcciones': 'Addresses',
    'Dirección': 'Address',
    'Fecha': 'Date',
    'Actor': 'Actor',
    'Acción': 'Action',
    'Objetivo': 'Target',
    'Rol': 'Role',
    'Creado': 'Created',
    'Último acceso': 'Last sign-in',
    'Versión': 'Version',
    'Servidor': 'Server',
    'Datos': 'Data',
    'Capacidades': 'Capabilities',
    'Puertos': 'Ports',
    'Puerto': 'Port',
    'Servicio': 'Service',
    'Fabricante': 'Vendor',
    'Modelo': 'Model',
    'Sistema operativo': 'Operating system',
    'Etiquetas': 'Tags',
    'Notas': 'Notes',
    'Favorito': 'Favorite',
    'Quitar favorito': 'Remove favorite',
    'Añadir a favoritos': 'Add to favorites',
    'RTT': 'RTT',
    'Latencia': 'Latency',
    'Disponibilidad': 'Availability',
    'Cobertura': 'Coverage',
    'Eventos': 'Events',
    'Resumen': 'Summary',
    'Vecinos': 'Neighbors',
    'Certificados': 'Certificates',
    'Sin puertos abiertos detectados': 'No open ports detected',
    'Puerta de enlace': 'Gateway',

    // ---- estados y severidad ----
    'En línea': 'Online',
    'Caído': 'Down',
    'Desconocido': 'Unknown',
    'Archivado': 'Archived',
    'Habilitada': 'Enabled',
    'Pausada': 'Paused',
    'Up': 'Up',
    'Down': 'Down',
    'Info': 'Info',
    'Aviso': 'Warning',
    'Crítico': 'Critical',
    'Crítica': 'Critical',
    'En cola': 'Queued',
    'En curso': 'Running',
    'Completado': 'Completed',
    'Cancelado': 'Canceled',
    'Error': 'Error',
    'Interrumpido': 'Interrupted',
    'Activas': 'Active',
    'Reconocidas': 'Acknowledged',
    'Resueltas': 'Resolved',
    'Toda severidad': 'Any severity',

    // ---- tipos de dispositivo ----
    'Router': 'Router',
    'Switch': 'Switch',
    'Punto de acceso': 'Access point',
    'Estación': 'Workstation',
    'Portátil': 'Laptop',
    'Teléfono': 'Phone',
    'Tablet': 'Tablet',
    'Impresora': 'Printer',
    'Cámara': 'Camera',
    'IoT': 'IoT',
    'NAS': 'NAS',
    'Virtual': 'Virtual',
    'Multimedia': 'Media',
    'Consola': 'Console',

    // ---- reglas / canales ----
    'Host caído': 'Host down',
    'Host nuevo': 'New host',
    'Latencia alta': 'High latency',
    'Puerto abierto': 'Port opened',
    'Puerto cerrado': 'Port closed',
    'Servicio cambiado': 'Service changed',
    'Dispositivo cambiado': 'Device changed',
    'Uso de subred': 'Subnet usage',
    'Escaneo fallido': 'Scan failed',
    'Webhook': 'Webhook',
    'Slack': 'Slack',
    'Discord': 'Discord',
    'Telegram': 'Telegram',
    'PagerDuty': 'PagerDuty',
    'Correo SMTP': 'SMTP email',
    'Syslog': 'Syslog',

    // ---- dashboard ----
    'Escaneo en curso': 'Scan in progress',
    'Escaneos en curso': 'Scans in progress',
    'Alertas activas': 'Active alerts',
    'Latencia media': 'Average latency',
    'Disponibilidad media': 'Average availability',
    'Tipos de dispositivo': 'Device types',
    'Puertos más expuestos': 'Most exposed ports',
    'en línea': 'online',
    'caídos': 'down',
    'redes': 'networks',
    'nuevos en 24 h': 'new in 24 h',
    'escaneos en 24 h': 'scans in 24 h',
    'críticas': 'critical',
    'ahora': 'just now',
    'hace 1 min': '1 min ago',

    // ---- hosts ----
    'Filtros': 'Filters',
    'Todos los estados': 'All statuses',
    'Todos los tipos': 'All types',
    'Todas las redes': 'All networks',
    'Solo favoritos': 'Favorites only',
    'Limpiar filtros': 'Clear filters',
    'Exportar CSV': 'Export CSV',
    'Exportar JSON': 'Export JSON',
    'Nombre de host': 'Host name',
    'Visto': 'Last seen',
    'Primera vez': 'First seen',
    'Puertos abiertos': 'Open ports',
    'Escaneo completo': 'Full scan',
    'Escanear de nuevo': 'Scan again',
    'Eliminar del inventario': 'Remove from inventory',
    'Historial de latencia': 'Latency history',
    'Disponibilidad en el periodo': 'Availability in the period',
    'Últimos 7 días': 'Last 7 days',
    'Últimos 30 días': 'Last 30 days',
    'Todo el historial': 'All history',
    'Sin eventos registrados': 'No events recorded',
    'Sin vecinos conocidos': 'No known neighbors',
    'Sin certificados': 'No certificates',
    'Sin datos en el periodo seleccionado': 'No data in the selected period',

    // ---- alerts ----
    'Reglas': 'Rules',
    'Canales': 'Channels',
    'Nueva regla': 'New rule',
    'Nuevo canal': 'New channel',
    'Editar regla': 'Edit rule',
    'Editar canal': 'Edit channel',
    'Reconocer': 'Acknowledge',
    'Resolver': 'Resolve',
    'Integrada': 'Built-in',
    'Personal': 'Custom',
    'Sin secreto': 'No secret',
    'Credencial cifrada': 'Encrypted credential',
    'Todas las severidades': 'All severities',
    'Todas las reglas': 'All rules',
    'Nombre de la regla': 'Rule name',
    'Tipo de regla': 'Rule type',
    'Severidad': 'Severity',
    'Umbral de latencia (ms)': 'Latency threshold (ms)',
    'Segundos caído antes de alertar': 'Seconds down before alerting',
    'Muestras consecutivas': 'Consecutive samples',
    'Porcentaje de ocupación': 'Usage percentage',
    'Número de escaneos seguidos': 'Number of consecutive scans',
    'Evita falsos positivos por un fallo puntual.': 'Avoids false positives from a single failure.',
    'Alerta cuando la subred supera este uso.': 'Alert when the subnet exceeds this usage.',
    'RTT medio por encima de este valor.': 'Average RTT above this value.',
    'Destinatarios (separados por comas)': 'Recipients (comma separated)',
    'Remitente': 'Sender',
    'Servidor SMTP': 'SMTP server',
    'Transporte': 'Transport',
    'Usuario SMTP': 'SMTP user',
    'Clave SMTP': 'SMTP password',
    'URL del webhook': 'Webhook URL',
    'Webhook de Slack': 'Slack webhook',
    'Webhook de Discord': 'Discord webhook',
    'Token del bot': 'Bot token',
    'Chat ID': 'Chat ID',
    'Clave de integración': 'Integration key',
    'Routing key': 'Routing key',
    'Secreto HMAC (opcional)': 'HMAC secret (optional)',
    'Probar': 'Test',
    'Probando…': 'Testing…',
    'Notificación de prueba enviada': 'Test notification sent',
    'El canal rechazó la prueba': 'The channel rejected the test',
    'Regla creada': 'Rule created',
    'Regla actualizada': 'Rule updated',
    'Regla eliminada': 'Rule deleted',
    'Regla habilitada': 'Rule enabled',
    'Regla deshabilitada': 'Rule disabled',
    'Canal creado': 'Channel created',
    'Canal actualizado': 'Channel updated',
    'Canal eliminado': 'Channel deleted',
    'Canal habilitado': 'Channel enabled',
    'Canal deshabilitado': 'Channel disabled',
    'El nombre es obligatorio': 'Name is required',

    // ---- scans ----
    'Nuevo escaneo': 'New scan',
    'Iniciar escaneo': 'Start scan',
    'Iniciar': 'Start',
    'Iniciando…': 'Starting…',
    'Cancelar escaneo': 'Cancel scan',
    'Perfil': 'Profile',
    'Perfil de escaneo': 'Scan profile',
    'Rápido': 'Quick',
    'Común': 'Common',
    'Completo': 'Full',
    'Personalizado': 'Custom',
    'Puertos personalizados': 'Custom ports',
    'Descubrir topología L2': 'Discover L2 topology',
    'Red pública': 'Public network',
    'Permitir redes públicas': 'Allow public networks',
    'Hallazgos': 'Findings',
    'Progreso': 'Progress',
    'Etapa': 'Stage',
    'Iniciado': 'Started',
    'Finalizado': 'Finished',
    'Cancelación solicitada': 'Cancellation requested',
    'Escaneo cancelado': 'Scan canceled',
    'Escaneo finalizado': 'Scan finished',
    'El CIDR es obligatorio': 'CIDR is required',
    'Indica una red o un CIDR': 'Enter a network or CIDR',
    'El perfil personalizado requiere puertos': 'The custom profile requires ports',
    'Rango de puertos inválido: ': 'Invalid port range: ',
    'Puerto inválido: ': 'Invalid port: ',

    // ---- reports ----
    'Generar informe': 'Generate report',
    'Generando…': 'Generating…',
    'Generar': 'Generate',
    'Informe generado': 'Report generated',
    'Informe listo': 'Report ready',
    'Informe eliminado': 'Report deleted',
    'Inventario completo': 'Full inventory',
    'Ejecutivo': 'Executive',
    'Tipo de informe': 'Report type',
    'Formato': 'Format',
    'Título': 'Title',
    'Descargar informe': 'Download report',
    'Eliminar informe': 'Delete report',

    // ---- settings ----
    'General': 'General',
    'Escaneo': 'Scanning',
    'Seguridad': 'Security',
    'Almacenamiento': 'Storage',
    'Usuarios': 'Users',
    'Copias': 'Backups',
    'Roles:': 'Roles:',
    'Los cambios se guardan en el archivo de configuración.':
      'Changes are saved to the configuration file.',
    'Dirección de escucha': 'Listen address',
    'URL pública': 'Public URL',
    'Certificado TLS': 'TLS certificate',
    'Clave TLS': 'TLS key',
    'Tamaño máximo de cuerpo (MB)': 'Maximum body size (MB)',
    'Nivel de registro': 'Log level',
    'Formato de registro': 'Log format',
    'Cabeceras de seguridad': 'Security headers',
    'Cambiarla requiere reiniciar el servicio.': 'Changing it requires a service restart.',
    'Se usa para enlaces y cookies seguras.': 'Used for links and secure cookies.',
    'Los cambios de dirección, TLS y registro se aplican al reiniciar el proceso. El resto se aplica en caliente.':
      'Address, TLS and logging changes apply after a restart. Everything else applies live.',
    'Perfil por defecto': 'Default profile',
    'Timeout por host': 'Timeout per host',
    'Concurrencia de hosts': 'Host concurrency',
    'Concurrencia de puertos': 'Port concurrency',
    'Máximo de hosts': 'Maximum hosts',
    'Timeout SNMP': 'SNMP timeout',
    'Versión SNMP': 'SNMP version',
    'Comunidades SNMP': 'SNMP communities',
    'Usuario SNMPv3': 'SNMPv3 user',
    'Auth SNMPv3': 'SNMPv3 auth',
    'Priv SNMPv3': 'SNMPv3 priv',
    'Protocolo auth': 'Auth protocol',
    'Protocolo priv': 'Priv protocol',
    'Comportamiento': 'Behavior',
    'Resolver DNS inverso': 'Reverse DNS lookup',
    'Capturar banners': 'Grab banners',
    'SNMP activado por defecto': 'SNMP enabled by default',
    'Monitor periódico': 'Periodic monitor',
    'Monitor activo': 'Monitor enabled',
    'Intervalo': 'Interval',
    'Intervalo de escaneo profundo': 'Deep scan interval',
    'Redes monitorizadas': 'Monitored networks',
    'Motor de alertas': 'Alert engine',
    'Alertas activadas': 'Alerts enabled',
    'Timeout de webhooks': 'Webhook timeout',
    'Máximo por minuto': 'Maximum per minute',
    'Destinos privados': 'Private targets',
    'Permitir webhooks a redes privadas': 'Allow webhooks to private networks',
    'Omitir verificación TLS (no recomendado)': 'Skip TLS verification (not recommended)',
    'Autenticación': 'Authentication',
    'Exigir inicio de sesión': 'Require sign-in',
    'Duración de sesión (horas)': 'Session duration (hours)',
    'Intentos de login / minuto': 'Login attempts / minute',
    'Peticiones API / minuto': 'API requests / minute',
    'Hosts permitidos': 'Allowed hosts',
    'Proxy inverso': 'Reverse proxy',
    'Confiar en X-Forwarded-For': 'Trust X-Forwarded-For',
    'Cambiar contraseña': 'Change password',
    'Contraseña actual': 'Current password',
    'Nueva contraseña': 'New password',
    'Actualizar contraseña': 'Update password',
    'Al cambiarla se cierran todas las sesiones, incluida esta.':
      'Changing it closes all sessions, including this one.',
    'Directorio de datos': 'Data directory',
    'Base de datos': 'Database',
    'Retención de alertas (días)': 'Alert retention (days)',
    'Retención de métricas (días)': 'Metrics retention (days)',
    'Retención de auditoría (days)': 'Audit retention (days)',
    'Retención de auditoría (días)': 'Audit retention (days)',
    'Máximo de escaneos': 'Maximum scans',
    'Requiere reinicio; no lo cambies con datos existentes.':
      'Requires a restart; do not change it with existing data.',
    'Ruta absoluta o relativa al directorio de datos.':
      'Absolute path or path relative to the data directory.',
    'Copias de seguridad': 'Backups',
    'Copia automática': 'Automatic backup',
    'Directorio de copias': 'Backup directory',
    'Intervalo entre copias': 'Interval between backups',
    'Copias a conservar': 'Backups to keep',
    'Crear copia ahora': 'Create backup now',
    'Creando…': 'Creating…',
    'La copia incluye la base de datos, config.yaml y secret.key. Guárdala en un lugar cifrado.':
      'The backup includes the database, config.yaml and secret.key. Store it somewhere encrypted.',
    'Aún no hay copias': 'No backups yet',
    'Descargar copia': 'Download backup',
    'Eliminar copia': 'Delete backup',
    'Token de métricas': 'Metrics token',
    'Si se define, /metrics exige Authorization: Bearer <token>.':
      'When set, /metrics requires Authorization: Bearer <token>.',
    'Exigir token en /metrics': 'Require token on /metrics',
    'La autenticación está activa. Los visores pueden consultar pero no modificar.':
      'Authentication is enabled. Viewers can read but not modify.',
    'La autenticación está desactivada: cualquiera con acceso al puerto es administrador.':
      'Authentication is disabled: anyone with port access is an administrator.',
    'Nuevo usuario': 'New user',
    'Sin usuarios': 'No users',
    'Administrador': 'Administrator',
    'Operador': 'Operator',
    'Solo lectura': 'Read-only',
    'Restablecer contraseña': 'Reset password',
    'Cambiar rol': 'Change role',
    'Eliminar usuario': 'Delete user',
    'Contraseña restablecida': 'Password reset',
    'Rol actualizado': 'Role updated',
    'Usuario eliminado': 'User deleted',
    'Mínimo 10 caracteres, con letras y números.': 'At least 10 characters, with letters and numbers.',
    'commit': 'commit',
    'sin fecha': 'no date',
    'activo desde': 'active since',
    'Sin interfaces': 'No interfaces',
    'Auditoría reciente': 'Recent audit',
    'Sin actividad registrada': 'No activity recorded',
    'Redes detectadas en este equipo': 'Networks detected on this machine',
    'Sin redes configuradas': 'No networks configured',
    'Cada red define el alcance del monitor y del inventario. Las redes privadas se detectan automáticamente.':
      'Each network defines the monitor and inventory scope. Private networks are detected automatically.',
    'Nueva red': 'New network',
    'Editar red': 'Edit network',
    'Eliminar red': 'Delete network',
    'Red habilitada para monitorización': 'Network enabled for monitoring',
    'Interfaz (opcional)': 'Interface (optional)',
    'sin interfaz': 'no interface',
    'Red actualizada': 'Network updated',
    'Red añadida': 'Network added',
    'Red eliminada': 'Network deleted',

    // ---- guía ----
    'Guía de uso': 'User guide',
    'Primeros pasos': 'Getting started',
    'Conceptos': 'Concepts',
    'Flujos de trabajo': 'Workflows',
    'Atajos y trucos': 'Tips and shortcuts',
    'Solución de problemas': 'Troubleshooting',
    'Iniciar recorrido guiado': 'Start guided tour',
    'Repetir recorrido': 'Restart tour',
    'Buscar en la guía…': 'Search the guide…',
    'Todo lo que necesitas para operar NetDiag, sin salir del producto.':
      'Everything you need to operate NetDiag, without leaving the product.',
    'Bienvenido a NetDiag': 'Welcome to NetDiag',
    'Este recorrido te muestra las zonas principales en menos de un minuto.':
      'This tour shows you the main areas in under a minute.',
    'Siguiente': 'Next',
    'Anterior': 'Previous',
    'Saltar': 'Skip',
    'Finalizar': 'Finish',
    'Paso': 'Step',

    // ---- mensajes ----
    'Configuración guardada': 'Settings saved',
    'Cambios guardados': 'Changes saved',
    'Dispositivo eliminado': 'Device removed',
    'Alerta reconocida': 'Alert acknowledged',
    'Alerta resuelta': 'Alert resolved',
    'Copiado al portapapeles': 'Copied to clipboard',
    'No se pudo copiar': 'Could not copy',
    'No se pudo contactar con el servidor': 'Could not reach the server',
    'Contraseña actualizada; vuelve a iniciar sesión': 'Password updated; please sign in again',
    'Copia creada': 'Backup created',
    'Copia eliminada': 'Backup deleted',
    'No se pudo crear la copia': 'Could not create the backup',
    'Archivo': 'File',
    'Tamaño': 'Size',
    'Creada': 'Created',

    // ---- integraciones externas ----
    'Integraciones': 'Integrations',
    'Nueva integración': 'New integration',
    'Editar integración': 'Edit integration',
    'Integración creada': 'Integration created',
    'Integración actualizada': 'Integration updated',
    'Integración eliminada': 'Integration deleted',
    'Integración habilitada': 'Integration enabled',
    'Integración deshabilitada': 'Integration disabled',
    'Evento de prueba enviado': 'Test event sent',
    'El destino rechazó la prueba': 'The target rejected the test',
    'Sin integraciones configuradas': 'No integrations configured',
    'Última prueba': 'Last test',
    'Último error': 'Last error',
    'Credencial cifrada': 'Encrypted credential',
    'Sin secreto': 'No secret',
    'Motor de integraciones': 'Integrations engine',
    'Exportar eventos': 'Export events',
    'Timeout de entrega': 'Delivery timeout',
    'Máximo de eventos por minuto': 'Maximum events per minute',
    'Tamaño de la cola': 'Queue size',
    'Destinos privados': 'Private targets',
    'Permitir Elasticsearch/Kafka/Redis/syslog locales': 'Allow local Elasticsearch/Kafka/Redis/syslog',
    'Integración activa': 'Integration enabled',
    'Necesario para 127.0.0.1, 10/8, 172.16/12 y 192.168/16. Bloquea siempre metadatos cloud.':
      'Required for 127.0.0.1, 10/8, 172.16/12 and 192.168/16. Cloud metadata is always blocked.',
    'Límite global compartido por todas las integraciones.': 'Global limit shared by all integrations.',
    'Eventos en memoria; si se llena se descartan con aviso.': 'In-memory events; when full, events are dropped with a warning.',
    'Exporta alertas y escaneos a sistemas externos. Los secretos se cifran con AES-256-GCM y nunca se muestran después de guardarlos.':
      'Export alerts and scans to external systems. Secrets are encrypted with AES-256-GCM and never shown after saving.',
    'Elasticsearch': 'Elasticsearch',
    'Syslog': 'Syslog',
    'Webhook': 'Webhook',
    'Slack': 'Slack',
    'Discord': 'Discord',
    'MS Teams': 'MS Teams',
    'Telegram': 'Telegram',
    'PagerDuty': 'PagerDuty',
    'Kafka REST': 'Kafka REST',
    'Redis Streams': 'Redis Streams',

    // ---- diagnósticos empresariales ----
    'Diagnósticos': 'Diagnostics',
    'Problemas empresariales': 'Enterprise issues',
    'Lazo STP': 'STP loop',
    'Fuga de VLAN': 'VLAN leak',
    'Desajuste de dúplex': 'Duplex mismatch',
    'Problema de MTU': 'MTU problem',
    'Tormenta de broadcast': 'Broadcast storm',
    'AP no autorizado': 'Rogue AP',
    'Incumplimiento de SLA': 'SLA breach',
    'Evidencia': 'Evidence',
    'Recomendación': 'Recommendation',
    'Túneles': 'Tunnels',
    'Túneles observados': 'Observed tunnels',
    'túneles observados': 'tunnels observed',
    'Sin túneles detectados': 'No tunnels detected',
    'No se encontraron indicios de VPN en las tablas locales ni en los puertos conocidos.':
      'No VPN traces were found in local tables or well-known ports.',
    'Observar ahora': 'Observe now',
    'No se pudo observar': 'Could not observe',
    'Activo': 'Active',
    'Activos': 'Active',
    'Observado': 'Observed',
    'Obsoleto': 'Stale',
    'Tráfico': 'Traffic',
    'Confianza mínima': 'Minimum confidence',
    'umbral configurado': 'configured threshold',
    'observación periódica': 'periodic observation',
    'observaciones persistentes': 'persistent observations',
    'con endpoints o tráfico': 'with endpoints or traffic',
    'Detección pasiva y heurística a partir de tablas del kernel (sockets e interfaces) y puertos conocidos. No se inspecciona ni descifra tráfico.':
      'Passive heuristic detection from kernel tables (sockets and interfaces) and well-known ports. Traffic is neither inspected nor decrypted.',
    'Los contadores de interfaz son acumulativos desde el arranque, no tasas instantáneas.':
      'Interface counters are cumulative since boot, not instantaneous rates.',
    'TCP/443 es ambiguo: requiere otra evidencia para confirmar OpenVPN.':
      'TCP/443 is ambiguous: additional evidence is required to confirm OpenVPN.',
    'Un túnel inactivo o cifrado puede no dejar rastros en las tablas consultadas.':
      'An idle or encrypted tunnel may leave no traces in the inspected tables.',
    'Actualizado en cada observación': 'Refreshed on every observation',
    'WiFi': 'Wi-Fi',
    'APs, clientes e interferencias': 'APs, clients and interference',
    'APs observados': 'APs observed',
    'clientes observados': 'clients observed',
    'No se pudo capturar': 'Could not capture',
    '(oculta)': '(hidden)',
    'No autorizado': 'Unauthorized',
    'Autorizado': 'Authorized',
    'Sin APs observados': 'No APs observed',
    'Captura beacons con una interfaz en modo monitor para poblar esta vista.':
      'Capture beacons with a monitor-mode interface to populate this view.',
    'Sin datos de canales': 'No channel data',
    'Sin clientes observados': 'No clients observed',
    'Análisis pasivo de 802.11: APs, clientes, interferencia por canal y APs no autorizados según el inventario de referencia.':
      'Passive 802.11 analysis: APs, clients, channel interference and unauthorized APs according to the reference inventory.',
    'Capturar ahora': 'Capture now',
    'Clientes': 'Clients',
    'estaciones detectadas': 'stations detected',
    'según inventario configurado': 'per configured inventory',
    'Canales saturados': 'Congested channels',
    'carga alta': 'high load',
    'APs no autorizados': 'Unauthorized APs',
    'Puntos de acceso': 'Access points',
    'sin interfaz configurada': 'no interface configured',
    'ventana': 'window',
    'SSID / BSSID': 'SSID / BSSID',
    'Canal': 'Channel',
    'Cifrado': 'Encryption',
    'Señal': 'Signal',
    'Solape': 'Overlap',
    'Carga': 'Load',
    'Sugerencia': 'Suggestion',
    'Clientes por AP': 'Clients per AP',
    'últimas observaciones': 'latest observations',
    'MAC': 'MAC',
    'AP': 'AP',
    'baja': 'low',
    'media': 'medium',
    'alta': 'high',
    'Captura disponible': 'Capture available',
    'Captura no compilada': 'Capture not compiled',
    'La interfaz debe configurarse previamente en modo monitor; NetDiag no cambia el modo del dispositivo.':
      'The interface must be put into monitor mode beforehand; NetDiag never changes the device mode.',
    'El binario portable no incluye captura 802.11; se requiere compilar con -tags pcap.':
      'The portable binary does not include 802.11 capture; build with -tags pcap.',
    'El SSID puede suplantarse: el BSSID autorizado es la evidencia más fuerte; los SSID conocidos tienen menor confianza.':
      'SSIDs can be spoofed: an authorized BSSID is the strongest evidence; known SSIDs carry less confidence.',
    'El solape en 2,4 GHz agrupa canales con separación inferior a cinco; en 5/6 GHz solo se considera el mismo canal.':
      '2.4 GHz overlap groups channels closer than five; on 5/6 GHz only the same channel is considered.',
    'Dashboards': 'Dashboards',
    'Vistas personalizadas': 'Custom views',
    'Crea vistas con hasta 16 widgets. Arrastra una tarjeta para reordenarla; la rejilla tiene 12 columnas.':
      'Create views with up to 16 widgets. Drag a tile to reorder it; the grid has 12 columns.',
    'Ejemplo "Mi red"': '"My network" example',
    'Nombre del dashboard': 'Dashboard name',
    'Añadir widget': 'Add widget',
    'Añadir': 'Add',
    'Quitar widget': 'Remove widget',
    'Dashboard vacío': 'Empty dashboard',
    'Añade widgets para construir tu vista.': 'Add widgets to build your view.',
    'Todavía no hay dashboards': 'No dashboards yet',
    'Pulsa en ejemplo "Mi red" para crear uno con cuatro widgets.':
      'Click the "My network" example to create one with four widgets.',
    'Dashboard creado': 'Dashboard created',
    'No se pudo crear': 'Could not create',
    'widgets': 'widgets',
    '¿Eliminar este dashboard?': 'Delete this dashboard?',
    'Dashboard eliminado': 'Dashboard deleted',
    'No se pudo eliminar': 'Could not delete',
    'No se pudo guardar': 'Could not save',
    'Sin datos': 'No data',
    'Sin dashboards': 'No dashboards',
    'más': 'more',
    'Mi red': 'My network',
    'Puertos frecuentes': 'Frequent ports',
    'Escaneos recientes': 'Recent scans',
    'APs WiFi': 'Wi-Fi APs',
    'Túneles VPN': 'VPN tunnels',
    'Sin anomalías detectadas': 'No anomalies detected',
    'Las heurísticas no encontraron señales suficientes en la ventana analizada.':
      'The heuristics did not find enough signals in the selected window.',
    'Análisis heurístico a partir de vecindades LLDP/CDP/FDB, IF-MIB, inventario y muestras de disponibilidad. Cada hallazgo incluye evidencia, confianza y limitaciones.':
      'Heuristic analysis based on LLDP/CDP/FDB adjacency, IF-MIB, inventory and availability samples. Every finding includes evidence, confidence and limitations.',
    'Anomalías': 'Anomalies',
    'hosts analizados': 'hosts analyzed',
    'Críticas': 'Critical',
    'requieren atención inmediata': 'need immediate attention',
    'Avisos': 'Warnings',
    'requieren revisión': 'need review',
    'Muestras': 'Samples',
    'en la ventana': 'in the window',
    'Modo empresarial desactivado': 'Enterprise mode disabled',
    'Solo se evalúan los umbrales de SLA; configura diagnostics.enterprise_mode: true para el resto de detectores.':
      'Only SLA thresholds are evaluated; set diagnostics.enterprise_mode: true to enable the remaining detectors.',
    'Hosts': 'Hosts',
    'Confianza': 'Confidence',
    'Detectado': 'Detected',
    'Por categoría': 'By category',
    'stp': 'STP',
    'vlan': 'VLAN',
    'duplex': 'Duplex',
    'performance': 'Performance',
    'wireless': 'Wireless',
    'sla': 'SLA',
    'Limitaciones del análisis': 'Analysis limitations',
    'Heurístico': 'Heuristic',
    'Un hallazgo es una pista para investigar, no una confirmación del protocolo. Confirma siempre en el equipo de red.':
      'A finding is a lead to investigate, not a protocol confirmation. Always verify on the network device.',

    // ---- F21 comparativa temporal ----
    'Comparativa': 'Comparison',
    'Comparativa temporal': 'Time comparison',
    'Hoy vs hace 7 días': 'Today vs 7 days ago',
    'Periodo': 'Period',
    'muestras': 'samples',
    'disponibilidad': 'availability',
    'RTT medio': 'Average RTT',
    'activos': 'up',
    'Latencia (ms)': 'Latency (ms)',
    'topología': 'topology',
    'Compara dos momentos equivalentes de duración. Por defecto: hoy frente a la misma ventana hace 7 días.':
      'Compare two moments of equal duration. By default: today against the same window seven days ago.',
    '7 días': '7 days',
    '30 días': '30 days',
    '6 h': '6 h',
    '48 h': '48 h',
    'Ventana': 'Window',
    'Cambios detectados': 'Detected changes',
    'cambios': 'changes',
    'nuevos': 'new',
    'ausentes': 'missing',
    'truncado': 'truncated',
    'Entidad': 'Entity',
    'Objeto': 'Object',
    'Antes': 'Before',
    'Ahora': 'Now',
    'Sin cambios entre los periodos': 'No changes between periods',
    'Las métricas y la topología disponibles no muestran diferencias con los umbrales configurados.':
      'The available metrics and topology show no differences beyond the configured thresholds.',
    'Limitaciones del diff': 'Diff limitations',
    'Fuente: muestras de disponibilidad y latencia almacenadas; los cambios de topología usan snapshots capturados tras cada escaneo y cada hora. Un valor ausente se reporta como limitación, nunca como "sin cambios".':
      'Source: stored availability and latency samples; topology changes use snapshots captured after every scan and hourly. Missing data is reported as a limitation, never as "no changes".',
    'Dispositivo nuevo': 'New device',
    'Dispositivo ausente': 'Missing device',
    'Cambio de estado': 'Status change',
    'Cambio de disponibilidad': 'Availability change',
    'Cambio de latencia': 'Latency change',
    'Nodo nuevo': 'New node',
    'Nodo eliminado': 'Node removed',
    'Enlace nuevo': 'New link',
    'Enlace eliminado': 'Link removed',
    'Disponibilidad global': 'Global availability',
    'Latencia global': 'Global latency',

    // ---- F22 modo auditoría ----
    'Auditoría': 'Audit',
    'Snapshots y diff': 'Snapshots and diff',
    'Sin nombre': 'Unnamed',
    'Sin cambios entre snapshots': 'No changes between snapshots',
    'Las dos líneas base son equivalentes según los campos auditados.':
      'Both baselines are equivalent across the audited fields.',
    'Diff de auditoría': 'Audit diff',
    'Campo': 'Field',
    'Dispositivo modificado': 'Device changed',
    'Puerto modificado': 'Port changed',
    'Red nueva': 'New network',
    'Red modificada': 'Network changed',
    'Vecino nuevo': 'New neighbor',
    'Vecino eliminado': 'Neighbor removed',
    'Vecino modificado': 'Neighbor changed',
    'Regla nueva': 'New rule',
    'Regla modificada': 'Rule changed',
    'Canal nuevo': 'New channel',
    'Canal modificado': 'Channel changed',
    'Integración nueva': 'New integration',
    'Integración modificada': 'Integration changed',
    'Escaneo nuevo': 'New scan',
    'Escaneo eliminado': 'Scan removed',
    'Escaneo modificado': 'Scan changed',
    'Nodo modificado': 'Node changed',
    'Enlace modificado': 'Link changed',
    'Alertas modificadas': 'Alerts changed',
    'Configuración modificada': 'Configuration changed',
    'Todavía no hay snapshots': 'No snapshots yet',
    'Guarda una línea base, haz cambios en la red y comparámosla con otra snapshot.':
      'Save a baseline, change the network and compare it with another snapshot.',
    'JSON': 'JSON',
    'GZIP': 'GZIP',
    'Guarda una línea base del estado actual, realiza cambios y compara dos snapshots para ver qué cambió.':
      'Save a baseline of the current state, make changes and compare two snapshots to see what changed.',
    'Nombre del snapshot (opcional)': 'Snapshot name (optional)',
    'Guardar snapshot': 'Save snapshot',
    'Comparar snapshots': 'Compare snapshots',
    'Desde': 'From',
    'Hasta': 'To',
    'Ver diff': 'View diff',
    'Líneas base guardadas': 'Saved baselines',
    'snapshots': 'snapshots',
    'Acciones': 'Actions',
    'Snapshot guardado': 'Snapshot saved',
    'Snapshot eliminado': 'Snapshot deleted',
    '¿Eliminar este snapshot?': 'Delete this snapshot?',
    'Los snapshots se comprimen y se guardan sin secretos: solo se conservan los nombres de las claves de configuración, nunca sus valores.':
      'Snapshots are compressed and stored without secrets: only configuration key names are kept, never their values.',

    // ---- F23 auto-actualización ----
    'Actualizaciones': 'Updates',
    'Comprobar versión': 'Check for updates',
    'Versión instalada': 'Installed version',
    'Activadas': 'Enabled',
    'Desactivadas': 'Disabled',
    'No hay una versión más reciente comprobada': 'No newer version checked yet',
    'Actualización pendiente de confirmación': 'Update pending confirmation',
    'Instalar y reiniciar': 'Install and restart',
    'Revertir': 'Roll back',
    'Actualización disponible': 'Update available',
    'Actualización instalada': 'Update installed',
    'Actualización revertida': 'Update rolled back',
    'Reinicio': 'Restart',
    'Reinicio fallido': 'Restart failed',
    'El servidor se está reiniciando': 'The server is restarting',
    'Comprobación completada': 'Check completed',
    'Versión anterior restaurada': 'Previous version restored',
    '¿Instalar la última versión firmada y reiniciar el servicio?': 'Install the latest signed version and restart the service?',

    // ---- document title ----
    'NetDiag · Inventario y monitoreo de red': 'NetDiag · Network inventory and monitoring',
  };

  const patterns = [
    [/^(.+) · tú$/, '$1 · you'],
    [/^(\d+) copias?$/, '$1 backups'],
    [/^(\d+) usuarios?$/, '$1 users'],
    [/^(\d+) reglas?$/, '$1 rules'],
    [/^(\d+) canales?$/, '$1 channels'],
    [/^(\d+) alertas?$/, '$1 alerts'],
    [/^hace (\d+) min$/, '$1 min ago'],
    [/^hace (\d+) h$/, '$1 h ago'],
    [/^hace (\d+) d$/, '$1 d ago'],
    [/^hace (\d+) s$/, '$1 s ago'],
    [/^(\d+) nuevos en 24 h$/, '$1 new in 24 h'],
    [/^(\d+) caídos$/, '$1 down'],
    [/^(\d+) críticas$/, '$1 critical'],
    [/^(\d+) escaneos en 24 h$/, '$1 scans in 24 h'],
    [/^(\d+) en línea$/, '$1 online'],
    [/^(\d+) redes$/, '$1 networks'],
    [/^(\d+) encontrados$/, '$1 found'],
    [/^(\d+) dispositivos?$/, '$1 devices'],
    [/^(\d+) puertos?$/, '$1 ports'],
    [/^(\d+) abiertos$/, '$1 open'],
    [/^activo desde (.+)$/, 'active since $1'],
    [/^(\d+) corriendo$/, '$1 running'],
    [/^Versión (.+) disponible$/, 'Version $1 available'],
    [/^Versión (.+) instalada; reiniciando…$/, 'Version $1 installed; restarting…'],
    [/^Versión (.+) instalada$/, 'Version $1 installed'],
    [/^Versión (.+) · copia: (.+)$/, 'Version $1 · backup: $2'],
    [/^¿Instalar NetDiag (.+) y reiniciar el servicio\?$/, 'Install NetDiag $1 and restart the service?'],
  ];

  function savedLang() {
    let v = '';
    try { v = localStorage.getItem(STORAGE_KEY) || ''; } catch {}
    if (SUPPORTED.includes(v)) return v;
    const nav = (navigator.language || 'es').toLowerCase();
    return nav.startsWith('en') ? 'en' : 'es';
  }

  let lang = savedLang();

  function translate(src) {
    if (lang !== 'en') return src;
    if (Object.prototype.hasOwnProperty.call(EN, src)) return EN[src];
    for (const [re, repl] of patterns) {
      if (re.test(src)) return src.replace(re, repl);
    }
    return src;
  }

  function t(src, vars) {
    let out = translate(src);
    if (vars) {
      out = out.replace(/\{(\w+)\}/g, (m, k) => (vars[k] === undefined ? m : String(vars[k])));
    }
    return out;
  }

  function skipNode(el) {
    if (!el || el.nodeType !== 1) return false;
    return !!el.closest('script,style,code,pre,svg,.mono,[data-no-i18n]');
  }

  function applyText(node) {
    const parent = node.parentElement;
    if (!parent || skipNode(parent)) return;
    const value = node.nodeValue;
    if (!value) return;
    const trimmed = value.trim();
    if (trimmed.length < 2) return;
    const next = translate(trimmed);
    if (next === trimmed) return;
    const idx = value.indexOf(trimmed);
    node.nodeValue = value.slice(0, idx) + next + value.slice(idx + trimmed.length);
  }

  function applyAttributes(root) {
    if (!root || root.nodeType !== 1) return;
    const nodes = [root, ...root.querySelectorAll('[placeholder],[title],[aria-label],[alt]')];
    for (const el of nodes) {
      if (!el || !el.getAttribute || skipNode(el)) continue;
      for (const attr of ['placeholder', 'title', 'aria-label', 'alt']) {
        const val = el.getAttribute(attr);
        if (!val) continue;
        const trimmed = val.trim();
        const next = translate(trimmed);
        if (next !== trimmed) el.setAttribute(attr, val.replace(trimmed, next));
      }
    }
  }

  function apply(root) {
    if (lang !== 'en') return;
    const target = root || document.body;
    if (!target) return;
    if (target.nodeType === 3) {
      applyText(target);
      return;
    }
    if (target.nodeType !== 1 && target.nodeType !== 9) return;
    if (target.nodeType === 1) applyAttributes(target);
    const walker = document.createTreeWalker(target, NodeFilter.SHOW_TEXT);
    let n;
    while ((n = walker.nextNode())) applyText(n);
    if (target.querySelectorAll) {
      target.querySelectorAll('[placeholder],[title],[aria-label],[alt]').forEach((el) => applyAttributes(el));
    }
  }

  let observer = null;
  function startObserver() {
    if (observer || lang !== 'en' || !document.body) return;
    observer = new MutationObserver((mutations) => {
      for (const m of mutations) {
        if (m.type === 'characterData' && m.target) applyText(m.target);
        for (const node of m.addedNodes || []) {
          if (node.nodeType === 1 || node.nodeType === 3) apply(node);
        }
      }
    });
    observer.observe(document.body, { childList: true, subtree: true, characterData: true });
  }

  function setLang(next) {
    if (!SUPPORTED.includes(next)) return;
    try { localStorage.setItem(STORAGE_KEY, next); } catch {}
    location.reload();
  }

  function toggle() { setLang(lang === 'en' ? 'es' : 'en'); }

  function dateLocale() { return lang === 'en' ? 'en-GB' : 'es-ES'; }

  // Diálogos nativos: traducir el mensaje antes de mostrarlo.
  const nativeConfirm = window.confirm.bind(window);
  window.confirm = (msg) => nativeConfirm(t(String(msg)));

  lang = savedLang();
  document.documentElement.lang = lang;

  const api = {
    get lang() { return lang; },
    get isEnglish() { return lang === 'en'; },
    t, apply, setLang, toggle, dateLocale,
    label: lang === 'en' ? 'ES' : 'EN',
    nextLabel: lang === 'en' ? 'Español' : 'English',
  };
  window.NetDiagI18n = api;

  function applyTitle() {
    if (lang !== 'en') return;
    const next = translate(document.title);
    if (next !== document.title) document.title = next;
  }

  const boot = () => {
    document.documentElement.lang = lang;
    applyTitle();
    apply(document.body);
    startObserver();
  };
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', boot);
  else boot();
})();
