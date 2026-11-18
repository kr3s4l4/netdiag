package integrations

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// sendRedis appends the event to a Redis Stream using the minimal RESP
// protocol (AUTH + XADD). A bounded read deadline prevents hangs.
func (m *Manager) sendRedis(ctx context.Context, cfg map[string]any, ev Event) error {
	addr := cfgString(cfg, "addr")
	if addr == "" {
		addr = "127.0.0.1:6379"
	}
	if _, _, err := net.SplitHostPort(addr); err != nil {
		return fmt.Errorf("redis.addr inválido: %w", err)
	}
	timeout := m.timeout()
	allowPrivate := m.settings().AllowPrivateTargets
	dialer := &net.Dialer{
		Timeout: timeout,
		Control: func(network, address string, _ syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return err
			}
			ip := net.ParseIP(host)
			if ip == nil {
				return fmt.Errorf("destino redis inválido: %s", host)
			}
			if ip.Equal(net.ParseIP("169.254.169.254")) || ip.Equal(net.ParseIP("fd00:ec2::254")) {
				return errors.New("destino de metadatos bloqueado")
			}
			if !allowPrivate && (ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() || ip.IsMulticast()) {
				return fmt.Errorf("destino redis privado bloqueado: %s (activa integrations.allow_private_targets)", ip)
			}
			return nil
		},
	}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(timeout))
	r := bufio.NewReader(conn)

	if password := cfgString(cfg, "password"); password != "" {
		username := cfgString(cfg, "username")
		var auth []any
		if username != "" {
			auth = []any{"AUTH", username, password}
		} else {
			auth = []any{"AUTH", password}
		}
		if err := redisCommand(conn, r, auth...); err != nil {
			return fmt.Errorf("redis AUTH: %w", err)
		}
	}
	stream := cfgString(cfg, "stream")
	if stream == "" {
		stream = "netdiag:events"
	}
	doc, err := eventJSON(ev)
	if err != nil {
		return err
	}
	maxLen := cfgInt(cfg, "max_len", 10000)
	args := []any{"XADD", stream}
	if maxLen > 0 {
		args = append(args, "MAXLEN", "~", strconv.Itoa(maxLen))
	}
	args = append(args, "*",
		"kind", ev.Kind, "site_id", strconv.FormatInt(ev.SiteID, 10),
		"severity", ev.Severity, "title", ev.Title, "message", ev.Message,
		"event", string(doc))
	if err := redisCommand(conn, r, args...); err != nil {
		return fmt.Errorf("redis XADD: %w", err)
	}
	return nil
}

func redisCommand(conn net.Conn, r *bufio.Reader, args ...any) error {
	if _, err := fmt.Fprint(conn, "*"+strconv.Itoa(len(args))+"\r\n"); err != nil {
		return err
	}
	for _, arg := range args {
		s := fmt.Sprint(arg)
		if _, err := fmt.Fprintf(conn, "$%d\r\n%s\r\n", len(s), s); err != nil {
			return err
		}
	}
	prefix, err := r.ReadByte()
	if err != nil {
		return err
	}
	line, err := r.ReadString('\n')
	if err != nil {
		return err
	}
	if prefix == '-' {
		return errors.New(strings.TrimSpace(line))
	}
	return nil
}
