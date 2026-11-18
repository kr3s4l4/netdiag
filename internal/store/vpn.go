package store

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/kr3s4l4/netdiag/internal/model"
)

// UpsertVPNTunnel inserts or refreshes one tunnel observation. Byte counters
// are cumulative and replace the previous snapshot; first_seen is preserved.
func (s *Store) UpsertVPNTunnel(t *model.VPNTunnel) error {
	if t.Identity == "" {
		t.Identity = t.Type + "|" + t.LocalIP + ":" + strconv.Itoa(t.LocalPort) + "|" + t.RemoteIP + ":" + strconv.Itoa(t.RemotePort)
	}
	evidence, err := json.Marshal(t.Evidence)
	if err != nil {
		return err
	}
	now := nowMS()
	site := t.SiteID
	if site == 0 {
		site = 1
	}
	_, err = s.db.Exec(`INSERT INTO vpn_tunnels
		(site_id, identity, type, local_ip, remote_ip, local_port, remote_port,
		 bytes_sent, bytes_recv, state, evidence, confidence, first_seen, last_seen)
		VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?,?)
		ON CONFLICT(identity) DO UPDATE SET
			site_id=excluded.site_id, type=excluded.type,
			local_ip=excluded.local_ip, remote_ip=excluded.remote_ip,
			local_port=excluded.local_port, remote_port=excluded.remote_port,
			bytes_sent=excluded.bytes_sent, bytes_recv=excluded.bytes_recv,
			state=excluded.state, evidence=excluded.evidence,
			confidence=excluded.confidence, last_seen=excluded.last_seen`,
		site, t.Identity, t.Type, t.LocalIP, t.RemoteIP, t.LocalPort, t.RemotePort,
		t.BytesSent, t.BytesRecv, t.State, string(evidence), t.Confidence, now, now)
	if err != nil {
		return err
	}
	var firstSeen int64
	if err := s.db.QueryRow(`SELECT id, first_seen FROM vpn_tunnels WHERE identity = ?`, t.Identity).
		Scan(&t.ID, &firstSeen); err != nil {
		return err
	}
	t.FirstSeen = time.UnixMilli(firstSeen)
	return nil
}

// ListVPNTunnels returns tunnels for a site (0 = all sites), newest first.
func (s *Store) ListVPNTunnels(siteID int64) ([]model.VPNTunnel, error) {
	query := `SELECT id, site_id, identity, type, local_ip, remote_ip, local_port,
		remote_port, bytes_sent, bytes_recv, state, evidence, confidence, first_seen, last_seen
		FROM vpn_tunnels`
	args := []any{}
	if siteID > 0 {
		query += ` WHERE site_id = ?`
		args = append(args, siteID)
	}
	query += ` ORDER BY last_seen DESC, confidence DESC`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.VPNTunnel{}
	for rows.Next() {
		var t model.VPNTunnel
		var evidence string
		var firstSeen, lastSeen int64
		if err := rows.Scan(&t.ID, &t.SiteID, &t.Identity, &t.Type, &t.LocalIP, &t.RemoteIP,
			&t.LocalPort, &t.RemotePort, &t.BytesSent, &t.BytesRecv, &t.State, &evidence,
			&t.Confidence, &firstSeen, &lastSeen); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(evidence), &t.Evidence)
		if t.Evidence == nil {
			t.Evidence = []string{}
		}
		t.FirstSeen = time.UnixMilli(firstSeen)
		t.LastSeen = time.UnixMilli(lastSeen)
		out = append(out, t)
	}
	return out, rows.Err()
}

// MarkStaleVPNTunnels flags tunnels that have not been observed since before.
func (s *Store) MarkStaleVPNTunnels(before time.Time) (int64, error) {
	res, err := s.db.Exec(`UPDATE vpn_tunnels SET state = 'stale' WHERE last_seen < ? AND state != 'stale'`, before.UnixMilli())
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// DeleteVPNTunnel removes one observation by id.
func (s *Store) DeleteVPNTunnel(id int64) error {
	res, err := s.db.Exec(`DELETE FROM vpn_tunnels WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}
