package api

import (
	"crypto/subtle"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

// handleMetrics exposes the Prometheus text exposition format. When a token is
// configured, scrapers must send it as a Bearer token.
func (s *Server) handleMetrics(w http.ResponseWriter, r *http.Request) {
	if s.cfg != nil && s.cfg.Security.MetricsToken != "" {
		auth := r.Header.Get("Authorization")
		got := strings.TrimPrefix(auth, "Bearer ")
		if !strings.HasPrefix(auth, "Bearer ") ||
			subtle.ConstantTimeCompare([]byte(got), []byte(s.cfg.Security.MetricsToken)) != 1 {
			w.Header().Set("WWW-Authenticate", `Bearer realm="netdiag metrics"`)
			s.failCode(w, r, http.StatusUnauthorized, "unauthorized", "token de métricas inválido")
			return
		}
	}
	stats, err := s.st.Dashboard(queryInt64(r, "site_id"))
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "no se pudieron calcular las métricas")
		return
	}
	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	var b strings.Builder
	help := func(name, kind, text string) {
		fmt.Fprintf(&b, "# HELP %s %s\n# TYPE %s %s\n", name, text, name, kind)
	}
	gauge := func(name string, value float64) {
		fmt.Fprintf(&b, "%s %s\n", name, formatFloat(value))
	}
	label := func(name string, labels map[string]string, value float64) {
		keys := make([]string, 0, len(labels))
		for k := range labels {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var parts []string
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf(`%s="%s"`, k, escapeLabel(labels[k])))
		}
		fmt.Fprintf(&b, "%s{%s} %s\n", name, strings.Join(parts, ","), formatFloat(value))
	}

	help("netdiag_info", "gauge", "Build information for this NetDiag instance.")
	label("netdiag_info", map[string]string{"version": s.version, "commit": s.commit}, 1)
	help("netdiag_up", "gauge", "Whether the NetDiag API is serving requests.")
	gauge("netdiag_up", 1)
	help("netdiag_uptime_seconds", "gauge", "Seconds since the process started.")
	gauge("netdiag_uptime_seconds", time.Since(s.startedAt).Seconds())
	help("netdiag_db_size_bytes", "gauge", "SQLite database size in bytes.")
	gauge("netdiag_db_size_bytes", float64(s.st.Size()))
	help("netdiag_hosts", "gauge", "Discovered hosts by status.")
	for status, n := range stats.ByStatus {
		label("netdiag_hosts", map[string]string{"status": status}, float64(n))
	}
	help("netdiag_hosts_by_type", "gauge", "Discovered hosts by device type.")
	for typ, n := range stats.ByType {
		label("netdiag_hosts_by_type", map[string]string{"type": typ}, float64(n))
	}
	help("netdiag_hosts_total", "gauge", "Total discovered hosts.")
	gauge("netdiag_hosts_total", float64(stats.HostsTotal))
	help("netdiag_hosts_up", "gauge", "Hosts currently online.")
	gauge("netdiag_hosts_up", float64(stats.HostsUp))
	help("netdiag_hosts_down", "gauge", "Hosts currently offline.")
	gauge("netdiag_hosts_down", float64(stats.HostsDown))
	help("netdiag_hosts_new_24h", "gauge", "Hosts discovered in the last 24 hours.")
	gauge("netdiag_hosts_new_24h", float64(stats.HostsNew24h))
	help("netdiag_networks", "gauge", "Configured monitored networks.")
	gauge("netdiag_networks", float64(stats.Networks))
	help("netdiag_scans_24h", "gauge", "Scans started in the last 24 hours.")
	gauge("netdiag_scans_24h", float64(stats.Scans24h))
	help("netdiag_alerts_active", "gauge", "Alerts that are active or acknowledged.")
	gauge("netdiag_alerts_active", float64(stats.AlertsActive))
	help("netdiag_alerts_critical", "gauge", "Critical alerts currently open.")
	gauge("netdiag_alerts_critical", float64(stats.AlertsCritical))
	help("netdiag_avg_rtt_ms", "gauge", "Average RTT across the fleet in milliseconds (24 h).")
	gauge("netdiag_avg_rtt_ms", stats.AvgRTT)
	help("netdiag_availability_ratio", "gauge", "Fleet availability ratio between 0 and 1 (24 h).")
	gauge("netdiag_availability_ratio", stats.Availability/100)
	_, _ = w.Write([]byte(b.String()))
}

func formatFloat(v float64) string {
	if v == float64(int64(v)) {
		return strconv.FormatInt(int64(v), 10)
	}
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func escapeLabel(v string) string {
	v = strings.ReplaceAll(v, `\`, `\\`)
	v = strings.ReplaceAll(v, "\n", `\n`)
	return strings.ReplaceAll(v, `"`, `\"`)
}
