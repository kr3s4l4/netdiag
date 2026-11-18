package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/kr3s4l4/netdiag/internal/model"
)

// siteRequest is the payload for creating and renaming a site.
type siteRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

func (s *Server) handleListSites(w http.ResponseWriter, r *http.Request) {
	sites, err := s.st.ListSites()
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "no se pudieron listar las sedes: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": sites, "total": len(sites)})
}

func (s *Server) handleGetSite(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	site, err := s.st.GetSite(id)
	if err != nil {
		if isNotFound(err) {
			s.fail(w, r, http.StatusNotFound, "sede no encontrada")
			return
		}
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, site)
}

func (s *Server) handleCreateSite(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var req siteRequest
	if err := decode(r, &req); err != nil {
		s.fail(w, r, http.StatusBadRequest, "cuerpo inválido: "+err.Error())
		return
	}
	site, err := s.st.CreateSite(req.Name, req.Description)
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	s.audit(r, "site.created", site.Slug, map[string]any{"id": site.ID, "name": site.Name})
	s.hub.Broadcast("sites.updated", site)
	s.created(w, site)
}

func (s *Server) handleUpdateSite(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	var req siteRequest
	if err := decode(r, &req); err != nil {
		s.fail(w, r, http.StatusBadRequest, "cuerpo inválido: "+err.Error())
		return
	}
	site, err := s.st.UpdateSite(id, req.Name, req.Description)
	if err != nil {
		if isNotFound(err) {
			s.fail(w, r, http.StatusNotFound, "sede no encontrada")
			return
		}
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	s.audit(r, "site.updated", site.Slug, map[string]any{"id": site.ID, "name": site.Name})
	s.hub.Broadcast("sites.updated", site)
	writeJSON(w, http.StatusOK, site)
}

func (s *Server) handleDeleteSite(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	moveTo := queryInt64(r, "move_to")
	site, _ := s.st.GetSite(id)
	if err := s.st.DeleteSite(id, moveTo); err != nil {
		if isNotFound(err) {
			s.fail(w, r, http.StatusNotFound, "sede no encontrada")
			return
		}
		s.fail(w, r, http.StatusConflict, err.Error())
		return
	}
	target := strconvID(id)
	if site != nil {
		target = site.Name
	}
	s.audit(r, "site.deleted", target, map[string]any{"id": id, "move_to": moveTo})
	s.hub.Broadcast("sites.updated", map[string]any{"deleted": id})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleExportSite(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	bundle, err := s.st.ExportSite(id)
	if err != nil {
		if isNotFound(err) {
			s.fail(w, r, http.StatusNotFound, "sede no encontrada")
			return
		}
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	slug := bundle.Site.Slug
	if slug == "" {
		slug = "sede"
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="netdiag-sede-%s.json"`, slug))
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(bundle)
}

// handleImportSite accepts either a JSON bundle as the request body or a
// multipart upload with a "file" part. mode=replace clears the destination
// site first; mode=merge (default) only adds missing data.
func (s *Server) handleImportSite(w http.ResponseWriter, r *http.Request) {
	if !s.canWrite(w, r) {
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	replace := strings.EqualFold(queryString(r, "mode"), "replace")
	bundle, err := s.readSiteBundle(r)
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	if bundle.Schema != "" && bundle.Schema != "netdiag.site/v1" {
		s.fail(w, r, http.StatusBadRequest, "formato de paquete no compatible: "+bundle.Schema)
		return
	}
	summary, err := s.st.ImportSite(id, bundle, replace)
	if err != nil {
		if isNotFound(err) {
			s.fail(w, r, http.StatusNotFound, "sede no encontrada")
			return
		}
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	s.audit(r, "site.imported", strconvID(id), map[string]any{"mode": queryString(r, "mode"), "summary": summary})
	s.hub.Broadcast("sites.updated", map[string]any{"imported": id})
	writeJSON(w, http.StatusOK, map[string]any{"summary": summary})
}

func (s *Server) readSiteBundle(r *http.Request) (*model.SiteBundle, error) {
	if strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		if err := r.ParseMultipartForm(8 << 20); err != nil {
			return nil, fmt.Errorf("no se pudo leer el formulario: %w", err)
		}
		file, _, err := r.FormFile("file")
		if err != nil {
			return nil, fmt.Errorf("falta el archivo del paquete: %w", err)
		}
		defer file.Close()
		raw, err := io.ReadAll(io.LimitReader(file, 8<<20))
		if err != nil {
			return nil, err
		}
		var bundle model.SiteBundle
		if err := json.Unmarshal(raw, &bundle); err != nil {
			return nil, fmt.Errorf("paquete JSON inválido: %w", err)
		}
		return &bundle, nil
	}
	var bundle model.SiteBundle
	if err := decode(r, &bundle); err != nil {
		return nil, err
	}
	return &bundle, nil
}

func strconvID(id int64) string {
	return fmt.Sprintf("%d", id)
}
