package auth

import (
	"errors"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/kr3s4l4/netdiag/internal/config"
	"github.com/kr3s4l4/netdiag/internal/model"
	"github.com/kr3s4l4/netdiag/internal/store"
)

// Errors returned by the service. Handlers map them to HTTP codes.
var (
	ErrInvalidCredentials = errors.New("usuario o contraseña incorrectos")
	ErrRateLimited        = errors.New("demasiados intentos; espera un minuto")
	ErrSetupComplete      = errors.New("la configuración inicial ya se completó")
	ErrSessionExpired     = errors.New("sesión expirada")
	ErrForbidden          = errors.New("permiso insuficiente")
	ErrLastAdmin          = errors.New("no se puede eliminar ni degradar al último administrador")
)

// ValidRole reports whether role is a supported local account role.
func ValidRole(role string) bool {
	switch role {
	case "admin", "owner", "operator", "viewer":
		return true
	}
	return false
}

// Service coordinates accounts, sessions and throttling.
type Service struct {
	st  *store.Store
	cfg config.Security
	log *slog.Logger

	mu       sync.Mutex
	attempts map[string]*attemptRecord
	lastGC   time.Time
}

type attemptRecord struct {
	count int
	first time.Time
}

// New creates the auth service.
func New(st *store.Store, cfg config.Security, log *slog.Logger) *Service {
	if log == nil {
		log = slog.Default()
	}
	return &Service{
		st:       st,
		cfg:      cfg,
		log:      log,
		attempts: make(map[string]*attemptRecord),
	}
}

// Enabled reports whether authentication is active.
func (s *Service) Enabled() bool { return s.cfg.Auth }

// SetupRequired reports whether the instance has no accounts yet. Even with
// auth disabled we create an admin on first run so the instance is never
// unmanageable once auth is turned on.
func (s *Service) SetupRequired() bool {
	n, err := s.st.CountUsers()
	if err != nil {
		s.log.Error("no se pudo contar usuarios", "error", err)
		return false
	}
	return n == 0
}

// Setup creates the first administrator. It fails if any user exists.
func (s *Service) Setup(username, password, ip, ua string) (*model.Session, *model.User, error) {
	if !s.SetupRequired() {
		return nil, nil, ErrSetupComplete
	}
	username = strings.TrimSpace(strings.ToLower(username))
	if username == "" || len(username) > 64 {
		return nil, nil, errors.New("nombre de usuario inválido")
	}
	if err := ValidatePassword(password); err != nil {
		return nil, nil, err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return nil, nil, err
	}
	user, err := s.st.CreateUser(username, hash, "admin")
	if err != nil {
		if s.st != nil && !s.SetupRequired() {
			return nil, nil, ErrSetupComplete
		}
		return nil, nil, err
	}
	sess, err := s.createSession(user.ID, ip, ua)
	if err != nil {
		return nil, nil, err
	}
	_ = s.st.UpdateLastLogin(user.ID)
	s.audit(user.Username, "auth.setup", username, ip, ua)
	return sess, user, nil
}

// Login verifies credentials and creates a session.
func (s *Service) Login(username, password, ip, ua string) (*model.Session, *model.User, error) {
	if !s.allowAttempt(ip) {
		return nil, nil, ErrRateLimited
	}
	username = strings.TrimSpace(strings.ToLower(username))
	user, hash, err := s.st.GetUserWithHash(username)
	if err != nil {
		BurnTime(password)
		s.recordAttempt(ip)
		return nil, nil, ErrInvalidCredentials
	}
	if !VerifyPassword(hash, password) {
		s.recordAttempt(ip)
		s.audit(username, "auth.login_failed", username, ip, ua)
		return nil, nil, ErrInvalidCredentials
	}
	s.resetAttempts(ip)
	sess, err := s.createSession(user.ID, ip, ua)
	if err != nil {
		return nil, nil, err
	}
	_ = s.st.UpdateLastLogin(user.ID)
	s.audit(user.Username, "auth.login", username, ip, ua)
	return sess, user, nil
}

// Validate resolves a raw session token to a session and user.
func (s *Service) Validate(rawToken string) (*model.Session, *model.User, error) {
	if rawToken == "" {
		return nil, nil, ErrSessionExpired
	}
	sess, user, err := s.st.GetSession(HashToken(rawToken))
	if err != nil {
		return nil, nil, ErrSessionExpired
	}
	s.st.TouchSession(sess.ID)
	return sess, user, nil
}

// Logout deletes the session behind a raw token.
func (s *Service) Logout(rawToken string, user *model.User, ip, ua string) {
	if rawToken == "" {
		return
	}
	_ = s.st.DeleteSession(HashToken(rawToken))
	if user != nil {
		s.audit(user.Username, "auth.logout", user.Username, ip, ua)
	}
}

// ChangePassword rotates a user's password and invalidates other sessions.
func (s *Service) ChangePassword(userID int64, current, next, ip, ua string) error {
	user, hash, err := s.st.GetUserWithHash(s.usernameFor(userID))
	if err != nil {
		return err
	}
	if !VerifyPassword(hash, current) {
		return ErrInvalidCredentials
	}
	if err := ValidatePassword(next); err != nil {
		return err
	}
	newHash, err := HashPassword(next)
	if err != nil {
		return err
	}
	if err := s.st.UpdatePassword(user.ID, newHash); err != nil {
		return err
	}
	_ = s.st.DeleteUserSessions(user.ID)
	s.audit(user.Username, "auth.password_changed", user.Username, ip, ua)
	return nil
}

func (s *Service) usernameFor(id int64) string {
	u, err := s.st.GetUser(id)
	if err != nil || u == nil {
		return ""
	}
	return u.Username
}

// CreateUser adds an account (admin only, enforced by the API layer).
func (s *Service) CreateUser(username, password, role string) (*model.User, error) {
	username = strings.TrimSpace(strings.ToLower(username))
	if username == "" || len(username) > 64 {
		return nil, errors.New("nombre de usuario inválido")
	}
	if role == "" {
		role = "viewer"
	}
	if !ValidRole(role) {
		return nil, errors.New("rol inválido")
	}
	if err := ValidatePassword(password); err != nil {
		return nil, err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return nil, err
	}
	return s.st.CreateUser(username, hash, role)
}

// UpdateUserRole changes an account role. It refuses to demote the last
// administrator and invalidates existing sessions so the new role applies
// immediately.
func (s *Service) UpdateUserRole(userID int64, role string) (*model.User, error) {
	if !ValidRole(role) {
		return nil, errors.New("rol inválido")
	}
	user, err := s.st.GetUser(userID)
	if err != nil {
		return nil, err
	}
	isAdmin := user.Role == "admin" || user.Role == "owner"
	willBeAdmin := role == "admin" || role == "owner"
	if isAdmin && !willBeAdmin {
		n, err := s.st.CountAdmins()
		if err != nil {
			return nil, err
		}
		if n <= 1 {
			return nil, ErrLastAdmin
		}
	}
	if err := s.st.UpdateUserRole(userID, role); err != nil {
		return nil, err
	}
	_ = s.st.DeleteUserSessions(userID)
	return s.st.GetUser(userID)
}

// DeleteUser removes an account. It refuses to delete the last administrator.
func (s *Service) DeleteUser(userID int64) error {
	user, err := s.st.GetUser(userID)
	if err != nil {
		return err
	}
	if user.Role == "admin" || user.Role == "owner" {
		n, err := s.st.CountAdmins()
		if err != nil {
			return err
		}
		if n <= 1 {
			return ErrLastAdmin
		}
	}
	return s.st.DeleteUser(userID)
}

// ResetPassword sets a new password for an account and logs it out everywhere.
func (s *Service) ResetPassword(userID int64, password string) error {
	if err := ValidatePassword(password); err != nil {
		return err
	}
	hash, err := HashPassword(password)
	if err != nil {
		return err
	}
	if err := s.st.UpdatePassword(userID, hash); err != nil {
		return err
	}
	return s.st.DeleteUserSessions(userID)
}

func (s *Service) createSession(userID int64, ip, ua string) (*model.Session, error) {
	raw, err := NewToken(32)
	if err != nil {
		return nil, err
	}
	csrf, err := NewToken(32)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	hours := s.cfg.SessionHours
	if hours <= 0 {
		hours = 12
	}
	sess := &model.Session{
		ID:         HashToken(raw),
		UserID:     userID,
		CSRF:       csrf,
		CreatedAt:  now,
		ExpiresAt:  now.Add(time.Duration(hours) * time.Hour),
		LastSeenAt: now,
		IP:         ip,
		UserAgent:  trimUA(ua),
	}
	if err := s.st.CreateSession(sess); err != nil {
		return nil, err
	}
	// The raw token is what the cookie carries; the session record keeps only
	// the hash. Attach it to the returned struct via an unexported field is not
	// possible, so Login hands the token back separately through SessionToken.
	sess.ID = raw
	return sess, nil
}

// trimUA caps the user-agent length before storage.
func trimUA(ua string) string {
	if len(ua) > 255 {
		return ua[:255]
	}
	return ua
}

func (s *Service) audit(actor, action, target, ip, ua string) {
	_ = s.st.AddAudit(&model.AuditEntry{
		Actor:     actor,
		Action:    action,
		Target:    target,
		IP:        ip,
		UserAgent: trimUA(ua),
	})
}

// ---- login throttling ----

// allowAttempt reports whether ip may attempt a login now.
func (s *Service) allowAttempt(ip string) bool {
	limit := s.cfg.LoginRate
	if limit <= 0 {
		limit = 5
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.gcLocked()
	rec := s.attempts[ip]
	if rec == nil {
		return true
	}
	if time.Since(rec.first) > time.Minute {
		delete(s.attempts, ip)
		return true
	}
	return rec.count < limit
}

func (s *Service) recordAttempt(ip string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rec := s.attempts[ip]
	if rec == nil || time.Since(rec.first) > time.Minute {
		s.attempts[ip] = &attemptRecord{count: 1, first: time.Now()}
		return
	}
	rec.count++
}

func (s *Service) resetAttempts(ip string) {
	s.mu.Lock()
	delete(s.attempts, ip)
	s.mu.Unlock()
}

func (s *Service) gcLocked() {
	if time.Since(s.lastGC) < 5*time.Minute {
		return
	}
	s.lastGC = time.Now()
	for ip, rec := range s.attempts {
		if time.Since(rec.first) > 10*time.Minute {
			delete(s.attempts, ip)
		}
	}
}

// PurgeExpiredSessions removes sessions from the database.
func (s *Service) PurgeExpiredSessions() {
	_ = s.st.PurgeSessions()
}
