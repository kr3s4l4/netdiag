package api

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/kr3s4l4/netdiag/internal/fingerprint"
	"github.com/kr3s4l4/netdiag/internal/model"
)

func (s *Server) handleDashboard(w http.ResponseWriter, r *http.Request) {
	stats, err := s.st.Dashboard()
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "no se pudo calcular el panel: "+err.Error())
		return
	}
	ports, err := s.st.ListPortsSummary()
	if err != nil {
		ports = map[int]int{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"stats":      stats,
		"top_ports":  ports,
		"ws_clients": s.hub.Clients(),
	})
}

func (s *Server) hostFilter(r *http.Request) model.HostFilter {
	return model.HostFilter{
		Search:     queryString(r, "search"),
		Status:     queryString(r, "status"),
		DeviceType: queryString(r, "device_type"),
		NetworkID:  queryInt64(r, "network_id"),
		Tag:        queryString(r, "tag"),
		Favorite:   queryBool(r, "favorite"),
		Sort:       queryString(r, "sort"),
		Desc:       !queryBool(r, "asc"),
		Page:       queryInt(r, "page", 1),
		Limit:      queryInt(r, "limit", 50),
	}
}

func (s *Server) handleListHosts(w http.ResponseWriter, r *http.Request) {
	res, err := s.st.ListHosts(s.hostFilter(r))
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, "no se pudo listar dispositivos: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleGetHost(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	host, err := s.st.GetHost(id)
	if err != nil {
		if isNotFound(err) {
			s.fail(w, r, http.StatusNotFound, "dispositivo no encontrado")
			return
		}
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	events, _ := s.st.ListEvents(id, 30)
	neighbors, _ := s.st.ListNeighbors(id)
	writeJSON(w, http.StatusOK, map[string]any{
		"host":      host,
		"events":    events,
		"neighbors": neighbors,
	})
}

type hostPatch struct {
	Notes    *string   `json:"notes"`
	Tags     *[]string `json:"tags"`
	Favorite *bool     `json:"favorite"`
}

func (s *Server) handleUpdateHost(w http.ResponseWriter, r *http.Request) {
	if !s.canWrite(w, r) {
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	var req hostPatch
	if err := decode(r, &req); err != nil {
		s.fail(w, r, http.StatusBadRequest, "cuerpo inválido: "+err.Error())
		return
	}
	if req.Notes != nil {
		n := strings.TrimSpace(*req.Notes)
		if len(n) > 4000 {
			s.fail(w, r, http.StatusBadRequest, "la nota supera 4000 caracteres")
			return
		}
		req.Notes = &n
	}
	if req.Tags != nil {
		if len(*req.Tags) > 32 {
			s.fail(w, r, http.StatusBadRequest, "máximo 32 etiquetas")
			return
		}
		clean := make([]string, 0, len(*req.Tags))
		for _, t := range *req.Tags {
			t = strings.TrimSpace(t)
			if t == "" {
				continue
			}
			if len(t) > 40 {
				t = t[:40]
			}
			clean = append(clean, t)
		}
		req.Tags = &clean
	}
	if err := s.st.UpdateHostMeta(id, req.Notes, req.Tags, req.Favorite); err != nil {
		if isNotFound(err) {
			s.fail(w, r, http.StatusNotFound, "dispositivo no encontrado")
			return
		}
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	host, err := s.st.GetHost(id)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	s.audit(r, "host.updated", host.IP, nil)
	s.hub.Broadcast("host.updated", host)
	writeJSON(w, http.StatusOK, host)
}

func (s *Server) handleDeleteHost(w http.ResponseWriter, r *http.Request) {
	if !s.requireAdmin(w, r) {
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	host, _ := s.st.GetHost(id)
	if err := s.st.DeleteHost(id); err != nil {
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	target := strconv.FormatInt(id, 10)
	if host != nil {
		target = host.IP
	}
	s.audit(r, "host.deleted", target, nil)
	s.hub.Broadcast("host.deleted", map[string]any{"id": id, "ip": target})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleHostEvents(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	events, err := s.st.ListEvents(id, queryInt(r, "limit", 100))
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": events})
}

func (s *Server) handleHostHistory(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	from, to := parseWindow(r, 24*time.Hour)
	hist, err := s.st.HostHistory(id, from, to, queryInt(r, "points", 240))
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, hist)
}

func (s *Server) handleHostNeighbors(w http.ResponseWriter, r *http.Request) {
	id, err := pathInt64(r, "id")
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	neighbors, err := s.st.ListNeighbors(id)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": neighbors})
}

type rescanRequest struct {
	Profile string `json:"profile"`
	SNMP    *bool  `json:"snmp,omitempty"`
	Deep    bool   `json:"deep"`
}

func (s *Server) handleRescanHost(w http.ResponseWriter, r *http.Request) {
	if !s.canWrite(w, r) {
		return
	}
	id, err := pathInt64(r, "id")
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, err.Error())
		return
	}
	host, err := s.st.GetHost(id)
	if err != nil {
		if isNotFound(err) {
			s.fail(w, r, http.StatusNotFound, "dispositivo no encontrado")
			return
		}
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	var req rescanRequest
	if r.ContentLength > 0 {
		if err := decode(r, &req); err != nil {
			s.fail(w, r, http.StatusBadRequest, "cuerpo inválido: "+err.Error())
			return
		}
	}
	if req.Profile == "" {
		req.Profile = "full"
	}
	scan, err := s.scanner.StartScan(ScanRequest{
		NetworkID: host.NetworkID,
		CIDR:      host.IP,
		Profile:   req.Profile,
		SNMP:      req.SNMP,
		Deep:      req.Deep,
	}, s.actorName(r))
	if err != nil {
		s.fail(w, r, http.StatusConflict, err.Error())
		return
	}
	s.audit(r, "scan.host", host.IP, map[string]any{"scan_id": scan.ID})
	s.hub.Broadcast("scan.started", scan)
	s.created(w, scan)
}

func (s *Server) handlePortsSummary(w http.ResponseWriter, r *http.Request) {
	ports, err := s.st.ListPortsSummary()
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	type portRow struct {
		Port    int    `json:"port"`
		Service string `json:"service"`
		Hosts   int    `json:"hosts"`
	}
	rows := make([]portRow, 0, len(ports))
	for p, n := range ports {
		rows = append(rows, portRow{Port: p, Service: fingerprint.ServiceName(p), Hosts: n})
	}
	for i := 0; i < len(rows); i++ {
		for j := i + 1; j < len(rows); j++ {
			if rows[j].Hosts > rows[i].Hosts {
				rows[i], rows[j] = rows[j], rows[i]
			}
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"items": rows, "total": len(rows)})
}

func (s *Server) handleExportHosts(w http.ResponseWriter, r *http.Request) {
	filter := s.hostFilter(r)
	filter.Page = 1
	filter.Limit = 500
	var all []model.Host
	for {
		res, err := s.st.ListHosts(filter)
		if err != nil {
			s.fail(w, r, http.StatusInternalServerError, err.Error())
			return
		}
		all = append(all, res.Items...)
		if len(res.Items) < filter.Limit || len(all) >= 20000 {
			break
		}
		filter.Page++
	}
	format := strings.ToLower(queryString(r, "format"))
	stamp := time.Now().Format("20060102-1504")
	switch format {
	case "", "csv":
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="netdiag-inventario-%s.csv"`, stamp))
		cw := csv.NewWriter(w)
		_ = cw.Write([]string{"ip", "mac", "hostname", "tipo", "fabricante", "modelo", "so", "estado", "puertos_abiertos", "latencia_ms", "puerta_de_enlace", "red", "primera_vez", "ultima_vez"})
		for _, h := range all {
			_ = cw.Write([]string{
				h.IP, h.MAC, h.Hostname, fingerprint.TypeLabel(h.DeviceType), h.Vendor, h.Model, h.OS,
				string(h.Status), strconv.Itoa(h.OpenPortCount), strconv.FormatFloat(h.LatencyMS, 'f', 2, 64),
				strconv.FormatBool(h.IsGateway), strconv.FormatInt(h.NetworkID, 10),
				h.FirstSeen.Format(time.RFC3339), h.LastSeen.Format(time.RFC3339),
			})
		}
		cw.Flush()
	case "json":
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="netdiag-inventario-%s.json"`, stamp))
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{"generated_at": time.Now(), "items": all, "total": len(all)})
	default:
		s.fail(w, r, http.StatusBadRequest, "formato no soportado; usa csv o json")
	}
}

// parseWindow reads from/to (RFC3339 or durations like 24h) and falls back to
// the provided default window.
// windowDuration accepts Go durations plus the day and week suffixes used by
// the UI (24h, 7d, 30d, 2w). A bare number is interpreted as hours.
func windowDuration(v string) (time.Duration, bool) {
	v = strings.TrimSpace(strings.ToLower(v))
	if v == "" {
		return 0, false
	}
	if strings.HasSuffix(v, "d") || strings.HasSuffix(v, "w") {
		unit := 24 * time.Hour
		if strings.HasSuffix(v, "w") {
			unit = 7 * 24 * time.Hour
		}
		n, err := strconv.Atoi(strings.TrimSuffix(strings.TrimSuffix(v, "d"), "w"))
		if err != nil || n <= 0 {
			return 0, false
		}
		return time.Duration(n) * unit, true
	}
	if d, err := time.ParseDuration(v); err == nil && d > 0 {
		return d, true
	}
	return 0, false
}

func parseWindow(r *http.Request, def time.Duration) (time.Time, time.Time) {
	now := time.Now()
	to := now
	if v := queryString(r, "to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			to = t
		} else if d, ok := windowDuration(v); ok {
			to = now.Add(-d)
		}
	}
	from := to.Add(-def)
	if v := queryString(r, "from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			from = t
		} else if d, ok := windowDuration(v); ok {
			from = to.Add(-d)
		}
	} else if v := queryString(r, "range"); v != "" {
		if d, ok := windowDuration(v); ok {
			from = to.Add(-d)
		}
	}
	if !from.Before(to) {
		from = to.Add(-def)
	}
	return from, to
}

func (s *Server) actorName(r *http.Request) string {
	if u := userFrom(r); u != nil {
		return u.Username
	}
	return "sistema"
}
