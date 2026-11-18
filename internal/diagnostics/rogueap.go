package diagnostics

import (
	"fmt"
	"strings"

	"github.com/kr3s4l4/netdiag/internal/model"
)

// detectRogueAPs compares discovered access points against the authorized MAC
// list. When no list is configured it flags unmanaged APs (low fingerprint
// confidence or no SNMP identity), which are the usual rogue candidates in a
// small network.
func detectRogueAPs(in Input, report *model.DiagnosticsReport) {
	if !in.Config.EnterpriseMode {
		return
	}
	authorized := map[string]bool{}
	for _, mac := range in.Config.AuthorizedAPs {
		authorized[strings.ToLower(strings.TrimSpace(mac))] = true
	}
	for _, h := range in.Hosts {
		if h.DeviceType != model.DevAccessPoint {
			continue
		}
		mac := strings.ToLower(strings.TrimSpace(h.MAC))
		if len(authorized) > 0 {
			if authorized[mac] {
				continue
			}
		} else if h.Confidence >= 90 && h.SNMP != nil && h.SNMP.SysDescr != "" {
			continue
		}
		evidence := []string{
			fmt.Sprintf("Clasificado como punto de acceso con confianza %d%%", h.Confidence),
		}
		if mac != "" {
			evidence = append(evidence, "BSSID/MAC: "+mac)
		}
		if h.SNMP != nil && h.SNMP.SysDescr != "" {
			evidence = append(evidence, "SNMP: "+h.SNMP.SysDescr)
		}
		if len(authorized) > 0 {
			evidence = append(evidence, "No figura en diagnostics.authorized_aps")
		} else {
			evidence = append(evidence, "Sin identidad SNMP suficiente para considerarlo gestionado")
		}
		confidence := 70
		if len(authorized) > 0 {
			confidence = 95
		}
		add(report, model.DiagnosticAnomaly{
			ID: anomalyID("rogue_ap", h.ID), Type: "rogue_ap", Category: "wireless",
			Severity: model.SeverityWarning, Title: "Punto de acceso no autorizado",
			Detail:  hostLabel(&h) + " no parece un AP gestionado por el inventario.",
			HostIDs: []int64{h.ID}, HostIPs: []string{h.IP},
			Confidence: confidence, Evidence: evidence,
			Recommendation: "Confirma el BSSID con el equipo de red y añádelo a diagnostics.authorized_aps o retíralo.",
			DetectedAt:     report.GeneratedAt,
		})
	}
}
