package store

import (
	"testing"
	"time"

	"github.com/kr3s4l4/netdiag/internal/model"
)

func TestDHCPObservationsRoundTrip(t *testing.T) {
	st := newTestStore(t)
	now := time.Now()
	in := []model.DHCPObservation{
		{SiteID: 1, ScanID: 1, IP: "192.168.0.1", ServerID: "192.168.0.1", MAC: "11:22:33:44:55:01",
			MessageType: "ack", Gateway: "192.168.0.1", Interface: "wlan0", Routers: []string{"192.168.0.1"},
			DNS: []string{"1.1.1.1", "8.8.8.8"}, LeaseSeconds: 3600, ObservedAt: now.Add(-time.Hour)},
		{SiteID: 2, ScanID: 2, IP: "10.0.0.1", ServerID: "10.0.0.1", MessageType: "offer", ObservedAt: now},
	}
	if err := st.SaveDHCPObservations(in); err != nil {
		t.Fatal(err)
	}
	got, err := st.ListDHCPObservations(1, time.Time{}, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("esperaba 1 observación de la sede 1, obtuve %d", len(got))
	}
	row := got[0]
	if row.ServerID != "192.168.0.1" || row.MessageType != "ack" || row.LeaseSeconds != 3600 {
		t.Fatalf("observación incorrecta: %+v", row)
	}
	if len(row.Routers) != 1 || row.Routers[0] != "192.168.0.1" || len(row.DNS) != 2 {
		t.Fatalf("opciones IP perdidas: %+v", row)
	}
	all, err := st.ListDHCPObservations(0, time.Time{}, 100)
	if err != nil || len(all) != 2 {
		t.Fatalf("site_id=0 debe devolver todas: %d, %v", len(all), err)
	}
}

func TestPurgeDHCPObservations(t *testing.T) {
	st := newTestStore(t)
	now := time.Now()
	if err := st.SaveDHCPObservations([]model.DHCPObservation{
		{SiteID: 1, IP: "192.168.0.1", ObservedAt: now.Add(-48 * time.Hour)},
		{SiteID: 1, IP: "192.168.0.1", ObservedAt: now},
	}); err != nil {
		t.Fatal(err)
	}
	if err := st.PurgeDHCPObservations(now.Add(-24 * time.Hour)); err != nil {
		t.Fatal(err)
	}
	got, err := st.ListDHCPObservations(1, time.Time{}, 100)
	if err != nil || len(got) != 1 {
		t.Fatalf("esperaba 1 observación tras purgar: %d, %v", len(got), err)
	}
}
