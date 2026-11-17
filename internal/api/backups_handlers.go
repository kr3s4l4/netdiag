package api

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"

	"github.com/kr3s4l4/netdiag/internal/backup"
)

func (s *Server) handleListBackups(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	if s.backups == nil {
		s.fail(w, r, http.StatusServiceUnavailable, "las copias de seguridad no están disponibles")
		return
	}
	items, err := s.backups.List()
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "no se pudieron listar las copias: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": items, "dir": s.backups.Dir()})
}

func (s *Server) handleCreateBackup(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	if s.backups == nil {
		s.fail(w, r, http.StatusServiceUnavailable, "las copias de seguridad no están disponibles")
		return
	}
	entry, err := s.backups.Create()
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "no se pudo crear la copia: "+err.Error())
		return
	}
	s.audit(r, "backup.created", entry.Name, map[string]any{"size": entry.Size})
	s.created(w, entry)
}

func (s *Server) handleDownloadBackup(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	if s.backups == nil {
		s.fail(w, r, http.StatusServiceUnavailable, "las copias de seguridad no están disponibles")
		return
	}
	path, err := s.backups.Path(r.PathValue("name"))
	if err != nil {
		if errors.Is(err, backup.ErrInvalidName) {
			s.fail(w, r, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, os.ErrNotExist) {
			s.fail(w, r, http.StatusNotFound, "la copia no existe")
			return
		}
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filepath.Base(path)+`"`)
	http.ServeFile(w, r, path)
}

func (s *Server) handleDeleteBackup(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	if s.backups == nil {
		s.fail(w, r, http.StatusServiceUnavailable, "las copias de seguridad no están disponibles")
		return
	}
	name := r.PathValue("name")
	if err := s.backups.Delete(name); err != nil {
		if errors.Is(err, backup.ErrInvalidName) {
			s.fail(w, r, http.StatusBadRequest, err.Error())
			return
		}
		if errors.Is(err, os.ErrNotExist) {
			s.fail(w, r, http.StatusNotFound, "la copia no existe")
			return
		}
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	s.audit(r, "backup.deleted", name, nil)
	s.ok(w, map[string]any{"ok": true})
}
