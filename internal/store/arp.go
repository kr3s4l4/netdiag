package store

import (
	"fmt"
	"time"

	"github.com/kr3s4l4/netdiag/internal/model"
)

// SaveARPObservations stores the IP→MAC bindings discovered in one scan.
// Invalid or empty MACs are skipped by the caller; duplicates are allowed on
// purpose because repeated sightings strengthen the history.
func (s *Store) SaveARPObservations(obs []model.ARPObservation) error {
	if len(obs) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`INSERT INTO arp_observations(site_id, ip, mac, hostname, is_gateway, scan_id, observed_at)
		VALUES(?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, o := range obs {
		if o.IP == "" || o.MAC == "" {
			continue
		}
		if o.SiteID <= 0 {
			o.SiteID = 1
		}
		at := o.ObservedAt
		if at.IsZero() {
			at = time.Now()
		}
		if _, err := stmt.Exec(o.SiteID, o.IP, o.MAC, o.Hostname, boolInt(o.IsGateway), o.ScanID, at.UnixMilli()); err != nil {
			return fmt.Errorf("guardar observación ARP: %w", err)
		}
	}
	return tx.Commit()
}

// ListARPObservations returns the IP→MAC history for a site, oldest first.
// siteID 0 means every site. "since" may be zero to keep all rows.
func (s *Store) ListARPObservations(siteID int64, since time.Time, limit int) ([]model.ARPObservation, error) {
	query := `SELECT id, site_id, ip, mac, hostname, is_gateway, scan_id, observed_at FROM arp_observations WHERE 1=1`
	args := []any{}
	if siteID > 0 {
		query += ` AND site_id = ?`
		args = append(args, siteID)
	}
	if !since.IsZero() {
		query += ` AND observed_at >= ?`
		args = append(args, since.UnixMilli())
	}
	query += ` ORDER BY observed_at ASC, id ASC`
	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.ARPObservation{}
	for rows.Next() {
		var o model.ARPObservation
		var at int64
		var gateway int
		if err := rows.Scan(&o.ID, &o.SiteID, &o.IP, &o.MAC, &o.Hostname, &gateway, &o.ScanID, &at); err != nil {
			return nil, err
		}
		o.IsGateway = gateway != 0
		o.ObservedAt = time.UnixMilli(at)
		out = append(out, o)
	}
	return out, rows.Err()
}

// PurgeARPObservations removes history older than "before", matching the
// configured retention window.
func (s *Store) PurgeARPObservations(before time.Time) error {
	_, err := s.db.Exec(`DELETE FROM arp_observations WHERE observed_at < ?`, before.UnixMilli())
	return err
}
