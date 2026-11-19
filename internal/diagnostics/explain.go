// Package diagnostics — deterministic explanations for findings (F50/F56).
//
// Every text here is a fixed template selected by anomaly Type. There is no
// model, no network call and no free-text inference: the same evidence always
// produces the same explanation. A cause is a hypothesis to verify, never a
// confirmed root cause.
package diagnostics

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kr3s4l4/netdiag/internal/model"
)

type hypothesisTemplate struct {
	label      string
	confidence int
	why        string
	check      string
}

type explanationTemplate struct {
	cause  string
	impact string
	steps  []string
	limits string
	hypos  []hypothesisTemplate
}

// explanations is the single source of truth for the human-readable layer of
// a finding. Keys are the Type emitted by the detectors.
var explanations = map[string]explanationTemplate{
	"arp_gateway_mac_changed": {
		cause:  "Otro equipo está respondiendo por la IP del gateway, o el router cambió de interfaz.",
		impact: "El tráfico del segmento puede pasar por un equipo no autorizado: credenciales, DNS y sesiones.",
		steps: []string{
			"Comparar la MAC actual con la etiqueta física del router.",
			"Localizar esa MAC en el puerto del switch mediante FDB o LLDP.",
			"Revisar los servidores DHCP autorizados y los ARP gratuitos recientes.",
			"Si no es un cambio legítimo, desconectar el puerto y escalar.",
			"Repetir el escaneo y confirmar que el aviso desaparece.",
		},
		limits: "Sin captura L2 no se puede confirmar ARP spoofing; el dato viene del histórico de escaneos.",
		hypos: []hypothesisTemplate{
			{"Suplantación del gateway", 90, "La MAC del gateway cambió entre escaneos.", "Confirmar la MAC en el router y localizar el puerto."},
			{"Cambio legítimo de hardware o interfaz", 70, "Un reemplazo de router también cambia la MAC.", "Consultar el registro de mantenimiento de red."},
		},
	},
	"arp_ip_mac_changed": {
		cause:  "El dispositivo cambió de tarjeta de red, u otro equipo reclama su IP.",
		impact: "Puede haber una suplantación puntual o un inventario desactualizado.",
		steps: []string{
			"Comparar la MAC anterior y la actual con el inventario.",
			"Localizar ambas MAC en FDB o LLDP del switch.",
			"Preguntar al propietario del equipo si hubo cambio de NIC.",
			"Repetir el escaneo tras la verificación.",
		},
		limits: "Sin captura ARP en vivo el cambio es una pista, no una confirmación.",
		hypos: []hypothesisTemplate{
			{"Cambio de tarjeta o contenedor", 75, "Los cambios de NIC son habituales en virtualización.", "Contrastar con el inventario y el registro de mantenimiento."},
			{"Suplantación de IP", 70, "Dos MAC para una IP pueden indicar un cliente ilegítimo.", "Localizar ambas MAC en el switch."},
		},
	},
	"arp_mac_multi_ip": {
		cause:  "Una misma MAC está respondiendo por varias IP, o hay un equipo con múltiples interfaces en puente.",
		impact: "Puede indicar suplantación de direcciones o una configuración de red involuntaria.",
		steps: []string{
			"Listar las IP implicadas y compararlas con el inventario.",
			"Identificar el puerto switch al que pertenece la MAC.",
			"Comprobar si el equipo es un router, firewall o proxy legítimo.",
			"Escalar si no hay una función de enrutamiento autorizada.",
		},
		limits: "Un firewall o balanceador legítimo puede explicar el patrón sin anomalía.",
		hypos: []hypothesisTemplate{
			{"Equipo de enrutamiento legítimo", 75, "Los gateways suelen tener varias IP en una MAC.", "Verificar el tipo de dispositivo en el inventario."},
			{"Suplantación múltiple", 65, "Una MAC que reclama muchas IP puede secuestrar tráfico.", "Localizar el puerto y corroborar con FDB."},
		},
	},
	"arp_ip_multi_mac": {
		cause:  "Varios equipos están respondiendo por la misma IP, o el histórico mezcla dos sistemas distintos.",
		impact: "El tráfico a esa IP puede repartirse entre equipos no previstos.",
		steps: []string{
			"Ordenar las MAC por fecha de observación.",
			"Comprobar si una de ellas pertenece al equipo legítimo.",
			"Buscar la otra MAC en la tabla FDB del switch.",
			"Repetir el escaneo para ver si persiste.",
		},
		limits: "Sin correlación con FDB no se sabe qué puerto físico responde.",
		hypos: []hypothesisTemplate{
			{"Conflicto de IP", 80, "Dos hosts con la misma IP se solapan en el tiempo.", "Comparar las marcas de tiempo de cada MAC."},
			{"Cambio de dispositivo", 70, "Un reemplazo explica dos MAC en momentos distintos.", "Contrastar con el registro de mantenimiento."},
		},
	},
	"dhcp_multiple_servers": {
		cause:  "Más de un servidor DHCP respondió en el mismo sondeo.",
		impact: "Los clientes pueden recibir gateway, DNS o máscara incorrectos y perder conectividad.",
		steps: []string{
			"Listar los servidores que respondieron y su MAC.",
			"Comparar con la lista de servidores autorizados.",
			"Desactivar el servidor no autorizado desde el equipo legítimo.",
			"Repetir un DHCPINFORM y comprobar una sola respuesta.",
		},
		limits: "El sondeo ve solo respuestas a nuestras solicitudes, no el tráfico de otros clientes.",
		hypos: []hypothesisTemplate{
			{"Servidor DHCP rogue", 90, "Dos respuestas en el mismo segmento sin autorización.", "Identificar y desconectar el servidor no autorizado."},
			{"Segundo DHCP legítimo en migración", 65, "Una migración puede dejar dos servidores activos.", "Consultar el cambio planificado."},
		},
	},
	"dhcp_server_mac_changed": {
		cause:  "El servidor DHCP cambió de MAC, u otro equipo está suplantando sus ofertas.",
		impact: "Los clientes pueden recibir parámetros de red manipulados.",
		steps: []string{
			"Comparar la MAC del servidor con el inventario.",
			"Localizar la MAC actual en el switch.",
			"Verificar si hubo cambio de hardware planificado.",
			"Escalar si no hay mantenimiento documentado.",
		},
		limits: "Sin inspección de paquetes no se distingue un cambio legítimo de una suplantación.",
		hypos: []hypothesisTemplate{
			{"Suplantación del servidor DHCP", 85, "La MAC del servidor difiere del histórico.", "Localizar la MAC y confirmarla en el equipo autorizado."},
			{"Reemplazo de hardware", 70, "Un servidor nuevo también cambia la MAC.", "Revisar el registro de cambios."},
		},
	},
	"dhcp_gateway_spoof": {
		cause:  "La oferta DHCP anuncia un gateway distinto del observado en la red.",
		impact: "Riesgo de interceptación del tráfico saliente de los clientes.",
		steps: []string{
			"Comparar el gateway anunciado con el gateway real.",
			"Identificar el servidor que emitió la oferta.",
			"Desactivar ese servidor si no está autorizado.",
			"Renovar la configuración de un cliente de prueba y comprobar la ruta.",
		},
		limits: "El hallazgo se basa en la respuesta al sondeo, no en tráfico de producción.",
		hypos: []hypothesisTemplate{
			{"DHCP spoofing", 90, "El gateway ofrecido no coincide con el ARP observado.", "Localizar el servidor DHCP y verificar su MAC."},
			{"Cambio de gateway planificado", 60, "Una migración puede explicar el nuevo gateway.", "Confirmar con el equipo de red."},
		},
	},
	"dhcp_unauthorized_server": {
		cause:  "Un servidor DHCP distinto al autorizado respondió una oferta.",
		impact: "Puede entregar gateway, DNS o máscara incorrectos a los clientes.",
		steps: []string{
			"Anotar la IP y MAC del servidor que respondió.",
			"Comparar con la lista de servidores autorizados.",
			"Localizar el equipo en el switch por FDB o LLDP.",
			"Desactivarlo y repetir el sondeo.",
		},
		limits: "Solo se ven respuestas a DHCPINFORM/DHCPDISCOVER, no otros clientes.",
		hypos: []hypothesisTemplate{
			{"Servidor rogue o equipo del usuario", 85, "Un servidor no autorizado respondió en la red.", "Localizar el equipo y confirmar su propiedad."},
			{"Dispositivo con DHCP integrado", 70, "Un router doméstico puede repartir direcciones por defecto.", "Revisar el inventario de la zona."},
		},
	},
	"snmp_fdb_mac_moved": {
		cause:  "Una MAC apareció en un puerto distinto del esperado en la tabla FDB.",
		impact: "Puede indicar movimiento físico, bucle o suplantación de identidad.",
		steps: []string{
			"Comparar los puertos anterior y actual de la MAC.",
			"Contrastar con LLDP o CDP del puerto actual.",
			"Preguntar si hubo traslado físico del equipo.",
			"Repetir el sondeo SNMP para ver si se estabiliza.",
		},
		limits: "Sin BPDU ni captura L2 no se puede distinguir bucle de movimiento legítimo.",
		hypos: []hypothesisTemplate{
			{"Movimiento físico del equipo", 75, "La MAC cambió de puerto de forma puntual.", "Confirmar con el inventario de puestos."},
			{"Bucle o suplantación L2", 70, "El movimiento repetido es compatible con un bucle.", "Revisar STP y FDB en los switches."},
		},
	},
	"snmp_fdb_port_conflict": {
		cause:  "La misma MAC aparece en varios puertos del switch.",
		impact: "Inestabilidad L2 y posible falseo de identidad de dispositivo.",
		steps: []string{
			"Listar los puertos implicados y sus vecinos LLDP.",
			"Comprobar si hay enlaces entre esos puertos.",
			"Revisar la configuración de trunk y agregación.",
			"Corregir el cableado o la configuración tras validación.",
		},
		limits: "La tabla FDB es una foto instantánea; un valor envejecido puede explicar el conflicto.",
		hypos: []hypothesisTemplate{
			{"Lazo L2", 80, "Una MAC vista en varios puertos suele indicar bucle.", "Revisar STP y enlaces redundantes."},
			{"Entrada FDB envejecida", 60, "Las tablas pueden conservar filas antiguas.", "Repetir el sondeo y comparar."},
		},
	},
	"snmp_fdb_vlan_conflict": {
		cause:  "La misma MAC aparece en varias VLAN dentro de la tabla FDB.",
		impact: "Posible fuga de VLAN o dispositivo con trunk mal etiquetado.",
		steps: []string{
			"Identificar las VLAN implicadas y sus puertos.",
			"Comprobar el etiquetado 802.1Q de esos puertos.",
			"Verificar si el equipo tiene interfaces en varias VLAN.",
			"Corregir el trunk tras confirmación.",
		},
		limits: "Sin captura de tramas etiquetadas no se confirma fuga de VLAN.",
		hypos: []hypothesisTemplate{
			{"Trunk mal configurado", 80, "Una MAC en varias VLAN apunta a etiquetado incorrecto.", "Revisar puertos trunk y nativos."},
			{"Equipo multihomed legítimo", 60, "Un firewall puede pertenecer a varias VLAN.", "Contrastar con el inventario."},
		},
	},
	"snmp_lldp_fdb_disagree": {
		cause:  "LLDP y FDB señalan puertos distintos para la misma MAC.",
		impact: "Posible bucle, movimiento físico reciente o suplantación de un vecino.",
		steps: []string{
			"Comparar el vecino LLDP con el puerto FDB.",
			"Comprobar si el vecino tiene varios enlaces al switch.",
			"Confirmar el puerto físico en el armario.",
			"Repetir el sondeo tras la verificación.",
		},
		limits: "LLDP puede estar desactivado en el vecino; la ausencia no prueba un cambio.",
		hypos: []hypothesisTemplate{
			{"Movimiento reciente del dispositivo", 75, "FDB y LLDP se actualizan en momentos distintos.", "Repetir el sondeo y comparar marcas de tiempo."},
			{"Vecino suplantado", 70, "Un puerto distinto puede anunciar la misma identidad.", "Verificar físicamente el cableado."},
		},
	},
	"snmp_vlan_unknown": {
		cause:  "La tabla FDB referencia una VLAN sin nombre configurable.",
		impact: "Dificulta atribuir tráfico a un segmento y puede ocultar una VLAN no documentada.",
		steps: []string{
			"Anotar el ID de VLAN y el puerto donde aparece.",
			"Comprobar si existe en la configuración del switch.",
			"Documentarla o eliminarla si no está autorizada.",
			"Repetir el sondeo SNMP.",
		},
		limits: "Algunos switches no exponen nombres de VLAN por SNMP.",
		hypos: []hypothesisTemplate{
			{"VLAN sin documentar", 70, "El ID existe en FDB pero no en la configuración leída.", "Revisar la configuración del switch."},
			{"Limitación del OID", 60, "El dispositivo puede no publicar el nombre.", "Consultar el MIB del fabricante."},
		},
	},
	"stp_loop": {
		cause:  "Dos enlaces anuncian root bridge en puertos distintos.",
		impact: "Tormenta de broadcast y caída L2 del segmento.",
		steps: []string{
			"Listar los puertos y switches implicados.",
			"Comparar prioridades de root bridge y costes de puerto.",
			"Comprobar el estado STP en cada puerto.",
			"Corregir la topología tras validación humana.",
		},
		limits: "No se capturan BPDUs: el ciclo se infiere de LLDP/CDP/FDB de alta confianza.",
		hypos: []hypothesisTemplate{
			{"Lazo de spanning-tree", 90, "Múltiples anuncios de root bridge en puertos distintos.", "Revisar STP y desconectar el enlace sobrante."},
			{"Cambio de root bridge planificado", 60, "Una redistribución también cambia el root.", "Contrastar con el registro de cambios."},
		},
	},
	"broadcast_storm": {
		cause:  "Tráfico broadcast excesivo en la VLAN durante la ventana observada.",
		impact: "Latencia, pérdida de paquetes y saturación de CPU en los equipos.",
		steps: []string{
			"Identificar la VLAN y el rango de fechas del pico.",
			"Capturar tráfico en la VLAN con SPAN o tap.",
			"Revisar bucles L2 y storm control en los switches.",
			"Corregir la causa antes de ampliar el umbral.",
		},
		limits: "No hay contadores broadcast por puerto: el dato viene de métricas agregadas.",
		hypos: []hypothesisTemplate{
			{"Lazo L2 o tormenta de broadcast", 85, "El pico coincide con vecindades L2 anómalas.", "Capturar en la VLAN y revisar STP."},
			{"Tráfico legítimo elevado", 60, "Descubrimientos o backups pueden elevar el broadcast.", "Correlacionar con ventanas de mantenimiento."},
		},
	},
	"duplex_mismatch": {
		cause:  "Los extremos de un enlace negociaron velocidades o dúplex distintos.",
		impact: "Colisiones, retransmisiones y pérdida de rendimiento en el enlace.",
		steps: []string{
			"Comparar velocidad y dúplex de ambos extremos por SNMP.",
			"Revisar el cableado y el puerto del switch.",
			"Forzar la misma configuración en ambos lados tras validación.",
			"Repetir el sondeo y comprobar errores de interfaz.",
		},
		limits: "No hay OID de dúplex ni contadores de error: se comparan velocidades IF-MIB.",
		hypos: []hypothesisTemplate{
			{"Negociación automática fallida", 80, "Los extremos anuncian velocidades distintas.", "Forzar velocidad y dúplex de forma controlada."},
			{"Cable o puerto defectuoso", 65, "Un enlace degradado también baja la velocidad.", "Probar cable y puerto."},
		},
	},
	"mtu_problem": {
		cause:  "La MTU del camino es baja y hay fragmentación con pérdida.",
		impact: "Conexiones TLS o VPN que cuelgan y transferencias lentas.",
		steps: []string{
			"Anotar la red y la MTU observada en la interfaz local.",
			"Comprobar túneles y clamping de TCP MSS.",
			"Ejecutar un sondeo DF desde un equipo de prueba.",
			"Ajustar MTU o MSS tras la verificación.",
		},
		limits: "No hay sondeo de fragmentación activo: se correlacionan MTU local y pérdida observada.",
		hypos: []hypothesisTemplate{
			{"Túnel con MTU baja", 80, "La interfaz local anuncia una MTU reducida.", "Revisar VPN, VXLAN o PPPoE."},
			{"Pérdida por congestión", 60, "La pérdida puede ser independiente de la MTU.", "Correlacionar con latencia y descartes."},
		},
	},
	"vlan_leak": {
		cause:  "Las tablas L2 muestran equipos de VLAN distintas en el mismo dominio.",
		impact: "Exposición de tráfico entre segmentos que deberían estar aislados.",
		steps: []string{
			"Listar los puertos y VLAN implicados.",
			"Comprobar etiquetado 802.1Q y modo trunk/access.",
			"Verificar la pertenencia a VLAN de cada puerto.",
			"Corregir la configuración tras validación.",
		},
		limits: "Sin contadores broadcast ni captura no se confirma tráfico entre VLAN.",
		hypos: []hypothesisTemplate{
			{"Etiquetado incorrecto", 80, "Los puertos comparten dominio L2 pese a VLAN distintas.", "Revisar trunk y VLAN nativa."},
			{"Datos FDB desactualizados", 60, "Una foto antigua puede mezclar estados.", "Repetir el sondeo SNMP."},
		},
	},
	"sla_breach": {
		cause:  "La disponibilidad, latencia o caídas del dispositivo superan el umbral configurado.",
		impact: "Servicios dependientes pueden degradarse y afectar a usuarios o producción.",
		steps: []string{
			"Revisar las muestras que superaron el umbral.",
			"Comprobar si hay mantenimiento planificado.",
			"Correlacionar con otros hallazgos del mismo periodo.",
			"Investigar la causa antes de relajar el umbral.",
		},
		limits: "El SLA se mide con muestras activas; no incluye experiencia de usuario final.",
		hypos: []hypothesisTemplate{
			{"Fallo real del dispositivo", 85, "Las muestras muestran caídas o latencia sostenida.", "Correlacionar con alertas y cambios."},
			{"Umbral mal calibrado", 60, "Una ventana corta puede magnificar un pico puntual.", "Revisar la ventana y el número de muestras."},
		},
	},
	"rogue_ap": {
		cause:  "Un BSSID no autorizado emite un SSID parecido al corporativo.",
		impact: "Acceso no controlado a la red interna y riesgo de phishing de credenciales.",
		steps: []string{
			"Anotar BSSID, SSID, canal y MAC.",
			"Localizar físicamente el dispositivo con un escáner WLAN.",
			"Comparar con la lista de puntos de acceso autorizados.",
			"Retirarlo o añadirlo a la lista autorizada con justificación.",
		},
		limits: "La detección usa inventario y telemetría; no incluye geolocalización ni captura inalámbrica.",
		hypos: []hypothesisTemplate{
			{"Punto de acceso no autorizado", 85, "El BSSID no figura en la lista de autorizados.", "Localizarlo físicamente y retirarlo."},
			{"SSID vecino legítimo", 60, "Una oficina cercana puede usar el mismo nombre.", "Contrastar con el inventario inalámbrico."},
		},
	},
	"passive_gratuitous_arp": {
		cause:  "Un equipo envió tres o más ARP gratuitos en la ventana de captura.",
		impact: "Puede ser un failover legítimo o un intento de anticipar una suplantación ARP.",
		steps: []string{
			"Anotar IP, MAC e interfaz del origen.",
			"Comprobar si hay un cluster o failover en esa IP.",
			"Correlacionar con el histórico ARP del inventario.",
			"Validar con captura L2 antes de actuar.",
		},
		limits: "Solo se guardan metadatos; no hay payload ni confirmación de protocolo.",
		hypos: []hypothesisTemplate{
			{"Failover de alta disponibilidad", 75, "Los ARP gratuitos son habituales en VRRP o HSRP.", "Verificar si la IP pertenece a un cluster."},
			{"ARP spoofing", 65, "Los anuncios repetidos pueden preceder a una suplantación.", "Correlacionar con cambios de MAC."},
		},
	},
	"passive_icmp_redirect": {
		cause:  "Se observó un ICMP redirect hacia una ruta distinta de la esperada.",
		impact: "El tráfico de los clientes puede desviarse por un equipo no autorizado.",
		steps: []string{
			"Anotar origen, destino y nueva puerta anunciada.",
			"Comparar con la tabla de rutas del gateway legítimo.",
			"Comprobar si el emisor es un router autorizado.",
			"Desactivar redirects en el gateway si no son necesarios.",
		},
		limits: "La captura es de metadatos y no prueba que el redirect fuera aceptado.",
		hypos: []hypothesisTemplate{
			{"Redirect malicioso", 85, "La ruta anunciada no coincide con el gateway observado.", "Localizar al emisor y bloquearlo."},
			{"Router legítimo optimizando rutas", 60, "Algunos routers emiten redirects por diseño.", "Revisar su configuración."},
		},
	},
	"passive_dhcp_server": {
		cause:  "Se capturaron ofertas DHCP de un servidor no autorizado.",
		impact: "Los clientes pueden recibir parámetros de red manipulados.",
		steps: []string{
			"Anotar la MAC del servidor capturado.",
			"Comparar con la lista de servidores autorizados.",
			"Localizar el equipo en el switch.",
			"Retirarlo y repetir la captura.",
		},
		limits: "Sin payload: los metadatos no muestran las opciones ofrecidas.",
		hypos: []hypothesisTemplate{
			{"Servidor DHCP rogue", 90, "Un servidor no autorizado emitió OFFER o ACK.", "Localizar y desconectar el equipo."},
			{"Dispositivo con DHCP por defecto", 65, "Un router doméstico puede estar repartiendo direcciones.", "Revisar el inventario de la zona."},
		},
	},
	"passive_inbound_scan": {
		cause:  "Un origen realizó diez o más SYN a puertos distintos de un destino.",
		impact: "Posible reconocimiento previo a un acceso no autorizado.",
		steps: []string{
			"Anotar IP de origen, destino y puertos sondeados.",
			"Comprobar si el origen es un escáner de seguridad autorizado.",
			"Correlacionar con otras observaciones del mismo origen.",
			"Escalar si no hay mantenimiento documentado.",
		},
		limits: "Un SYN sweep no demuestra explotación ni acceso conseguido.",
		hypos: []hypothesisTemplate{
			{"Reconocimiento activo", 80, "Patrón de diez o más puertos desde un mismo origen.", "Correlacionar con alertas posteriores."},
			{"Herramienta de inventario autorizada", 65, "Los escáneres internos generan el mismo patrón.", "Contrastar con el registro de escaneos."},
		},
	},
	"passive_dns_tunnel": {
		cause:  "Volumen o patrón de consultas DNS incompatible con el uso normal.",
		impact: "Posible canal encubierto de salida de datos.",
		steps: []string{
			"Anotar origen, dominios y ventana temporal.",
			"Comprobar si el equipo usa DNS-over-HTTPS o software de sync.",
			"Correlacionar con tráfico saliente permitido.",
			"Validar con una persona antes de bloquear el dominio.",
		},
		limits: "Solo se guardan metadatos de consulta; no hay respuestas ni payload.",
		hypos: []hypothesisTemplate{
			{"Canal encubierto DNS", 80, "El volumen o la entropía supera el patrón habitual.", "Validar con el propietario del equipo."},
			{"Aplicación legítima ruidosa", 65, "Algunos clientes DNS generan muchas consultas.", "Identificar el proceso en el host."},
		},
	},
	"tls_expiring": {
		cause:  "El certificado presentado por el servicio caduca en la ventana de aviso.",
		impact: "Los clientes pueden perder confianza en el servicio y provocar incidencias de conexión.",
		steps: []string{
			"Anotar host, puerto, sujeto y fecha de caducidad.",
			"Comprobar quién es el responsable del certificado.",
			"Renovarlo e instalarlo antes de la fecha límite.",
			"Repetir el escaneo y confirmar la nueva fecha.",
		},
		limits: "Se lee el certificado ofrecido hoy; no se consulta autoridad de certificación ni fecha de renovación.",
		hypos: []hypothesisTemplate{
			{"Renovación pendiente", 85, "El certificado está próximo a caducar.", "Coordinar la renovación con el responsable."},
			{"Certificado temporal o autofirmado", 60, "Los certificados temporales suelen tener vigencia corta.", "Verificar si el servicio es de prueba."},
		},
	},
}

var genericExplanation = explanationTemplate{
	cause:  "Los datos observados encajan con un cambio o una anomalía en el segmento.",
	impact: "Puede afectar a la continuidad del servicio o a la confianza en el inventario.",
	steps: []string{
		"Revisar la evidencia y las fechas del hallazgo.",
		"Correlacionarlo con otros hallazgos del mismo periodo.",
		"Validar la lectura en el equipo de red.",
		"Repetir el escaneo tras la verificación.",
	},
	limits: "La evidencia disponible no permite confirmar una causa concreta.",
	hypos: []hypothesisTemplate{
		{"Cambio legítimo no documentado", 60, "El patrón puede corresponder a mantenimiento.", "Contrastar con el registro de cambios."},
		{"Anomalía real", 55, "La señal no se explica con el inventario actual.", "Validar en el equipo de red."},
	},
}

// Explain returns a copy enriched with deterministic cause, impact, steps,
// limitations and baseline hypotheses. It never mutates the input.
func Explain(a model.DiagnosticAnomaly) model.DiagnosticAnomaly {
	enrich(&a)
	return a
}

// enrich fills only empty fields so a detector can still provide its own text.
func enrich(a *model.DiagnosticAnomaly) {
	t, ok := explanations[a.Type]
	if !ok {
		t = genericExplanation
	}
	if a.Cause == "" {
		a.Cause = hedge(a, t.cause)
	}
	if a.Impact == "" {
		a.Impact = hedge(a, t.impact)
	}
	if len(a.NextSteps) == 0 {
		a.NextSteps = append([]string(nil), t.steps...)
	}
	if a.Limitations == "" {
		a.Limitations = t.limits
	}
	if a.Recommendation == "" && len(a.NextSteps) > 0 {
		a.Recommendation = a.NextSteps[0]
	}
	if len(a.Hypotheses) == 0 {
		a.Hypotheses = make([]model.Hypothesis, 0, len(t.hypos))
		for _, h := range t.hypos {
			a.Hypotheses = append(a.Hypotheses, model.Hypothesis{
				Label:       h.label,
				Confidence:  h.confidence,
				Why:         h.why,
				Check:       h.check,
				Limitations: t.limits,
			})
		}
	}
	// Keep every hypothesis at or below the evidence confidence. This prevents
	// a fixed template from sounding more certain than the observation.
	for i := range a.Hypotheses {
		if a.Hypotheses[i].Confidence > a.Confidence && a.Confidence > 0 {
			a.Hypotheses[i].Confidence = a.Confidence
		}
		if len(a.Hypotheses[i].Evidence) == 0 {
			a.Hypotheses[i].Evidence = firstEvidence(a, 2)
		}
	}
	sortHypotheses(a.Hypotheses)
}

// hedge softens template language when the detector confidence is low.
func hedge(a *model.DiagnosticAnomaly, text string) string {
	if a.Confidence >= 90 || text == "" {
		return text
	}
	lower := strings.ToLower(text)
	if strings.HasPrefix(lower, "posible") || strings.HasPrefix(lower, "compatible") ||
		strings.HasPrefix(lower, "puede") || strings.HasPrefix(lower, "el tráfico puede") {
		return text
	}
	return "Posible " + strings.ToLower(text[:1]) + text[1:]
}

func firstEvidence(a *model.DiagnosticAnomaly, n int) []string {
	if len(a.Evidence) == 0 {
		return nil
	}
	if n > len(a.Evidence) {
		n = len(a.Evidence)
	}
	return append([]string(nil), a.Evidence[:n]...)
}

func sortHypotheses(h []model.Hypothesis) {
	sort.SliceStable(h, func(i, j int) bool { return h[i].Confidence > h[j].Confidence })
	for i := range h {
		h[i].Rank = i + 1
	}
}

type correlation struct {
	types      []string
	label      string
	confidence int
	why        string
	check      string
}

// correlations encodes F56: three fixed cross-finding rules. It adds a ranked
// hypothesis to every involved finding, then re-sorts the list.
var correlations = []correlation{
	{
		types:      []string{"arp_gateway_mac_changed", "dhcp_unauthorized_server"},
		label:      "Suplantación de gateway",
		confidence: 95,
		why:        "Cambió la MAC del gateway y apareció un servidor DHCP no autorizado en la misma ventana.",
		check:      "Localizar la MAC y el servidor DHCP, y confirmar ambos en el equipo legítimo.",
	},
	{
		types:      []string{"arp_gateway_mac_changed", "dhcp_gateway_spoof"},
		label:      "Suplantación de gateway",
		confidence: 95,
		why:        "La MAC del gateway cambió y una oferta DHCP anuncia otro gateway.",
		check:      "Comparar MAC y gateway ofrecido con el router autorizado.",
	},
	{
		types:      []string{"stp_loop", "broadcast_storm"},
		label:      "Lazo L2 activo",
		confidence: 90,
		why:        "El posible lazo coincide con un pico de broadcast en la misma ventana.",
		check:      "Capturar en la VLAN y revisar STP en los switches implicados.",
	},
	{
		types:      []string{"snmp_fdb_mac_moved", "arp_ip_mac_changed"},
		label:      "Movimiento físico o suplantación",
		confidence: 80,
		why:        "La MAC se movió de puerto y cambió la asociación IP→MAC.",
		check:      "Localizar físicamente la MAC y contrastar con el inventario.",
	},
}

// rankCorrelations applies the fixed cross-finding rules to a finished report.
func rankCorrelations(report *model.DiagnosticsReport) {
	present := map[string][]int{}
	for i := range report.Anomalies {
		present[report.Anomalies[i].Type] = append(present[report.Anomalies[i].Type], i)
	}
	for _, c := range correlations {
		all := true
		for _, t := range c.types {
			if len(present[t]) == 0 {
				all = false
				break
			}
		}
		if !all {
			continue
		}
		seen := map[int]bool{}
		for _, t := range c.types {
			for _, idx := range present[t] {
				if seen[idx] {
					continue
				}
				seen[idx] = true
				ev := []string{}
				for _, other := range c.types {
					if len(present[other]) == 0 {
						continue
					}
					ev = append(ev, fmt.Sprintf("%s: %s", other, anomalyRef(report.Anomalies[present[other][0]])))
				}
				report.Anomalies[idx].Hypotheses = append(report.Anomalies[idx].Hypotheses, model.Hypothesis{
					Label:       c.label,
					Confidence:  c.confidence,
					Why:         c.why,
					Evidence:    ev,
					Check:       c.check,
					Limitations: report.Anomalies[idx].Limitations,
				})
				sortHypotheses(report.Anomalies[idx].Hypotheses)
			}
		}
	}
}

// anomalyRef keeps correlation evidence short and deterministic.
func anomalyRef(a model.DiagnosticAnomaly) string {
	if a.Title != "" {
		return a.Title
	}
	return a.ID
}
