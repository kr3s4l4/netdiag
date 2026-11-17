package fingerprint

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"html"
	"io"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/kr3s4l4/netdiag/internal/model"
)

var titleRe = regexp.MustCompile(`(?is)<title[^>]*>(.*?)</title>`)

// TLS-first ports, HTTP-first ports and line-banner ports.
var tlsPorts = map[int]bool{443: true, 465: true, 636: true, 993: true, 995: true, 5061: true,
	5986: true, 8443: true, 8883: true, 9443: true, 2376: true, 5001: true, 6443: true, 8006: true, 32400: true}

var httpPorts = map[int]bool{80: true, 81: true, 3000: true, 5000: true, 5601: true, 7000: true, 8000: true,
	8008: true, 8080: true, 8081: true, 8086: true, 8090: true, 8123: true, 8200: true, 8500: true,
	8888: true, 9000: true, 9090: true, 9200: true, 10000: true, 15672: true, 32400: true}

var linePorts = map[int]bool{21: true, 22: true, 23: true, 25: true, 110: true, 143: true, 587: true,
	5900: true, 6379: true, 11211: true}

// Probe connects to an open port and enriches it with service, product,
// version, banner and TLS metadata. It never follows redirects.
func Probe(ctx context.Context, ip string, port int, timeout time.Duration) model.Port {
	p := model.Port{Number: port, Protocol: "tcp", State: "open", Service: ServiceName(port)}
	if timeout <= 0 {
		timeout = 900 * time.Millisecond
	}
	d := net.Dialer{Timeout: timeout, KeepAlive: -1}
	conn, err := d.DialContext(ctx, "tcp", net.JoinHostPort(ip, strconv.Itoa(port)))
	if err != nil {
		return p
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))

	if tlsPorts[port] {
		tconn := tls.Client(conn, &tls.Config{InsecureSkipVerify: true, ServerName: ip, MinVersion: tls.VersionTLS10})
		if err := tconn.HandshakeContext(ctx); err == nil {
			p.TLS = certInfo(tconn.ConnectionState())
			p.Service = "https"
			if httpPorts[port] || port == 443 || port == 8443 || port == 9443 || port == 5001 || port == 8006 {
				if status, server, title, _ := httpProbe(tconn, ip, timeout); status != "" {
					p.Product, p.Version = splitProduct(server)
					p.Banner = sanitizeBanner("HTTP " + status + " · " + server + " · " + title)
					return p
				}
			}
			if p.Banner == "" {
				cert := "TLS"
				if p.TLS != nil && p.TLS.Subject != "" {
					cert += " · CN=" + p.TLS.Subject
				}
				p.Banner = cert
			}
			return p
		}
		// Fall back to a plain read: some "TLS" ports speak another protocol.
	}

	switch {
	case httpPorts[port] || port == 80:
		if status, server, title, body := httpProbe(conn, ip, timeout); status != "" {
			p.Product, p.Version = splitProduct(server)
			p.Banner = sanitizeBanner("HTTP " + status + " · " + server + " · " + title)
			_ = body
			p.Service = "http"
			return p
		}
	case linePorts[port]:
		switch port {
		case 6379:
			_, _ = conn.Write([]byte("PING\r\n"))
		case 11211:
			_, _ = conn.Write([]byte("version\r\n"))
		}
		banner := readBanner(conn, 512, timeout)
		applyBanner(&p, banner)
		return p
	}

	banner := readBanner(conn, 1024, timeout)
	applyBanner(&p, banner)
	return p
}

func certInfo(state tls.ConnectionState) *model.TLSCert {
	if len(state.PeerCertificates) == 0 {
		return nil
	}
	c := state.PeerCertificates[0]
	out := &model.TLSCert{
		Subject:  c.Subject.CommonName,
		Issuer:   c.Issuer.CommonName,
		NotAfter: c.NotAfter,
		DNSNames: append([]string{}, c.DNSNames...),
	}
	out.SelfSigned = c.Subject.String() == c.Issuer.String() || verifySelfSigned(c)
	return out
}

func verifySelfSigned(c *x509.Certificate) bool {
	if len(c.Issuer.Organization) == 0 || len(c.Subject.Organization) == 0 {
		return false
	}
	return c.Issuer.String() == c.Subject.String()
}

func httpProbe(conn net.Conn, ip string, timeout time.Duration) (status, server, title string, body []byte) {
	_ = conn.SetDeadline(time.Now().Add(timeout))
	req := fmt.Sprintf("GET / HTTP/1.1\r\nHost: %s\r\nUser-Agent: NetDiag/1.0 (+network-inventory)\r\nAccept: */*\r\nConnection: close\r\n\r\n", ip)
	if _, err := conn.Write([]byte(req)); err != nil {
		return "", "", "", nil
	}
	br := bufio.NewReader(io.LimitReader(conn, 64*1024))
	reqObj := &http.Request{Method: http.MethodGet}
	resp, err := http.ReadResponse(br, reqObj)
	if err != nil {
		return "", "", "", nil
	}
	defer resp.Body.Close()
	body, _ = io.ReadAll(io.LimitReader(resp.Body, 32*1024))
	status = resp.Status
	if status == "" {
		status = strconv.Itoa(resp.StatusCode)
	}
	server = resp.Header.Get("Server")
	if m := titleRe.FindSubmatch(body); len(m) == 2 {
		title = strings.TrimSpace(html.UnescapeString(string(m[1])))
		if len([]rune(title)) > 80 {
			title = string([]rune(title)[:80]) + "…"
		}
	}
	return status, server, title, body
}

func readBanner(conn net.Conn, max int, timeout time.Duration) string {
	_ = conn.SetReadDeadline(time.Now().Add(timeout))
	buf := make([]byte, max)
	n, _ := conn.Read(buf)
	if n <= 0 {
		return ""
	}
	return string(buf[:n])
}

func applyBanner(p *model.Port, raw string) {
	raw = sanitizeBanner(raw)
	if raw == "" {
		return
	}
	p.Banner = raw
	low := strings.ToLower(raw)
	switch {
	case strings.HasPrefix(raw, "SSH-"):
		p.Service = "ssh"
		if parts := strings.SplitN(raw, "-", 3); len(parts) == 3 {
			body := parts[2]
			if i := strings.Index(body, "_"); i > 0 {
				p.Product = body[:i]
				rest := body[i+1:]
				if j := strings.IndexAny(rest, " \r\n"); j > 0 {
					rest = rest[:j]
				}
				p.Version = rest
				if strings.Contains(rest, "Ubuntu") || strings.Contains(rest, "ubuntu") {
					p.Product = p.Product + " (Ubuntu)"
				}
			} else {
				p.Product = body
			}
		}
	case strings.HasPrefix(raw, "220"):
		if strings.Contains(low, "smtp") {
			p.Service = "smtp"
		}
		if strings.Contains(low, "ftp") {
			p.Service = "ftp"
		}
		for _, prod := range []string{"postfix", "exim", "sendmail", "vsftpd", "proftpd", "pure-ftpd", "filezilla"} {
			if strings.Contains(low, prod) {
				p.Product = prod
			}
		}
	case strings.HasPrefix(raw, "+OK"):
		p.Service = "pop3"
		if strings.Contains(low, "dovecot") {
			p.Product = "Dovecot"
		}
	case strings.HasPrefix(raw, "* OK"):
		p.Service = "imap"
		if strings.Contains(low, "dovecot") {
			p.Product = "Dovecot"
		}
	case strings.HasPrefix(raw, "RFB "):
		p.Service = "vnc"
		p.Product = strings.TrimSpace(strings.TrimPrefix(raw, "RFB "))
	case strings.HasPrefix(raw, "+PONG"):
		p.Service = "redis"
		p.Product = "Redis"
	case strings.HasPrefix(raw, "HTTP/"):
		p.Service = "http"
		line := strings.SplitN(raw, "\r\n", 2)[0]
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			p.Banner = sanitizeBanner(line)
			_ = fields
		}
	}
}

func splitProduct(server string) (string, string) {
	server = strings.TrimSpace(server)
	if server == "" {
		return "", ""
	}
	first := server
	if i := strings.Index(server, " "); i > 0 {
		first = server[:i]
	}
	parts := strings.SplitN(first, "/", 2)
	if len(parts) == 2 {
		return strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
	}
	return first, ""
}

func sanitizeBanner(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	var b strings.Builder
	lastSpace := false
	for _, r := range s {
		switch {
		case r == '\n' || r == '\r' || r == '\t':
			if !lastSpace {
				b.WriteByte(' ')
				lastSpace = true
			}
		case r < 32 || r == 127:
			// drop control characters
		default:
			b.WriteRune(r)
			lastSpace = r == ' '
		}
		if b.Len() >= 240 {
			break
		}
	}
	return strings.TrimSpace(b.String())
}
