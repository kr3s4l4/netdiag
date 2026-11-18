package store

import (
	"testing"
	"time"

	"github.com/kr3s4l4/netdiag/internal/model"
)

func TestARPObservationsRoundTrip(t *testing.T) {
	st := newTestStore(t)
	now := time.Now()
	in := []model.ARPObservation{
		{SiteID: 1, ScanID: 4, IP: "192.168.0.1", MAC: "aa:bb:cc:dd:ee:01", IsGateway: true, ObservedAt: now.Add(-time.Hour)},
		{SiteID: 1, ScanID: 5, IP: "192.168.0.1", MAC: "aa:bb:cc:dd:ee:02", IsGateway: true, ObservedAt: now},
		{SiteID: 2, ScanID: 5, IP: "10.0.0.9", MAC: "aa:bb:cc:dd:ee:09", ObservedAt: now},
	}
	if err := st.SaveARPObservations(in); err != nil {
		t.Fatal(err)
	}
	got, err := st.ListARPObservations(1, time.Time{}, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("esperaba 2 observaciones de la sede 1, obtuve %d", len(got))
	}
	if !got[0].IsGateway || got[0].MAC != "aa:bb:cc:dd:ee:01" || got[1].MAC != "aa:bb:cc:dd:ee:02" {
		t.Fatalf("historial incorrecto: %+v", got)
	}
	if got[0].ObservedAt.After(got[1].ObservedAt) {
		t.Fatal("el historial debe venir ordenado por fecha")
	}
	all, err := st.ListARPObservations(0, time.Time{}, 100)
	if err != nil || len(all) != 3 {
		t.Fatalf("site_id=0 debe devolver todas: %d, %v", len(all), err)
	}
}

func TestPurgeARPObservations(t *testing.T) {
	st := newTestStore(t)
	now := time.Now()
	if err := st.SaveARPObservations([]model.ARPObservation{
		{SiteID: 1, ScanID: 1, IP: "10.0.0.1", MAC: "aa:bb:cc:dd:ee:01", ObservedAt: now.Add(-48 * time.Hour)},
		{SiteID: 1, ScanID: 2, IP: "10.0.0.1", MAC: "aa:bb:cc:dd:ee:01", ObservedAt: now},
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.PurgeARPObservations(now.Add(-24 * time.Hour)); err != nil {
		t.Fatal(err)
	}
	got, err := st.ListARPObservations(1, time.Time{}, 100)
	if err != nil || len(got) != 1 {
		t.Fatalf("esperaba 1 observación tras purgar: %d, %v", len(got), err)
	}
}
