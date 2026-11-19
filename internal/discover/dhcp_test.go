package discover

import (
	"encoding/binary"
	"net"
	"testing"
	"time"
)

func TestBuildDHCPRequest(t *testing.T) {
	hw, _ := net.ParseMAC("a0:a4:c5:c9:b2:ba")
	ep := dhcpEndpoint{IP: net.ParseIP("192.168.0.105").To4(), MAC: hw}
	pkt := buildDHCPRequest(8, 0x11223344, ep, false)
	if pkt[0] != 1 || pkt[1] != 1 || pkt[2] != 6 {
		t.Fatalf("cabecera BOOTP incorrecta: %v", pkt[:4])
	}
	if binary.BigEndian.Uint32(pkt[236:240]) != dhcpMagic {
		t.Fatal(" falta la cookie DHCP")
	}
	if got := dhcpOption(pkt, 53); len(got) != 1 || got[0] != 8 {
		t.Fatalf("mensaje DHCP incorrecto: %v", got)
	}
	if got := net.IP(pkt[12:16]).To4(); got == nil || got.String() != "192.168.0.105" {
		t.Fatalf("DHCPINFORM debe llevar la IP del cliente: %v", got)
	}
}

func TestParseDHCPReply(t *testing.T) {
	client, _ := net.ParseMAC("a0:a4:c5:c9:b2:ba")
	ep := dhcpEndpoint{IP: net.ParseIP("192.168.0.105").To4(), MAC: client}
	xid := uint32(0x11223344)
	pkt := buildDHCPRequest(2, xid, ep, true) // DHCPOFFER
	pkt = pkt[:len(pkt)-1]                    // quitar END para añadir opciones de respuesta
	pkt[0] = 2                                // BOOTREPLY
	copy(pkt[28:34], client)
	pkt = append(pkt,
		54, 4, 192, 168, 0, 1,
		3, 4, 192, 168, 0, 1,
		1, 4, 255, 255, 255, 0,
		6, 8, 1, 1, 1, 1, 8, 8, 8, 8,
		51, 4, 0, 0, 14, 16,
		255)
	pkt[0] = 2
	pkt[1] = 1
	pkt[2] = 6

	arp := map[string]string{"192.168.0.1": "11:22:33:44:55:01"}
	xids := map[uint32]string{xid: "discover"}
	obs := parseDHCPReply(pkt, net.ParseIP("192.168.0.1").To4(), arp,
		"192.168.0.1", "wlan0", xids, time.Now())
	if obs == nil {
		t.Fatal("no se pudo decodificar la respuesta DHCP")
	}
	if obs.MessageType != "offer" || obs.ServerID != "192.168.0.1" || obs.MAC != "11:22:33:44:55:01" {
		t.Fatalf("respuesta mal interpretada: %+v", obs)
	}
	if obs.Subnet != "255.255.255.0" || obs.LeaseSeconds != 3600 {
		t.Fatalf("opciones mal interpretadas: %+v", obs)
	}
	if len(obs.Routers) != 1 || obs.Routers[0] != "192.168.0.1" || len(obs.DNS) != 2 {
		t.Fatalf("listas de IP mal interpretadas: %+v", obs)
	}
}

func TestParseDHCPReplyRejectsRequests(t *testing.T) {
	hw, _ := net.ParseMAC("a0:a4:c5:c9:b2:ba")
	ep := dhcpEndpoint{IP: net.ParseIP("192.168.0.105").To4(), MAC: hw}
	pkt := buildDHCPRequest(1, 1, ep, true)
	if obs := parseDHCPReply(pkt, net.ParseIP("192.168.0.1").To4(), nil, "", "", nil, time.Now()); obs != nil {
		t.Fatalf("una petición no debe aceptarse como respuesta: %+v", obs)
	}
}
