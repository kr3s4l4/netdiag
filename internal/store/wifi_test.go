package store

import (
	"testing"

	"github.com/kr3s4l4/netdiag/internal/model"
)

func TestWifiObservationsPersistAndClearBySite(t *testing.T) {
	st := newTestStore(t)
	ap := &model.WifiAP{
		SiteID: 1, BSSID: "00:11:22:33:44:55", SSID: "Corp", Channel: 6, Band: "2.4GHz",
		Encryption: "WPA2", SignalDBM: -45, Beacons: 3, Clients: 1,
		Rogue: true, RogueInfo: "BSSID no autorizado", RogueConfidence: 90,
	}
	if err := st.UpsertWifiAP(ap); err != nil {
		t.Fatal(err)
	}
	ap.Beacons = 2
	ap.SignalDBM = -40
	if err := st.UpsertWifiAP(ap); err != nil {
		t.Fatal(err)
	}
	if ap.ID == 0 || ap.FirstSeen.IsZero() {
		t.Fatal("AP no devolvió id/first_seen")
	}
	client := &model.WifiClient{SiteID: 1, MAC: "de:ad:be:ef:00:01", APBSSID: ap.BSSID, SignalDBM: -60}
	if err := st.UpsertWifiClient(client); err != nil {
		t.Fatal(err)
	}
	others := &model.WifiAP{SiteID: 2, BSSID: "aa:bb:cc:dd:ee:ff", SSID: "Otra", Channel: 36, Band: "5GHz"}
	if err := st.UpsertWifiAP(others); err != nil {
		t.Fatal(err)
	}

	aps, err := st.ListWifiAPs(1, 100)
	if err != nil || len(aps) != 1 {
		t.Fatalf("listado sede 1: %v %+v", err, aps)
	}
	if aps[0].Beacons != 5 || aps[0].SignalDBM != -40 || !aps[0].Rogue || aps[0].RogueConfidence != 90 {
		t.Fatalf("acumulación incorrecta: %+v", aps[0])
	}
	clients, err := st.ListWifiClients(1, 100)
	if err != nil || len(clients) != 1 || clients[0].ID == 0 {
		t.Fatalf("listado clientes: %v %+v", err, clients)
	}
	if err := st.ClearWifiObservations(1); err != nil {
		t.Fatal(err)
	}
	aps, err = st.ListWifiAPs(1, 100)
	if err != nil || len(aps) != 0 {
		t.Fatalf("observaciones no borradas: %v %+v", err, aps)
	}
	aps, err = st.ListWifiAPs(2, 100)
	if err != nil || len(aps) != 1 {
		t.Fatalf("borrado afectó a otra sede: %v %+v", err, aps)
	}
}
