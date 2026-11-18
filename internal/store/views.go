package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
)

// SavedView is a per-user filter for the inventory or the alert list. It is
// defined here (and not in internal/model) so that persistence, validation and
// the API all share exactly the same transport representation.
type SavedView struct {
	ID        int64     `json:"id"`
	SiteID    int64     `json:"site_id"`
	Name      string    `json:"name"`
	Kind      string    `json:"kind"` // hosts | alerts
	Query     string    `json:"query"`
	Severity  string    `json:"severity,omitempty"`
	Status    string    `json:"status,omitempty"`
	Window    string    `json:"window,omitempty"`
	Columns   []string  `json:"columns"`
	Sort      string    `json:"sort,omitempty"`
	Desc      bool      `json:"desc"`
	Builtin   bool      `json:"builtin"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// UserID is never serialized: a view belongs to exactly one account.
	UserID int64 `json:"-"`
}

const (
	ViewKindHosts  = "hosts"
	ViewKindAlerts = "alerts"

	// MaxViewsPerUser caps persisted views so one account cannot fill the DB.
	MaxViewsPerUser = 100

	maxViewNameRunes  = 80
	maxViewQueryRunes = 200
	maxViewColumns    = 16
)

var (
	// ErrViewLimit is returned when a user tries to exceed MaxViewsPerUser.
	ErrViewLimit = errors.New("límite de vistas por usuario alcanzado")
	// ErrViewDuplicate is returned for a repeated name within the same user.
	ErrViewDuplicate = errors.New("ya existe una vista con ese nombre")
)

var viewColumns = map[string][]string{
	ViewKindHosts: {
		"hostname", "ip", "mac", "vendor", "model", "device_type", "status",
		"latency_ms", "open_port_count", "first_seen", "last_seen", "tags",
	},
	ViewKindAlerts: {
		"severity", "status", "title", "message", "host_ip", "rule_name",
		"created_at", "acked_at", "resolved_at",
	},
}

var viewSorts = map[string][]string{
	ViewKindHosts:  {"ip", "hostname", "vendor", "device_type", "status", "latency_ms", "first_seen", "last_seen"},
	ViewKindAlerts: {"created_at", "severity", "status", "title"},
}

var viewStatuses = map[string][]string{
	ViewKindHosts:  {"", "all", "up", "down", "unknown", "archived"},
	ViewKindAlerts: {"", "all", "active", "acked", "resolved"},
}

var viewWindows = []string{"", "24h", "7d", "30d"}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

// sanitizeViewText trims control characters, normalises HTML entities and
// escapes markup before persisting. It is idempotent for already escaped text.
func sanitizeViewText(in string, max int, field string) (string, error) {
	s := strings.TrimSpace(html.UnescapeString(in))
	var b strings.Builder
	for _, r := range s {
		if unicode.IsControl(r) {
			b.WriteRune(' ')
			continue
		}
		b.WriteRune(r)
	}
	s = html.EscapeString(strings.TrimSpace(b.String()))
	if s == "" {
		return "", fmt.Errorf("%s no puede estar vacío", field)
	}
	if utf8.RuneCountInString(s) > max {
		return "", fmt.Errorf("%s supera %d caracteres", field, max)
	}
	return s, nil
}

// ValidateView normalises and validates a view. Both the API and the store
// call it: a caller can never persist unescaped names or arbitrary SQL-ish
// column/sort identifiers.
func ValidateView(v *SavedView) error {
	if v == nil {
		return errors.New("vista inválida")
	}
	if !containsString([]string{ViewKindHosts, ViewKindAlerts}, v.Kind) {
		return errors.New("tipo de vista inválido")
	}
	name, err := sanitizeViewText(v.Name, maxViewNameRunes, "el nombre")
	if err != nil {
		return err
	}
	query := ""
	if strings.TrimSpace(html.UnescapeString(v.Query)) != "" {
		query, err = sanitizeViewText(v.Query, maxViewQueryRunes, "la consulta")
		if err != nil {
			return err
		}
	}
	if v.SiteID < 0 {
		return errors.New("sede inválida")
	}
	severity := strings.ToLower(strings.TrimSpace(v.Severity))
	if severity != "" && !containsString([]string{"info", "warning", "critical"}, severity) {
		return errors.New("severidad inválida")
	}
	if v.Kind == ViewKindHosts {
		severity = ""
	}
	status := strings.ToLower(strings.TrimSpace(v.Status))
	if !containsString(viewStatuses[v.Kind], status) {
		return errors.New("estado inválido para el tipo de vista")
	}
	if status == "all" {
		status = ""
	}
	window := strings.ToLower(strings.TrimSpace(v.Window))
	if !containsString(viewWindows, window) {
		return errors.New("ventana temporal inválida")
	}
	if len(v.Columns) > maxViewColumns {
		return fmt.Errorf("demasiadas columnas (máximo %d)", maxViewColumns)
	}
	cols := make([]string, 0, len(v.Columns))
	for _, c := range v.Columns {
		c = strings.ToLower(strings.TrimSpace(c))
		if !containsString(viewColumns[v.Kind], c) {
			return fmt.Errorf("columna no permitida: %q", c)
		}
		if !containsString(cols, c) {
			cols = append(cols, c)
		}
	}
	if len(cols) == 0 {
		cols = append(cols, viewColumns[v.Kind]...)
	}
	sortField := strings.ToLower(strings.TrimSpace(v.Sort))
	if sortField == "" {
		sortField = viewSorts[v.Kind][0]
	}
	if !containsString(viewSorts[v.Kind], sortField) {
		return errors.New("orden no permitido")
	}

	v.Name, v.Query = name, query
	v.Severity, v.Status, v.Window = severity, status, window
	v.Columns, v.Sort = cols, sortField
	if v.UserID < 0 {
		v.UserID = 0
	}
	return nil
}

// DefaultViews returns the four built-in F40 views. site_id=0 means "all
// sites" so they keep working when the operator changes the active site.
func DefaultViews() []SavedView {
	hostCols := append([]string(nil), viewColumns[ViewKindHosts]...)
	alertCols := append([]string(nil), viewColumns[ViewKindAlerts]...)
	return []SavedView{
		{
			Name: "Críticas", Kind: ViewKindAlerts, Severity: "critical", Status: "active",
			Window: "24h", Columns: alertCols, Sort: "created_at", Desc: true, Builtin: true,
		},
		{
			Name: "Nuevos hosts", Kind: ViewKindHosts, Window: "24h",
			Columns: hostCols, Sort: "first_seen", Desc: true, Builtin: true,
		},
		{
			Name: "Caídos", Kind: ViewKindHosts, Status: "down",
			Columns: hostCols, Sort: "last_seen", Desc: true, Builtin: true,
		},
		{
			Name: "Cambios de la semana", Kind: ViewKindAlerts, Status: "all",
			Window: "7d", Columns: alertCols, Sort: "created_at", Desc: true, Builtin: true,
		},
	}
}

// ViewCatalog describes the accepted values for the saved-view editor.
func ViewCatalog() map[string]any {
	return map[string]any{
		"kinds":       []string{ViewKindHosts, ViewKindAlerts},
		"columns":     viewColumns,
		"sorts":       viewSorts,
		"statuses":    viewStatuses,
		"severities":  []string{"", "info", "warning", "critical"},
		"windows":     []string{"", "24h", "7d", "30d"},
		"max_views":   MaxViewsPerUser,
		"max_columns": maxViewColumns,
		"defaults":    DefaultViews(),
	}
}

// EnsureDefaultViews seeds the four built-in views once per user. The marker
// lives in settings so deleting a default does not resurrect it on reload.
func (s *Store) EnsureDefaultViews(userID int64) error {
	if userID < 0 {
		userID = 0
	}
	key := fmt.Sprintf("views_seeded:%d", userID)
	if s.GetSetting(key, "") == "1" {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := nowMS()
	for _, v := range DefaultViews() {
		v.UserID = userID
		if err := ValidateView(&v); err != nil {
			return err
		}
		raw := jsonMarshal(v.Columns)
		_, err := tx.Exec(`INSERT OR IGNORE INTO saved_views
			(user_id, site_id, name, kind, query, severity, status, window, columns_json, sort, desc, builtin, created_at, updated_at)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			v.UserID, v.SiteID, v.Name, v.Kind, v.Query, v.Severity, v.Status, v.Window,
			raw, v.Sort, boolInt(v.Desc), 1, now, now)
		if err != nil {
			return err
		}
	}
	if _, err := tx.Exec(`INSERT INTO settings(key, value, updated_at) VALUES(?,?,?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`, key, "1", now); err != nil {
		return err
	}
	return tx.Commit()
}

const viewColumnsSQL = `id, user_id, site_id, name, kind, query, severity, status, window,
	columns_json, sort, desc, builtin, created_at, updated_at`

func scanView(scanner interface{ Scan(...any) error }) (*SavedView, error) {
	var v SavedView
	var raw string
	var desc, builtin int
	var created, updated int64
	if err := scanner.Scan(&v.ID, &v.UserID, &v.SiteID, &v.Name, &v.Kind, &v.Query,
		&v.Severity, &v.Status, &v.Window, &raw, &v.Sort, &desc, &builtin, &created, &updated); err != nil {
		return nil, err
	}
	v.Columns = []string{}
	_ = json.Unmarshal([]byte(raw), &v.Columns)
	v.Desc = desc != 0
	v.Builtin = builtin != 0
	v.CreatedAt = time.UnixMilli(created)
	v.UpdatedAt = time.UnixMilli(updated)
	return &v, nil
}

// ListViews returns the views of one user (kind/site filters are optional).
// site_id=0 lists every view; a positive site also includes global views.
func (s *Store) ListViews(userID int64, kind string, siteID int64) ([]SavedView, error) {
	if userID < 0 {
		userID = 0
	}
	where := []string{"user_id = ?"}
	args := []any{userID}
	if kind != "" {
		where = append(where, "kind = ?")
		args = append(args, kind)
	}
	if siteID > 0 {
		where = append(where, "(site_id = 0 OR site_id = ?)")
		args = append(args, siteID)
	}
	q := `SELECT ` + viewColumnsSQL + ` FROM saved_views WHERE ` + strings.Join(where, " AND ") +
		` ORDER BY builtin DESC, name, id`
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []SavedView{}
	for rows.Next() {
		v, err := scanView(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *v)
	}
	return out, rows.Err()
}

// GetView returns one view, but only when it belongs to userID.
func (s *Store) GetView(id, userID int64) (*SavedView, error) {
	if userID < 0 {
		userID = 0
	}
	v, err := scanView(s.db.QueryRow(`SELECT `+viewColumnsSQL+` FROM saved_views WHERE id = ? AND user_id = ?`, id, userID))
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return v, nil
}

// CreateView persists a new view for its UserID and populates ID/timestamps.
func (s *Store) CreateView(v *SavedView) error {
	if err := ValidateView(v); err != nil {
		return err
	}
	var n int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM saved_views WHERE user_id = ? AND name = ?`, v.UserID, v.Name).Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return ErrViewDuplicate
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM saved_views WHERE user_id = ?`, v.UserID).Scan(&n); err != nil {
		return err
	}
	if n >= MaxViewsPerUser {
		return ErrViewLimit
	}
	now := nowMS()
	res, err := s.db.Exec(`INSERT INTO saved_views
		(user_id, site_id, name, kind, query, severity, status, window, columns_json, sort, desc, builtin, created_at, updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		v.UserID, v.SiteID, v.Name, v.Kind, v.Query, v.Severity, v.Status, v.Window,
		jsonMarshal(v.Columns), v.Sort, boolInt(v.Desc), 0, now, now)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return ErrViewDuplicate
		}
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	v.ID = id
	v.Builtin = false
	v.CreatedAt = time.UnixMilli(now)
	v.UpdatedAt = v.CreatedAt
	return nil
}

// UpdateView replaces an owned view. The user_id predicate prevents a user
// from editing somebody else's view even if they know its id.
func (s *Store) UpdateView(v *SavedView, userID int64) error {
	if err := ValidateView(v); err != nil {
		return err
	}
	if userID < 0 {
		userID = 0
	}
	v.UserID = userID
	now := nowMS()
	res, err := s.db.Exec(`UPDATE saved_views SET site_id=?, name=?, kind=?, query=?, severity=?, status=?,
		window=?, columns_json=?, sort=?, desc=?, updated_at=? WHERE id=? AND user_id=?`,
		v.SiteID, v.Name, v.Kind, v.Query, v.Severity, v.Status, v.Window,
		jsonMarshal(v.Columns), v.Sort, boolInt(v.Desc), now, v.ID, userID)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return ErrViewDuplicate
		}
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	v.UpdatedAt = time.UnixMilli(now)
	return nil
}

// DeleteView removes an owned view.
func (s *Store) DeleteView(id, userID int64) error {
	if userID < 0 {
		userID = 0
	}
	res, err := s.db.Exec(`DELETE FROM saved_views WHERE id = ? AND user_id = ?`, id, userID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// ViewShareParams serializes all filters, not just the id, so another
// operator can open exactly the same view without access to this user's row.
func ViewShareParams(v *SavedView) url.Values {
	q := url.Values{}
	if v.Name != "" {
		q.Set("name", v.Name)
	}
	q.Set("kind", v.Kind)
	if v.SiteID > 0 {
		q.Set("site_id", strconv.FormatInt(v.SiteID, 10))
	}
	if v.Query != "" {
		q.Set("q", v.Query)
	}
	if v.Severity != "" {
		q.Set("severity", v.Severity)
	}
	if v.Status != "" {
		q.Set("status", v.Status)
	}
	if v.Window != "" {
		q.Set("window", v.Window)
	}
	if len(v.Columns) > 0 {
		q.Set("columns", strings.Join(v.Columns, ","))
	}
	if v.Sort != "" {
		q.Set("sort", v.Sort)
	}
	if v.Desc {
		q.Set("desc", "1")
	}
	return q
}

// ViewSharePath returns the hash route used by the SPA share links.
func ViewSharePath(v *SavedView) string {
	kind := v.Kind
	if kind != ViewKindAlerts {
		kind = ViewKindHosts
	}
	return "#/" + kind + "?" + ViewShareParams(v).Encode()
}

// ParseShareParams validates the parameters of a shared link. It never
// persists anything and is used by GET /views/resolve.
func ParseShareParams(q url.Values) (*SavedView, error) {
	name := strings.TrimSpace(q.Get("name"))
	if name == "" {
		name = "Vista compartida"
	}
	v := &SavedView{
		Name:     name,
		Kind:     strings.ToLower(strings.TrimSpace(q.Get("kind"))),
		Query:    q.Get("q"),
		Severity: q.Get("severity"),
		Status:   q.Get("status"),
		Window:   q.Get("window"),
		Sort:     q.Get("sort"),
		Desc:     q.Get("desc") == "1" || strings.EqualFold(q.Get("desc"), "true"),
	}
	if raw := strings.TrimSpace(q.Get("site_id")); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || n < 0 {
			return nil, errors.New("sede inválida en el enlace")
		}
		v.SiteID = n
	}
	if raw := strings.TrimSpace(q.Get("columns")); raw != "" {
		v.Columns = strings.Split(raw, ",")
	}
	if err := ValidateView(v); err != nil {
		return nil, err
	}
	return v, nil
}
