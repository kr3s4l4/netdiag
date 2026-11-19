package store

import (
	"strings"
	"time"

	"github.com/kr3s4l4/netdiag/internal/model"
)

// SavePassiveObservations stores the metadata emitted by one capture window.
// Empty or unknown kinds are skipped; a window with no hits is not an error.
func (s *Store) SavePassiveObservations(obs []model.PassiveObservation) error {
	if len(obs) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`INSERT INTO passive_observations(site_id, scan_id, kind, interface, src_ip,
		dst_ip, src_mac, protocol, detail, packets, observed_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, o := range obs {
		o.Kind = strings.TrimSpace(o.Kind)
		if o.Kind == "" {
			continue
		}
		if o.SiteID <= 0 {
			o.SiteID = 1
		}
		if o.Count <= 0 {
			o.Count = 1
		}
		at := o.ObservedAt
		if at.IsZero() {
			at = time.Now()
		}
		if _, err := stmt.Exec(o.SiteID, o.ScanID, o.Kind, o.Interface, o.SrcIP, o.DstIP,
			o.SrcMAC, o.Protocol, truncateDetail(o.Detail), o.Count, at.UnixMilli()); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ListPassiveObservations returns retained metadata for one site, newest first.
func (s *Store) ListPassiveObservations(siteID int64, since time.Time, limit int) ([]model.PassiveObservation, error) {
	query := `SELECT id, site_id, scan_id, kind, interface, src_ip, dst_ip, src_mac, protocol,
		detail, packets, observed_at FROM passive_observations WHERE 1=1`
	args := []any{}
	if siteID > 0 {
		query += ` AND site_id = ?`
		args = append(args, siteID)
	}
	if !since.IsZero() {
		query += ` AND observed_at >= ?`
		args = append(args, since.UnixMilli())
	}
	query += ` ORDER BY observed_at DESC, id DESC`
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.PassiveObservation{}
	for rows.Next() {
		var o model.PassiveObservation
		var at int64
		if err := rows.Scan(&o.ID, &o.SiteID, &o.ScanID, &o.Kind, &o.Interface, &o.SrcIP,
			&o.DstIP, &o.SrcMAC, &o.Protocol, &o.Detail, &o.Count, &at); err != nil {
			return nil, err
		}
		o.ObservedAt = time.UnixMilli(at)
		out = append(out, o)
	}
	return out, rows.Err()
}

// PurgePassiveObservations removes metadata older than the retention window.
func (s *Store) PurgePassiveObservations(before time.Time) error {
	_, err := s.db.Exec(`DELETE FROM passive_observations WHERE observed_at < ?`, before.UnixMilli())
	return err
}

// truncateDetail caps a DNS name or DHCP label so a hostile frame cannot grow
// the database row without bound.
func truncateDetail(value string) string {
	value = strings.TrimSpace(value)
	const max = 253
	if len(value) > max {
		return value[:max]
	}
	return value
}
