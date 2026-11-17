package api

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestWindowDuration(t *testing.T) {
	cases := []struct {
		in   string
		want time.Duration
		ok   bool
	}{
		{"90m", 90 * time.Minute, true},
		{"24h", 24 * time.Hour, true},
		{"7d", 7 * 24 * time.Hour, true},
		{"30D", 30 * 24 * time.Hour, true},
		{"2w", 14 * 24 * time.Hour, true},
		{"", 0, false},
		{"0d", 0, false},
		{"-3d", 0, false},
		{"abc", 0, false},
	}
	for _, c := range cases {
		got, ok := windowDuration(c.in)
		if ok != c.ok || got != c.want {
			t.Errorf("windowDuration(%q) = %v, %v; se esperaba %v, %v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestParseWindowRange(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/hosts/1/history?range=7d", nil)
	from, to := parseWindow(req, 24*time.Hour)
	span := to.Sub(from)
	if span < 7*24*time.Hour-time.Minute || span > 7*24*time.Hour+time.Minute {
		t.Fatalf("ventana de 7d inesperada: %v", span)
	}

	// from tiene prioridad sobre range y admite el formato del frontend.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/hosts/1/history?range=7d&from=30d", nil)
	from, to = parseWindow(req, 24*time.Hour)
	span = to.Sub(from)
	if span < 30*24*time.Hour-time.Minute || span > 30*24*time.Hour+time.Minute {
		t.Fatalf("from debería ganar a range: %v", span)
	}

	// Sin parámetros se aplica la ventana por defecto.
	req = httptest.NewRequest(http.MethodGet, "/api/v1/hosts/1/history", nil)
	from, to = parseWindow(req, 24*time.Hour)
	span = to.Sub(from)
	if span < 24*time.Hour-time.Minute || span > 24*time.Hour+time.Minute {
		t.Fatalf("ventana por defecto inesperada: %v", span)
	}
}
