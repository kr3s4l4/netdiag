package api

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/kr3s4l4/netdiag/internal/store"
)

// viewRequest is the JSON payload for creating or replacing a saved view.
// Columns and sort are validated against a fixed catalog by the store.
type viewRequest struct {
	Name     string   `json:"name"`
	Kind     string   `json:"kind"`
	SiteID   int64    `json:"site_id,omitempty"`
	Query    string   `json:"query,omitempty"`
	Severity string   `json:"severity,omitempty"`
	Status   string   `json:"status,omitempty"`
	Window   string   `json:"window,omitempty"`
	Columns  []string `json:"columns,omitempty"`
	Sort     string   `json:"sort,omitempty"`
	Desc     bool     `json:"desc,omitempty"`
}

func (req viewRequest) view() *store.SavedView {
	return &store.SavedView{
		SiteID:   req.SiteID,
		Name:     req.Name,
		Kind:     req.Kind,
		Query:    req.Query,
		Severity: req.Severity,
		Status:   req.Status,
		Window:   req.Window,
		Columns:  req.Columns,
		Sort:     req.Sort,
		Desc:     req.Desc,
	}
}

// viewUser resolves the authenticated account. Every authenticated role,
// including viewer, may keep personal views: they contain no system changes.
func (s *Server) viewUser(w http.ResponseWriter, r *http.Request) (int64, bool) {
	u := userFrom(r)
	if u == nil {
		s.failCode(w, r, http.StatusUnauthorized, "unauthorized", "se requiere iniciar sesión")
		return 0, false
	}
	return u.ID, true
}

func (s *Server) failView(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		s.failCode(w, r, http.StatusNotFound, "not_found", "vista no encontrada")
	case errors.Is(err, store.ErrViewLimit):
		s.failCode(w, r, http.StatusConflict, "view_limit", err.Error())
	case errors.Is(err, store.ErrViewDuplicate):
		s.failCode(w, r, http.StatusConflict, "duplicate_view", err.Error())
	default:
		s.failCode(w, r, http.StatusBadRequest, "invalid_view", err.Error())
	}
}

// handleListViews lists only the caller's views, with the built-in four
// seeded on first use.
func (s *Server) handleListViews(w http.ResponseWriter, r *http.Request) {
	uid, ok := s.viewUser(w, r)
	if !ok {
		return
	}
	if err := s.st.EnsureDefaultViews(uid); err != nil {
		s.failCode(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	items, err := s.st.ListViews(uid, queryString(r, "kind"), queryInt64(r, "site_id"))
	if err != nil {
		s.failCode(w, r, http.StatusInternalServerError, "store_error", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"items": items, "total": len(items), "max_views": store.MaxViewsPerUser,
	})
}

// handleViewCatalog exposes allowed columns, sorts, statuses and defaults.
func (s *Server) handleViewCatalog(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.viewUser(w, r); !ok {
		return
	}
	writeJSON(w, http.StatusOK, store.ViewCatalog())
}

// handleGetView returns a view owned by the caller.
func (s *Server) handleGetView(w http.ResponseWriter, r *http.Request) {
	uid, ok := s.viewUser(w, r)
	if !ok {
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		s.failCode(w, r, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	v, err := s.st.GetView(id, uid)
	if err != nil {
		s.failView(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}

// handleCreateView stores a personal filter set after sanitising its text.
func (s *Server) handleCreateView(w http.ResponseWriter, r *http.Request) {
	uid, ok := s.viewUser(w, r)
	if !ok {
		return
	}
	var req viewRequest
	if err := decode(r, &req); err != nil {
		s.failCode(w, r, http.StatusBadRequest, "invalid_body", "cuerpo inválido: "+err.Error())
		return
	}
	v := req.view()
	v.UserID = uid
	if err := store.ValidateView(v); err != nil {
		s.failView(w, r, err)
		return
	}
	if v.SiteID > 0 && !s.st.SiteExists(v.SiteID) {
		s.failCode(w, r, http.StatusBadRequest, "invalid_view", "sede no encontrada")
		return
	}
	if err := s.st.CreateView(v); err != nil {
		s.failView(w, r, err)
		return
	}
	s.audit(r, "view.created", v.Name, map[string]any{"id": v.ID, "kind": v.Kind})
	s.created(w, v)
}

// handleUpdateView replaces a view owned by the caller.
func (s *Server) handleUpdateView(w http.ResponseWriter, r *http.Request) {
	uid, ok := s.viewUser(w, r)
	if !ok {
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		s.failCode(w, r, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	current, err := s.st.GetView(id, uid)
	if err != nil {
		s.failView(w, r, err)
		return
	}
	var req viewRequest
	if err := decode(r, &req); err != nil {
		s.failCode(w, r, http.StatusBadRequest, "invalid_body", "cuerpo inválido: "+err.Error())
		return
	}
	next := req.view()
	next.ID = current.ID
	next.UserID = uid
	next.Builtin = current.Builtin
	if err := store.ValidateView(next); err != nil {
		s.failView(w, r, err)
		return
	}
	if next.SiteID > 0 && !s.st.SiteExists(next.SiteID) {
		s.failCode(w, r, http.StatusBadRequest, "invalid_view", "sede no encontrada")
		return
	}
	if err := s.st.UpdateView(next, uid); err != nil {
		s.failView(w, r, err)
		return
	}
	s.audit(r, "view.updated", next.Name, map[string]any{"id": next.ID})
	writeJSON(w, http.StatusOK, next)
}

// handleDeleteView removes a view owned by the caller.
func (s *Server) handleDeleteView(w http.ResponseWriter, r *http.Request) {
	uid, ok := s.viewUser(w, r)
	if !ok {
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		s.failCode(w, r, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	if err := s.st.DeleteView(id, uid); err != nil {
		s.failView(w, r, err)
		return
	}
	s.audit(r, "view.deleted", strconv.FormatInt(id, 10), nil)
	w.WriteHeader(http.StatusNoContent)
}

// handleShareView builds a hash URL containing every filter parameter, not
// the internal view id, so it works for any other authenticated operator.
func (s *Server) handleShareView(w http.ResponseWriter, r *http.Request) {
	uid, ok := s.viewUser(w, r)
	if !ok {
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		s.failCode(w, r, http.StatusBadRequest, "invalid_id", err.Error())
		return
	}
	v, err := s.st.GetView(id, uid)
	if err != nil {
		s.failView(w, r, err)
		return
	}
	path := store.ViewSharePath(v)
	writeJSON(w, http.StatusOK, map[string]any{
		"id": v.ID, "name": v.Name, "path": path, "url": path,
		"params": store.ViewShareParams(v),
	})
}

// handleResolveView validates the parameters of a shared URL. The result is
// returned but never persisted in this user's account.
func (s *Server) handleResolveView(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.viewUser(w, r); !ok {
		return
	}
	v, err := store.ParseShareParams(r.URL.Query())
	if err != nil {
		s.failView(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, v)
}
