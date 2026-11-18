package store

import (
	"database/sql"
	"encoding/json"
	"time"

	"github.com/kr3s4l4/netdiag/internal/model"
)

// SaveHistorySnapshot stores a compact inventory/topology capture.
func (s *Store) SaveHistorySnapshot(snap *model.HistorySnapshot) error {
	if snap == nil {
		return nil
	}
	if snap.TS.IsZero() {
		snap.TS = time.Now()
	}
	if snap.SiteID <= 0 {
		snap.SiteID = 1
	}
	if snap.Hosts == nil {
		snap.Hosts = []model.HistoryHost{}
	}
	hosts, err := json.Marshal(snap.Hosts)
	if err != nil {
		return err
	}
	graph, err := json.Marshal(snap.Topology)
	if err != nil {
		return err
	}
	res, err := s.db.Exec(`INSERT INTO history_snapshots(site_id, ts, hosts, graph) VALUES(?,?,?,?)`,
		snap.SiteID, snap.TS.UnixMilli(), string(hosts), string(graph))
	if err != nil {
		return err
	}
	snap.ID, err = res.LastInsertId()
	return err
}

// HistorySnapshotIn returns the snapshot closest to the end of a window,
// restricted to that window. It returns (nil, nil) when no snapshot exists.
func (s *Store) HistorySnapshotIn(siteID int64, from, to time.Time) (*model.HistorySnapshot, error) {
	var snap model.HistorySnapshot
	var ts int64
	var hostRaw, graphRaw string
	query := `SELECT id, site_id, ts, hosts, graph FROM history_snapshots
		WHERE ts BETWEEN ? AND ?
		ORDER BY ABS(ts - ?) ASC, ts DESC LIMIT 1`
	args := []any{from.UnixMilli(), to.UnixMilli(), to.UnixMilli()}
	if siteID > 0 {
		query = `SELECT id, site_id, ts, hosts, graph FROM history_snapshots
			WHERE site_id = ? AND ts BETWEEN ? AND ?
			ORDER BY ABS(ts - ?) ASC, ts DESC LIMIT 1`
		args = append([]any{siteID}, args...)
	}
	err := s.db.QueryRow(query, args...).
		Scan(&snap.ID, &snap.SiteID, &ts, &hostRaw, &graphRaw)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	snap.TS = time.UnixMilli(ts)
	if json.Unmarshal([]byte(hostRaw), &snap.Hosts) != nil {
		snap.Hosts = []model.HistoryHost{}
	}
	if json.Unmarshal([]byte(graphRaw), &snap.Topology) != nil {
		snap.Topology = model.HistoryTopology{}
	}
	return &snap, nil
}

// BuildHistoryPeriod fills one side of a diff with metric aggregates,
// per-device samples and the closest topology snapshot in the window.
func (s *Store) BuildHistoryPeriod(siteID int64, period model.HistoryPeriod) (model.HistoryPeriod, error) {
	fromMS, toMS := period.From.UnixMilli(), period.To.UnixMilli()

	var avail, avgRTT, minRTT, maxRTT sql.NullFloat64
	var samples, hostsUp, hostsDown int
	scope := ` FROM metrics m`
	scopeArgs := []any{}
	if siteID > 0 {
		scope += ` JOIN hosts h ON h.id = m.host_id AND h.site_id = ?`
		scopeArgs = append(scopeArgs, siteID)
	}
	scope += ` WHERE m.ts BETWEEN ? AND ?`
	scopeArgs = append(scopeArgs, fromMS, toMS)

	globalQuery := `SELECT COUNT(*),
			AVG(CASE WHEN m.up = 1 THEN 1.0 ELSE 0.0 END) * 100,
			AVG(CASE WHEN m.up = 1 AND m.rtt_ms > 0 THEN m.rtt_ms END),
			MIN(CASE WHEN m.up = 1 AND m.rtt_ms > 0 THEN m.rtt_ms END),
			MAX(CASE WHEN m.up = 1 AND m.rtt_ms > 0 THEN m.rtt_ms END),
			COUNT(DISTINCT CASE WHEN m.up = 1 THEN m.host_id END),
			COUNT(DISTINCT CASE WHEN m.up = 0 THEN m.host_id END)` + scope
	if err := s.db.QueryRow(globalQuery, scopeArgs...).Scan(&samples, &avail, &avgRTT, &minRTT, &maxRTT, &hostsUp, &hostsDown); err != nil {
		return period, err
	}
	period.Metrics = model.HistoryAggregate{
		Samples: samples, Availability: round2(nullOr(avail)), AvgRTT: round2(nullOr(avgRTT)),
		MinRTT: round2(nullOr(minRTT)), MaxRTT: round2(nullOr(maxRTT)),
		HostsUp: hostsUp, HostsDown: hostsDown,
	}

	span := period.To.Sub(period.From)
	if span <= 0 {
		span = 24 * time.Hour
	}
	bucket := span / 100
	if bucket < time.Minute {
		bucket = time.Minute
	}
	bms := bucket.Milliseconds()
	period.RTT, period.Uptime = []model.SeriesPoint{}, []model.SeriesPoint{}
	seriesQuery := `SELECT (m.ts / ?) * ? AS bucket,
			AVG(CASE WHEN m.up = 1 AND m.rtt_ms > 0 THEN m.rtt_ms END),
			AVG(CASE WHEN m.up = 1 THEN 1.0 ELSE 0.0 END) * 100` + scope + ` GROUP BY bucket ORDER BY bucket`
	seriesArgs := append([]any{bms, bms}, scopeArgs...)
	rowsSeries, err := s.db.Query(seriesQuery, seriesArgs...)
	if err != nil {
		return period, err
	}
	for rowsSeries.Next() {
		var b int64
		var rtt, up sql.NullFloat64
		if err := rowsSeries.Scan(&b, &rtt, &up); err != nil {
			rowsSeries.Close()
			return period, err
		}
		ts := time.UnixMilli(b)
		if rtt.Valid {
			period.RTT = append(period.RTT, model.SeriesPoint{TS: ts, Value: round2(rtt.Float64)})
		}
		period.Uptime = append(period.Uptime, model.SeriesPoint{TS: ts, Value: round2(nullOr(up))})
	}
	if err := rowsSeries.Err(); err != nil {
		rowsSeries.Close()
		return period, err
	}
	rowsSeries.Close()

	hostQuery := `SELECT m.host_id, h.ip, h.hostname, h.mac, h.device_type, h.status,
			COUNT(*),
			SUM(CASE WHEN m.up = 1 THEN 1 ELSE 0 END),
			SUM(CASE WHEN m.up = 0 THEN 1 ELSE 0 END),
			AVG(CASE WHEN m.up = 1 AND m.rtt_ms > 0 THEN m.rtt_ms END),
			AVG(CASE WHEN m.up = 1 THEN 1.0 ELSE 0.0 END) * 100,
			MIN(m.ts), MAX(m.ts)
		FROM metrics m JOIN hosts h ON h.id = m.host_id`
	hostArgs := []any{}
	if siteID > 0 {
		hostQuery += ` AND h.site_id = ?`
		hostArgs = append(hostArgs, siteID)
	}
	hostQuery += ` WHERE m.ts BETWEEN ? AND ? GROUP BY m.host_id ORDER BY m.host_id`
	hostArgs = append(hostArgs, fromMS, toMS)
	rows, err := s.db.Query(hostQuery, hostArgs...)
	if err != nil {
		return period, err
	}
	defer rows.Close()
	period.Hosts = []model.HistoryHost{}
	for rows.Next() {
		var h model.HistoryHost
		var first, last int64
		var rtt, availability sql.NullFloat64
		var up, down sql.NullInt64
		if err := rows.Scan(&h.ID, &h.IP, &h.Hostname, &h.MAC, &h.DeviceType, &h.Status,
			&h.Samples, &up, &down, &rtt, &availability, &first, &last); err != nil {
			return period, err
		}
		h.UpSamples, h.DownSamples = int(up.Int64), int(down.Int64)
		h.AvgRTT, h.Availability = round2(nullOr(rtt)), round2(nullOr(availability))
		h.FirstSeen, h.LastSeen = time.UnixMilli(first), time.UnixMilli(last)
		period.Hosts = append(period.Hosts, h)
	}
	if err := rows.Err(); err != nil {
		return period, err
	}

	snap, err := s.HistorySnapshotIn(siteID, period.From, period.To)
	if err != nil {
		return period, err
	}
	if snap != nil {
		period.Topology = snap.Topology
		at := snap.TS
		period.Topology.SnapshotAt = &at
	}
	return period, nil
}

// PurgeHistorySnapshots removes captures older than the retention window.
func (s *Store) PurgeHistorySnapshots(days int) error {
	if days < 1 {
		days = 30
	}
	cut := time.Now().AddDate(0, 0, -days).UnixMilli()
	_, err := s.db.Exec(`DELETE FROM history_snapshots WHERE ts < ?`, cut)
	return err
}
