package discover

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/kr3s4l4/netdiag/internal/model"
	"github.com/kr3s4l4/netdiag/internal/netutil"
)

const (
	dhcpServerPort = 67
	dhcpClientPort = 68
	dhcpMagic      = 0x63825363
)

// DHCPProbeResult is the outcome of one short active probe. Available means
// the UDP client socket could be bound; Error explains why no replies were
// collected and is safe to show as a diagnostic limitation.
type DHCPProbeResult struct {
	Observations []model.DHCPObservation
	Available    bool
	Error        string
}

type dhcpEndpoint struct {
	IP  net.IP
	MAC net.HardwareAddr
}

// ProbeDHCP sends one DHCPINFORM and one DHCPDISCOVER on the local segment and
// reads replies for a short window. It never opens a raw socket, never changes
// the interface mode and degrades to an empty result when UDP/68 is unavailable.
func ProbeDHCP(ctx context.Context, iface, gateway string, timeout time.Duration) DHCPProbeResult {
	res := DHCPProbeResult{Observations: []model.DHCPObservation{}}
	if err := ctx.Err(); err != nil {
		res.Error = "escaneo cancelado antes del sondeo DHCP"
		return res
	}
	if timeout <= 0 {
		timeout = 700 * time.Millisecond
	}
	if timeout > 5*time.Second {
		timeout = 5 * time.Second
	}
	endpoint, err := localDHCPEndpoint(iface)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	conn, err := listenDHCP(ctx, endpoint.IP)
	if err != nil {
		res.Error = "no se pudo enlazar UDP/" + fmt.Sprint(dhcpClientPort) + ": " + err.Error()
		return res
	}
	res.Available = true
	defer conn.Close()

	deadline := time.Now().Add(timeout)
	if ctxDeadline, ok := ctx.Deadline(); ok && ctxDeadline.Before(deadline) {
		deadline = ctxDeadline
	}
	_ = conn.SetReadDeadline(deadline)
	stop := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = conn.Close()
		case <-stop:
		}
	}()
	defer close(stop)

	informXID, discoverXID := newDHCPXID(), newDHCPXID()
	xids := map[uint32]string{informXID: "inform", discoverXID: "discover"}
	inform := buildDHCPRequest(8, informXID, endpoint, false)    // DHCPINFORM
	discover := buildDHCPRequest(1, discoverXID, endpoint, true) // DHCPDISCOVER

	targets := []*net.UDPAddr{{IP: net.IPv4(255, 255, 255, 255), Port: dhcpServerPort}}
	if gw := net.ParseIP(strings.TrimSpace(gateway)).To4(); gw != nil && netutil.IsPrivate(gw) {
		targets = append([]*net.UDPAddr{{IP: gw, Port: dhcpServerPort}}, targets...)
	}
	sendErr := ""
	for _, target := range targets {
		if ctx.Err() != nil {
			break
		}
		if _, err := conn.WriteToUDP(inform, target); err != nil && sendErr == "" {
			sendErr = "no se pudo enviar DHCPINFORM: " + err.Error()
		}
	}
	if ctx.Err() == nil {
		if _, err := conn.WriteToUDP(discover, targets[len(targets)-1]); err != nil && sendErr == "" {
			sendErr = "no se pudo enviar DHCPDISCOVER: " + err.Error()
		}
	}

	arp := netutil.ARPTable()
	seen := map[string]bool{}
	buf := make([]byte, 2048)
	for ctx.Err() == nil {
		n, from, err := conn.ReadFromUDP(buf)
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				break
			}
			if ctx.Err() == nil && res.Error == "" {
				res.Error = "lectura DHCP: " + err.Error()
			}
			break
		}
		obs := parseDHCPReply(buf[:n], from.IP, arp, gateway, iface, xids, time.Now())
		if obs == nil {
			continue
		}
		key := obs.ServerID + "|" + obs.MessageType + "|" + obs.IP
		if seen[key] {
			continue
		}
		seen[key] = true
		res.Observations = append(res.Observations, *obs)
	}
	if len(res.Observations) == 0 && res.Error == "" {
		if sendErr != "" {
			res.Error = sendErr
		} else {
			res.Error = "sin respuesta DHCP en la ventana de sondeo"
		}
	}
	return res
}

// DHCPAvailable reports whether this process can bind the DHCP client port on
// a private interface. It is used by doctor and /system/info; it does not send
// any packet.
func DHCPAvailable() bool {
	endpoint, err := localDHCPEndpoint("")
	if err != nil {
		return false
	}
	conn, err := listenDHCP(context.Background(), endpoint.IP)
	if err != nil {
		return false
	}
	_ = conn.Close()
	return true
}

// localDHCPEndpoint picks the IPv4 address and MAC of the interface that owns
// the local segment. It reuses the same private-address rule as NudgeARP.
func localDHCPEndpoint(iface string) (dhcpEndpoint, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return dhcpEndpoint{}, fmt.Errorf("no se pudieron enumerar interfaces: %w", err)
	}
	for _, i := range ifaces {
		if iface != "" && i.Name != iface {
			continue
		}
		if i.Flags&net.FlagUp == 0 || i.Flags&net.FlagLoopback != 0 || len(i.HardwareAddr) != 6 {
			continue
		}
		addrs, _ := i.Addrs()
		for _, a := range addrs {
			ipn, ok := a.(*net.IPNet)
			if !ok || ipn.IP.To4() == nil || !netutil.IsPrivate(ipn.IP) {
				continue
			}
			return dhcpEndpoint{IP: ipn.IP.To4(), MAC: append(net.HardwareAddr(nil), i.HardwareAddr...)}, nil
		}
	}
	return dhcpEndpoint{}, fmt.Errorf("sin interfaz IPv4 privada para el sondeo DHCP (iface=%q)", iface)
}

func listenDHCP(ctx context.Context, ip net.IP) (*net.UDPConn, error) {
	addr := &net.UDPAddr{IP: ip, Port: dhcpClientPort}
	lc := net.ListenConfig{Control: func(network, address string, c syscall.RawConn) error {
		var sockErr error
		err := c.Control(func(fd uintptr) {
			sockErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_REUSEADDR, 1)
			if sockErr == nil {
				sockErr = syscall.SetsockoptInt(int(fd), syscall.SOL_SOCKET, syscall.SO_BROADCAST, 1)
			}
		})
		if err != nil {
			return err
		}
		return sockErr
	}}
	pc, err := lc.ListenPacket(ctx, "udp4", addr.String())
	if err != nil {
		return nil, err
	}
	conn, ok := pc.(*net.UDPConn)
	if !ok {
		_ = pc.Close()
		return nil, fmt.Errorf("socket UDP inesperado")
	}
	return conn, nil
}

func newDHCPXID() uint32 {
	var b [4]byte
	if _, err := rand.Read(b[:]); err == nil {
		return binary.BigEndian.Uint32(b[:])
	}
	return uint32(time.Now().UnixNano())
}

// buildDHCPRequest writes a BOOTP request with the DHCP magic cookie and the
// small option set needed for INFORM/DISCOVER. It is kept separate from the
// socket so packet construction can be tested offline.
func buildDHCPRequest(msgType byte, xid uint32, ep dhcpEndpoint, broadcast bool) []byte {
	pkt := make([]byte, 240)
	pkt[0] = 1 // BOOTREQUEST
	pkt[1] = 1 // Ethernet
	pkt[2] = 6
	binary.BigEndian.PutUint32(pkt[4:8], xid)
	if broadcast {
		binary.BigEndian.PutUint16(pkt[10:12], 0x8000)
	}
	if msgType == 8 && ep.IP != nil { // DHCPINFORM carries the client address
		copy(pkt[12:16], ep.IP.To4())
	}
	copy(pkt[28:34], ep.MAC)
	binary.BigEndian.PutUint32(pkt[236:240], dhcpMagic)

	opts := []byte{53, 1, msgType, 61, 7, 1}
	opts = append(opts, ep.MAC...)
	if host := dhcpHostname(); host != "" {
		opts = append(opts, 12, byte(len(host)))
		opts = append(opts, []byte(host)...)
	}
	opts = append(opts, 55, 3, 1, 3, 6) // subnet, router, DNS
	opts = append(opts, 57, 2, 0x05, 0xdc)
	opts = append(opts, 255)
	return append(pkt, opts...)
}

func dhcpHostname() string {
	host := strings.TrimSpace(os.Getenv("NETDIAG_DHCP_HOSTNAME"))
	if host == "" {
		host, _ = os.Hostname()
	}
	host = strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-' || r == '.' {
			return r
		}
		return -1
	}, host)
	if len(host) > 63 {
		host = host[:63]
	}
	return host
}

// parseDHCPReply decodes a BOOTREPLY and enriches it with the kernel ARP table.
// It is intentionally tolerant: partial options still produce useful evidence.
func parseDHCPReply(packet []byte, from net.IP, arp map[string]string,
	gateway, iface string, xids map[uint32]string, at time.Time) *model.DHCPObservation {
	from = from.To4()
	if from == nil || len(packet) < 240 || packet[0] != 2 || binary.BigEndian.Uint32(packet[236:240]) != dhcpMagic {
		return nil
	}
	msgType := dhcpOption(packet, 53)
	if len(msgType) != 1 {
		return nil
	}
	kind := ""
	switch msgType[0] {
	case 2:
		kind = "offer"
	case 5:
		kind = "ack"
	case 6:
		kind = "nak"
	default:
		return nil
	}
	xid := binary.BigEndian.Uint32(packet[4:8])
	if xids[xid] == "inform" && kind == "ack" {
		kind = "inform_ack"
	}

	serverID := from.String()
	if id := dhcpOption(packet, 54); len(id) == 4 && net.IP(id).To4() != nil {
		serverID = net.IP(id).To4().String()
	}
	mac := strings.ToLower(arp[from.String()])
	if mac == "" && serverID != from.String() {
		mac = strings.ToLower(arp[serverID])
	}
	if mac == "" {
		// A reply from the gateway may arrive before ARP has been refreshed.
		mac = strings.ToLower(arp[gateway])
	}
	obs := &model.DHCPObservation{
		IP:          from.String(),
		ServerID:    serverID,
		MAC:         mac,
		MessageType: kind,
		Gateway:     gateway,
		Interface:   iface,
		Routers:     dhcpOptionIPs(packet, 3),
		DNS:         dhcpOptionIPs(packet, 6),
		ObservedAt:  at,
	}
	if subnet := dhcpOption(packet, 1); len(subnet) == 4 {
		obs.Subnet = net.IP(subnet).To4().String()
	}
	if lease := dhcpOption(packet, 51); len(lease) == 4 {
		obs.LeaseSeconds = int64(binary.BigEndian.Uint32(lease))
	}
	return obs
}

func dhcpOption(packet []byte, code byte) []byte {
	if len(packet) < 240 {
		return nil
	}
	for i := 240; i < len(packet); {
		switch packet[i] {
		case 255:
			return nil
		case 0:
			i++
			continue
		}
		if i+1 >= len(packet) {
			return nil
		}
		c, length := packet[i], int(packet[i+1])
		i += 2
		if i+length > len(packet) {
			return nil
		}
		if c == code {
			return packet[i : i+length]
		}
		i += length
	}
	return nil
}

func dhcpOptionIPs(packet []byte, code byte) []string {
	data := dhcpOption(packet, code)
	out := []string{}
	for len(data) >= 4 {
		if ip := net.IP(data[:4]).To4(); ip != nil && !ip.IsUnspecified() {
			out = append(out, ip.String())
		}
		data = data[4:]
	}
	return out
}
