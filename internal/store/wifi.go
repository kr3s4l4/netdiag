package store

import (
	"time"

	"github.com/kr3s4l4/netdiag/internal/model"
)

// UpsertWifiAP inserts or refreshes one AP observation, preserving first_seen.
func (s *Store) UpsertWifiAP(ap *model.WifiAP) error {
	if ap.SiteID == 0 {
		ap.SiteID = 1
	}
	now := nowMS()
	rogue := boolInt(ap.Rogue)
	_, err := s.db.Exec(`INSERT INTO wifi_aps
		(site_id, bssid, ssid, channel, band, encryption, signal_dbm, vendor,
		 beacons, clients, rogue, rogue_info, rogue_confidence, first_seen, last_seen)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(site_id, bssid) DO UPDATE SET
			ssid=excluded.ssid, channel=excluded.channel, band=excluded.band,
			encryption=excluded.encryption, signal_dbm=excluded.signal_dbm,
			vendor=excluded.vendor, beacons=wifi_aps.beacons+excluded.beacons,
			clients=excluded.clients, rogue=excluded.rogue,
			rogue_info=excluded.rogue_info, rogue_confidence=excluded.rogue_confidence,
			last_seen=excluded.last_seen`,
		ap.SiteID, ap.BSSID, ap.SSID, ap.Channel, ap.Band, ap.Encryption,
		ap.SignalDBM, ap.Vendor, ap.Beacons, ap.Clients, rogue, ap.RogueInfo,
		ap.RogueConfidence, now, now)
	if err != nil {
		return err
	}
	var firstSeen int64
	if err := s.db.QueryRow(`SELECT id, first_seen FROM wifi_aps WHERE site_id = ? AND bssid = ?`,
		ap.SiteID, ap.BSSID).Scan(&ap.ID, &firstSeen); err != nil {
		return err
	}
	ap.FirstSeen = time.UnixMilli(firstSeen)
	return nil
}

// ListWifiAPs returns APs for a site (0 = all sites), strongest signal first.
func (s *Store) ListWifiAPs(siteID int64, limit int) ([]model.WifiAP, error) {
	if limit <= 0 || limit > 5000 {
		limit = 5000
	}
	query := `SELECT id, site_id, bssid, ssid, channel, band, encryption, signal_dbm,
		vendor, beacons, clients, rogue, rogue_info, rogue_confidence, first_seen, last_seen
		FROM wifi_aps`
	args := []any{}
	if siteID > 0 {
		query += ` WHERE site_id = ?`
		args = append(args, siteID)
	}
	query += ` ORDER BY signal_dbm DESC, last_seen DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.WifiAP{}
	for rows.Next() {
		var ap model.WifiAP
		var rogue int
		var firstSeen, lastSeen int64
		if err := rows.Scan(&ap.ID, &ap.SiteID, &ap.BSSID, &ap.SSID, &ap.Channel, &ap.Band,
			&ap.Encryption, &ap.SignalDBM, &ap.Vendor, &ap.Beacons, &ap.Clients, &rogue,
			&ap.RogueInfo, &ap.RogueConfidence, &firstSeen, &lastSeen); err != nil {
			return nil, err
		}
		ap.Rogue = rogue != 0
		ap.FirstSeen = time.UnixMilli(firstSeen)
		ap.LastSeen = time.UnixMilli(lastSeen)
		out = append(out, ap)
	}
	return out, rows.Err()
}

// UpsertWifiClient inserts or refreshes one station observation.
func (s *Store) UpsertWifiClient(c *model.WifiClient) error {
	if c.SiteID == 0 {
		c.SiteID = 1
	}
	now := nowMS()
	_, err := s.db.Exec(`INSERT INTO wifi_clients(site_id, mac, ap_bssid, signal_dbm, last_seen)
		VALUES(?,?,?,?,?)
		ON CONFLICT(site_id, mac) DO UPDATE SET
			ap_bssid=excluded.ap_bssid, signal_dbm=excluded.signal_dbm,
			last_seen=excluded.last_seen`,
		c.SiteID, c.MAC, c.APBSSID, c.SignalDBM, now)
	if err != nil {
		return err
	}
	var id int64
	if err := s.db.QueryRow(`SELECT id FROM wifi_clients WHERE site_id = ? AND mac = ?`,
		c.SiteID, c.MAC).Scan(&id); err != nil {
		return err
	}
	c.ID = id
	return nil
}

// ListWifiClients returns stations for a site (0 = all sites).
func (s *Store) ListWifiClients(siteID int64, limit int) ([]model.WifiClient, error) {
	if limit <= 0 || limit > 10000 {
		limit = 10000
	}
	query := `SELECT id, site_id, mac, ap_bssid, signal_dbm, last_seen FROM wifi_clients`
	args := []any{}
	if siteID > 0 {
		query += ` WHERE site_id = ?`
		args = append(args, siteID)
	}
	query += ` ORDER BY last_seen DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.WifiClient{}
	for rows.Next() {
		var c model.WifiClient
		var lastSeen int64
		if err := rows.Scan(&c.ID, &c.SiteID, &c.MAC, &c.APBSSID, &c.SignalDBM, &lastSeen); err != nil {
			return nil, err
		}
		c.LastSeen = time.UnixMilli(lastSeen)
		out = append(out, c)
	}
	return out, rows.Err()
}

// ClearWifiObservations removes all APs and clients for a fresh scan.
func (s *Store) ClearWifiObservations(siteID int64) error {
	if siteID <= 0 {
		siteID = 1
	}
	if _, err := s.db.Exec(`DELETE FROM wifi_clients WHERE site_id = ?`, siteID); err != nil {
		return err
	}
	_, err := s.db.Exec(`DELETE FROM wifi_aps WHERE site_id = ?`, siteID)
	return err
}
