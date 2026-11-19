package store

import (
	"fmt"
	"strings"
	"time"

	"github.com/kr3s4l4/netdiag/internal/model"
)

// SaveDHCPObservations stores the DHCP replies collected by one scan. Empty
// addresses are skipped; repeated sightings are kept as raw evidence.
func (s *Store) SaveDHCPObservations(obs []model.DHCPObservation) error {
	if len(obs) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`INSERT INTO dhcp_observations(site_id, ip, server_id, mac, message_type,
		gateway, interface, lease_seconds, subnet, routers, dns, scan_id, observed_at)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, o := range obs {
		o.IP = strings.TrimSpace(o.IP)
		if o.IP == "" {
			continue
		}
		if o.ServerID == "" {
			o.ServerID = o.IP
		}
		if o.SiteID <= 0 {
			o.SiteID = 1
		}
		if o.MessageType == "" {
			o.MessageType = "unknown"
		}
		at := o.ObservedAt
		if at.IsZero() {
			at = time.Now()
		}
		if _, err := stmt.Exec(o.SiteID, o.IP, o.ServerID, o.MAC, o.MessageType, o.Gateway,
			o.Interface, o.LeaseSeconds, o.Subnet, strings.Join(o.Routers, ","),
			strings.Join(o.DNS, ","), o.ScanID, at.UnixMilli()); err != nil {
			return fmt.Errorf("guardar observación DHCP: %w", err)
		}
	}
	return tx.Commit()
}

// ListDHCPObservations returns DHCP replies for a site, oldest first.
// siteID 0 means every site; "since" may be zero to keep all rows.
func (s *Store) ListDHCPObservations(siteID int64, since time.Time, limit int) ([]model.DHCPObservation, error) {
	query := `SELECT id, site_id, ip, server_id, mac, message_type, gateway, interface,
		lease_seconds, subnet, routers, dns, scan_id, observed_at FROM dhcp_observations WHERE 1=1`
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
	out := []model.DHCPObservation{}
	for rows.Next() {
		var o model.DHCPObservation
		var at int64
		var routers, dns string
		if err := rows.Scan(&o.ID, &o.SiteID, &o.IP, &o.ServerID, &o.MAC, &o.MessageType,
			&o.Gateway, &o.Interface, &o.LeaseSeconds, &o.Subnet, &routers, &dns,
			&o.ScanID, &at); err != nil {
			return nil, err
		}
		o.Routers = splitList(routers)
		o.DNS = splitList(dns)
		o.ObservedAt = time.UnixMilli(at)
		out = append(out, o)
	}
	return out, rows.Err()
}

// PurgeDHCPObservations removes DHCP history older than the retention window.
func (s *Store) PurgeDHCPObservations(before time.Time) error {
	_, err := s.db.Exec(`DELETE FROM dhcp_observations WHERE observed_at < ?`, before.UnixMilli())
	return err
}

func splitList(value string) []string {
	out := []string{}
	for _, part := range strings.Split(value, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}
