package diagnostics

import (
	"fmt"

	"github.com/kr3s4l4/netdiag/internal/model"
)

type hostStats struct {
	Samples      int
	Up           int
	AvgLoss      float64
	AvgRTT       float64
	Availability float64
	LastTS       string
}

// statsByHost aggregates the raw samples in the window per host.
func statsByHost(in Input) map[int64]hostStats {
	out := map[int64]hostStats{}
	type acc struct {
		samples, up, rtt, loss, count int
		rttSum, lossSum               float64
	}
	accs := map[int64]*acc{}
	for _, m := range in.Metrics {
		if m.HostID == 0 {
			continue
		}
		a := accs[m.HostID]
		if a == nil {
			a = &acc{}
			accs[m.HostID] = a
		}
		a.samples++
		if m.Up {
			a.up++
			if m.RTTMS > 0 {
				a.rttSum += m.RTTMS
				a.rtt++
			}
		}
		a.lossSum += m.Loss
		a.count++
	}
	for id, a := range accs {
		s := hostStats{Samples: a.samples, Up: a.up}
		if a.count > 0 {
			s.AvgLoss = round1(a.lossSum / float64(a.count))
		}
		if a.rtt > 0 {
			s.AvgRTT = round1(a.rttSum / float64(a.rtt))
		}
		if a.samples > 0 {
			s.Availability = round1(float64(a.up) / float64(a.samples) * 100)
		}
		out[id] = s
	}
	return out
}

// detectSLABreaches evaluates per-host availability and latency thresholds.
// Unlike the other detectors it runs even when the enterprise mode is off.
func detectSLABreaches(in Input, report *model.DiagnosticsReport) []model.DiagnosticAnomaly {
	minAvail := in.Config.MinAvailability
	if minAvail <= 0 {
		minAvail = 99
	}
	maxRTT := in.Config.MaxRTTMS
	if maxRTT <= 0 {
		maxRTT = 200
	}
	stats := statsByHost(in)
	var out []model.DiagnosticAnomaly
	for _, h := range in.Hosts {
		s := stats[h.ID]
		if s.Samples < 3 {
			continue
		}
		breaches := []string{}
		if s.Availability < minAvail {
			breaches = append(breaches, fmt.Sprintf("disponibilidad %.1f%% (mínimo %.1f%%)", s.Availability, minAvail))
		}
		if s.AvgRTT > maxRTT {
			breaches = append(breaches, fmt.Sprintf("latencia media %.1f ms (máximo %.1f ms)", s.AvgRTT, maxRTT))
		}
		if len(breaches) == 0 {
			continue
		}
		severity := model.SeverityWarning
		if s.Availability < minAvail-5 || s.AvgRTT > maxRTT*1.5 {
			severity = model.SeverityCritical
		}
		anomaly := model.DiagnosticAnomaly{
			ID: anomalyID("sla_breach", h.ID), Type: "sla_breach", Category: "sla",
			Severity: severity, Title: "Incumplimiento de SLA",
			Detail:  hostLabel(&h) + ": " + joinStrings(breaches, "; "),
			HostIDs: []int64{h.ID}, HostIPs: []string{h.IP},
			Confidence: 95, Evidence: []string{
				fmt.Sprintf("%d muestras en la ventana", s.Samples),
				fmt.Sprintf("Pérdida media: %.1f%%", s.AvgLoss),
			},
			Recommendation: "Investiga la causa antes de relajar el umbral: pérdida, latencia o caídas repetidas.",
			DetectedAt:     report.GeneratedAt,
		}
		add(report, anomaly)
		out = append(out, anomaly)
	}
	return out
}

func joinStrings(values []string, sep string) string {
	out := ""
	for i, v := range values {
		if i > 0 {
			out += sep
		}
		out += v
	}
	return out
}
