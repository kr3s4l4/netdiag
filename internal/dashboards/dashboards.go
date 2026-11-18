// Package dashboards validates user-defined views. Widgets can only read from
// a fixed catalog of already-authenticated API endpoints: a dashboard cannot
// turn into an arbitrary URL fetcher or bypass the application's RBAC.
package dashboards

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kr3s4l4/netdiag/internal/model"
)

// Item describes one available widget type.
type Item struct {
	Type  string `json:"type"`
	Label string `json:"label"`
	// Source is the relative API path the SPA reads for this widget.
	Source string `json:"source"`
}

var catalog = []Item{
	{Type: "dashboard", Label: "Resumen", Source: "/api/v1/dashboard"},
	{Type: "hosts", Label: "Dispositivos", Source: "/api/v1/hosts?limit=6"},
	{Type: "ports", Label: "Puertos frecuentes", Source: "/api/v1/ports/summary"},
	{Type: "topology", Label: "Topología", Source: "/api/v1/topology"},
	{Type: "alerts", Label: "Alertas", Source: "/api/v1/alerts/stats"},
	{Type: "scans", Label: "Escaneos recientes", Source: "/api/v1/scans?limit=5"},
	{Type: "wifi", Label: "APs WiFi", Source: "/api/v1/wifi/aps?limit=8"},
	{Type: "vpn", Label: "Túneles VPN", Source: "/api/v1/vpn/tunnels"},
}

// Catalog returns a copy of the widget catalog.
func Catalog() []Item {
	out := make([]Item, len(catalog))
	copy(out, catalog)
	return out
}

// ItemFor returns the catalog entry for a widget type.
func ItemFor(widgetType string) (Item, bool) {
	for _, item := range catalog {
		if item.Type == widgetType {
			return item, true
		}
	}
	return Item{}, false
}

// Normalize fills defaults and rejects unsupported types or positions. The
// grid has 12 columns and at most 16 widgets per dashboard.
func Normalize(d *model.Dashboard) error {
	if d == nil {
		return fmt.Errorf("dashboard vacío")
	}
	d.Name = strings.TrimSpace(d.Name)
	if d.Name == "" || len([]rune(d.Name)) > 80 {
		return fmt.Errorf("el nombre del dashboard debe tener entre 1 y 80 caracteres")
	}
	if d.SiteID <= 0 {
		d.SiteID = 1
	}
	if len(d.Widgets) > 16 {
		return fmt.Errorf("un dashboard admite como máximo 16 widgets")
	}
	seen := map[string]bool{}
	widths := map[int]bool{2: true, 3: true, 4: true, 6: true, 12: true}
	for i := range d.Widgets {
		w := &d.Widgets[i]
		w.Type = strings.ToLower(strings.TrimSpace(w.Type))
		item, ok := ItemFor(w.Type)
		if !ok {
			return fmt.Errorf("tipo de widget no soportado: %q", w.Type)
		}
		if w.ID == "" {
			w.ID = fmt.Sprintf("w%d", i+1)
		}
		if seen[w.ID] {
			return fmt.Errorf("identificador de widget duplicado: %s", w.ID)
		}
		seen[w.ID] = true
		if w.Position.X < 0 || w.Position.X > 11 || w.Position.Y < 0 || w.Position.Y > 23 {
			return fmt.Errorf("widget %s fuera de la rejilla", w.ID)
		}
		if !widths[w.Size.W] {
			w.Size.W = 4
		}
		if w.Size.H < 1 || w.Size.H > 3 {
			w.Size.H = 1
		}
		if w.Config == nil {
			w.Config = map[string]any{}
		}
		// Regardless of client input, source is always derived from the
		// validated catalog type.
		w.Config["source"] = item.Source
		if title, ok := w.Config["title"].(string); !ok || strings.TrimSpace(title) == "" {
			w.Config["title"] = item.Label
		}
	}
	sort.SliceStable(d.Widgets, func(i, j int) bool {
		if d.Widgets[i].Position.Y != d.Widgets[j].Position.Y {
			return d.Widgets[i].Position.Y < d.Widgets[j].Position.Y
		}
		return d.Widgets[i].Position.X < d.Widgets[j].Position.X
	})
	return nil
}
