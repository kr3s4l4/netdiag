package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/kr3s4l4/netdiag/internal/history"
	"github.com/kr3s4l4/netdiag/internal/model"
)

type historyPeriodRequest struct {
	Label string `json:"label,omitempty"`
	From  string `json:"from,omitempty"`
	To    string `json:"to,omitempty"`
}

type historyCompareRequest struct {
	SiteID int64                 `json:"site_id,omitempty"`
	Days   int                   `json:"days,omitempty"`
	Window string                `json:"window,omitempty"`
	A      *historyPeriodRequest `json:"period_a,omitempty"`
	B      *historyPeriodRequest `json:"period_b,omitempty"`
}

// handleHistoryCompare compares two moments. By default it answers the F21
// closure case: today (last 24 h) vs the same window seven days ago.
func (s *Server) handleHistoryCompare(w http.ResponseWriter, r *http.Request) {
	var req historyCompareRequest
	if r.Body != nil && r.ContentLength != 0 {
		if err := decode(r, &req); err != nil {
			s.fail(w, r, http.StatusBadRequest, "cuerpo inválido: "+err.Error())
			return
		}
	}
	if req.SiteID == 0 {
		req.SiteID = queryInt64(r, "site_id")
	}
	if req.Days == 0 {
		req.Days = queryInt(r, "days", 7)
	}
	if req.Window == "" {
		req.Window = queryString(r, "window")
	}
	if req.Days < 1 || req.Days > 365 {
		s.fail(w, r, http.StatusBadRequest, "days debe estar entre 1 y 365")
		return
	}
	window := 24 * time.Hour
	if req.Window != "" {
		parsed, ok := windowDuration(req.Window)
		if !ok || parsed < time.Hour || parsed > 30*24*time.Hour {
			s.fail(w, r, http.StatusBadRequest, "window debe estar entre 1h y 30d")
			return
		}
		window = parsed
	}

	now := time.Now()
	defA, defB := history.DefaultPeriods(now, req.Days, window)
	periodA, err := resolvePeriod(req.A, defA, now)
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, "period_a: "+err.Error())
		return
	}
	periodB, err := resolvePeriod(req.B, defB, now)
	if err != nil {
		s.fail(w, r, http.StatusBadRequest, "period_b: "+err.Error())
		return
	}

	report, err := history.BuildReport(s.st, req.SiteID, periodA, periodB)
	if err != nil {
		s.fail(w, r, http.StatusInternalServerError, err.Error())
		return
	}
	s.audit(r, "history.compare", periodA.Label+" vs "+periodB.Label, map[string]any{
		"site_id": req.SiteID, "changes": len(report.Changes),
		"from_a": periodA.From, "to_a": periodA.To,
		"from_b": periodB.From, "to_b": periodB.To,
	})
	writeJSON(w, http.StatusOK, report)
}

func resolvePeriod(req *historyPeriodRequest, def model.HistoryPeriod, now time.Time) (model.HistoryPeriod, error) {
	period := def
	if req == nil {
		return period, nil
	}
	if strings.TrimSpace(req.Label) != "" {
		period.Label = strings.TrimSpace(req.Label)
	}
	var err error
	if period.From, err = resolveTime(req.From, def.From, now); err != nil {
		return period, fmt.Errorf("from inválido: %w", err)
	}
	if period.To, err = resolveTime(req.To, def.To, now); err != nil {
		return period, fmt.Errorf("to inválido: %w", err)
	}
	if !period.From.Before(period.To) {
		return period, errors.New("from debe ser anterior a to")
	}
	if period.To.Sub(period.From) > 60*24*time.Hour {
		return period, errors.New("la ventana no puede superar los 60 días")
	}
	return period, nil
}

func resolveTime(raw string, fallback, now time.Time) (time.Time, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return fallback, nil
	}
	if t, err := time.Parse(time.RFC3339, raw); err == nil {
		return t, nil
	}
	if d, ok := windowDuration(raw); ok {
		return now.Add(-d), nil
	}
	return time.Time{}, fmt.Errorf("usa RFC3339 o una duración (24h, 7d, 2w): %q", raw)
}
