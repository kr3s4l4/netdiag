package api

import (
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/kr3s4l4/netdiag/internal/model"
	"github.com/kr3s4l4/netdiag/internal/store"
)

type findingReviewRequest struct {
	SiteID      int64  `json:"site_id"`
	HostID      int64  `json:"host_id"`
	FindingType string `json:"finding_type"`
	HostKey     string `json:"host_key"`
	Status      string `json:"status"`
	Note        string `json:"note"`
}

// findingHostKey returns the stable key used to pair a review with a finding.
// MAC wins over IP because inventory identity survives address changes.
func (s *Server) findingHostKey(siteID int64, a model.DiagnosticAnomaly) string {
	if len(a.HostIDs) > 0 && a.HostIDs[0] > 0 {
		if host, err := s.st.GetHost(a.HostIDs[0]); err == nil {
			if mac := strings.ToLower(strings.TrimSpace(host.MAC)); mac != "" {
				if _, err := net.ParseMAC(mac); err == nil {
					return "mac:" + mac
				}
			}
			if ip := strings.TrimSpace(host.IP); ip != "" {
				return "ip:" + strings.ToLower(ip)
			}
		}
	}
	if len(a.HostIPs) > 0 {
		if ip := strings.TrimSpace(a.HostIPs[0]); ip != "" {
			return "ip:" + strings.ToLower(ip)
		}
	}
	return "global"
}

// attachFindingReviews adds the local triage state to computed findings.
// Reviews are never persisted with the finding itself and never close alerts.
func (s *Server) attachFindingReviews(siteID int64, items []model.DiagnosticAnomaly) []model.DiagnosticAnomaly {
	reviews, err := s.st.ListFindingReviews(siteID, "", 500)
	if err != nil {
		return items
	}
	byKey := make(map[string]model.FindingReview, len(reviews))
	for _, r := range reviews {
		byKey[r.FindingType+"\x00"+r.HostKey] = r
	}
	for i := range items {
		key := s.findingHostKey(siteID, items[i])
		items[i].HostKey = key
		if r, ok := byKey[items[i].Type+"\x00"+key]; ok {
			items[i].Review = &r
		}
	}
	return items
}

func (s *Server) handleListFindingReviews(w http.ResponseWriter, r *http.Request) {
	items, err := s.st.ListFindingReviews(queryInt64(r, "site_id"), queryString(r, "status"), queryInt(r, "limit", 200))
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "no se pudieron leer las revisiones")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

func (s *Server) handleUpsertFindingReview(w http.ResponseWriter, r *http.Request) {
	if !s.canWrite(w, r) {
		return
	}
	var req findingReviewRequest
	if err := decode(r, &req); err != nil {
		s.fail(w, r, http.StatusBadRequest, "cuerpo inválido")
		return
	}
	if req.SiteID == 0 {
		req.SiteID = queryInt64(r, "site_id")
	}
	req.FindingType = strings.TrimSpace(req.FindingType)
	if req.FindingType == "" || len(req.FindingType) > 120 {
		s.fail(w, r, http.StatusBadRequest, "finding_type obligatorio (máx. 120 caracteres)")
		return
	}
	if !store.ValidFindingReviewStatuses[strings.ToLower(strings.TrimSpace(req.Status))] {
		s.fail(w, r, http.StatusBadRequest, "status debe ser pending|confirmed|false_positive|resolved")
		return
	}
	if len(req.Note) > 500 {
		s.fail(w, r, http.StatusBadRequest, "note supera 500 caracteres")
		return
	}
	hostKey := strings.TrimSpace(req.HostKey)
	if hostKey == "" && req.HostID > 0 {
		if host, err := s.st.GetHost(req.HostID); err == nil {
			if mac := strings.ToLower(strings.TrimSpace(host.MAC)); mac != "" {
				if _, err := net.ParseMAC(mac); err == nil {
					hostKey = "mac:" + mac
				}
			}
			if hostKey == "" && strings.TrimSpace(host.IP) != "" {
				hostKey = "ip:" + strings.ToLower(strings.TrimSpace(host.IP))
			}
		}
	}
	if hostKey == "" {
		hostKey = "global"
	}
	review := &model.FindingReview{
		SiteID:      req.SiteID,
		FindingType: req.FindingType,
		HostKey:     hostKey,
		Status:      req.Status,
		Note:        req.Note,
		Reviewer:    userFrom(r).Username,
		ExpiresAt:   time.Now().UTC().Add(30 * 24 * time.Hour),
	}
	if err := s.st.UpsertFindingReview(review); err != nil {
		s.fail(w, r, http.StatusInternalServerError, "no se pudo guardar la revisión")
		return
	}
	s.audit(r, "finding_review.upsert", reviewTarget(review.FindingType, review.HostKey), map[string]any{
		"status": review.Status, "expires_at": review.ExpiresAt,
	})
	writeJSON(w, http.StatusOK, review)
}

func (s *Server) handleDeleteFindingReview(w http.ResponseWriter, r *http.Request) {
	if !s.canWrite(w, r) {
		return
	}
	req := findingReviewRequest{
		SiteID:      queryInt64(r, "site_id"),
		FindingType: queryString(r, "finding_type"),
		HostKey:     queryString(r, "host_key"),
	}
	if r.Body != nil && r.ContentLength != 0 {
		var body findingReviewRequest
		if err := decode(r, &body); err != nil {
			s.fail(w, r, http.StatusBadRequest, "cuerpo inválido")
			return
		}
		if body.FindingType != "" {
			req.FindingType = body.FindingType
		}
		if body.HostKey != "" {
			req.HostKey = body.HostKey
		}
		if body.SiteID > 0 {
			req.SiteID = body.SiteID
		}
	}
	req.FindingType = strings.TrimSpace(req.FindingType)
	req.HostKey = store.NormalizeFindingKey(req.HostKey)
	if req.FindingType == "" || req.HostKey == "" {
		s.fail(w, r, http.StatusBadRequest, "finding_type y host_key son obligatorios")
		return
	}
	if err := s.st.DeleteFindingReview(req.SiteID, req.FindingType, req.HostKey); err != nil {
		s.fail(w, r, http.StatusInternalServerError, "no se pudo borrar la revisión")
		return
	}
	s.audit(r, "finding_review.deleted", reviewTarget(req.FindingType, req.HostKey), nil)
	w.WriteHeader(http.StatusNoContent)
}

func reviewTarget(findingType, hostKey string) string {
	return strings.TrimSpace(findingType) + "/" + store.NormalizeFindingKey(hostKey)
}
