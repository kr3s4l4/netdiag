package store

import (
	"database/sql"
	"math"
	"time"

	"github.com/kr3s4l4/netdiag/internal/model"
)

// AddMetric stores one latency/availability sample.
func (s *Store) AddMetric(hostID int64, rtt float64, up bool, loss float64, ts time.Time) error {
	if ts.IsZero() {
		ts = time.Now()
	}
	_, err := s.db.Exec(`INSERT INTO metrics(host_id, ts, rtt_ms, up, loss) VALUES(?,?,?,?,?)`,
		hostID, ts.UnixMilli(), rtt, boolInt(up), loss)
	return err
}

// AddMetrics inserts many samples in one transaction.
func (s *Store) AddMetrics(metrics []model.Metric) error {
	if len(metrics) == 0 {
		return nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	stmt, err := tx.Prepare(`INSERT INTO metrics(host_id, ts, rtt_ms, up, loss) VALUES(?,?,?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for _, m := range metrics {
		ts := m.TS
		if ts.IsZero() {
			ts = time.Now()
		}
		if _, err := stmt.Exec(m.HostID, ts.UnixMilli(), m.RTTMS, boolInt(m.Up), m.Loss); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// HostHistory returns downsampled metrics for a host.
func (s *Store) HostHistory(hostID int64, from, to time.Time, maxPoints int) (*model.HostHistory, error) {
	if maxPoints < 10 {
		maxPoints = 120
	}
	if maxPoints > 2000 {
		maxPoints = 2000
	}
	if from.IsZero() {
		from = to.Add(-24 * time.Hour)
	}
	span := to.Sub(from)
	if span <= 0 {
		span = time.Hour
	}
	bucket := span / time.Duration(maxPoints)
	if bucket < time.Minute {
		bucket = time.Minute
	}
	bms := bucket.Milliseconds()

	out := &model.HostHistory{HostID: hostID, From: from, To: to, RTT: []model.SeriesPoint{}, Uptime: []model.SeriesPoint{}}

	var avail, avgR, minR, maxR sql.NullFloat64
	var samples int
	err := s.db.QueryRow(`SELECT
			AVG(CASE WHEN up = 1 THEN 1.0 ELSE 0.0 END) * 100,
			AVG(CASE WHEN up = 1 AND rtt_ms > 0 THEN rtt_ms END),
			MIN(CASE WHEN up = 1 AND rtt_ms > 0 THEN rtt_ms END),
			MAX(CASE WHEN up = 1 AND rtt_ms > 0 THEN rtt_ms END),
			COUNT(*)
		FROM metrics WHERE host_id = ? AND ts BETWEEN ? AND ?`,
		hostID, from.UnixMilli(), to.UnixMilli()).Scan(&avail, &avgR, &minR, &maxR, &samples)
	if err != nil {
		return nil, err
	}
	out.Availability = round2(nullOr(avail))
	out.AvgRTT = round2(nullOr(avgR))
	out.MinRTT = round2(nullOr(minR))
	out.MaxRTT = round2(nullOr(maxR))
	out.Samples = samples

	rows, err := s.db.Query(`SELECT (ts / ?) * ? AS bucket,
			AVG(CASE WHEN up = 1 AND rtt_ms > 0 THEN rtt_ms END),
			AVG(CASE WHEN up = 1 THEN 1.0 ELSE 0.0 END) * 100
		FROM metrics WHERE host_id = ? AND ts BETWEEN ? AND ?
		GROUP BY bucket ORDER BY bucket`, bms, bms, hostID, from.UnixMilli(), to.UnixMilli())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var b int64
		var rtt, uptime sql.NullFloat64
		if err := rows.Scan(&b, &rtt, &uptime); err != nil {
			return nil, err
		}
		ts := time.UnixMilli(b)
		if rtt.Valid {
			out.RTT = append(out.RTT, model.SeriesPoint{TS: ts, Value: round2(rtt.Float64)})
		}
		out.Uptime = append(out.Uptime, model.SeriesPoint{TS: ts, Value: round2(nullOr(uptime))})
	}
	return out, rows.Err()
}

// GlobalHistory returns fleet-wide average RTT and availability.
func (s *Store) GlobalHistory(from, to time.Time, maxPoints int) (*model.HostHistory, error) {
	if maxPoints < 10 {
		maxPoints = 120
	}
	span := to.Sub(from)
	if span <= 0 {
		span = 24 * time.Hour
	}
	bucket := span / time.Duration(maxPoints)
	if bucket < time.Minute {
		bucket = time.Minute
	}
	bms := bucket.Milliseconds()
	out := &model.HostHistory{From: from, To: to, RTT: []model.SeriesPoint{}, Uptime: []model.SeriesPoint{}}
	rows, err := s.db.Query(`SELECT (ts / ?) * ? AS bucket,
			AVG(CASE WHEN up = 1 AND rtt_ms > 0 THEN rtt_ms END),
			AVG(CASE WHEN up = 1 THEN 1.0 ELSE 0.0 END) * 100,
			COUNT(*)
		FROM metrics WHERE ts BETWEEN ? AND ?
		GROUP BY bucket ORDER BY bucket`, bms, bms, from.UnixMilli(), to.UnixMilli())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var b int64
		var rtt, uptime sql.NullFloat64
		var n int
		if err := rows.Scan(&b, &rtt, &uptime, &n); err != nil {
			return nil, err
		}
		ts := time.UnixMilli(b)
		out.Samples += n
		if rtt.Valid {
			out.RTT = append(out.RTT, model.SeriesPoint{TS: ts, Value: round2(rtt.Float64)})
		}
		out.Uptime = append(out.Uptime, model.SeriesPoint{TS: ts, Value: round2(nullOr(uptime))})
	}
	return out, rows.Err()
}

// MetricsForSite returns recent samples for one site (0 = all sites), newest
// first. It is used by the enterprise diagnostics analyzers.
func (s *Store) MetricsForSite(siteID int64, since time.Time, limit int) ([]model.Metric, error) {
	if limit < 1 {
		limit = 5000
	}
	if limit > 50000 {
		limit = 50000
	}
	query := `SELECT m.host_id, m.ts, m.rtt_ms, m.up, m.loss FROM metrics m`
	args := []any{}
	if siteID > 0 {
		query += ` JOIN hosts h ON h.id = m.host_id AND h.site_id = ?`
		args = append(args, siteID)
	}
	query += ` WHERE m.ts >= ? ORDER BY m.ts DESC LIMIT ?`
	args = append(args, since.UnixMilli(), limit)
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []model.Metric
	for rows.Next() {
		var m model.Metric
		var ts int64
		var up int
		if err := rows.Scan(&m.HostID, &ts, &m.RTTMS, &up, &m.Loss); err != nil {
			return nil, err
		}
		m.TS = time.UnixMilli(ts)
		m.Up = up == 1
		out = append(out, m)
	}
	return out, rows.Err()
}

// FleetSummary returns availability and average RTT since a time.
func (s *Store) FleetSummary(since time.Time) (availability, avgRTT float64, err error) {
	return s.FleetSummaryForSite(0, since)
}

// FleetSummaryForSite restricts the availability/RTT sample to one site.
func (s *Store) FleetSummaryForSite(siteID int64, since time.Time) (availability, avgRTT float64, err error) {
	var avail, rtt sql.NullFloat64
	query := `SELECT
			AVG(CASE WHEN m.up = 1 THEN 1.0 ELSE 0.0 END) * 100,
			AVG(CASE WHEN m.up = 1 AND m.rtt_ms > 0 THEN m.rtt_ms END)
		FROM metrics m`
	args := []any{since.UnixMilli()}
	if siteID > 0 {
		query += ` JOIN hosts h ON h.id = m.host_id AND h.site_id = ?`
		args = append(args, siteID)
	}
	query += ` WHERE m.ts >= ?`
	if siteID > 0 {
		args = []any{siteID, since.UnixMilli()}
	}
	err = s.db.QueryRow(query, args...).Scan(&avail, &rtt)
	if err == sql.ErrNoRows {
		return 0, 0, nil
	}
	return round2(nullOr(avail)), round2(nullOr(rtt)), err
}

// PurgeMetrics removes samples older than the retention window.
func (s *Store) PurgeMetrics(days int) error {
	if days < 1 {
		days = 30
	}
	cut := time.Now().AddDate(0, 0, -days).UnixMilli()
	if _, err := s.db.Exec(`DELETE FROM metrics WHERE ts < ?`, cut); err != nil {
		return err
	}
	_, err := s.db.Exec(`DELETE FROM host_events WHERE created_at < ?`, cut)
	return err
}

func nullOr(v sql.NullFloat64) float64 {
	if !v.Valid {
		return 0
	}
	return v.Float64
}

func round2(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	return math.Round(v*100) / 100
}
