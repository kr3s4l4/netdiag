package api

import (
	"net/http"
	"strings"
	"time"

	"github.com/kr3s4l4/netdiag/internal/model"
)

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type sessionResponse struct {
	User      *model.User `json:"user"`
	CSRF      string      `json:"csrf"`
	ExpiresAt time.Time   `json:"expires_at"`
}

func (s *Server) handleAuthStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.sessionStatus(r))
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if s.auth.SetupRequired() {
		s.failCode(w, r, http.StatusPreconditionRequired, "setup_required", "configura la cuenta de administrador primero")
		return
	}
	var req loginRequest
	if err := decode(r, &req); err != nil {
		s.fail(w, r, http.StatusBadRequest, "cuerpo inválido: "+err.Error())
		return
	}
	ip := s.reqIP(r)
	ua := r.UserAgent()
	sess, user, err := s.auth.Login(req.Username, req.Password, ip, ua)
	if err != nil {
		if !s.detectAuthErrors(w, r, err) {
			s.fail(w, r, http.StatusInternalServerError, "no se pudo iniciar sesión")
		}
		return
	}
	s.loginCookie(w, sess.ID, sess.ExpiresAt)
	writeJSON(w, http.StatusOK, sessionResponse{User: user, CSRF: sess.CSRF, ExpiresAt: sess.ExpiresAt})
}

func (s *Server) handleSetup(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := decode(r, &req); err != nil {
		s.fail(w, r, http.StatusBadRequest, "cuerpo inválido: "+err.Error())
		return
	}
	if strings.TrimSpace(req.Username) == "" {
		s.fail(w, r, http.StatusBadRequest, "el usuario es obligatorio")
		return
	}
	sess, user, err := s.auth.Setup(req.Username, req.Password, s.reqIP(r), r.UserAgent())
	if err != nil {
		if !s.detectAuthErrors(w, r, err) {
			s.fail(w, r, http.StatusInternalServerError, "no se pudo crear la cuenta: "+err.Error())
		}
		return
	}
	s.loginCookie(w, sess.ID, sess.ExpiresAt)
	writeJSON(w, http.StatusCreated, sessionResponse{User: user, CSRF: sess.CSRF, ExpiresAt: sess.ExpiresAt})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie(sessionCookie); err == nil {
		s.auth.Logout(c.Value, userFrom(r), s.reqIP(r), r.UserAgent())
	}
	s.clearCookie(w)
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

type passwordRequest struct {
	Current string `json:"current_password"`
	New     string `json:"new_password"`
}

func (s *Server) handleChangePassword(w http.ResponseWriter, r *http.Request) {
	u := userFrom(r)
	if u == nil || u.ID == 0 {
		s.failCode(w, r, http.StatusBadRequest, "no_account", "la autenticación está desactivada")
		return
	}
	var req passwordRequest
	if err := decode(r, &req); err != nil {
		s.fail(w, r, http.StatusBadRequest, "cuerpo inválido: "+err.Error())
		return
	}
	if err := s.auth.ChangePassword(u.ID, req.Current, req.New, s.reqIP(r), r.UserAgent()); err != nil {
		if !s.detectAuthErrors(w, r, err) {
			s.fail(w, r, http.StatusInternalServerError, "no se pudo cambiar la contraseña")
		}
		return
	}
	// Every session for this user was invalidated.
	s.clearCookie(w)
	writeJSON(w, http.StatusOK, map[string]any{"ok": true, "relogin": true})
}

func (s *Server) handleListUsers(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	users, err := s.st.ListUsers()
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "no se pudieron listar los usuarios")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": users})
}

type createUserRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

func (s *Server) handleCreateUser(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	var req createUserRequest
	if err := decode(r, &req); err != nil {
		s.fail(w, r, http.StatusBadRequest, "cuerpo inválido: "+err.Error())
		return
	}
	u, err := s.auth.CreateUser(req.Username, req.Password, req.Role)
	if err != nil {
		if !s.detectAuthErrors(w, r, err) {
			s.fail(w, r, http.StatusBadRequest, err.Error())
		}
		return
	}
	s.audit(r, "auth.user_created", req.Username, map[string]any{"role": u.Role})
	s.created(w, u)
}

type updateUserRequest struct {
	Role string `json:"role"`
}

func (s *Server) handleUpdateUser(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	var req updateUserRequest
	if err := decode(r, &req); err != nil {
		s.fail(w, r, http.StatusBadRequest, "cuerpo inválido: "+err.Error())
		return
	}
	u, err := s.auth.UpdateUserRole(id, req.Role)
	if err != nil {
		if !s.detectAuthErrors(w, r, err) {
			s.fail(w, r, http.StatusBadRequest, err.Error())
		}
		return
	}
	s.audit(r, "auth.user_role_changed", u.Username, map[string]any{"role": u.Role})
	s.ok(w, u)
}

func (s *Server) handleDeleteUser(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	if me := userFrom(r); me != nil && me.ID == id {
		s.failCode(w, r, http.StatusConflict, "self_delete", "no puedes eliminar tu propia cuenta")
		return
	}
	u, err := s.st.GetUser(id)
	if err != nil {
		if !s.detectAuthErrors(w, r, err) {
			s.fail(w, r, http.StatusInternalServerError, "no se pudo leer el usuario")
		}
		return
	}
	if err := s.auth.DeleteUser(id); err != nil {
		if !s.detectAuthErrors(w, r, err) {
			s.fail(w, r, http.StatusInternalServerError, err.Error())
		}
		return
	}
	s.audit(r, "auth.user_deleted", u.Username, nil)
	s.ok(w, map[string]any{"ok": true})
}

type resetPasswordRequest struct {
	Password string `json:"password"`
}

func (s *Server) handleResetUserPassword(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	var req resetPasswordRequest
	if err := decode(r, &req); err != nil {
		s.fail(w, r, http.StatusBadRequest, "cuerpo inválido: "+err.Error())
		return
	}
	u, err := s.st.GetUser(id)
	if err != nil {
		if !s.detectAuthErrors(w, r, err) {
			s.fail(w, r, http.StatusInternalServerError, "no se pudo leer el usuario")
		}
		return
	}
	if err := s.auth.ResetPassword(id, req.Password); err != nil {
		if !s.detectAuthErrors(w, r, err) {
			s.fail(w, r, http.StatusBadRequest, err.Error())
		}
		return
	}
	s.audit(r, "auth.user_password_reset", u.Username, nil)
	s.ok(w, map[string]any{"ok": true})
}
