package store

import (
	"bytes"
	"compress/gzip"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/kr3s4l4/netdiag/internal/model"
)

const (
	// MaxAuditSnapshotRaw bounds the JSON representation before compression.
	MaxAuditSnapshotRaw = 64 << 20
	// MaxAuditSnapshotStored bounds the gzip payload persisted in SQLite.
	MaxAuditSnapshotStored = 16 << 20
	// auditSnapshotsPerSite keeps baseline history bounded without silently
	// dropping the most recent captures.
	auditSnapshotsPerSite = 100
)

// CreateAuditSnapshot stores a gzip-compressed audit baseline and prunes the
// oldest captures beyond the per-site limit.
func (s *Store) CreateAuditSnapshot(snap *model.AuditSnapshot) error {
	if snap == nil || snap.Data == nil {
		return errors.New("snapshot de auditoría vacío")
	}
	if snap.SiteID <= 0 {
		snap.SiteID = 1
	}
	if snap.TS.IsZero() {
		snap.TS = time.Now()
	}
	if snap.Name == "" {
		snap.Name = "Auditoría " + snap.TS.Local().Format("2006-01-02 15:04")
	}
	raw, err := json.Marshal(snap.Data)
	if err != nil {
		return err
	}
	if len(raw) > MaxAuditSnapshotRaw {
		return fmt.Errorf("snapshot demasiado grande antes de comprimir (%d bytes)", len(raw))
	}
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write(raw); err != nil {
		return err
	}
	if err := zw.Close(); err != nil {
		return err
	}
	if buf.Len() > MaxAuditSnapshotStored {
		return fmt.Errorf("snapshot comprimido demasiado grande (%d bytes)", buf.Len())
	}
	snap.RawSize, snap.Size = len(raw), buf.Len()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	res, err := tx.Exec(`INSERT INTO audit_snapshots(site_id, name, notes, ts, raw_size, size, data)
		VALUES(?,?,?,?,?,?,?)`, snap.SiteID, snap.Name, snap.Notes, snap.TS.UnixMilli(),
		snap.RawSize, snap.Size, buf.Bytes())
	if err != nil {
		return err
	}
	snap.ID, err = res.LastInsertId()
	if err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM audit_snapshots WHERE site_id = ? AND id NOT IN
		(SELECT id FROM audit_snapshots WHERE site_id = ? ORDER BY ts DESC, id DESC LIMIT ?)`,
		snap.SiteID, snap.SiteID, auditSnapshotsPerSite); err != nil {
		return err
	}
	return tx.Commit()
}

// ListAuditSnapshots returns only metadata, never the compressed payload.
func (s *Store) ListAuditSnapshots(siteID int64, limit int) ([]model.AuditSnapshot, error) {
	if limit < 1 || limit > 200 {
		limit = 50
	}
	query := `SELECT id, site_id, name, notes, ts, raw_size, size FROM audit_snapshots`
	args := []any{}
	if siteID > 0 {
		query += ` WHERE site_id = ?`
		args = append(args, siteID)
	}
	query += ` ORDER BY ts DESC, id DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []model.AuditSnapshot{}
	for rows.Next() {
		var snap model.AuditSnapshot
		var ts int64
		if err := rows.Scan(&snap.ID, &snap.SiteID, &snap.Name, &snap.Notes, &ts, &snap.RawSize, &snap.Size); err != nil {
			return nil, err
		}
		snap.TS = time.UnixMilli(ts)
		items = append(items, snap)
	}
	return items, rows.Err()
}

// GetAuditSnapshot loads one baseline. When withData is false the payload is
// left nil, which keeps list/detail responses lightweight.
func (s *Store) GetAuditSnapshot(id int64, withData bool) (*model.AuditSnapshot, error) {
	var snap model.AuditSnapshot
	var ts int64
	var data []byte
	err := s.db.QueryRow(`SELECT id, site_id, name, notes, ts, raw_size, size, data
		FROM audit_snapshots WHERE id = ?`, id).
		Scan(&snap.ID, &snap.SiteID, &snap.Name, &snap.Notes, &ts, &snap.RawSize, &snap.Size, &data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	snap.TS = time.UnixMilli(ts)
	if withData {
		zr, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("snapshot %d: %w", id, err)
		}
		plain, err := io.ReadAll(zr)
		closeErr := zr.Close()
		if err != nil {
			return nil, fmt.Errorf("snapshot %d: %w", id, err)
		}
		if closeErr != nil {
			return nil, closeErr
		}
		var payload model.AuditSnapshotData
		if err := json.Unmarshal(plain, &payload); err != nil {
			return nil, fmt.Errorf("snapshot %d: %w", id, err)
		}
		snap.Data = &payload
	}
	return &snap, nil
}

// DeleteAuditSnapshot removes one baseline.
func (s *Store) DeleteAuditSnapshot(id int64) error {
	_, err := s.db.Exec(`DELETE FROM audit_snapshots WHERE id = ?`, id)
	return err
}
