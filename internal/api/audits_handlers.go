package api

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/kr3s4l4/netdiag/internal/audits"
	"github.com/kr3s4l4/netdiag/internal/model"
)

type auditSnapshotRequest struct {
	SiteID int64  `json:"site_id,omitempty"`
	Name   string `json:"name,omitempty"`
	Notes  string `json:"notes,omitempty"`
}

// handleCreateAuditSnapshot saves an explicit, redacted baseline (F22).
func (s *Server) handleCreateAuditSnapshot(w http.ResponseWriter, r *http.Request) {
	if !s.canWrite(w, r) {
		return
	}
	var req auditSnapshotRequest
	if r.Body != nil && r.ContentLength != 0 {
		if err := decode(r, &req); err != nil {
			s.fail(w, r, http.StatusBadRequest, "cuerpo inválido: "+err.Error())
			return
		}
	}
	if req.SiteID == 0 {
		req.SiteID = queryInt64(r, "site_id")
	}
	if req.SiteID == 0 {
		req.SiteID = 1
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Notes = strings.TrimSpace(req.Notes)
	if len(req.Name) > 120 || len(req.Notes) > 1000 {
		s.fail(w, r, http.StatusBadRequest, "nombre máximo 120 caracteres y notas máximo 1000")
		return
	}
	data, err := audits.Capture(s.st, s.cfg, req.SiteID)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "no se pudo capturar el snapshot: "+err.Error())
		return
	}
	snap := &model.AuditSnapshot{SiteID: req.SiteID, Name: req.Name, Notes: req.Notes, TS: time.Now(), Data: data}
	if err := s.st.CreateAuditSnapshot(snap); err != nil {
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	snap.Name = auditDisplayName(snap.Name, snap.TS)
	// The stored payload is available on demand; keep the mutation response small.
	snap.Data = nil
	s.audit(r, "audits.snapshot.create", fmt.Sprintf("#%d %s", snap.ID, snap.Name), map[string]any{
		"site_id": snap.SiteID, "raw_size": snap.RawSize, "size": snap.Size,
	})
	writeJSON(w, http.StatusCreated, snap)
}

func (s *Server) handleListAuditSnapshots(w http.ResponseWriter, r *http.Request) {
	items, err := s.st.ListAuditSnapshots(queryInt64(r, "site_id"), queryInt(r, "limit", 50))
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

func (s *Server) handleGetAuditSnapshot(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	// The full payload can be large; list/detail requests stay lightweight and
	// callers must opt in with ?data=1 to retrieve it.
	withData := strings.ToLower(queryString(r, "data")) == "1"
	snap, err := s.st.GetAuditSnapshot(id, withData)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	if snap == nil {
		s.fail(w, r, http.StatusNotFound, "snapshot no encontrado")
		return
	}
	writeJSON(w, http.StatusOK, snap)
}

func (s *Server) handleDeleteAuditSnapshot(w http.ResponseWriter, r *http.Request) {
	if !s.canWrite(w, r) {
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	snap, err := s.st.GetAuditSnapshot(id, false)
	if err != nil || snap == nil {
		s.fail(w, r, http.StatusNotFound, "snapshot no encontrado")
		return
	}
	if err := s.st.DeleteAuditSnapshot(id); err != nil {
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	s.audit(r, "audits.snapshot.delete", fmt.Sprintf("#%d %s", id, snap.Name), nil)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleDiffAuditSnapshots(w http.ResponseWriter, r *http.Request) {
	fromID, err := pathInt64(r, "id")
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	toID, err := strconv.ParseInt(strings.TrimSpace(r.PathValue("id2")), 10, 64)
	if err != nil || toID <= 0 {
		s.fail(w, r, http.StatusBadRequest, "id2 inválido")
		return
	}
	from, err := s.st.GetAuditSnapshot(fromID, true)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	to, err := s.st.GetAuditSnapshot(toID, true)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	if from == nil || to == nil {
		s.fail(w, r, http.StatusNotFound, "uno de los snapshots no existe")
		return
	}
	report := audits.Diff(from, to)
	s.audit(r, "audits.snapshot.diff", fmt.Sprintf("#%d → #%d", fromID, toID), map[string]any{
		"changes": len(report.Changes),
	})
	writeJSON(w, http.StatusOK, report)
}

// handleExportAuditSnapshot serves a portable JSON or gzip baseline.
func (s *Server) handleExportAuditSnapshot(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	snap, err := s.st.GetAuditSnapshot(id, true)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	if snap == nil || snap.Data == nil {
		s.fail(w, r, http.StatusNotFound, "snapshot no encontrado")
		return
	}
	raw, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	base := fmt.Sprintf("netdiag-audit-%d-%s", snap.ID, snap.TS.UTC().Format("20060102-150405"))
	format := strings.ToLower(queryString(r, "format"))
	if format == "gzip" {
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		if _, err := zw.Write(raw); err != nil {
			s.fail(w, r, http.StatusInternalServerError, err.Error())
			return
		}
		if err := zw.Close(); err != nil {
			s.fail(w, r, http.StatusInternalServerError, err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/gzip")
		w.Header().Set("Content-Disposition", "attachment; filename="+filepath.Base(base)+".json.gz")
		w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
		_, _ = w.Write(buf.Bytes())
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", "attachment; filename="+filepath.Base(base)+".json")
	w.Header().Set("Content-Length", strconv.Itoa(len(raw)))
	_, _ = w.Write(raw)
}

func auditDisplayName(name string, ts time.Time) string {
	if strings.TrimSpace(name) != "" {
		return name
	}
	return "Auditoría " + ts.Local().Format("2006-01-02 15:04")
}
