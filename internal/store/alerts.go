package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/kr3s4l4/netdiag/internal/model"
)

// --- alert rules ---

const ruleColumns = `id, name, type, enabled, severity, config, channel_ids, cooldown_sec, builtin, created_at, updated_at`

func scanRule(r rowScanner) (*model.AlertRule, error) {
	var (
		rule          model.AlertRule
		enabled       int
		builtin       int
		cfg, channels string
		created, upd  int64
	)
	if err := r.Scan(&rule.ID, &rule.Name, &rule.Type, &enabled, &rule.Severity, &cfg, &channels,
		&rule.CooldownSec, &builtin, &created, &upd); err != nil {
		return nil, err
	}
	rule.Enabled = enabled != 0
	rule.Builtin = builtin != 0
	rule.Config = jsonObject(cfg)
	rule.ChannelIDs = jsonInt64s(channels)
	if rule.ChannelIDs == nil {
		rule.ChannelIDs = []int64{}
	}
	rule.CreatedAt = fromMS(created)
	rule.UpdatedAt = fromMS(upd)
	return &rule, nil
}

// ListRules returns all alert rules.
func (s *Store) ListRules() ([]model.AlertRule, error) {
	rows, err := s.db.Query(`SELECT ` + ruleColumns + ` FROM alert_rules ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.AlertRule{}
	for rows.Next() {
		r, err := scanRule(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *r)
	}
	return out, rows.Err()
}

// GetRule returns one rule.
func (s *Store) GetRule(id int64) (*model.AlertRule, error) {
	r, err := scanRule(s.db.QueryRow(`SELECT `+ruleColumns+` FROM alert_rules WHERE id = ?`, id))
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return r, err
}

// CreateRule inserts a rule.
func (s *Store) CreateRule(r *model.AlertRule) (*model.AlertRule, error) {
	now := time.Now()
	if r.Severity == "" {
		r.Severity = model.SeverityWarning
	}
	if r.CooldownSec < 0 {
		r.CooldownSec = 0
	}
	res, err := s.db.Exec(`INSERT INTO alert_rules(name,type,enabled,severity,config,channel_ids,cooldown_sec,builtin,created_at,updated_at)
		VALUES(?,?,?,?,?,?,?,?,?,?)`, r.Name, r.Type, boolInt(r.Enabled), r.Severity, jsonMarshal(r.Config),
		jsonMarshal(r.ChannelIDs), r.CooldownSec, boolInt(r.Builtin), now.UnixMilli(), now.UnixMilli())
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetRule(id)
}

// UpdateRule updates a rule.
func (s *Store) UpdateRule(r *model.AlertRule) error {
	_, err := s.db.Exec(`UPDATE alert_rules SET name=?, type=?, enabled=?, severity=?, config=?, channel_ids=?,
		cooldown_sec=?, updated_at=? WHERE id=?`,
		r.Name, r.Type, boolInt(r.Enabled), r.Severity, jsonMarshal(r.Config), jsonMarshal(r.ChannelIDs),
		r.CooldownSec, nowMS(), r.ID)
	return err
}

// DeleteRule deletes a rule.
func (s *Store) DeleteRule(id int64) error {
	_, err := s.db.Exec(`DELETE FROM alert_rules WHERE id = ?`, id)
	return err
}

// --- channels ---

const channelColumns = `id, name, type, enabled, config_enc, has_secret, created_at, updated_at, last_test_at, last_error`

func (s *Store) scanChannel(r rowScanner) (*model.AlertChannel, error) {
	var (
		ch        model.AlertChannel
		enabled   int
		hasSecret int
		enc       []byte
		created   int64
		updated   int64
		lastTest  sql.NullInt64
	)
	if err := r.Scan(&ch.ID, &ch.Name, &ch.Type, &enabled, &enc, &hasSecret, &created, &updated, &lastTest, &ch.LastError); err != nil {
		return nil, err
	}
	ch.Enabled = enabled != 0
	ch.HasSecret = hasSecret != 0
	ch.CreatedAt = fromMS(created)
	ch.UpdatedAt = fromMS(updated)
	ch.LastTestAt = fromMSNull(lastTest)
	ch.Config = map[string]any{}
	if s.box != nil && len(enc) > 0 {
		plain, err := s.box.Decrypt(enc)
		if err != nil {
			return nil, fmt.Errorf("canal %d: no se pudo descifrar la configuración: %w", ch.ID, err)
		}
		_ = json.Unmarshal(plain, &ch.Config)
	}
	return &ch, nil
}

// ListChannels returns all channels with decrypted configuration. Callers that
// expose this data to clients must mask secret fields first.
func (s *Store) ListChannels() ([]model.AlertChannel, error) {
	rows, err := s.db.Query(`SELECT ` + channelColumns + ` FROM channels ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.AlertChannel{}
	for rows.Next() {
		ch, err := s.scanChannel(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *ch)
	}
	return out, rows.Err()
}

// GetChannel returns one channel.
func (s *Store) GetChannel(id int64) (*model.AlertChannel, error) {
	ch, err := s.scanChannel(s.db.QueryRow(`SELECT `+channelColumns+` FROM channels WHERE id = ?`, id))
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return ch, err
}

// ChannelHasSecret inspects config for any non-empty secret field.
func ChannelHasSecret(typ string, cfg map[string]any) bool {
	for _, f := range ChannelSecretFields(typ) {
		if v, ok := cfg[f]; ok {
			if str, _ := v.(string); str != "" {
				return true
			}
		}
	}
	return false
}

func (s *Store) encryptChannelConfig(cfg map[string]any) ([]byte, error) {
	if s.box == nil {
		return nil, fmt.Errorf("no hay clave de cifrado configurada")
	}
	plain, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	return s.box.Encrypt(plain)
}

// CreateChannel encrypts and stores a channel.
func (s *Store) CreateChannel(ch *model.AlertChannel) (*model.AlertChannel, error) {
	if ch.Config == nil {
		ch.Config = map[string]any{}
	}
	enc, err := s.encryptChannelConfig(ch.Config)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	res, err := s.db.Exec(`INSERT INTO channels(name,type,enabled,config_enc,has_secret,created_at,updated_at,last_error)
		VALUES(?,?,?,?,?,?,?,'')`, ch.Name, ch.Type, boolInt(ch.Enabled), enc,
		boolInt(ChannelHasSecret(ch.Type, ch.Config)), now.UnixMilli(), now.UnixMilli())
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetChannel(id)
}

// UpdateChannel replaces a channel configuration.
func (s *Store) UpdateChannel(ch *model.AlertChannel) error {
	if ch.Config == nil {
		ch.Config = map[string]any{}
	}
	enc, err := s.encryptChannelConfig(ch.Config)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`UPDATE channels SET name=?, type=?, enabled=?, config_enc=?, has_secret=?, updated_at=? WHERE id=?`,
		ch.Name, ch.Type, boolInt(ch.Enabled), enc, boolInt(ChannelHasSecret(ch.Type, ch.Config)), nowMS(), ch.ID)
	return err
}

// DeleteChannel deletes a channel.
func (s *Store) DeleteChannel(id int64) error {
	_, err := s.db.Exec(`DELETE FROM channels WHERE id = ?`, id)
	return err
}

// MarkChannelTest records the outcome of a test delivery.
func (s *Store) MarkChannelTest(id int64, errMsg string) error {
	_, err := s.db.Exec(`UPDATE channels SET last_test_at=?, last_error=? WHERE id=?`, nowMS(), errMsg, id)
	return err
}

// --- alerts ---

const alertColumns = `id, rule_id, rule_name, host_id, host_ip, host_name, severity, status, title,
	message, data, created_at, acked_at, resolved_at, dedupe_key`

func scanAlert(r rowScanner) (*model.Alert, error) {
	var (
		a        model.Alert
		data     string
		created  int64
		acked    sql.NullInt64
		resolved sql.NullInt64
	)
	if err := r.Scan(&a.ID, &a.RuleID, &a.RuleName, &a.HostID, &a.HostIP, &a.HostName, &a.Severity,
		&a.Status, &a.Title, &a.Message, &data, &created, &acked, &resolved, &a.DedupeKey); err != nil {
		return nil, err
	}
	a.Data = jsonObject(data)
	a.CreatedAt = fromMS(created)
	a.AckedAt = fromMSNull(acked)
	a.ResolvedAt = fromMSNull(resolved)
	return &a, nil
}

// CreateAlert inserts an alert unless an active alert with the same dedupe key exists.
func (s *Store) CreateAlert(a *model.Alert, dedupe string) (*model.Alert, bool, error) {
	if dedupe != "" {
		var id int64
		err := s.db.QueryRow(`SELECT id FROM alerts WHERE dedupe_key = ? AND status IN (?,?) LIMIT 1`,
			dedupe, model.AlertActive, model.AlertAcked).Scan(&id)
		if err == nil {
			existing, err := s.GetAlert(id)
			return existing, false, err
		}
		if err != sql.ErrNoRows {
			return nil, false, err
		}
	}
	if a.Severity == "" {
		a.Severity = model.SeverityWarning
	}
	res, err := s.db.Exec(`INSERT INTO alerts(rule_id, rule_name, host_id, host_ip, host_name, severity, status,
		title, message, data, created_at, dedupe_key) VALUES(?,?,?,?,?,?,?,?,?,?,?,?)`,
		a.RuleID, a.RuleName, a.HostID, a.HostIP, a.HostName, a.Severity, model.AlertActive,
		a.Title, a.Message, jsonMarshal(a.Data), nowMS(), dedupe)
	if err != nil {
		return nil, false, err
	}
	id, _ := res.LastInsertId()
	if a.HostID > 0 {
		addEventTx2(s, a.HostID, "alert", a.Title, map[string]any{"severity": a.Severity, "alert_id": id})
	}
	out, err := s.GetAlert(id)
	return out, true, err
}

func addEventTx2(s *Store, hostID int64, kind, message string, data map[string]any) {
	_ = s.AddEvent(hostID, kind, message, data)
}

// GetAlert returns one alert.
func (s *Store) GetAlert(id int64) (*model.Alert, error) {
	a, err := scanAlert(s.db.QueryRow(`SELECT `+alertColumns+` FROM alerts WHERE id = ?`, id))
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return a, err
}

// ListAlerts returns a filtered, paginated alert list.
func (s *Store) ListAlerts(f model.AlertFilter) (*model.ListResult[model.Alert], error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.Limit <= 0 {
		f.Limit = 50
	}
	if f.Limit > 500 {
		f.Limit = 500
	}
	where := []string{"1=1"}
	var args []any
	if f.Status != "" && f.Status != "all" {
		where = append(where, "status = ?")
		args = append(args, f.Status)
	}
	if f.Severity != "" && f.Severity != "all" {
		where = append(where, "severity = ?")
		args = append(args, f.Severity)
	}
	if f.HostID > 0 {
		where = append(where, "host_id = ?")
		args = append(args, f.HostID)
	}
	if f.Search != "" {
		q := "%" + escapeLike(f.Search) + "%"
		where = append(where, `(title LIKE ? ESCAPE '\' OR message LIKE ? ESCAPE '\' OR host_ip LIKE ? ESCAPE '\' OR host_name LIKE ? ESCAPE '\')`)
		args = append(args, q, q, q, q)
	}
	if f.From != nil {
		where = append(where, "created_at >= ?")
		args = append(args, f.From.UnixMilli())
	}
	if f.To != nil {
		where = append(where, "created_at <= ?")
		args = append(args, f.To.UnixMilli())
	}
	whereSQL := strings.Join(where, " AND ")
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM alerts WHERE `+whereSQL, args...).Scan(&total); err != nil {
		return nil, err
	}
	args = append(args, f.Limit, (f.Page-1)*f.Limit)
	rows, err := s.db.Query(`SELECT `+alertColumns+` FROM alerts WHERE `+whereSQL+` ORDER BY id DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Alert{}
	for rows.Next() {
		a, err := scanAlert(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *a)
	}
	return &model.ListResult[model.Alert]{Items: out, Total: total, Page: f.Page, Limit: f.Limit}, rows.Err()
}

// SetAlertStatus acks or resolves an alert.
func (s *Store) SetAlertStatus(id int64, status string) error {
	now := nowMS()
	switch status {
	case model.AlertAcked:
		_, err := s.db.Exec(`UPDATE alerts SET status=?, acked_at=? WHERE id=? AND status=?`, status, now, id, model.AlertActive)
		return err
	case model.AlertResolved:
		_, err := s.db.Exec(`UPDATE alerts SET status=?, resolved_at=? WHERE id=? AND status IN (?,?)`,
			status, now, id, model.AlertActive, model.AlertAcked)
		return err
	}
	return fmt.Errorf("estado de alerta inválido: %s", status)
}

// ResolveAlertsByDedupe resolves active alerts matching a key (used on recovery).
func (s *Store) ResolveAlertsByDedupe(dedupe string) (int64, error) {
	res, err := s.db.Exec(`UPDATE alerts SET status=?, resolved_at=? WHERE dedupe_key=? AND status IN (?,?)`,
		model.AlertResolved, nowMS(), dedupe, model.AlertActive, model.AlertAcked)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// LastAlertTime returns the time of the latest alert for a dedupe key.
func (s *Store) LastAlertTime(dedupe string) (time.Time, bool) {
	var ts int64
	if err := s.db.QueryRow(`SELECT created_at FROM alerts WHERE dedupe_key=? ORDER BY id DESC LIMIT 1`, dedupe).Scan(&ts); err != nil {
		return time.Time{}, false
	}
	return fromMS(ts), true
}

// AlertStats returns current counters.
func (s *Store) AlertStats() (model.AlertStats, error) {
	var st model.AlertStats
	rows, err := s.db.Query(`SELECT status, COUNT(*) FROM alerts GROUP BY status`)
	if err != nil {
		return st, err
	}
	defer rows.Close()
	for rows.Next() {
		var status string
		var n int
		if err := rows.Scan(&status, &n); err != nil {
			return st, err
		}
		switch status {
		case model.AlertActive:
			st.Active = n
		case model.AlertAcked:
			st.Acked = n
		case model.AlertResolved:
			st.Resolved = n
		}
	}
	rows.Close()
	rows, err = s.db.Query(`SELECT severity, COUNT(*) FROM alerts WHERE status IN (?,?) GROUP BY severity`, model.AlertActive, model.AlertAcked)
	if err != nil {
		return st, err
	}
	defer rows.Close()
	for rows.Next() {
		var sev string
		var n int
		if err := rows.Scan(&sev, &n); err != nil {
			return st, err
		}
		switch sev {
		case model.SeverityCritical:
			st.Critical = n
		case model.SeverityWarning:
			st.Warning = n
		}
	}
	return st, rows.Err()
}

// PurgeAlerts deletes resolved alerts older than days.
func (s *Store) PurgeAlerts(days int) error {
	if days < 1 {
		days = 90
	}
	cut := time.Now().AddDate(0, 0, -days).UnixMilli()
	_, err := s.db.Exec(`DELETE FROM alerts WHERE status=? AND created_at < ?`, model.AlertResolved, cut)
	return err
}

// --- audit ---

// AddAudit appends an audit entry.
func (s *Store) AddAudit(e *model.AuditEntry) error {
	_, err := s.db.Exec(`INSERT INTO audit(ts, actor, action, target, detail, ip, user_agent)
		VALUES(?,?,?,?,?,?,?)`, nowMS(), e.Actor, e.Action, e.Target, jsonMarshal(e.Detail), e.IP, e.UserAgent)
	return err
}

// ListAudit returns filtered audit entries.
func (s *Store) ListAudit(f model.AuditFilter) (*model.ListResult[model.AuditEntry], error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.Limit <= 0 {
		f.Limit = 50
	}
	if f.Limit > 500 {
		f.Limit = 500
	}
	where := []string{"1=1"}
	var args []any
	if f.From != nil {
		where = append(where, "ts >= ?")
		args = append(args, f.From.UnixMilli())
	}
	if f.To != nil {
		where = append(where, "ts <= ?")
		args = append(args, f.To.UnixMilli())
	}
	if f.Actor != "" {
		where = append(where, "actor = ?")
		args = append(args, f.Actor)
	}
	if f.Action != "" {
		where = append(where, "action = ?")
		args = append(args, f.Action)
	}
	whereSQL := strings.Join(where, " AND ")
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM audit WHERE `+whereSQL, args...).Scan(&total); err != nil {
		return nil, err
	}
	args = append(args, f.Limit, (f.Page-1)*f.Limit)
	rows, err := s.db.Query(`SELECT id, ts, actor, action, target, detail, ip, user_agent FROM audit WHERE `+whereSQL+` ORDER BY id DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.AuditEntry{}
	for rows.Next() {
		var e model.AuditEntry
		var ts int64
		var detail string
		if err := rows.Scan(&e.ID, &ts, &e.Actor, &e.Action, &e.Target, &detail, &e.IP, &e.UserAgent); err != nil {
			return nil, err
		}
		e.TS = fromMS(ts)
		e.Detail = jsonObject(detail)
		out = append(out, e)
	}
	return &model.ListResult[model.AuditEntry]{Items: out, Total: total, Page: f.Page, Limit: f.Limit}, rows.Err()
}

// PurgeAudit removes old audit entries.
func (s *Store) PurgeAudit(days int) error {
	if days < 1 {
		days = 90
	}
	cut := time.Now().AddDate(0, 0, -days).UnixMilli()
	_, err := s.db.Exec(`DELETE FROM audit WHERE ts < ?`, cut)
	return err
}

// --- neighbors ---

// ReplaceNeighbors replaces all adjacency rows for a host.
func (s *Store) ReplaceNeighbors(hostID int64, neighbors []model.Neighbor) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.Exec(`DELETE FROM neighbors WHERE local_host_id = ?`, hostID); err != nil {
		return err
	}
	now := nowMS()
	for _, n := range neighbors {
		if _, err := tx.Exec(`INSERT INTO neighbors(local_host_id, protocol, remote_name, remote_ip, remote_mac,
			remote_port, local_port, remote_chassis, remote_sys_desc, confidence, updated_at)
			VALUES(?,?,?,?,?,?,?,?,?,?,?)`, hostID, n.Protocol, n.RemoteName, n.RemoteIP, n.RemoteMAC,
			n.RemotePort, n.LocalPort, n.RemoteChassis, n.RemoteSysDesc, n.Confidence, now); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ListNeighbors returns adjacency rows for one host.
func (s *Store) ListNeighbors(hostID int64) ([]model.Neighbor, error) {
	rows, err := s.db.Query(`SELECT protocol, remote_name, remote_ip, remote_mac, remote_port, local_port,
		remote_chassis, remote_sys_desc, confidence FROM neighbors WHERE local_host_id = ?`, hostID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Neighbor
	for rows.Next() {
		var n model.Neighbor
		if err := rows.Scan(&n.Protocol, &n.RemoteName, &n.RemoteIP, &n.RemoteMAC, &n.RemotePort,
			&n.LocalPort, &n.RemoteChassis, &n.RemoteSysDesc, &n.Confidence); err != nil {
			return nil, err
		}
		out = append(out, n)
	}
	return out, rows.Err()
}

// AllNeighbors returns adjacency rows keyed by local host ID.
func (s *Store) AllNeighbors() (map[int64][]model.Neighbor, error) {
	rows, err := s.db.Query(`SELECT local_host_id, protocol, remote_name, remote_ip, remote_mac, remote_port,
		local_port, remote_chassis, remote_sys_desc, confidence FROM neighbors`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int64][]model.Neighbor{}
	for rows.Next() {
		var hostID int64
		var n model.Neighbor
		if err := rows.Scan(&hostID, &n.Protocol, &n.RemoteName, &n.RemoteIP, &n.RemoteMAC, &n.RemotePort,
			&n.LocalPort, &n.RemoteChassis, &n.RemoteSysDesc, &n.Confidence); err != nil {
			return nil, err
		}
		out[hostID] = append(out[hostID], n)
	}
	return out, rows.Err()
}

// --- reports ---

// AddReport records a generated report.
func (s *Store) AddReport(r *model.Report) (*model.Report, error) {
	now := time.Now()
	if r.CreatedAt.IsZero() {
		r.CreatedAt = now
	}
	res, err := s.db.Exec(`INSERT INTO reports(name, format, kind, size, path, created_at) VALUES(?,?,?,?,?,?)`,
		r.Name, r.Format, r.Kind, r.Size, r.Path, r.CreatedAt.UnixMilli())
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetReport(id)
}

// ListReports returns generated reports, newest first.
func (s *Store) ListReports() ([]model.Report, error) {
	rows, err := s.db.Query(`SELECT id, name, format, kind, size, path, created_at FROM reports ORDER BY id DESC LIMIT 200`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Report{}
	for rows.Next() {
		var r model.Report
		var created int64
		if err := rows.Scan(&r.ID, &r.Name, &r.Format, &r.Kind, &r.Size, &r.Path, &created); err != nil {
			return nil, err
		}
		r.CreatedAt = fromMS(created)
		out = append(out, r)
	}
	return out, rows.Err()
}

// GetReport returns one report record.
func (s *Store) GetReport(id int64) (*model.Report, error) {
	var r model.Report
	var created int64
	err := s.db.QueryRow(`SELECT id, name, format, kind, size, path, created_at FROM reports WHERE id = ?`, id).
		Scan(&r.ID, &r.Name, &r.Format, &r.Kind, &r.Size, &r.Path, &created)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	r.CreatedAt = fromMS(created)
	return &r, err
}

// DeleteReport deletes a report record and its file.
func (s *Store) DeleteReport(id int64) error {
	r, err := s.GetReport(id)
	if err != nil {
		return err
	}
	_ = os.Remove(r.Path)
	_, err = s.db.Exec(`DELETE FROM reports WHERE id = ?`, id)
	return err
}

// --- dashboard ---

// Dashboard returns the KPI payload used by the main screen.
func (s *Store) Dashboard() (*model.DashboardStats, error) {
	st := &model.DashboardStats{ByType: map[string]int{}, ByStatus: map[string]int{}}
	status, err := s.StatusCounts()
	if err != nil {
		return nil, err
	}
	st.ByStatus = status
	for _, n := range status {
		st.HostsTotal += n
	}
	st.HostsUp = status[string(model.StatusUp)]
	st.HostsDown = status[string(model.StatusDown)]
	types, err := s.TypeCounts()
	if err != nil {
		return nil, err
	}
	st.ByType = types
	if st.HostsNew24h, err = s.CountHostsNewSince(time.Now().Add(-24 * time.Hour)); err != nil {
		return nil, err
	}
	if st.Scans24h, err = s.CountScansSince(time.Now().Add(-24 * time.Hour)); err != nil {
		return nil, err
	}
	var networks int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM networks`).Scan(&networks); err != nil {
		return nil, err
	}
	st.Networks = networks
	ast, err := s.AlertStats()
	if err != nil {
		return nil, err
	}
	st.AlertsActive = ast.Active + ast.Acked
	st.AlertsCritical = ast.Critical
	st.Availability, st.AvgRTT, err = s.FleetSummary(time.Now().Add(-24 * time.Hour))
	if err != nil {
		return nil, err
	}
	st.LastScan, err = s.LastScan()
	if err != nil {
		return nil, err
	}
	return st, nil
}
