package snmp

import (
	"testing"

	"github.com/gosnmp/gosnmp"
)

func TestParseIPAddrTable(t *testing.T) {
	rows := parseIPAddrTable(
		map[string]string{"192.168.0.1": "192.168.0.1"},
		map[string]string{"192.168.0.1": "2"},
		map[string]string{"192.168.0.1": "255.255.255.0"},
	)
	if len(rows) != 1 || rows[0].IP != "192.168.0.1" || rows[0].IfIndex != 2 || rows[0].Mask != "255.255.255.0" {
		t.Fatalf("fila inesperada: %+v", rows)
	}
	bad := parseIPAddrTable(map[string]string{"no-es-ip": "x"}, nil, nil)
	if len(bad) != 0 {
		t.Fatalf("se aceptó una IP inválida: %+v", bad)
	}
}

func TestParseNetToMedia(t *testing.T) {
	suffix := "2.192.168.0.10"
	rows := parseNetToMedia(
		map[string]string{suffix: "2"},
		map[string]string{suffix: "aa:bb:cc:dd:ee:10"},
		map[string]string{suffix: "192.168.0.10"},
		map[string]string{suffix: "3"},
	)
	if len(rows) != 1 {
		t.Fatalf("esperaba 1 fila: %+v", rows)
	}
	row := rows[0]
	if row.IP != "192.168.0.10" || row.MAC != "aa:bb:cc:dd:ee:10" || row.IfIndex != 2 || row.Type != "dynamic" || row.Table != "net_to_media" {
		t.Fatalf("fila inesperada: %+v", row)
	}
}

func TestParseNetToPhysical(t *testing.T) {
	suffix := "3.1.4.192.168.0.11"
	rows := parseNetToPhysical(
		map[string]string{suffix: "3"},
		map[string]string{suffix: "192.168.0.11"},
		map[string]string{suffix: "00:11:22:33:44:0b"},
		map[string]string{suffix: "4"},
	)
	if len(rows) != 1 {
		t.Fatalf("esperaba 1 fila: %+v", rows)
	}
	row := rows[0]
	if row.IP != "192.168.0.11" || row.MAC != "00:11:22:33:44:0b" || row.IfIndex != 3 || row.Type != "static" || row.Table != "net_to_physical" {
		t.Fatalf("fila inesperada: %+v", row)
	}
}

func TestParseQBridgeAndBridgeFDB(t *testing.T) {
	suffix := "10.170.187.204.221.238.1"
	vlans, fdb := parseQBridge(
		map[string]string{"10": "Red"},
		map[string]string{suffix: ""},
		map[string]string{suffix: "5"},
		map[string]string{suffix: "3"},
		map[string]string{"5": "7"},
	)
	if len(vlans) != 1 || vlans[0].VLAN != 10 || vlans[0].Name != "Red" {
		t.Fatalf("VLANes inesperadas: %+v", vlans)
	}
	if len(fdb) != 1 {
		t.Fatalf("esperaba 1 fila FDB: %+v", fdb)
	}
	row := fdb[0]
	if row.MAC != "aa:bb:cc:dd:ee:01" || row.Port != 5 || row.IfIndex != 7 || row.VLAN != 10 || row.Status != "learned" || row.Table != "q_bridge" {
		t.Fatalf("FDB inesperada: %+v", row)
	}

	flat := parseBridgeFDB(
		map[string]string{"170.187.204.221.238.2": ""},
		map[string]string{"170.187.204.221.238.2": "6"},
		map[string]string{"170.187.204.221.238.2": "4"},
		map[string]string{"6": "8"},
	)
	if len(flat) != 1 || flat[0].MAC != "aa:bb:cc:dd:ee:02" || flat[0].IfIndex != 8 || flat[0].Status != "static" || flat[0].Table != "bridge" {
		t.Fatalf("FDB plana inesperada: %+v", flat)
	}
}

func TestMaskCommunityNeverReturnsSecret(t *testing.T) {
	for input, want := range map[string]string{
		"public":  "pu***ic",
		"private": "pr***te",
		"abc":     "***",
		"":        "",
	} {
		if got := maskCommunity(input); got != want {
			t.Errorf("maskCommunity(%q) = %q, esperaba %q", input, got, want)
		}
	}
}

func TestPduMACDecodesRawBytes(t *testing.T) {
	pdu := gosnmp.SnmpPDU{Value: []byte{0xaa, 0xbb, 0xcc, 0xdd, 0xee, 0x01}}
	if got := pduMAC(pdu); got != "aa:bb:cc:dd:ee:01" {
		t.Fatalf("MAC decodificada como %q", got)
	}
}
