package integrations

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"syscall"
	"time"
)

// sendSyslog emits an RFC 5424 message over UDP or TCP. Destination addresses
// are inspected with the same SSRF guard used for HTTP deliveries.
func (m *Manager) sendSyslog(ctx context.Context, cfg map[string]any, ev Event) error {
	host := cfgString(cfg, "host")
	if host == "" {
		host = "127.0.0.1"
	}
	port := cfgInt(cfg, "port", 514)
	network := strings.ToLower(cfgString(cfg, "protocol"))
	if network != "tcp" {
		network = "udp"
	}
	addr := net.JoinHostPort(host, fmt.Sprint(port))
	timeout := m.timeout()
	allowPrivate := m.settings().AllowPrivateTargets
	dialer := &net.Dialer{
		Timeout: timeout,
		Control: func(netw, address string, _ syscall.RawConn) error {
			h, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip := net.ParseIP(h)
			if ip == nil {
				return fmt.Errorf("destino syslog inválido: %s", h)
			}
			if ip.Equal(net.ParseIP("169.254.169.254")) || ip.Equal(net.ParseIP("fd00:ec2::254")) {
				return errors.New("destino de metadatos bloqueado")
			}
			if !allowPrivate && (ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() || ip.IsMulticast()) {
				return fmt.Errorf("destino syslog privado bloqueado: %s (activa integrations.allow_private_targets)", ip)
			}
			return nil
		},
	}
	conn, err := dialer.DialContext(ctx, network, addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))

	doc, err := eventJSON(ev)
	if err != nil {
		return err
	}
	appName := cfgString(cfg, "app_name")
	if appName == "" {
		appName = "netdiag"
	}
	// RFC5424: <PRI>VERSION TIMESTAMP HOSTNAME APP-NAME PROCID MSGID STRUCTURED-DATA MSG
	severity := 6 // informational
	switch ev.Severity {
	case "critical":
		severity = 2
	case "warning":
		severity = 4
	}
	pri := 16*5 + severity // local0 facility
	ts := ev.Timestamp.UTC().Format(time.RFC3339)
	hostname := cfgString(cfg, "hostname")
	if hostname == "" {
		hostname = "netdiag"
	}
	msg := fmt.Sprintf("<%d>1 %s %s %s - - [netdiag site_id=\"%d\" kind=\"%s\"] %s %s\n",
		pri, ts, hostname, appName, ev.SiteID, ev.Kind, ev.Title, string(doc))
	if _, err := conn.Write([]byte(msg)); err != nil {
		return err
	}
	return nil
}
