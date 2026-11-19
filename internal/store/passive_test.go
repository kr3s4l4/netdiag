package store

import (
	"strings"
	"testing"
	"time"

	"github.com/kr3s4l4/netdiag/internal/model"
)

func TestPassiveObservationsRoundTripAndPurge(t *testing.T) {
	st := newTestStore(t)
	defer st.Close()
	now := time.Now()
	obs := []model.PassiveObservation{
		{SiteID: 1, ScanID: 7, Kind: "garp", Interface: "wlan0", SrcIP: "10.0.0.9",
			Detail: "GARP 10.0.0.9", ObservedAt: now},
		{SiteID: 1, ScanID: 7, Kind: "dhcp_server", SrcIP: "192.168.0.51",
			Detail: "DHCP OFFER", ObservedAt: now},
		{SiteID: 2, ScanID: 7, Kind: "icmp_redirect", SrcIP: "10.1.0.1", ObservedAt: now},
	}
	if err := st.SavePassiveObservations(obs); err != nil {
		t.Fatal(err)
	}
	rows, err := st.ListPassiveObservations(1, time.Time{}, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("filas inesperadas: %+v", rows)
	}
	if rows[0].Kind != "garp" && rows[1].Kind != "garp" {
		t.Fatalf("falta GARP: %+v", rows)
	}
	other, err := st.ListPassiveObservations(2, time.Time{}, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(other) != 1 || other[0].Kind != "icmp_redirect" {
		t.Fatalf("filtrado por sede incorrecto: %+v", other)
	}
	if err := st.PurgePassiveObservations(now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	purged, err := st.ListPassiveObservations(1, time.Time{}, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(purged) != 0 {
		t.Fatalf("la purga no eliminó las filas: %+v", purged)
	}
}

func TestPassiveDetailIsBounded(t *testing.T) {
	long := strings.Repeat("a", 400)
	if got := truncateDetail(long); len(got) != 253 {
		t.Fatalf("detalle no acotado: %d", len(got))
	}
}
