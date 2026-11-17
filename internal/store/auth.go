package store

import (
	"database/sql"
	"time"

	"github.com/kr3s4l4/netdiag/internal/model"
)

// CountUsers returns the number of local accounts.
func (s *Store) CountUsers() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&n)
	return n, err
}

// CreateUser inserts a local account.
func (s *Store) CreateUser(username, passwordHash, role string) (*model.User, error) {
	if role == "" {
		role = "admin"
	}
	res, err := s.db.Exec(`INSERT INTO users(username, password_hash, role, created_at) VALUES(?,?,?,?)`,
		username, passwordHash, role, nowMS())
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetUser(id)
}

// GetUser returns a user by ID.
func (s *Store) GetUser(id int64) (*model.User, error) {
	u := &model.User{}
	var created int64
	var last sql.NullInt64
	err := s.db.QueryRow(`SELECT id, username, role, created_at, last_login_at FROM users WHERE id = ?`, id).
		Scan(&u.ID, &u.Username, &u.Role, &created, &last)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	u.CreatedAt = fromMS(created)
	u.LastLoginAt = fromMSNull(last)
	return u, nil
}

// GetUserWithHash returns a user and its password hash for login.
func (s *Store) GetUserWithHash(username string) (*model.User, string, error) {
	u := &model.User{}
	var created int64
	var last sql.NullInt64
	var hash string
	err := s.db.QueryRow(`SELECT id, username, role, created_at, last_login_at, password_hash FROM users WHERE username = ?`, username).
		Scan(&u.ID, &u.Username, &u.Role, &created, &last, &hash)
	if err == sql.ErrNoRows {
		return nil, "", ErrNotFound
	}
	if err != nil {
		return nil, "", err
	}
	u.CreatedAt = fromMS(created)
	u.LastLoginAt = fromMSNull(last)
	return u, hash, nil
}

// ListUsers returns all local accounts.
func (s *Store) ListUsers() ([]model.User, error) {
	rows, err := s.db.Query(`SELECT id, username, role, created_at, last_login_at FROM users ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.User{}
	for rows.Next() {
		var u model.User
		var created int64
		var last sql.NullInt64
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &created, &last); err != nil {
			return nil, err
		}
		u.CreatedAt = fromMS(created)
		u.LastLoginAt = fromMSNull(last)
		out = append(out, u)
	}
	return out, rows.Err()
}

// UpdatePassword changes a user's password hash.
func (s *Store) UpdatePassword(userID int64, hash string) error {
	_, err := s.db.Exec(`UPDATE users SET password_hash = ? WHERE id = ?`, hash, userID)
	return err
}

// UpdateLastLogin records a successful login.
func (s *Store) UpdateLastLogin(userID int64) error {
	_, err := s.db.Exec(`UPDATE users SET last_login_at = ? WHERE id = ?`, nowMS(), userID)
	return err
}

// UpdateUserRole changes a user's role.
func (s *Store) UpdateUserRole(userID int64, role string) error {
	res, err := s.db.Exec(`UPDATE users SET role = ? WHERE id = ?`, role, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DeleteUser removes an account and all of its sessions.
func (s *Store) DeleteUser(userID int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM sessions WHERE user_id = ?`, userID); err != nil {
		return err
	}
	res, err := tx.Exec(`DELETE FROM users WHERE id = ?`, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return tx.Commit()
}

// CountAdmins returns how many administrator accounts exist.
func (s *Store) CountAdmins() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM users WHERE role IN ('admin','owner')`).Scan(&n)
	return n, err
}

// CreateSession stores a session. The caller generates ID (token hash) and CSRF.
func (s *Store) CreateSession(sess *model.Session) error {
	if sess.CreatedAt.IsZero() {
		sess.CreatedAt = time.Now()
	}
	if sess.LastSeenAt.IsZero() {
		sess.LastSeenAt = sess.CreatedAt
	}
	_, err := s.db.Exec(`INSERT INTO sessions(id, user_id, csrf, created_at, expires_at, last_seen_at, ip, user_agent)
		VALUES(?,?,?,?,?,?,?,?)`, sess.ID, sess.UserID, sess.CSRF, sess.CreatedAt.UnixMilli(),
		sess.ExpiresAt.UnixMilli(), sess.LastSeenAt.UnixMilli(), sess.IP, sess.UserAgent)
	return err
}

// GetSession returns a session and its user if not expired.
func (s *Store) GetSession(id string) (*model.Session, *model.User, error) {
	var (
		sess             model.Session
		user             model.User
		created, expires int64
		lastSeen, uCreat int64
		uLast            sql.NullInt64
	)
	err := s.db.QueryRow(`SELECT s.id, s.user_id, s.csrf, s.created_at, s.expires_at, s.last_seen_at,
			s.ip, s.user_agent, u.username, u.role, u.created_at, u.last_login_at
		FROM sessions s JOIN users u ON u.id = s.user_id
		WHERE s.id = ? AND s.expires_at > ?`, id, nowMS()).
		Scan(&sess.ID, &sess.UserID, &sess.CSRF, &created, &expires, &lastSeen, &sess.IP, &sess.UserAgent,
			&user.Username, &user.Role, &uCreat, &uLast)
	if err == sql.ErrNoRows {
		return nil, nil, ErrNotFound
	}
	if err != nil {
		return nil, nil, err
	}
	sess.CreatedAt = fromMS(created)
	sess.ExpiresAt = fromMS(expires)
	sess.LastSeenAt = fromMS(lastSeen)
	user.ID = sess.UserID
	user.CreatedAt = fromMS(uCreat)
	user.LastLoginAt = fromMSNull(uLast)
	return &sess, &user, nil
}

// TouchSession refreshes last-seen (at most once per minute).
func (s *Store) TouchSession(id string) {
	_, _ = s.db.Exec(`UPDATE sessions SET last_seen_at = ? WHERE id = ? AND last_seen_at < ?`,
		nowMS(), id, time.Now().Add(-time.Minute).UnixMilli())
}

// DeleteSession deletes one session.
func (s *Store) DeleteSession(id string) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE id = ?`, id)
	return err
}

// DeleteUserSessions logs a user out everywhere.
func (s *Store) DeleteUserSessions(userID int64) error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE user_id = ?`, userID)
	return err
}

// PurgeSessions removes expired sessions.
func (s *Store) PurgeSessions() error {
	_, err := s.db.Exec(`DELETE FROM sessions WHERE expires_at <= ?`, nowMS())
	return err
}
