package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/kr3s4l4/netdiag/internal/model"
)

func encodeWidgets(widgets []model.Widget) (string, error) {
	if widgets == nil {
		widgets = []model.Widget{}
	}
	raw, err := json.Marshal(widgets)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func decodeWidgets(raw string) []model.Widget {
	widgets := []model.Widget{}
	if raw != "" && json.Unmarshal([]byte(raw), &widgets) != nil {
		return widgets
	}
	return widgets
}

func scanDashboard(scanner interface{ Scan(...any) error }) (*model.Dashboard, error) {
	var d model.Dashboard
	var raw string
	var created, modified int64
	if err := scanner.Scan(&d.ID, &d.SiteID, &d.Name, &raw, &created, &modified); err != nil {
		return nil, err
	}
	d.Widgets = decodeWidgets(raw)
	d.CreatedAt = time.UnixMilli(created)
	d.Modified = time.UnixMilli(modified)
	return &d, nil
}

// CreateDashboard persists a new view and populates its ID/timestamps.
func (s *Store) CreateDashboard(d *model.Dashboard) error {
	raw, err := encodeWidgets(d.Widgets)
	if err != nil {
		return err
	}
	now := nowMS()
	res, err := s.db.Exec(`INSERT INTO dashboards(site_id, name, widgets, created_at, modified) VALUES(?,?,?,?,?)`,
		d.SiteID, d.Name, raw, now, now)
	if err != nil {
		return err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return err
	}
	d.ID = id
	d.CreatedAt = time.UnixMilli(now)
	d.Modified = d.CreatedAt
	return nil
}

// ListDashboards returns views for a site (0 = all sites).
func (s *Store) ListDashboards(siteID int64) ([]model.Dashboard, error) {
	query := `SELECT id, site_id, name, widgets, created_at, modified FROM dashboards`
	args := []any{}
	if siteID > 0 {
		query += ` WHERE site_id = ?`
		args = append(args, siteID)
	}
	query += ` ORDER BY name, id`
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Dashboard{}
	for rows.Next() {
		d, err := scanDashboard(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *d)
	}
	return out, rows.Err()
}

// GetDashboard returns one view by id.
func (s *Store) GetDashboard(id int64) (*model.Dashboard, error) {
	d, err := scanDashboard(s.db.QueryRow(`SELECT id, site_id, name, widgets, created_at, modified FROM dashboards WHERE id = ?`, id))
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return d, nil
}

// UpdateDashboard replaces the name/widgets of an existing view.
func (s *Store) UpdateDashboard(d *model.Dashboard) error {
	raw, err := encodeWidgets(d.Widgets)
	if err != nil {
		return err
	}
	now := nowMS()
	res, err := s.db.Exec(`UPDATE dashboards SET site_id=?, name=?, widgets=?, modified=? WHERE id=?`,
		d.SiteID, d.Name, raw, now, d.ID)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	d.Modified = time.UnixMilli(now)
	return nil
}

// DeleteDashboard removes one view.
func (s *Store) DeleteDashboard(id int64) error {
	res, err := s.db.Exec(`DELETE FROM dashboards WHERE id = ?`, id)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// DashboardExists is a small helper for validation paths.
func (s *Store) DashboardExists(id int64) bool {
	var one int
	return s.db.QueryRow(`SELECT 1 FROM dashboards WHERE id = ?`, id).Scan(&one) == nil
}

// ExampleDashboard builds the "Mi red" view used in the F18 closure test.
func ExampleDashboard(siteID int64) *model.Dashboard {
	types := []string{"dashboard", "hosts", "alerts", "topology"}
	d := &model.Dashboard{SiteID: siteID, Name: "Mi red"}
	for i, t := range types {
		var w model.Widget
		w.ID = fmt.Sprintf("w%d", i+1)
		w.Type = t
		w.Position.X = (i % 2) * 6
		w.Position.Y = i / 2
		w.Size.W = 6
		w.Size.H = 1
		w.Config = map[string]any{}
		d.Widgets = append(d.Widgets, w)
	}
	return d
}
