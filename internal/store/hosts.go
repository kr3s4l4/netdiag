package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/kr3s4l4/netdiag/internal/model"
)

const hostColumns = `id, identity, site_id, ip, mac, hostname, vendor, model, os, device_type,
	confidence, status, is_gateway, favorite, notes, tags, ports, latency_ms, ttl,
	first_seen, last_seen, network_id, last_scan_id, snmp, evidence`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanHost(r rowScanner) (*model.Host, error) {
	var (
		h                          model.Host
		tags, ports, snmpRaw, evid string
		gateway, favorite          int
		firstSeen, lastSeen        int64
		deviceType                 string
		status                     string
	)
	err := r.Scan(&h.ID, &h.Identity, &h.SiteID, &h.IP, &h.MAC, &h.Hostname, &h.Vendor, &h.Model, &h.OS, &deviceType,
		&h.Confidence, &status, &gateway, &favorite, &h.Notes, &tags, &ports, &h.LatencyMS, &h.TTL,
		&firstSeen, &lastSeen, &h.NetworkID, &h.LastScanID, &snmpRaw, &evid)
	if err != nil {
		return nil, err
	}
	h.DeviceType = model.DeviceType(deviceType)
	h.Status = model.HostStatus(status)
	h.IsGateway = gateway != 0
	h.Favorite = favorite != 0
	h.FirstSeen = fromMS(firstSeen)
	h.LastSeen = fromMS(lastSeen)
	_ = json.Unmarshal([]byte(tags), &h.Tags)
	if h.Tags == nil {
		h.Tags = []string{}
	}
	_ = json.Unmarshal([]byte(ports), &h.Ports)
	if h.Ports == nil {
		h.Ports = []model.Port{}
	}
	for _, p := range h.Ports {
		if p.State == "open" || p.State == "" {
			h.OpenPortCount++
		}
	}
	_ = json.Unmarshal([]byte(evid), &h.Evidence)
	if snmpRaw != "" && snmpRaw != "null" {
		var sn model.SNMPSummary
		if json.Unmarshal([]byte(snmpRaw), &sn) == nil {
			h.SNMP = &sn
		}
	}
	return &h, nil
}

func validMAC(mac string) bool {
	if mac == "" {
		return false
	}
	hw, err := net.ParseMAC(mac)
	if err != nil || len(hw) < 6 {
		return false
	}
	// All-zero and broadcast addresses are not identities.
	allZero, allFF := true, true
	for _, b := range hw {
		if b != 0 {
			allZero = false
		}
		if b != 0xff {
			allFF = false
		}
	}
	return !allZero && !allFF
}

// IdentityFor returns the stable identity key for a host.
func IdentityFor(ip, mac string) string {
	if validMAC(mac) {
		return strings.ToLower(mac)
	}
	return "ip:" + ip
}

// siteIdentity namespaces a host identity with its site. The same IP or MAC
// may legitimately exist in several sites, so identities must be unique only
// inside one site. Exported bundles carry the raw (site-free) identity.
func siteIdentity(siteID int64, ip, mac string) string {
	if siteID <= 0 {
		siteID = 1
	}
	return fmt.Sprintf("s%d:%s", siteID, IdentityFor(ip, mac))
}

// rawIdentity strips the site prefix from a stored identity, making bundles
// portable across sites and instances. The "s<id>:" marker cannot be confused
// with a MAC (hex pairs) or an IP identity ("ip:…").
func rawIdentity(identity string) string {
	if strings.HasPrefix(identity, "s") {
		if i := strings.IndexByte(identity, ':'); i > 1 {
			if _, err := strconv.ParseInt(identity[1:i], 10, 64); err == nil {
				return identity[i+1:]
			}
		}
	}
	return identity
}

// UpsertHost inserts or updates a host from a scan. It preserves user metadata
// (notes, tags, favourite) and records lifecycle events. A nil Ports slice means
// "ports were not scanned", so previous ports are preserved. A non-nil empty
// slice means "no ports found".
func (s *Store) UpsertHost(h *model.Host) (id int64, created bool, prev *model.Host, changes []string, err error) {
	if h.IP == "" {
		return 0, false, nil, nil, fmt.Errorf("host sin IP")
	}
	if h.Status == "" {
		h.Status = model.StatusUp
	}
	if h.FirstSeen.IsZero() {
		h.FirstSeen = time.Now()
	}
	if h.LastSeen.IsZero() {
		h.LastSeen = time.Now()
	}
	if h.SiteID == 0 && h.NetworkID > 0 {
		if err := s.db.QueryRow(`SELECT site_id FROM networks WHERE id = ?`, h.NetworkID).Scan(&h.SiteID); err != nil {
			h.SiteID = 1
		}
	}
	if h.SiteID == 0 {
		h.SiteID = 1
	}
	identity := siteIdentity(h.SiteID, h.IP, h.MAC)

	tx, err := s.db.Begin()
	if err != nil {
		return 0, false, nil, nil, err
	}
	defer tx.Rollback()

	var foundID int64
	var foundIdentity string
	row := tx.QueryRow(`SELECT id, identity FROM hosts WHERE identity = ?`, identity)
	switch e := row.Scan(&foundID, &foundIdentity); {
	case e == nil:
	case e == sql.ErrNoRows:
		// Try by MAC (host may have been discovered before with an IP identity).
		if validMAC(h.MAC) {
			row = tx.QueryRow(`SELECT id, identity FROM hosts WHERE lower(mac) = ? AND site_id = ? ORDER BY last_seen DESC LIMIT 1`, strings.ToLower(h.MAC), h.SiteID)
			if e2 := row.Scan(&foundID, &foundIdentity); e2 == nil {
				// rekey below
			} else if e2 != sql.ErrNoRows {
				return 0, false, nil, nil, e2
			}
		}
		if foundID == 0 {
			// Try by IP only when this host still has no MAC-based identity.
			var oldMAC string
			row = tx.QueryRow(`SELECT id, identity, mac FROM hosts WHERE ip = ? AND site_id = ? ORDER BY last_seen DESC LIMIT 1`, h.IP, h.SiteID)
			var oldIdentity string
			if e2 := row.Scan(&foundID, &oldIdentity, &oldMAC); e2 == nil {
				foundIdentity = oldIdentity
			} else if e2 != sql.ErrNoRows {
				return 0, false, nil, nil, e2
			}
			if foundID != 0 && validMAC(oldMAC) && validMAC(h.MAC) && !strings.EqualFold(oldMAC, h.MAC) {
				// The IP has been reassigned to a different device. Do not merge.
				foundID, foundIdentity = 0, ""
			}
		}
	default:
		return 0, false, nil, nil, e
	}

	prevHost := (*model.Host)(nil)
	if foundID != 0 {
		prevHost, err = scanHost(tx.QueryRow(`SELECT `+hostColumns+` FROM hosts WHERE id = ?`, foundID))
		if err != nil {
			return 0, false, nil, nil, err
		}
		prev = prevHost
	}

	// Rekey to MAC identity once a MAC is known.
	if foundID != 0 && identity != foundIdentity {
		var clash int64
		if err := tx.QueryRow(`SELECT id FROM hosts WHERE identity = ? AND site_id = ?`, identity, h.SiteID).Scan(&clash); err == nil && clash != foundID {
			// Another row already owns the MAC identity: merge into it instead.
			foundID = clash
			prevHost, err = scanHost(tx.QueryRow(`SELECT `+hostColumns+` FROM hosts WHERE id = ?`, clash))
			if err != nil {
				return 0, false, nil, nil, err
			}
			prev = prevHost
		} else {
			if _, err := tx.Exec(`UPDATE hosts SET identity = ? WHERE id = ?`, identity, foundID); err != nil {
				return 0, false, nil, nil, err
			}
		}
	}

	// If this IP is currently assigned to a different identity, detach it.
	if foundID == 0 {
		rows, err := tx.Query(`SELECT id, identity FROM hosts WHERE ip = ? AND site_id = ?`, h.IP, h.SiteID)
		if err != nil {
			return 0, false, nil, nil, err
		}
		var stale []int64
		for rows.Next() {
			var id int64
			var ident string
			if err := rows.Scan(&id, &ident); err != nil {
				rows.Close()
				return 0, false, nil, nil, err
			}
			if ident != identity {
				stale = append(stale, id)
			}
		}
		rows.Close()
		for _, id := range stale {
			if _, err := tx.Exec(`UPDATE hosts SET ip = '', status = 'down', last_seen = ? WHERE id = ?`, nowMS(), id); err == nil {
				addEventTx(tx, id, "ip_reassigned", "La IP fue reasignada a otro dispositivo", map[string]any{"ip": h.IP})
			}
		}
	}

	if foundID == 0 {
		res, err := tx.Exec(`INSERT INTO hosts(identity, site_id, ip, mac, hostname, vendor, model, os, device_type,
			confidence, status, is_gateway, favorite, notes, tags, ports, latency_ms, ttl,
			first_seen, last_seen, network_id, last_scan_id, snmp, evidence)
			VALUES(?,?,?,?,?,?,?,?,?,?,?,?,0,'','[]',?,?,?,?,?,?,?,?,?)`,
			identity, h.SiteID, h.IP, h.MAC, h.Hostname, h.Vendor, h.Model, h.OS, string(h.DeviceType),
			h.Confidence, string(h.Status), boolInt(h.IsGateway), jsonMarshal(nonNilPorts(h.Ports)),
			h.LatencyMS, h.TTL, h.FirstSeen.UnixMilli(), h.LastSeen.UnixMilli(), h.NetworkID, h.LastScanID,
			jsonMarshal(h.SNMP), jsonMarshal(nonNilStrings(h.Evidence)))
		if err != nil {
			return 0, false, nil, nil, err
		}
		id, _ := res.LastInsertId()
		addEventTx(tx, id, "discovered", "Dispositivo descubierto", map[string]any{"ip": h.IP, "mac": h.MAC})
		if err := tx.Commit(); err != nil {
			return 0, false, nil, nil, err
		}
		return id, true, nil, []string{"discovered"}, nil
	}

	merged := *prevHost
	merged.Identity = identity
	merged.IP = h.IP
	if h.MAC != "" {
		merged.MAC = h.MAC
	}
	if h.Hostname != "" {
		merged.Hostname = h.Hostname
	}
	if h.Vendor != "" {
		merged.Vendor = h.Vendor
	}
	if h.Model != "" {
		merged.Model = h.Model
	}
	if h.OS != "" {
		merged.OS = h.OS
	}
	if h.DeviceType != "" && h.DeviceType != model.DevUnknown {
		merged.DeviceType = h.DeviceType
	}
	if h.Confidence >= merged.Confidence {
		merged.Confidence = h.Confidence
	}
	if h.LatencyMS > 0 {
		merged.LatencyMS = h.LatencyMS
	}
	if h.TTL > 0 {
		merged.TTL = h.TTL
	}
	if h.NetworkID != 0 {
		merged.NetworkID = h.NetworkID
	}
	if h.LastScanID != 0 {
		merged.LastScanID = h.LastScanID
	}
	if h.SNMP != nil {
		merged.SNMP = h.SNMP
	}
	if h.Ports != nil {
		merged.Ports = h.Ports
	}
	if h.Evidence != nil {
		merged.Evidence = h.Evidence
	}
	wasDown := prevHost.Status != model.StatusUp
	merged.Status = h.Status
	if h.FirstSeen.Before(merged.FirstSeen) && !h.FirstSeen.IsZero() {
		merged.FirstSeen = h.FirstSeen
	}
	merged.LastSeen = h.LastSeen

	if wasDown && merged.Status == model.StatusUp {
		changes = append(changes, "online")
	} else if !wasDown && merged.Status == model.StatusDown {
		changes = append(changes, "offline")
	}
	if prevHost.Hostname != "" && merged.Hostname != "" && !strings.EqualFold(prevHost.Hostname, merged.Hostname) {
		changes = append(changes, "hostname_changed")
	}
	if prevHost.Vendor != "" && merged.Vendor != "" && prevHost.Vendor != merged.Vendor {
		changes = append(changes, "vendor_changed")
	}
	if prevHost.DeviceType != merged.DeviceType && prevHost.DeviceType != "" {
		changes = append(changes, "device_changed")
	}
	opened, closed := diffPorts(prevHost.Ports, merged.Ports)
	if len(opened) > 0 {
		changes = append(changes, "ports_opened")
	}
	if len(closed) > 0 {
		changes = append(changes, "ports_closed")
	}
	if len(h.Ports) > 0 || h.Ports != nil {
		if serviceChanged(prevHost.Ports, merged.Ports) {
			changes = append(changes, "service_changed")
		}
	}

	_, err = tx.Exec(`UPDATE hosts SET identity=?, site_id=?, ip=?, mac=?, hostname=?, vendor=?, model=?, os=?,
		device_type=?, confidence=?, status=?, is_gateway=?, ports=?, latency_ms=?, ttl=?,
		first_seen=?, last_seen=?, network_id=?, last_scan_id=?, snmp=?, evidence=?
		WHERE id=?`,
		identity, h.SiteID, merged.IP, merged.MAC, merged.Hostname, merged.Vendor, merged.Model, merged.OS,
		string(merged.DeviceType), merged.Confidence, string(merged.Status), boolInt(merged.IsGateway),
		jsonMarshal(nonNilPorts(merged.Ports)), merged.LatencyMS, merged.TTL,
		merged.FirstSeen.UnixMilli(), merged.LastSeen.UnixMilli(), merged.NetworkID, merged.LastScanID,
		jsonMarshal(merged.SNMP), jsonMarshal(nonNilStrings(merged.Evidence)), foundID)
	if err != nil {
		return 0, false, nil, nil, err
	}

	for _, c := range changes {
		switch c {
		case "online":
			addEventTx(tx, foundID, c, "El dispositivo volvió a estar en línea", nil)
		case "offline":
			addEventTx(tx, foundID, c, "El dispositivo dejó de responder", nil)
		case "hostname_changed":
			addEventTx(tx, foundID, c, "Nombre de host actualizado", map[string]any{"from": prevHost.Hostname, "to": merged.Hostname})
		case "vendor_changed":
			addEventTx(tx, foundID, c, "Fabricante actualizado", map[string]any{"from": prevHost.Vendor, "to": merged.Vendor})
		case "device_changed":
			addEventTx(tx, foundID, c, "Tipo de dispositivo actualizado", map[string]any{"from": prevHost.DeviceType, "to": merged.DeviceType})
		case "ports_opened":
			addEventTx(tx, foundID, c, fmt.Sprintf("%d puerto(s) abierto(s)", len(opened)), map[string]any{"ports": opened})
		case "ports_closed":
			addEventTx(tx, foundID, c, fmt.Sprintf("%d puerto(s) cerrado(s)", len(closed)), map[string]any{"ports": closed})
		case "service_changed":
			addEventTx(tx, foundID, c, "La firma de servicios cambió", nil)
		}
	}
	return foundID, false, prev, changes, tx.Commit()
}

func nonNilPorts(p []model.Port) []model.Port {
	if p == nil {
		return []model.Port{}
	}
	return p
}

func nonNilStrings(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func addEventTx(tx *sql.Tx, hostID int64, kind, message string, data map[string]any) {
	_, _ = tx.Exec(`INSERT INTO host_events(host_id, kind, message, data, created_at) VALUES(?,?,?,?,?)`,
		hostID, kind, message, jsonMarshal(data), nowMS())
}

// AddEvent records a host event.
func (s *Store) AddEvent(hostID int64, kind, message string, data map[string]any) error {
	_, err := s.db.Exec(`INSERT INTO host_events(host_id, kind, message, data, created_at) VALUES(?,?,?,?,?)`,
		hostID, kind, message, jsonMarshal(data), nowMS())
	return err
}

func portKey(p model.Port) string {
	proto := p.Protocol
	if proto == "" {
		proto = "tcp"
	}
	return fmt.Sprintf("%s/%d", proto, p.Number)
}

func diffPorts(prev, next []model.Port) (opened, closed []int) {
	pm := map[string]model.Port{}
	nm := map[string]model.Port{}
	for _, p := range prev {
		if p.State == "open" || p.State == "" {
			pm[portKey(p)] = p
		}
	}
	for _, p := range next {
		if p.State == "open" || p.State == "" {
			nm[portKey(p)] = p
		}
	}
	for k, p := range nm {
		if _, ok := pm[k]; !ok {
			opened = append(opened, p.Number)
		}
	}
	for k, p := range pm {
		if _, ok := nm[k]; !ok {
			closed = append(closed, p.Number)
		}
	}
	sort.Ints(opened)
	sort.Ints(closed)
	return
}

func serviceChanged(prev, next []model.Port) bool {
	if len(prev) == 0 {
		return false
	}
	pm := map[string]string{}
	for _, p := range prev {
		pm[portKey(p)] = p.Service + "|" + p.Product + "|" + p.Version
	}
	for _, p := range next {
		k := portKey(p)
		if sig, ok := pm[k]; ok {
			if sig != p.Service+"|"+p.Product+"|"+p.Version {
				return true
			}
		}
	}
	return false
}

// GetHost returns a host by ID.
func (s *Store) GetHost(id int64) (*model.Host, error) {
	h, err := scanHost(s.db.QueryRow(`SELECT `+hostColumns+` FROM hosts WHERE id = ?`, id))
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return h, err
}

// GetHostByIP returns the most recently seen host with an IP.
func (s *Store) GetHostByIP(ip string) (*model.Host, error) {
	h, err := scanHost(s.db.QueryRow(`SELECT `+hostColumns+` FROM hosts WHERE ip = ? ORDER BY last_seen DESC LIMIT 1`, ip))
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return h, err
}

// ListHosts returns a filtered, paginated host list.
func (s *Store) ListHosts(f model.HostFilter) (*model.ListResult[model.Host], error) {
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
	if f.SiteID > 0 {
		where = append(where, "site_id = ?")
		args = append(args, f.SiteID)
	}
	if f.Search != "" {
		q := "%" + escapeLike(f.Search) + "%"
		where = append(where, `(ip LIKE ? ESCAPE '\' OR hostname LIKE ? ESCAPE '\' OR mac LIKE ? ESCAPE '\' OR vendor LIKE ? ESCAPE '\' OR notes LIKE ? ESCAPE '\' OR os LIKE ? ESCAPE '\')`)
		args = append(args, q, q, q, q, q, q)
	}
	if f.Status != "" && f.Status != "all" {
		where = append(where, "status = ?")
		args = append(args, f.Status)
	}
	if f.DeviceType != "" && f.DeviceType != "all" {
		where = append(where, "device_type = ?")
		args = append(args, f.DeviceType)
	}
	if f.NetworkID > 0 {
		where = append(where, "network_id = ?")
		args = append(args, f.NetworkID)
	}
	if f.Favorite {
		where = append(where, "favorite = 1")
	}
	if f.Tag != "" {
		where = append(where, "EXISTS (SELECT 1 FROM json_each(hosts.tags) WHERE value = ?)")
		args = append(args, f.Tag)
	}
	sortCol := "last_seen"
	switch f.Sort {
	case "ip", "hostname", "vendor", "status", "device_type", "first_seen", "last_seen", "latency_ms":
		sortCol = f.Sort
	}
	dir := "DESC"
	if !f.Desc {
		dir = "ASC"
	}
	whereSQL := strings.Join(where, " AND ")
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM hosts WHERE `+whereSQL, args...).Scan(&total); err != nil {
		return nil, err
	}
	args = append(args, f.Limit, (f.Page-1)*f.Limit)
	rows, err := s.db.Query(`SELECT `+hostColumns+` FROM hosts WHERE `+whereSQL+` ORDER BY `+sortCol+` `+dir+` LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []model.Host{}
	for rows.Next() {
		h, err := scanHost(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, *h)
	}
	return &model.ListResult[model.Host]{Items: items, Total: total, Page: f.Page, Limit: f.Limit}, rows.Err()
}

// ListAllHosts returns every host, newest first, capped at limit.
func (s *Store) ListAllHosts(limit int) ([]model.Host, error) {
	return s.ListAllHostsForSite(0, limit)
}

// ListAllHostsForSite returns the hosts of one site (0 means all sites).
func (s *Store) ListAllHostsForSite(siteID int64, limit int) ([]model.Host, error) {
	if limit <= 0 {
		limit = 10000
	}
	query := `SELECT ` + hostColumns + ` FROM hosts`
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
	var out []model.Host
	for rows.Next() {
		h, err := scanHost(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *h)
	}
	return out, rows.Err()
}

// ListPortsSummary returns all open ports grouped by number for the dashboard.
func (s *Store) ListPortsSummary() (map[int]int, error) {
	return s.ListPortsSummaryForSite(0)
}

// ListPortsSummaryForSite groups open ports of one site (0 means all sites).
func (s *Store) ListPortsSummaryForSite(siteID int64) (map[int]int, error) {
	query := `SELECT ports FROM hosts WHERE ports != '[]'`
	args := []any{}
	if siteID > 0 {
		query += ` AND site_id = ?`
		args = append(args, siteID)
	}
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[int]int{}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var ports []model.Port
		if json.Unmarshal([]byte(raw), &ports) != nil {
			continue
		}
		for _, p := range ports {
			if p.State == "open" || p.State == "" {
				out[p.Number]++
			}
		}
	}
	return out, rows.Err()
}

// UpdateHostMeta updates user-editable fields.
func (s *Store) UpdateHostMeta(id int64, notes *string, tags *[]string, favorite *bool) error {
	var sets []string
	var args []any
	if notes != nil {
		sets = append(sets, "notes = ?")
		args = append(args, *notes)
	}
	if tags != nil {
		sets = append(sets, "tags = ?")
		args = append(args, jsonMarshal(*tags))
	}
	if favorite != nil {
		sets = append(sets, "favorite = ?")
		args = append(args, boolInt(*favorite))
	}
	if len(sets) == 0 {
		return nil
	}
	args = append(args, id)
	_, err := s.db.Exec(`UPDATE hosts SET `+strings.Join(sets, ", ")+` WHERE id = ?`, args...)
	return err
}

// SetHostStatus updates reachability and last seen.
func (s *Store) SetHostStatus(id int64, status model.HostStatus, latency float64, ttl int) error {
	_, err := s.db.Exec(`UPDATE hosts SET status=?, latency_ms=?, ttl=?, last_seen=? WHERE id=?`,
		string(status), latency, ttl, nowMS(), id)
	return err
}

// DeleteHost removes a host and its events/metrics.
func (s *Store) DeleteHost(id int64) error {
	_, err := s.db.Exec(`DELETE FROM hosts WHERE id = ?`, id)
	return err
}

// ListEvents returns recent events, optionally for one host.
func (s *Store) ListEvents(hostID int64, limit int) ([]model.Event, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	q := `SELECT e.id, e.host_id, h.ip, e.kind, e.message, e.data, e.created_at
		FROM host_events e LEFT JOIN hosts h ON h.id = e.host_id`
	var args []any
	if hostID > 0 {
		q += ` WHERE e.host_id = ?`
		args = append(args, hostID)
	}
	q += ` ORDER BY e.created_at DESC LIMIT ?`
	args = append(args, limit)
	rows, err := s.db.Query(q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Event{}
	for rows.Next() {
		var e model.Event
		var data string
		var ts int64
		if err := rows.Scan(&e.ID, &e.HostID, &e.HostIP, &e.Kind, &e.Message, &data, &ts); err != nil {
			return nil, err
		}
		e.Data = jsonObject(data)
		e.CreatedAt = fromMS(ts)
		out = append(out, e)
	}
	return out, rows.Err()
}

// StatusCounts returns host counts by status.
func (s *Store) StatusCounts() (map[string]int, error) {
	return s.StatusCountsForSite(0)
}

// StatusCountsForSite returns host counts by status for one site (0 = all).
func (s *Store) StatusCountsForSite(siteID int64) (map[string]int, error) {
	query := `SELECT status, COUNT(*) FROM hosts`
	args := []any{}
	if siteID > 0 {
		query += ` WHERE site_id = ?`
		args = append(args, siteID)
	}
	query += ` GROUP BY status`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var st string
		var n int
		if err := rows.Scan(&st, &n); err != nil {
			return nil, err
		}
		out[st] = n
	}
	return out, rows.Err()
}

// TypeCounts returns host counts by device type.
func (s *Store) TypeCounts() (map[string]int, error) {
	return s.TypeCountsForSite(0)
}

// TypeCountsForSite returns host counts by device type for one site (0 = all).
func (s *Store) TypeCountsForSite(siteID int64) (map[string]int, error) {
	query := `SELECT device_type, COUNT(*) FROM hosts`
	args := []any{}
	if siteID > 0 {
		query += ` WHERE site_id = ?`
		args = append(args, siteID)
	}
	query += ` GROUP BY device_type ORDER BY COUNT(*) DESC`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var t string
		var n int
		if err := rows.Scan(&t, &n); err != nil {
			return nil, err
		}
		out[t] = n
	}
	return out, rows.Err()
}

// CountHostsNewSince counts hosts first seen after t.
func (s *Store) CountHostsNewSince(t time.Time) (int, error) {
	return s.CountHostsNewSinceForSite(0, t)
}

// CountHostsNewSinceForSite counts new hosts of one site (0 = all).
func (s *Store) CountHostsNewSinceForSite(siteID int64, t time.Time) (int, error) {
	var n int
	query := `SELECT COUNT(*) FROM hosts WHERE first_seen >= ?`
	args := []any{t.UnixMilli()}
	if siteID > 0 {
		query += ` AND site_id = ?`
		args = append(args, siteID)
	}
	err := s.db.QueryRow(query, args...).Scan(&n)
	return n, err
}

// CountHosts returns the total number of hosts.
func (s *Store) CountHosts() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM hosts`).Scan(&n)
	return n, err
}

// MarkMissingHosts marks previously-up hosts in a network as down when they
// were not seen in the latest scan. It returns the affected hosts so callers
// can record availability metrics and raise alerts.
func (s *Store) MarkMissingHosts(networkID int64, seen []int64) ([]model.Host, error) {
	if networkID <= 0 {
		return nil, nil
	}
	seenSet := make(map[int64]bool, len(seen))
	for _, id := range seen {
		seenSet[id] = true
	}
	rows, err := s.db.Query(`SELECT `+hostColumns+` FROM hosts WHERE network_id = ? AND status = ?`, networkID, string(model.StatusUp))
	if err != nil {
		return nil, err
	}
	var candidates []model.Host
	for rows.Next() {
		h, err := scanHost(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		if !seenSet[h.ID] {
			candidates = append(candidates, *h)
		}
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(candidates) == 0 {
		return nil, nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	now := nowMS()
	for _, h := range candidates {
		if _, err := tx.Exec(`UPDATE hosts SET status = ?, last_seen = ? WHERE id = ?`, string(model.StatusDown), now, h.ID); err != nil {
			return nil, err
		}
		addEventTx(tx, h.ID, "offline", "El dispositivo dejó de responder", nil)
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return candidates, nil
}

// CountManagedHosts returns the number of distinct site-qualified identities
// observed since "since", excluding hosts tagged "guest" or "excluded". It is
// the billing definition documented in docs/comercializacion.md.
func (s *Store) CountManagedHosts(since time.Time) (int, error) {
	rows, err := s.db.Query(`SELECT identity, tags FROM hosts WHERE last_seen >= ?`, since.UnixMilli())
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	seen := make(map[string]struct{})
	for rows.Next() {
		var identity, raw string
		if err := rows.Scan(&identity, &raw); err != nil {
			return 0, err
		}
		var tags []string
		if err := json.Unmarshal([]byte(raw), &tags); err != nil {
			continue
		}
		skip := false
		for _, tag := range tags {
			t := strings.ToLower(strings.TrimSpace(tag))
			if t == "guest" || t == "excluded" {
				skip = true
				break
			}
		}
		if !skip {
			seen[identity] = struct{}{}
		}
	}
	return len(seen), rows.Err()
}

// CountManagedHostsBySiteSince returns the same billing-style count grouped by
// site. Only integers leave this method: no identity, IP, MAC or hostname is
// exposed to callers building anonymous usage evidence.
func (s *Store) CountManagedHostsBySiteSince(since time.Time) (map[int64]int, error) {
	rows, err := s.db.Query(`SELECT site_id, identity, tags FROM hosts WHERE last_seen >= ?`, since.UnixMilli())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	seen := make(map[string]struct{})
	counts := make(map[int64]int)
	for rows.Next() {
		var siteID int64
		var identity, raw string
		if err := rows.Scan(&siteID, &identity, &raw); err != nil {
			return nil, err
		}
		var tags []string
		if err := json.Unmarshal([]byte(raw), &tags); err != nil {
			continue
		}
		skip := false
		for _, tag := range tags {
			t := strings.ToLower(strings.TrimSpace(tag))
			if t == "guest" || t == "excluded" {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		key := fmt.Sprintf("%d:%s", siteID, identity)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		counts[siteID]++
	}
	return counts, rows.Err()
}
