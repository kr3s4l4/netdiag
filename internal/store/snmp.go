package store

import (
	"fmt"
	"strings"
	"time"

	"github.com/kr3s4l4/netdiag/internal/model"
)

// SaveSNMPDeep persists the F46 evidence of one polled device. The community
// is already masked by the collector; the raw credential never reaches SQLite.
func (s *Store) SaveSNMPDeep(siteID, scanID int64, hostIP, community string, at time.Time, deep model.SNMPDeep) error {
	if siteID <= 0 {
		siteID = 1
	}
	if at.IsZero() {
		at = time.Now()
	}
	hostIP = strings.TrimSpace(hostIP)
	ms := at.UnixMilli()

	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if len(deep.IPAddresses) > 0 {
		stmt, err := tx.Prepare(`INSERT INTO snmp_ip_addresses(site_id, host_ip, community, ip, if_index, mask,
			scan_id, observed_at) VALUES(?,?,?,?,?,?,?,?)`)
		if err != nil {
			return err
		}
		for _, row := range deep.IPAddresses {
			if strings.TrimSpace(row.IP) == "" {
				continue
			}
			if _, err := stmt.Exec(siteID, hostIP, community, row.IP, row.IfIndex, row.Mask, scanID, ms); err != nil {
				stmt.Close()
				return fmt.Errorf("guardar dirección SNMP: %w", err)
			}
		}
		stmt.Close()
	}

	if len(deep.ARP) > 0 {
		stmt, err := tx.Prepare(`INSERT INTO snmp_arp_rows(site_id, host_ip, community, ip, mac, if_index,
			row_type, table_name, scan_id, observed_at) VALUES(?,?,?,?,?,?,?,?,?,?)`)
		if err != nil {
			return err
		}
		for _, row := range deep.ARP {
			if strings.TrimSpace(row.IP) == "" || strings.TrimSpace(row.MAC) == "" {
				continue
			}
			if _, err := stmt.Exec(siteID, hostIP, community, row.IP, row.MAC, row.IfIndex,
				row.Type, row.Table, scanID, ms); err != nil {
				stmt.Close()
				return fmt.Errorf("guardar ARP SNMP: %w", err)
			}
		}
		stmt.Close()
	}

	if len(deep.VLANs) > 0 {
		stmt, err := tx.Prepare(`INSERT INTO snmp_vlans(site_id, host_ip, community, vlan_id, name,
			scan_id, observed_at) VALUES(?,?,?,?,?,?,?)`)
		if err != nil {
			return err
		}
		for _, row := range deep.VLANs {
			if row.VLAN <= 0 {
				continue
			}
			if _, err := stmt.Exec(siteID, hostIP, community, row.VLAN, row.Name, scanID, ms); err != nil {
				stmt.Close()
				return fmt.Errorf("guardar VLAN SNMP: %w", err)
			}
		}
		stmt.Close()
	}

	if len(deep.FDB) > 0 {
		stmt, err := tx.Prepare(`INSERT INTO snmp_fdb_rows(site_id, host_ip, community, mac, port, if_index,
			vlan, status, table_name, scan_id, observed_at) VALUES(?,?,?,?,?,?,?,?,?,?,?)`)
		if err != nil {
			return err
		}
		for _, row := range deep.FDB {
			if strings.TrimSpace(row.MAC) == "" || row.Port <= 0 {
				continue
			}
			if _, err := stmt.Exec(siteID, hostIP, community, row.MAC, row.Port, row.IfIndex,
				row.VLAN, row.Status, row.Table, scanID, ms); err != nil {
				stmt.Close()
				return fmt.Errorf("guardar FDB SNMP: %w", err)
			}
		}
		stmt.Close()
	}

	return tx.Commit()
}

// ListSNMPDeep returns the retained deep evidence for one site, newest first.
// siteID 0 means every site; "since" may be zero to keep all rows.
func (s *Store) ListSNMPDeep(siteID int64, since time.Time, limit int) (*model.SNMPDeepSnapshot, error) {
	out := &model.SNMPDeepSnapshot{
		IPAddresses: []model.SNMPIPAddress{},
		ARP:         []model.SNMPARPRow{},
		VLANs:       []model.SNMPVLAN{},
		FDB:         []model.SNMPFDBRow{},
	}
	args := []any{}
	filter := ` WHERE 1=1`
	if siteID > 0 {
		filter += ` AND site_id = ?`
		args = append(args, siteID)
	}
	if !since.IsZero() {
		filter += ` AND observed_at >= ?`
		args = append(args, since.UnixMilli())
	}
	tail := ` ORDER BY observed_at DESC, id DESC`
	if limit > 0 {
		tail += ` LIMIT ?`
		args = append(args, limit)
	}

	rows, err := s.db.Query(`SELECT id, site_id, host_ip, community, ip, if_index, mask, scan_id, observed_at
		FROM snmp_ip_addresses`+filter+tail, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var row model.SNMPIPAddress
		var at int64
		if err := rows.Scan(&row.ID, &row.SiteID, &row.HostIP, &row.Community, &row.IP,
			&row.IfIndex, &row.Mask, &row.ScanID, &at); err != nil {
			rows.Close()
			return nil, err
		}
		row.ObservedAt = time.UnixMilli(at)
		out.IPAddresses = append(out.IPAddresses, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	rows, err = s.db.Query(`SELECT id, site_id, host_ip, community, ip, mac, if_index, row_type,
		table_name, scan_id, observed_at FROM snmp_arp_rows`+filter+tail, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var row model.SNMPARPRow
		var at int64
		if err := rows.Scan(&row.ID, &row.SiteID, &row.HostIP, &row.Community, &row.IP, &row.MAC,
			&row.IfIndex, &row.Type, &row.Table, &row.ScanID, &at); err != nil {
			rows.Close()
			return nil, err
		}
		row.ObservedAt = time.UnixMilli(at)
		out.ARP = append(out.ARP, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	rows, err = s.db.Query(`SELECT id, site_id, host_ip, community, vlan_id, name, scan_id, observed_at
		FROM snmp_vlans`+filter+tail, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var row model.SNMPVLAN
		var at int64
		if err := rows.Scan(&row.ID, &row.SiteID, &row.HostIP, &row.Community, &row.VLAN,
			&row.Name, &row.ScanID, &at); err != nil {
			rows.Close()
			return nil, err
		}
		row.ObservedAt = time.UnixMilli(at)
		out.VLANs = append(out.VLANs, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()

	rows, err = s.db.Query(`SELECT id, site_id, host_ip, community, mac, port, if_index, vlan,
		status, table_name, scan_id, observed_at FROM snmp_fdb_rows`+filter+tail, args...)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var row model.SNMPFDBRow
		var at int64
		if err := rows.Scan(&row.ID, &row.SiteID, &row.HostIP, &row.Community, &row.MAC,
			&row.Port, &row.IfIndex, &row.VLAN, &row.Status, &row.Table, &row.ScanID, &at); err != nil {
			rows.Close()
			return nil, err
		}
		row.ObservedAt = time.UnixMilli(at)
		out.FDB = append(out.FDB, row)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	return out, nil
}

// PurgeSNMPDeep removes evidence older than the retention window.
func (s *Store) PurgeSNMPDeep(before time.Time) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, table := range []string{"snmp_ip_addresses", "snmp_arp_rows", "snmp_vlans", "snmp_fdb_rows"} {
		if _, err := tx.Exec(`DELETE FROM `+table+` WHERE observed_at < ?`, before.UnixMilli()); err != nil {
			return err
		}
	}
	return tx.Commit()
}
