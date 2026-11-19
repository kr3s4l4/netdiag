package diagnostics

import (
	"fmt"
	"strings"
	"time"

	"github.com/kr3s4l4/netdiag/internal/config"
	"github.com/kr3s4l4/netdiag/internal/model"
)

// TLS notice thresholds, in days before not_after. A certificate is reported
// at 30, 14 and 1 days; the severity grows as the deadline approaches.
const (
	tlsNoticeDays   = 30
	tlsWarningDays  = 14
	tlsCriticalDays = 1
)

// DetectTLSExpiry reads only the not_after field already collected from the
// TLS handshake. It makes no external call: no CA lookup, no OCSP, no DNS and
// no attempt to check whether a renewal is in progress.
func DetectTLSExpiry(hosts []model.Host, now time.Time, cfg config.Diagnostics) []model.DiagnosticAnomaly {
	out := []model.DiagnosticAnomaly{}
	if now.IsZero() {
		now = time.Now()
	}
	minConf := cfg.MinConfidence
	if minConf < 1 {
		minConf = 60
	}
	const confidence = 95
	if confidence < minConf {
		return out
	}
	seen := map[string]bool{}
	for _, h := range hosts {
		for _, p := range h.Ports {
			if p.TLS == nil || p.TLS.NotAfter.IsZero() {
				continue
			}
			days := int(p.TLS.NotAfter.Sub(now).Hours() / 24)
			if days > tlsNoticeDays {
				continue
			}
			id := fmt.Sprintf("tls:expiring:%d:%d:%s", h.ID, p.Number, strings.ToLower(p.Protocol))
			if seen[id] {
				continue
			}
			seen[id] = true

			severity := model.SeverityWarning
			if days <= tlsCriticalDays {
				severity = model.SeverityCritical
			}
			detail := tlsDeadline(days, p.TLS.NotAfter)
			if p.TLS.Subject != "" {
				detail += " Sujeto: " + p.TLS.Subject + "."
			}
			evidence := []string{
				fmt.Sprintf("Puerto %d/%s", p.Number, p.Protocol),
				"not_after: " + p.TLS.NotAfter.UTC().Format("2006-01-02 15:04 UTC"),
			}
			if p.TLS.Issuer != "" {
				evidence = append(evidence, "Emisor: "+p.TLS.Issuer)
			}
			ips := []string{}
			ids := []int64{}
			if h.IP != "" {
				ips = append(ips, h.IP)
			}
			if h.ID > 0 {
				ids = append(ids, h.ID)
			}
			a := model.DiagnosticAnomaly{
				ID:             id,
				Type:           "tls_expiring",
				Category:       "tls",
				Severity:       severity,
				Title:          "Certificado por caducar",
				Detail:         detail,
				HostIDs:        ids,
				HostIPs:        ips,
				Confidence:     confidence,
				Evidence:       evidence,
				Recommendation: explanations["tls_expiring"].steps[0],
				DetectedAt:     now,
			}
			enrich(&a)
			out = append(out, a)
		}
	}
	return out
}

func tlsDeadline(days int, notAfter time.Time) string {
	date := notAfter.UTC().Format("2006-01-02")
	switch {
	case days < 0:
		return fmt.Sprintf("El certificado caducó hace %d días (%s).", -days, date)
	case days == 0:
		return "El certificado caduca hoy (" + date + ")."
	case days == 1:
		return "El certificado caduca en 1 día (" + date + ")."
	default:
		return fmt.Sprintf("El certificado caduca en %d días (%s).", days, date)
	}
}
