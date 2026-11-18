package api

import (
	"net/http"

	"github.com/kr3s4l4/netdiag/internal/dashboards"
	"github.com/kr3s4l4/netdiag/internal/model"
	"github.com/kr3s4l4/netdiag/internal/store"
)

type dashboardRequest struct {
	SiteID  int64          `json:"site_id,omitempty"`
	Name    string         `json:"name"`
	Widgets []model.Widget `json:"widgets,omitempty"`
}

// handleListDashboards lists the custom views of a site.
func (s *Server) handleListDashboards(w http.ResponseWriter, r *http.Request) {
	items, err := s.st.ListDashboards(queryInt64(r, "site_id"))
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "total": len(items)})
}

// handleDashboardCatalog exposes the allowed widget types and their sources.
func (s *Server) handleDashboardCatalog(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"items": dashboards.Catalog(), "max_widgets": 16})
}

// handleGetDashboard returns one view including its widgets.
func (s *Server) handleGetDashboard(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	d, err := s.st.GetDashboard(id)
	if err != nil {
		if isNotFound(err) {
			s.fail(w, r, http.StatusNotFound, "dashboard no encontrado")
			return
		}
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, d)
}

// handleCreateDashboard stores a new user-defined view.
func (s *Server) handleCreateDashboard(w http.ResponseWriter, r *http.Request) {
	if !s.canWrite(w, r) {
		return
	}
	var req dashboardRequest
	if err := decode(r, &req); err != nil {
		s.fail(w, r, http.StatusBadRequest, "cuerpo inválido: "+err.Error())
		return
	}
	d := &model.Dashboard{SiteID: req.SiteID, Name: req.Name, Widgets: req.Widgets}
	if err := dashboards.Normalize(d); err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	if !s.st.SiteExists(d.SiteID) {
		s.fail(w, r, http.StatusBadRequest, "sede no encontrada")
		return
	}
	if err := s.st.CreateDashboard(d); err != nil {
		s.fail(w, r, http.StatusConflict, err.Error())
		return
	}
	s.audit(r, "dashboard.created", d.Name, map[string]any{"id": d.ID, "site_id": d.SiteID})
	s.created(w, d)
}

// handleCreateExampleDashboard creates the "Mi red" view with four widgets.
func (s *Server) handleCreateExampleDashboard(w http.ResponseWriter, r *http.Request) {
	if !s.canWrite(w, r) {
		return
	}
	var req struct {
		SiteID int64 `json:"site_id,omitempty"`
	}
	_ = decode(r, &req)
	d := store.ExampleDashboard(req.SiteID)
	if err := dashboards.Normalize(d); err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	if !s.st.SiteExists(d.SiteID) {
		s.fail(w, r, http.StatusBadRequest, "sede no encontrada")
		return
	}
	if err := s.st.CreateDashboard(d); err != nil {
		s.fail(w, r, http.StatusConflict, err.Error())
		return
	}
	s.audit(r, "dashboard.created", d.Name, map[string]any{"id": d.ID, "example": true})
	s.created(w, d)
}

// handleUpdateDashboard replaces a view. Widget sources are revalidated.
func (s *Server) handleUpdateDashboard(w http.ResponseWriter, r *http.Request) {
	if !s.canWrite(w, r) {
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	current, err := s.st.GetDashboard(id)
	if err != nil {
		if isNotFound(err) {
			s.fail(w, r, http.StatusNotFound, "dashboard no encontrado")
			return
		}
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	var req dashboardRequest
	if err := decode(r, &req); err != nil {
		s.fail(w, r, http.StatusBadRequest, "cuerpo inválido: "+err.Error())
		return
	}
	current.Name = req.Name
	current.SiteID = req.SiteID
	current.Widgets = req.Widgets
	if err := dashboards.Normalize(current); err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	if !s.st.SiteExists(current.SiteID) {
		s.fail(w, r, http.StatusBadRequest, "sede no encontrada")
		return
	}
	if err := s.st.UpdateDashboard(current); err != nil {
		if isNotFound(err) {
			s.fail(w, r, http.StatusNotFound, "dashboard no encontrado")
			return
		}
		s.fail(w, r, http.StatusConflict, err.Error())
		return
	}
	s.audit(r, "dashboard.updated", current.Name, map[string]any{"id": current.ID})
	writeJSON(w, http.StatusOK, current)
}

// handleDeleteDashboard removes a view.
func (s *Server) handleDeleteDashboard(w http.ResponseWriter, r *http.Request) {
	if !s.canWrite(w, r) {
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.st.DeleteDashboard(id); err != nil {
		if isNotFound(err) {
			s.fail(w, r, http.StatusNotFound, "dashboard no encontrado")
			return
		}
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	s.audit(r, "dashboard.deleted", strconvID(id), nil)
	w.WriteHeader(http.StatusNoContent)
}
