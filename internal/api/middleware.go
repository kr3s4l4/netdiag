package api

import (
	"bufio"
	"context"
	"crypto/subtle"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/kr3s4l4/netdiag/internal/auth"
	"github.com/kr3s4l4/netdiag/internal/model"
	"github.com/kr3s4l4/netdiag/internal/store"
)

type ctxKey string

const (
	ctxUser    ctxKey = "user"
	ctxSession ctxKey = "session"
	ctxIP      ctxKey = "ip"
	ctxReqID   ctxKey = "request_id"

	sessionCookie = "netdiag_session"
)

// publicPaths are reachable without a session.
var publicPaths = map[string]bool{
	"/api/v1/auth/status":   true,
	"/api/v1/auth/login":    true,
	"/api/v1/auth/setup":    true,
	"/api/v1/system/health": true,
}

func userFrom(r *http.Request) *model.User {
	u, _ := r.Context().Value(ctxUser).(*model.User)
	return u
}

func sessionFrom(r *http.Request) *model.Session {
	s, _ := r.Context().Value(ctxSession).(*model.Session)
	return s
}

func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// clientIPTrusted honours X-Forwarded-For only when the operator opted in.
func (s *Server) clientIPTrusted(r *http.Request) string {
	if s.cfg.Security.TrustProxy {
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			parts := strings.Split(xff, ",")
			if ip := strings.TrimSpace(parts[0]); ip != "" {
				return ip
			}
		}
		if rip := strings.TrimSpace(r.Header.Get("X-Real-IP")); rip != "" {
			return rip
		}
	}
	return clientIP(r)
}

func (s *Server) middleware(next http.Handler) http.Handler {
	return s.recoverer(s.securityHeaders(s.requestLogger(s.authenticate(s.rateLimit(next)))))
}

func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				s.log.Error("pánico en handler HTTP", "ruta", r.URL.Path, "pánico", rec)
				writeJSON(w, http.StatusInternalServerError, apiError{Error: "error interno del servidor"})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.hostAllowed(r.Host) {
			writeJSON(w, http.StatusMisdirectedRequest, apiError{Error: "host no permitido"})
			return
		}
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=(), interest-cohort=()")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		if s.cfg.Server.EnableCSP {
			h.Set("Content-Security-Policy",
				"default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; "+
					"img-src 'self' data:; font-src 'self'; connect-src 'self' ws: wss:; "+
					"object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'")
		}
		if s.cfg.Server.EnableHSTS || strings.HasPrefix(s.cfg.Server.PublicURL, "https://") {
			h.Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		}
		next.ServeHTTP(w, r)
	})
}

// hostAllowed enforces security.allowed_hosts when it is configured. It
// compares the host without the port and ignores a trailing dot, which is the
// usual way to bypass naive Host header checks.
func (s *Server) hostAllowed(raw string) bool {
	allowed := s.cfg.Security.AllowedHosts
	if len(allowed) == 0 {
		return true
	}
	host := raw
	if h, _, err := net.SplitHostPort(raw); err == nil {
		host = h
	}
	host = strings.TrimSuffix(strings.Trim(host, "[]"), ".")
	host = strings.ToLower(host)
	for _, entry := range allowed {
		entry = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(entry)), ".")
		if entry != "" && entry == host {
			return true
		}
	}
	return false
}

type statusRecorder struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *statusRecorder) WriteHeader(code int) {
	if w.status == 0 {
		w.status = code
	}
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusRecorder) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytes += n
	return n, err
}

func (w *statusRecorder) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (w *statusRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := w.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, errors.New("el servidor no admite hijack")
	}
	w.status = http.StatusSwitchingProtocols
	return hj.Hijack()
}

func (s *Server) requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w}
		next.ServeHTTP(rec, r)
		if rec.status == 0 {
			rec.status = http.StatusOK
		}
		dur := time.Since(start)
		if rec.status >= 500 {
			s.log.Error("petición fallida", "metodo", r.Method, "ruta", r.URL.Path,
				"estado", rec.status, "duracion_ms", dur.Milliseconds(), "ip", s.clientIPTrusted(r))
			return
		}
		if rec.status >= 400 || dur > 2*time.Second || (r.Method != http.MethodGet && r.Method != http.MethodHead) {
			s.log.Info("petición", "metodo", r.Method, "ruta", r.URL.Path,
				"estado", rec.status, "duracion_ms", dur.Milliseconds(), "ip", s.clientIPTrusted(r))
		}
	})
}

func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := s.clientIPTrusted(r)
		ctx := context.WithValue(r.Context(), ctxIP, ip)

		if s.auth.SetupRequired() && !publicPaths[r.URL.Path] && r.URL.Path != "/api/v1/auth/status" {
			s.failCode(w, r, http.StatusPreconditionRequired, "setup_required",
				"configura la cuenta de administrador antes de continuar")
			return
		}

		if !s.cfg.Security.Auth {
			implicit := &model.User{ID: 0, Username: "local", Role: "admin"}
			next.ServeHTTP(w, r.WithContext(context.WithValue(ctx, ctxUser, implicit)))
			return
		}

		cookie, cerr := r.Cookie(sessionCookie)
		var (
			sess *model.Session
			user *model.User
			verr error
		)
		if cerr == nil && cookie.Value != "" {
			sess, user, verr = s.auth.Validate(cookie.Value)
		}

		if publicPaths[r.URL.Path] {
			if verr == nil && user != nil {
				ctx = context.WithValue(ctx, ctxUser, user)
				ctx = context.WithValue(ctx, ctxSession, sess)
			}
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		if verr != nil || user == nil || sess == nil {
			s.failCode(w, r, http.StatusUnauthorized, "unauthorized", "se requiere iniciar sesión")
			return
		}

		// CSRF: mutating requests must echo the per-session token.
		switch r.Method {
		case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
			got := r.Header.Get("X-CSRF-Token")
			if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(sess.CSRF)) != 1 {
				s.failCode(w, r, http.StatusForbidden, "csrf", "token CSRF inválido o ausente")
				return
			}
		}

		ctx = context.WithValue(ctx, ctxUser, user)
		ctx = context.WithValue(ctx, ctxSession, sess)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) rateLimit(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/v1/ws" || r.URL.Path == "/api/v1/system/health" {
			next.ServeHTTP(w, r)
			return
		}
		ip := s.clientIPTrusted(r)
		if !s.limiter.allow(ip) {
			w.Header().Set("Retry-After", "60")
			s.failCode(w, r, http.StatusTooManyRequests, "rate_limited", "demasiadas peticiones; intenta en un minuto")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// requireAdmin rejects viewer accounts on mutating endpoints.
func (s *Server) requireAdmin(w http.ResponseWriter, r *http.Request) bool {
	u := userFrom(r)
	if u == nil {
		s.failCode(w, r, http.StatusUnauthorized, "unauthorized", "se requiere iniciar sesión")
		return false
	}
	if u.Role != "admin" && u.Role != "owner" {
		s.failCode(w, r, http.StatusForbidden, "forbidden", "requiere permisos de administrador")
		return false
	}
	return true
}

// canWrite reports whether the caller may mutate data at all.
func (s *Server) canWrite(w http.ResponseWriter, r *http.Request) bool {
	u := userFrom(r)
	if u == nil {
		s.failCode(w, r, http.StatusUnauthorized, "unauthorized", "se requiere iniciar sesión")
		return false
	}
	return u.Role == "admin" || u.Role == "owner" || u.Role == "viewer"
}

func (s *Server) audit(r *http.Request, action, target string, detail map[string]any) {
	u := userFrom(r)
	actor := "sistema"
	if u != nil {
		actor = u.Username
	}
	_ = s.st.AddAudit(&model.AuditEntry{
		Actor:     actor,
		Action:    action,
		Target:    target,
		Detail:    detail,
		IP:        s.reqIP(r),
		UserAgent: r.UserAgent(),
	})
}

func (s *Server) reqIP(r *http.Request) string {
	if ip, ok := r.Context().Value(ctxIP).(string); ok {
		return ip
	}
	return s.clientIPTrusted(r)
}

// isNotFound maps store errors to a 404 decision.
func isNotFound(err error) bool {
	return errors.Is(err, store.ErrNotFound) || strings.Contains(err.Error(), "no encontrado")
}

// ---- per-IP rate limiter ----

type ipLimiter struct {
	mu      sync.Mutex
	limit   int
	windows map[string]*ipWindow
	lastGC  time.Time
}

type ipWindow struct {
	count int
	start time.Time
}

func newIPLimiter(limit int) *ipLimiter {
	return &ipLimiter{limit: limit, windows: make(map[string]*ipWindow)}
}

func (l *ipLimiter) setLimit(limit int) {
	l.mu.Lock()
	l.limit = limit
	l.mu.Unlock()
}

func (l *ipLimiter) allow(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.limit <= 0 {
		return true
	}
	now := time.Now()
	if now.Sub(l.lastGC) > 5*time.Minute {
		l.lastGC = now
		for k, w := range l.windows {
			if now.Sub(w.start) > 2*time.Minute {
				delete(l.windows, k)
			}
		}
	}
	w := l.windows[ip]
	if w == nil || now.Sub(w.start) > time.Minute {
		l.windows[ip] = &ipWindow{count: 1, start: now}
		return true
	}
	w.count++
	return w.count <= l.limit
}

// loginCookie builds the session cookie for the current scheme.
func (s *Server) loginCookie(w http.ResponseWriter, token string, expires time.Time) {
	secure := strings.HasPrefix(s.cfg.Server.PublicURL, "https://") || s.cfg.Server.TLSCert != ""
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteStrictMode,
		Expires:  expires,
		MaxAge:   int(time.Until(expires).Seconds()),
	})
}

func (s *Server) clearCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
		SameSite: http.SameSiteStrictMode,
	})
}

func (s *Server) sessionStatus(r *http.Request) model.AuthStatus {
	st := model.AuthStatus{
		SetupRequired: s.auth.SetupRequired(),
		AuthEnabled:   s.cfg.Security.Auth,
		Version:       s.version,
	}
	u := userFrom(r)
	if u != nil {
		st.Authenticated = true
		st.User = u
	}
	if sess := sessionFrom(r); sess != nil {
		st.CSRF = sess.CSRF
	}
	return st
}

func (s *Server) detectAuthErrors(w http.ResponseWriter, r *http.Request, err error) bool {
	switch {
	case errors.Is(err, auth.ErrInvalidCredentials):
		s.failCode(w, r, http.StatusUnauthorized, "invalid_credentials", err.Error())
	case errors.Is(err, auth.ErrRateLimited):
		w.Header().Set("Retry-After", "60")
		s.failCode(w, r, http.StatusTooManyRequests, "rate_limited", err.Error())
	case errors.Is(err, auth.ErrSetupComplete):
		s.failCode(w, r, http.StatusConflict, "setup_complete", err.Error())
	case errors.Is(err, auth.ErrWeakPassword):
		s.failCode(w, r, http.StatusBadRequest, "weak_password", err.Error())
	default:
		if err != nil && strings.Contains(err.Error(), "contraseña") {
			s.failCode(w, r, http.StatusBadRequest, "weak_password", err.Error())
			return true
		}
		return false
	}
	return true
}
