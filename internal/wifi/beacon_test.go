package wifi

import (
	"net"
	"testing"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

func mustMAC(t *testing.T, s string) net.HardwareAddr {
	t.Helper()
	mac, err := net.ParseMAC(s)
	if err != nil {
		t.Fatal(err)
	}
	return mac
}

func makeFrame(t *testing.T, dot layers.Dot11, body gopacket.SerializableLayer, ies []gopacket.SerializableLayer, signal int8, freq uint16) gopacket.Packet {
	t.Helper()
	rt := layers.RadioTap{
		Version:          0,
		Present:          layers.RadioTapPresentChannel | layers.RadioTapPresentDBMAntennaSignal,
		ChannelFrequency: layers.RadioTapChannelFrequency(freq),
		DBMAntennaSignal: signal,
	}
	all := []gopacket.SerializableLayer{&rt, &dot}
	if body != nil {
		all = append(all, body)
	}
	all = append(all, ies...)
	buf := gopacket.NewSerializeBuffer()
	if err := gopacket.SerializeLayers(buf, gopacket.SerializeOptions{FixLengths: true}, all...); err != nil {
		t.Fatal(err)
	}
	return gopacket.NewPacket(buf.Bytes(), layers.LayerTypeRadioTap, gopacket.Default)
}

func infoLayer(id layers.Dot11InformationElementID, data []byte) gopacket.SerializableLayer {
	// gopacket's decoder requires at least four bytes after the header.
	for len(data) < 4 {
		data = append(data, 0)
	}
	return &layers.Dot11InformationElement{ID: id, Info: data}
}

func TestParseBeaconWPA2(t *testing.T) {
	bssid := mustMAC(t, "00:11:22:33:44:55")
	dot := layers.Dot11{
		Type:     layers.Dot11TypeMgmtBeacon,
		Address1: mustMAC(t, "ff:ff:ff:ff:ff:ff"),
		Address2: bssid,
		Address3: bssid,
	}
	ies := []gopacket.SerializableLayer{
		infoLayer(layers.Dot11InformationElementIDSSID, []byte("RedWiFi")),
		infoLayer(layers.Dot11InformationElementIDDSSet, []byte{6}),
		infoLayer(layers.Dot11InformationElementIDRSNInfo, []byte{1, 0, 0, 0}),
	}
	ap, client := ParsePacket(makeFrame(t, dot, &layers.Dot11MgmtBeacon{Interval: 100}, ies, -42, 2437))
	if client != nil {
		t.Fatalf("beacon detectado como cliente: %+v", client)
	}
	if ap == nil {
		t.Fatal("beacon no detectado")
	}
	if ap.BSSID != "00:11:22:33:44:55" || ap.SSID != "RedWiFi" || ap.Channel != 6 || ap.Band != "2.4GHz" {
		t.Fatalf("AP incorrecto: %+v", ap)
	}
	if ap.Encryption != "WPA2" || ap.SignalDBM != -42 || ap.Beacons != 1 {
		t.Fatalf("cifrado/señal incorrectos: %+v", ap)
	}
}

func TestParseProbeResponseWPA5GHz(t *testing.T) {
	bssid := mustMAC(t, "aa:bb:cc:dd:ee:ff")
	dot := layers.Dot11{
		Type:     layers.Dot11TypeMgmtProbeResp,
		Address1: mustMAC(t, "ff:ff:ff:ff:ff:ff"),
		Address2: bssid,
		Address3: bssid,
	}
	vendor := &layers.Dot11InformationElement{
		ID:   layers.Dot11InformationElementIDVendor,
		OUI:  []byte{0x00, 0x0f, 0x4c, 0x01},
		Info: []byte{0, 0, 0, 0},
	}
	ap, _ := ParsePacket(makeFrame(t, dot, &layers.Dot11MgmtProbeResp{Interval: 100},
		[]gopacket.SerializableLayer{infoLayer(layers.Dot11InformationElementIDSSID, []byte("Corp-5G")), vendor}, -60, 5180))
	if ap == nil || ap.Encryption != "WPA" || ap.Channel != 36 || ap.Band != "5GHz" {
		t.Fatalf("probe response incorrecto: %+v", ap)
	}
}

func TestParseClientFromDataFrame(t *testing.T) {
	dot := layers.Dot11{
		Type:     layers.Dot11TypeData,
		Flags:    layers.Dot11FlagsFromDS,
		Address1: mustMAC(t, "de:ad:be:ef:00:01"),
		Address2: mustMAC(t, "00:11:22:33:44:55"),
		Address3: mustMAC(t, "00:11:22:33:44:55"),
	}
	ap, client := ParsePacket(makeFrame(t, dot, nil, nil, -70, 2437))
	if ap != nil || client == nil {
		t.Fatalf("cliente no detectado: ap=%+v client=%+v", ap, client)
	}
	if client.MAC != "de:ad:be:ef:00:01" || client.APBSSID != "00:11:22:33:44:55" || client.SignalDBM != -70 {
		t.Fatalf("cliente incorrecto: %+v", client)
	}
}

func TestParseIgnoresIrrelevantFrames(t *testing.T) {
	broadcast := mustMAC(t, "ff:ff:ff:ff:ff:ff")
	dot := layers.Dot11{
		Type:     layers.Dot11TypeMgmtBeacon,
		Address1: broadcast,
		Address2: broadcast,
		Address3: broadcast,
	}
	ap, client := ParsePacket(makeFrame(t, dot, &layers.Dot11MgmtBeacon{Interval: 100}, nil, -40, 2412))
	if ap != nil || client != nil {
		t.Fatalf("beacon de difusión aceptado: %+v %+v", ap, client)
	}
}

func TestFrequencyToChannelAndBand(t *testing.T) {
	cases := []struct {
		freq    int
		channel int
		band    string
	}{
		{2412, 1, "2.4GHz"},
		{2437, 6, "2.4GHz"},
		{2484, 14, "2.4GHz"},
		{5180, 36, "5GHz"},
		{5955, 1, "6GHz"},
		{0, 0, ""},
	}
	for _, tc := range cases {
		if got := FrequencyToChannel(tc.freq); got != tc.channel {
			t.Errorf("frecuencia %d: canal %d, esperado %d", tc.freq, got, tc.channel)
		}
		if got := bandFromFrequency(tc.freq); got != tc.band {
			t.Errorf("frecuencia %d: banda %q, esperada %q", tc.freq, got, tc.band)
		}
	}
	if BandForChannel(11) != "2.4GHz" || BandForChannel(149) != "5GHz" || BandForChannel(0) != "" {
		t.Fatal("clasificación de banda por canal incorrecta")
	}
}
