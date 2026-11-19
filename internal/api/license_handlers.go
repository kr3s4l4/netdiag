package api

import (
	"net/http"
	"time"

	"github.com/kr3s4l4/netdiag/internal/license"
)

// handleSystemLicense reports the local licence to any authenticated role.
// Missing or invalid licences are not errors: Community remains fully usable
// and the reason is exposed as a warning instead.
func (s *Server) handleSystemLicense(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.licenseStatus(time.Now()))
}

// licenseStatus is the safe representation shared by the API and exports.
func (s *Server) licenseStatus(now time.Time) license.Status {
	managed := 0
	sites := 0
	if s.st != nil {
		count, err := s.st.CountManagedHosts(now.Add(-30 * 24 * time.Hour))
		if err != nil {
			if s.log != nil {
				s.log.Warn("no se pudo contar los hosts gestionados", "error", err)
			}
		} else {
			managed = count
		}
		if list, err := s.st.ListSites(); err == nil {
			sites = len(list)
		}
	}
	if s.licenses == nil {
		return license.Status{
			Status:       "community",
			Edition:      "community",
			Capabilities: []string{},
			Limits:       license.Limits{RetentionDays: 7},
			Warnings:     []string{},
			ManagedHosts: managed,
			Sites:        sites,
		}
	}
	return s.licenses.Status(now, managed, sites)
}
