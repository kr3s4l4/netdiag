package probe

import (
	"bytes"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/kr3s4l4/netdiag/internal/cryptoutil"
	"github.com/kr3s4l4/netdiag/internal/model"
	"github.com/kr3s4l4/netdiag/internal/store"
	"github.com/kr3s4l4/netdiag/internal/update"
)

func TestSignVerifyRoundTrip(t *testing.T) {
	seed := bytes.Repeat([]byte{0x11}, 32)
	keyID := []byte("NDTEST01")
	publicKey, err := update.PublicKeyFromSeed(seed, keyID)
	if err != nil {
		t.Fatal(err)
	}
	d := New(1)
	d.ARP = append(d.ARP, model.ARPObservation{
		SiteID: 1, IP: "192.168.1.1", MAC: "aa:bb:cc:dd:ee:ff", ObservedAt: time.Unix(1, 0).UTC(),
	})
	if err := d.Sign(seed, keyID, "prueba"); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(d); err != nil {
		t.Fatal(err)
	}
	var loaded Document
	if err := json.Unmarshal(buf.Bytes(), &loaded); err != nil {
		t.Fatal(err)
	}
	if err := loaded.Verify(publicKey); err != nil {
		t.Fatalf("firma válida rechazada: %v", err)
	}
	loaded.SiteID = 2
	if err := loaded.Verify(publicKey); err == nil {
		t.Fatal("se aceptó una sonda modificada")
	}
}

func TestImportWritesObservations(t *testing.T) {
	dir := t.TempDir()
	box, err := cryptoutil.LoadOrCreate(filepath.Join(dir, "secret.key"))
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(dir, "netdiag.db"), box)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	site, err := st.CreateSite("Prueba", "")
	if err != nil {
		t.Fatal(err)
	}

	d := New(site.ID)
	d.ARP = append(d.ARP, model.ARPObservation{IP: "10.0.0.1", MAC: "aa:bb:cc:dd:ee:ff"})
	d.SNMP = append(d.SNMP, SNMPRecord{
		HostIP: "10.0.0.1",
		Data: model.SNMPSummary{
			SysName:   "switch",
			Neighbors: []model.Neighbor{{Protocol: "lldp", RemoteName: "core"}},
		},
	})
	report, err := Import(st, d)
	if err != nil {
		t.Fatal(err)
	}
	if report.ARP != 1 || report.SNMP != 1 || report.Neighbors != 1 {
		t.Fatalf("import incompleto: %+v", report)
	}
	arp, err := st.ListARPObservations(site.ID, time.Time{}, 10)
	if err != nil || len(arp) != 1 {
		t.Fatalf("ARP no persistido: %v %+v", err, arp)
	}
}
