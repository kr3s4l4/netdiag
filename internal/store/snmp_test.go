package store

import (
	"testing"
	"time"

	"github.com/kr3s4l4/netdiag/internal/model"
)

func TestSNMPDeepRoundTripAndPurge(t *testing.T) {
	st := newTestStore(t)
	defer st.Close()
	now := time.Now()
	deep := model.SNMPDeep{
		IPAddresses: []model.SNMPIPAddress{{IP: "10.0.0.2", IfIndex: 2, Mask: "255.255.255.0"}},
		ARP: []model.SNMPARPRow{{
			IP: "192.168.0.1", MAC: "aa:bb:cc:dd:ee:01", IfIndex: 2,
			Type: "dynamic", Table: "net_to_media",
		}},
		VLANs: []model.SNMPVLAN{{VLAN: 10, Name: "Red"}},
		FDB: []model.SNMPFDBRow{{
			MAC: "aa:bb:cc:dd:ee:01", Port: 5, IfIndex: 7, VLAN: 10,
			Status: "learned", Table: "q_bridge",
		}},
	}
	if err := st.SaveSNMPDeep(1, 7, "10.0.0.2", "pu***ic", now, deep); err != nil {
		t.Fatal(err)
	}
	snap, err := st.ListSNMPDeep(1, time.Time{}, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.IPAddresses) != 1 || len(snap.ARP) != 1 || len(snap.VLANs) != 1 || len(snap.FDB) != 1 {
		t.Fatalf("filas inesperadas: %+v", snap)
	}
	if snap.ARP[0].Community != "pu***ic" || snap.ARP[0].HostIP != "10.0.0.2" || snap.FDB[0].VLAN != 10 {
		t.Fatalf("metadatos inesperados: %+v", snap)
	}
	other, err := st.ListSNMPDeep(2, time.Time{}, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(other.FDB) != 0 {
		t.Fatalf("la sede 2 no tenía filas: %+v", other.FDB)
	}
	if err := st.PurgeSNMPDeep(now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	purged, err := st.ListSNMPDeep(1, time.Time{}, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(purged.IPAddresses)+len(purged.ARP)+len(purged.VLANs)+len(purged.FDB) != 0 {
		t.Fatalf("la purga no eliminó las filas: %+v", purged)
	}
}
