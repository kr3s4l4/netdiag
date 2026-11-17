package store

import (
	"database/sql"
	"encoding/json"
	"strings"
	"time"

	"github.com/kr3s4l4/netdiag/internal/model"
)

const networkColumns = `id, name, cidr, iface, enabled, is_local, created_at, last_scan_at`

type rowScannerNetwork interface {
	Scan(dest ...any) error
}

func scanNetwork(r rowScannerNetwork, withCounts bool) (*model.Network, error) {
	var (
		n         model.Network
		enabled   int
		local     int
		created   int64
		lastScan  sql.NullInt64
		hostCount int
		upCount   int
	)
	dest := []any{&n.ID, &n.Name, &n.CIDR, &n.Interface, &enabled, &local, &created, &lastScan}
	if withCounts {
		dest = append(dest, &hostCount, &upCount)
	}
	if err := r.Scan(dest...); err != nil {
		return nil, err
	}
	n.Enabled = enabled != 0
	n.IsLocal = local != 0
	n.CreatedAt = fromMS(created)
	n.LastScanAt = fromMSNull(lastScan)
	n.HostCount = hostCount
	n.UpCount = upCount
	return &n, nil
}

// ListNetworks returns all configured networks with host counts.
func (s *Store) ListNetworks() ([]model.Network, error) {
	rows, err := s.db.Query(`SELECT ` + networkColumns + `,
		(SELECT COUNT(*) FROM hosts h WHERE h.network_id = networks.id),
		(SELECT COUNT(*) FROM hosts h WHERE h.network_id = networks.id AND h.status = 'up')
		FROM networks ORDER BY is_local DESC, id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Network{}
	for rows.Next() {
		n, err := scanNetwork(rows, true)
		if err != nil {
			return nil, err
		}
		out = append(out, *n)
	}
	return out, rows.Err()
}

// GetNetwork returns one network.
func (s *Store) GetNetwork(id int64) (*model.Network, error) {
	n, err := scanNetwork(s.db.QueryRow(`SELECT `+networkColumns+` FROM networks WHERE id = ?`, id), false)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return n, err
}

// CreateNetwork inserts a network.
func (s *Store) CreateNetwork(n *model.Network) (*model.Network, error) {
	if n.CreatedAt.IsZero() {
		n.CreatedAt = time.Now()
	}
	res, err := s.db.Exec(`INSERT INTO networks(name, cidr, iface, enabled, is_local, created_at, last_scan_at)
		VALUES(?,?,?,?,?,?,?)`,
		n.Name, n.CIDR, n.Interface, boolInt(n.Enabled), boolInt(n.IsLocal), n.CreatedAt.UnixMilli(), timePtrMS(n.LastScanAt))
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetNetwork(id)
}

// UpdateNetwork updates editable network fields.
func (s *Store) UpdateNetwork(n *model.Network) error {
	_, err := s.db.Exec(`UPDATE networks SET name=?, cidr=?, iface=?, enabled=?, is_local=? WHERE id=?`,
		n.Name, n.CIDR, n.Interface, boolInt(n.Enabled), boolInt(n.IsLocal), n.ID)
	return err
}

// DeleteNetwork removes a network definition (hosts are kept).
func (s *Store) DeleteNetwork(id int64) error {
	_, err := s.db.Exec(`DELETE FROM networks WHERE id = ?`, id)
	return err
}

// FindNetworkByCIDR returns a network by exact CIDR.
func (s *Store) FindNetworkByCIDR(cidr string) (*model.Network, error) {
	n, err := scanNetwork(s.db.QueryRow(`SELECT `+networkColumns+` FROM networks WHERE cidr = ?`, cidr), false)
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return n, err
}

// TouchNetworkScan records the last scan time.
func (s *Store) TouchNetworkScan(id int64, t time.Time) error {
	_, err := s.db.Exec(`UPDATE networks SET last_scan_at = ? WHERE id = ?`, t.UnixMilli(), id)
	return err
}

// EnsureNetwork inserts a network if its CIDR is new.
func (s *Store) EnsureNetwork(n *model.Network) (*model.Network, bool, error) {
	if existing, err := s.FindNetworkByCIDR(n.CIDR); err == nil {
		return existing, false, nil
	} else if err != ErrNotFound {
		return nil, false, err
	}
	created, err := s.CreateNetwork(n)
	return created, err == nil, err
}

// --- scans ---

const scanColumns = `id, network_id, cidr, profile, status, stage, progress, total, scanned,
	hosts_found, trigger, options, error, started_at, finished_at`

func scanScan(r rowScanner) (*model.Scan, error) {
	var (
		sc       model.Scan
		opts     string
		started  sql.NullInt64
		finished sql.NullInt64
	)
	if err := r.Scan(&sc.ID, &sc.NetworkID, &sc.CIDR, &sc.Profile, &sc.Status, &sc.Stage, &sc.Progress,
		&sc.Total, &sc.Scanned, &sc.HostsFound, &sc.Trigger, &opts, &sc.Error, &started, &finished); err != nil {
		return nil, err
	}
	_ = json.Unmarshal([]byte(opts), &sc.Options)
	sc.StartedAt = fromMSNull(started)
	sc.FinishedAt = fromMSNull(finished)
	return &sc, nil
}

// CreateScan inserts a queued scan.
func (s *Store) CreateScan(sc *model.Scan) (*model.Scan, error) {
	res, err := s.db.Exec(`INSERT INTO scans(network_id, cidr, profile, status, stage, progress, total,
		scanned, hosts_found, trigger, options, error, started_at, finished_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,'',NULL,NULL)`,
		sc.NetworkID, sc.CIDR, sc.Profile, model.ScanQueued, "en cola", 0, sc.Total, 0, 0,
		sc.Trigger, jsonMarshal(sc.Options))
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetScan(id)
}

// StartScan marks a scan as running.
func (s *Store) StartScan(id int64, total int) error {
	_, err := s.db.Exec(`UPDATE scans SET status=?, stage=?, started_at=?, total=? WHERE id=?`,
		model.ScanRunning, "descubriendo", nowMS(), total, id)
	return err
}

// UpdateScanProgress updates counters.
func (s *Store) UpdateScanProgress(id int64, stage string, scanned, total, found int, progress float64) error {
	_, err := s.db.Exec(`UPDATE scans SET stage=?, scanned=?, total=?, hosts_found=?, progress=? WHERE id=?`,
		stage, scanned, total, found, progress, id)
	return err
}

// FinishScan marks a scan terminal and stores an optional error.
func (s *Store) FinishScan(id int64, status string, found int, errMsg string) error {
	if status == "" {
		status = model.ScanDone
	}
	_, err := s.db.Exec(`UPDATE scans SET status=?, stage=?, hosts_found=?, progress=1, finished_at=?, error=? WHERE id=?`,
		status, status, found, nowMS(), errMsg, id)
	return err
}

// GetScan returns one scan.
func (s *Store) GetScan(id int64) (*model.Scan, error) {
	sc, err := scanScan(s.db.QueryRow(`SELECT `+scanColumns+` FROM scans WHERE id = ?`, id))
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return sc, err
}

// ListScans returns recent scans, newest first.
func (s *Store) ListScans(f model.ScanFilter) (*model.ListResult[model.Scan], error) {
	if f.Page < 1 {
		f.Page = 1
	}
	if f.Limit <= 0 {
		f.Limit = 50
	}
	where := "1=1"
	var args []any
	if f.Status != "" && f.Status != "all" {
		where += " AND status = ?"
		args = append(args, f.Status)
	}
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM scans WHERE `+where, args...).Scan(&total); err != nil {
		return nil, err
	}
	args = append(args, f.Limit, (f.Page-1)*f.Limit)
	rows, err := s.db.Query(`SELECT `+scanColumns+` FROM scans WHERE `+where+` ORDER BY id DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Scan{}
	for rows.Next() {
		sc, err := scanScan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *sc)
	}
	return &model.ListResult[model.Scan]{Items: out, Total: total, Page: f.Page, Limit: f.Limit}, rows.Err()
}

// LastScan returns the most recent finished scan, if any.
func (s *Store) LastScan() (*model.Scan, error) {
	sc, err := scanScan(s.db.QueryRow(`SELECT ` + scanColumns + ` FROM scans ORDER BY id DESC LIMIT 1`))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return sc, err
}

// MarkInterruptedScans closes scans left running by a crash or restart.
func (s *Store) MarkInterruptedScans() error {
	_, err := s.db.Exec(`UPDATE scans SET status=?, stage=?, finished_at=?, error=?
		WHERE status IN (?,?)`, model.ScanInterrupted, "interrumpido", nowMS(),
		"El proceso terminó antes de completar el escaneo", model.ScanRunning, model.ScanQueued)
	return err
}

// CountScansSince counts scans started after t.
func (s *Store) CountScansSince(t time.Time) (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM scans WHERE COALESCE(started_at, 0) >= ?`, t.UnixMilli()).Scan(&n)
	return n, err
}

// PurgeScans keeps the newest keep scans and deletes older finished ones.
func (s *Store) PurgeScans(keep int) error {
	if keep <= 0 {
		keep = 500
	}
	_, err := s.db.Exec(`DELETE FROM scans WHERE status IN (?,?,?,?) AND id NOT IN (
		SELECT id FROM scans ORDER BY id DESC LIMIT ?)`,
		model.ScanDone, model.ScanCanceled, model.ScanError, model.ScanInterrupted, keep)
	return err
}

// ActiveScans returns queued or running scans.
func (s *Store) ActiveScans() ([]model.Scan, error) {
	rows, err := s.db.Query(`SELECT `+scanColumns+` FROM scans WHERE status IN (?,?) ORDER BY id`, model.ScanQueued, model.ScanRunning)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Scan
	for rows.Next() {
		sc, err := scanScan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *sc)
	}
	return out, rows.Err()
}

// scanStatusList is used by callers that need a SQL list of terminal states.
func scanStatusList() string {
	return strings.Join([]string{"'" + model.ScanDone + "'", "'" + model.ScanCanceled + "'", "'" + model.ScanError + "'", "'" + model.ScanInterrupted + "'"}, ",")
}
