package fingerprint

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kr3s4l4/netdiag/internal/model"
)

// Input is every observation available for one host.
type Input struct {
	IP        string
	MAC       string
	Hostname  string
	Vendor    string
	Ports     []model.Port
	SNMP      *model.SNMPSummary
	IsGateway bool
	TTL       int
}

// Result is the inferred identity.
type Result struct {
	Vendor     string
	DeviceType model.DeviceType
	OS         string
	Model      string
	Hostname   string
	Confidence int
	Evidence   []string
}

type scoreBoard struct {
	scores   map[model.DeviceType]int
	evidence []string
	seen     map[string]bool
}

func newScoreBoard() *scoreBoard {
	return &scoreBoard{scores: map[model.DeviceType]int{}, seen: map[string]bool{}}
}

func (b *scoreBoard) add(t model.DeviceType, weight int, evidence string) {
	if evidence != "" && !b.seen[evidence] {
		b.seen[evidence] = true
		b.evidence = append(b.evidence, evidence)
	}
	if t == "" || t == model.DevUnknown {
		return
	}
	if weight > b.scores[t] {
		b.scores[t] = weight
	} else if weight > 0 {
		b.scores[t] += weight / 4
	}
}

func (b *scoreBoard) top() (model.DeviceType, int, int) {
	type kv struct {
		t model.DeviceType
		v int
	}
	var list []kv
	for t, v := range b.scores {
		list = append(list, kv{t, v})
	}
	sort.Slice(list, func(i, j int) bool {
		if list[i].v != list[j].v {
			return list[i].v > list[j].v
		}
		return list[i].t < list[j].t
	})
	if len(list) == 0 {
		return model.DevUnknown, 0, 0
	}
	second := 0
	if len(list) > 1 {
		second = list[1].v
	}
	return list[0].t, list[0].v, second
}

// Classify combines vendor, hostname, port and SNMP evidence. It deliberately
// takes the maximum evidence weight instead of a sum so that a single weak
// signal cannot overrule a strong one. Coherent signals increase confidence.
func Classify(in Input) Result {
	res := Result{Vendor: in.Vendor, DeviceType: model.DevUnknown, Hostname: in.Hostname}
	board := newScoreBoard()

	if res.Vendor == "" && in.MAC != "" {
		if v, ok := Vendor(in.MAC); ok {
			res.Vendor = v
			board.add(model.DevUnknown, 0, "OUI: "+v)
		} else if IsRandomMAC(in.MAC) {
			board.add(model.DevUnknown, 0, "MAC aleatoria (privacidad)")
		}
	} else if res.Vendor != "" {
		board.add(model.DevUnknown, 0, "Fabricante: "+res.Vendor)
	}
	if in.MAC != "" && IsVirtualPlatform(in.MAC) {
		board.add(model.DevVirtual, 85, "MAC de plataforma virtual ("+mapVirtual(in.MAC)+")")
	}
	if in.SNMP != nil {
		if in.SNMP.SysName != "" {
			res.Hostname = in.SNMP.SysName
		}
		board.add(model.DevUnknown, 0, "SNMP: "+firstLine(in.SNMP.SysDescr, 80))
	}

	// Vendor hints.
	for _, h := range vendorHints(res.Vendor) {
		board.add(h.typ, h.weight, "Fabricante sugiere "+typeLabel(h.typ))
		if h.os != "" && res.OS == "" {
			res.OS = h.os
		}
	}

	// Hostname hints.
	for _, h := range hostnameHints(res.Hostname) {
		board.add(h.typ, h.weight, "Hostname \""+res.Hostname+"\" sugiere "+typeLabel(h.typ))
	}

	// Port hints.
	open := map[int]model.Port{}
	for _, p := range in.Ports {
		if p.State == "open" || p.State == "" {
			open[p.Number] = p
		}
	}
	has := func(ports ...int) bool {
		for _, p := range ports {
			if _, ok := open[p]; ok {
				return true
			}
		}
		return false
	}
	if has(9100, 631, 515) {
		board.add(model.DevPrinter, 85, "Puertos de impresión (9100/631/515)")
	}
	if has(554, 8554, 37777, 34567) {
		board.add(model.DevCamera, 85, "Puertos de cámara IP (RTSP/Dahua)")
	}
	if has(62078) {
		board.add(model.DevPhone, 80, "Puerto 62078 (sincronización iPhone)")
	}
	if has(32400) {
		board.add(model.DevMedia, 80, "Puerto 32400 (Plex)")
	}
	if has(1883, 8883) {
		board.add(model.DevIoT, 65, "Broker MQTT (1883/8883)")
	}
	if has(5060, 5061) {
		board.add(model.DevPhone, 60, "Señalización SIP (teléfono VoIP)")
	}
	if has(3389) {
		board.add(model.DevWorkstation, 55, "Escritorio remoto Windows (3389)")
	}
	if has(445, 139) && !has(80, 443) {
		board.add(model.DevWorkstation, 40, "Compartición SMB")
	}
	if has(8008, 8009, 8060, 3689) {
		board.add(model.DevMedia, 55, "Servicios de reproducción multimedia")
	}
	if has(2375, 2376) {
		board.add(model.DevServer, 50, "Docker API expuesta")
	}
	if has(6443) {
		board.add(model.DevServer, 50, "API de Kubernetes")
	}
	if has(8006) {
		board.add(model.DevServer, 50, "Interfaz Proxmox (8006)")
	}
	if has(3306, 5432, 27017, 6379, 9200) {
		board.add(model.DevServer, 60, "Servicios de base de datos")
	}
	if has(548) {
		board.add(model.DevNAS, 35, "Apple Filing Protocol (548)")
	}
	if has(22) && has(80, 443) {
		board.add(model.DevServer, 45, "SSH + web (patrón de servidor)")
	}
	if has(80, 443) {
		board.add(model.DevServer, 15, "Servicios web")
	}
	if has(53) {
		board.add(model.DevRouter, 25, "Servidor DNS en el dispositivo")
	}
	if len(open) >= 6 {
		board.add(model.DevServer, 20, fmt.Sprintf("%d puertos abiertos", len(open)))
	}

	// Service/banner hints.
	for _, p := range open {
		low := strings.ToLower(p.Banner + " " + p.Product + " " + p.Service)
		switch {
		case strings.Contains(low, "printer"), strings.Contains(low, "jetdirect"), strings.Contains(low, "cups"):
			board.add(model.DevPrinter, 75, "Banner de impresora")
		case strings.Contains(low, "camera"), strings.Contains(low, "hikvision"), strings.Contains(low, "dahua"), strings.Contains(low, "onvif"):
			board.add(model.DevCamera, 75, "Banner de cámara")
		case strings.Contains(low, "synology"), strings.Contains(low, "qnap"), strings.Contains(low, "truenas"):
			board.add(model.DevNAS, 75, "Banner de NAS")
		case strings.Contains(low, "plex"), strings.Contains(low, "sonos"), strings.Contains(low, "roku"):
			board.add(model.DevMedia, 65, "Banner multimedia")
		case strings.Contains(low, "mikrotik"), strings.Contains(low, "routeros"), strings.Contains(low, "openwrt"), strings.Contains(low, "pfsense"):
			board.add(model.DevRouter, 70, "Sistema de router detectado")
		case strings.Contains(low, "openssh"):
			board.add(model.DevServer, 25, "OpenSSH")
		case strings.Contains(low, "nginx"), strings.Contains(low, "apache"), strings.Contains(low, "lighttpd"), strings.Contains(low, "caddy"):
			board.add(model.DevServer, 25, "Servidor web")
		}
	}

	if in.SNMP != nil {
		desc := strings.ToLower(in.SNMP.SysDescr + " " + in.SNMP.SysObjectID)
		switch {
		case strings.Contains(desc, "switch"):
			board.add(model.DevSwitch, 80, "sysDescr de switch")
		case strings.Contains(desc, "router"), strings.Contains(desc, "ios xe"), strings.Contains(desc, "ios software"), strings.Contains(desc, "junos"), strings.Contains(desc, "routeros"):
			board.add(model.DevRouter, 80, "sysDescr de router")
		case strings.Contains(desc, "access point"), strings.Contains(desc, "wireless"), strings.Contains(desc, "unifi"):
			board.add(model.DevAccessPoint, 75, "sysDescr de punto de acceso")
		case strings.Contains(desc, "printer"), strings.Contains(desc, "jetdirect"):
			board.add(model.DevPrinter, 80, "sysDescr de impresora")
		case strings.Contains(desc, "camera"):
			board.add(model.DevCamera, 80, "sysDescr de cámara")
		case strings.Contains(desc, "synology"), strings.Contains(desc, "qnap"), strings.Contains(desc, "network attached storage"):
			board.add(model.DevNAS, 80, "sysDescr de NAS")
		case strings.Contains(desc, "linux"):
			board.add(model.DevServer, 25, "sysDescr Linux")
		case strings.Contains(desc, "windows"):
			board.add(model.DevWorkstation, 25, "sysDescr Windows")
		}
	}

	if in.IsGateway {
		board.add(model.DevRouter, 92, "Puerta de enlace por defecto")
	}

	top, weight, second := board.top()
	res.DeviceType = top
	res.Confidence = weight
	if weight > 0 && second > 0 && weight-second < 15 {
		res.Confidence -= 10
	}
	if res.Confidence > 99 {
		res.Confidence = 99
	}
	if res.Confidence < 15 && top != model.DevUnknown && top != "" {
		res.Confidence = 15
	}
	if top == model.DevUnknown || top == "" {
		res.DeviceType = model.DevUnknown
		res.Confidence = 0
	}
	res.Evidence = board.evidence

	// OS inference from SNMP, banners, TTL and ports.
	res.OS = inferOS(in, open, res.OS)
	if res.Model == "" && in.SNMP != nil {
		res.Model = snmpModel(in.SNMP.SysDescr)
	}
	return res
}

func mapVirtual(mac string) string {
	n := NormalizeMAC(mac)
	if v, ok := virtualOUIs[n[:8]]; ok {
		return v
	}
	return "virtual"
}

func firstLine(s string, max int) string {
	s = strings.TrimSpace(strings.Split(strings.ReplaceAll(s, "\r", ""), "\n")[0])
	if len(s) > max {
		return s[:max] + "…"
	}
	return s
}

func snmpModel(descr string) string {
	d := strings.TrimSpace(descr)
	if d == "" || len(d) > 120 {
		return ""
	}
	return d
}

type hint struct {
	typ    model.DeviceType
	weight int
	os     string
}

func vendorHints(vendor string) []hint {
	v := strings.ToLower(vendor)
	if v == "" {
		return nil
	}
	contains := func(words ...string) bool {
		for _, w := range words {
			if strings.Contains(v, w) {
				return true
			}
		}
		return false
	}
	var out []hint
	add := func(t model.DeviceType, w int) { out = append(out, hint{typ: t, weight: w}) }

	switch {
	case contains("vmware", "virtualbox", "qemu", "xen", "parallels", "hyper-v", "docker", "kubernetes", "proxmox"):
		add(model.DevVirtual, 88)
	case contains("synology", "qnap", "terramaster", "asustor", "drobo", "netapp", "ixsystems", "western digital"):
		add(model.DevNAS, 85)
	case contains("hikvision", "dahua", "axis communications", "reolink", "amcrest", "foscam", "vivotek", "hanwha", "uniview"):
		add(model.DevCamera, 82)
	case contains("canon", "epson", "brother", "xerox", "kyocera", "ricoh", "lexmark", "zebra technologies", "dymo"):
		add(model.DevPrinter, 78)
	case contains("espressif", "tuya", "shelly", "allterco", "signify", "philips lighting", "sonoff", "itead", "broadlink", "aqara", "lumi"):
		add(model.DevIoT, 72)
	case contains("ubiquiti", "aruba", "ruckus", "meraki", "extreme networks", "allied telesis"):
		add(model.DevAccessPoint, 60)
		add(model.DevSwitch, 45)
	case contains("cisco", "juniper", "mikrotik", "fortinet", "palo alto", "sophos", "zyxel", "draytek", "netgear", "tp-link", "d-link", "huawei", "zte", "h3c", "ruijie", "avaya", "brodcom", "brocade", "edgecore"):
		add(model.DevRouter, 65)
		add(model.DevSwitch, 50)
	case contains("nintendo"):
		add(model.DevGameConsole, 80)
	case contains("sonos", "roku", "vizio", "nvidia", "sony", "samsung", "lg electronics", "google", "amazon technologies", "apple"):
		add(model.DevMedia, 35)
		add(model.DevPhone, 20)
	case contains("raspberry pi"):
		add(model.DevServer, 55)
	case contains("dell", "supermicro", "lenovo", "inspur", "hpe", "hewlett packard enterprise"):
		add(model.DevServer, 45)
	case contains("hewlett packard", "hp inc"):
		add(model.DevPrinter, 35)
	}
	return out
}

func hostnameHints(host string) []hint {
	h := strings.ToLower(strings.TrimSpace(host))
	if h == "" {
		return nil
	}
	has := func(words ...string) bool {
		for _, w := range words {
			if strings.Contains(h, w) {
				return true
			}
		}
		return false
	}
	var out []hint
	add := func(t model.DeviceType, w int) { out = append(out, hint{typ: t, weight: w}) }
	switch {
	case has("iphone", "ipad", "android", "pixel", "galaxy", "oneplus", "smartphone"):
		add(model.DevPhone, 72)
	case has("macbook", "imac", "laptop", "notebook", "thinkpad", "portatil"):
		add(model.DevLaptop, 70)
	case has("printer", "print", "mfp", "scanner"):
		add(model.DevPrinter, 72)
	case has("ipcam", "camera", "cam-", "-cam", "nvr", "dvr"):
		add(model.DevCamera, 72)
	case has("router", "gateway", "gw-", "firewall", "-fw", "openwrt", "routeros", "pfsense", "opnsense"):
		add(model.DevRouter, 70)
	case has("switch", "sw-", "-sw"):
		add(model.DevSwitch, 68)
	case has("accesspoint", "access-point", "wifi", "wlan", "ap-", "-ap"):
		add(model.DevAccessPoint, 65)
	case has("synology", "qnap", "nas-", "-nas", "truenas"):
		add(model.DevNAS, 72)
	case has("xbox", "playstation", "ps4", "ps5", "nintendo"):
		add(model.DevGameConsole, 70)
	case has("roku", "chromecast", "appletv", "firetv", "shield", "sonos", "tv-", "-tv"):
		add(model.DevMedia, 65)
	case has("esp-", "esp32", "shelly", "tuya", "sonoff", "iot", "sensor"):
		add(model.DevIoT, 65)
	case has("server", "srv", "proxmox", "esxi", "hyperv", "docker", "k8s"):
		add(model.DevServer, 60)
	case has("desktop", "workstation", "pc-", "-pc"):
		add(model.DevWorkstation, 55)
	case has("raspberry", "rpi", "pi-"):
		add(model.DevServer, 50)
	}
	return out
}

func inferOS(in Input, open map[int]model.Port, current string) string {
	if current != "" {
		return current
	}
	text := strings.ToLower(in.Hostname)
	if in.SNMP != nil {
		text += " " + strings.ToLower(in.SNMP.SysDescr)
	}
	var products []string
	for _, p := range open {
		products = append(products, p.Product, p.Version, p.Banner)
	}
	text += " " + strings.ToLower(strings.Join(products, " "))

	switch {
	case strings.Contains(text, "openwrt"):
		return "OpenWrt"
	case strings.Contains(text, "routeros"), strings.Contains(text, "mikrotik"):
		return "RouterOS"
	case strings.Contains(text, "pfsense"):
		return "pfSense"
	case strings.Contains(text, "opnsense"):
		return "OPNsense"
	case strings.Contains(text, "junos"):
		return "Junos"
	case strings.Contains(text, "cisco ios"):
		return "Cisco IOS"
	case strings.Contains(text, "raspbian"), strings.Contains(text, "raspberry pi os"):
		return "Raspberry Pi OS"
	case strings.Contains(text, "ubuntu"):
		return "Ubuntu"
	case strings.Contains(text, "debian"):
		return "Debian"
	case strings.Contains(text, "centos"):
		return "CentOS"
	case strings.Contains(text, "red hat"), strings.Contains(text, "rhel"):
		return "Red Hat"
	case strings.Contains(text, "fedora"):
		return "Fedora"
	case strings.Contains(text, "alpine"):
		return "Alpine Linux"
	case strings.Contains(text, "freebsd"):
		return "FreeBSD"
	case strings.Contains(text, "windows"):
		return "Windows"
	case strings.Contains(text, "android"):
		return "Android"
	case strings.Contains(text, "ios ") && strings.Contains(text, "cisco"):
		return "Cisco IOS"
	case strings.Contains(text, "macos"), strings.Contains(text, "darwin"):
		return "macOS"
	case strings.Contains(text, "linux"):
		return "Linux"
	}
	// Weak heuristics from reachable services.
	if _, ok := open[3389]; ok {
		return "Windows"
	}
	if _, ok := open[62078]; ok {
		return "iOS"
	}
	if _, ok := open[445]; ok {
		return "Windows (probable)"
	}
	if _, ok := open[22]; ok {
		return "Linux/Unix (probable)"
	}
	switch {
	case in.TTL >= 1 && in.TTL <= 64:
		return "Linux/Unix (probable)"
	case in.TTL > 64 && in.TTL <= 128:
		return "Windows (probable)"
	case in.TTL > 128:
		return "Sistema de red (probable)"
	}
	return ""
}

// TypeLabel returns the Spanish label for a device type.
func TypeLabel(t model.DeviceType) string {
	switch t {
	case model.DevRouter:
		return "Router"
	case model.DevSwitch:
		return "Switch"
	case model.DevAccessPoint:
		return "Punto de acceso"
	case model.DevFirewall:
		return "Firewall"
	case model.DevServer:
		return "Servidor"
	case model.DevWorkstation:
		return "Estación de trabajo"
	case model.DevLaptop:
		return "Portátil"
	case model.DevPhone:
		return "Teléfono"
	case model.DevTablet:
		return "Tablet"
	case model.DevPrinter:
		return "Impresora"
	case model.DevCamera:
		return "Cámara IP"
	case model.DevIoT:
		return "IoT"
	case model.DevNAS:
		return "NAS"
	case model.DevVirtual:
		return "Máquina virtual"
	case model.DevMedia:
		return "Multimedia"
	case model.DevGameConsole:
		return "Consola"
	default:
		return "Desconocido"
	}
}

func typeLabel(t model.DeviceType) string { return strings.ToLower(TypeLabel(t)) }
