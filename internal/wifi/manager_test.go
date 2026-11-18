package wifi

import (
	"testing"

	"github.com/kr3s4l4/netdiag/internal/config"
	"github.com/kr3s4l4/netdiag/internal/model"
)

func TestClassifyRogue(t *testing.T) {
	ap := model.WifiAP{BSSID: "00:11:22:33:44:55", SSID: "CorpWiFi"}
	cases := []struct {
		name    string
		cfg     config.WiFi
		want    bool
		confMin int
	}{
		{"sin inventario", config.WiFi{}, false, 0},
		{"bssid autorizado", config.WiFi{AuthorizedBSSIDs: []string{"00:11:22:33:44:55"}}, false, 0},
		{"bssid desconocido", config.WiFi{AuthorizedBSSIDs: []string{"aa:bb:cc:dd:ee:ff"}}, true, 90},
		{"bssid desconocido con ssid conocido", config.WiFi{
			AuthorizedBSSIDs: []string{"aa:bb:cc:dd:ee:ff"}, KnownSSIDs: []string{"CorpWiFi"},
		}, true, 60},
		{"ssid conocido sin bssids", config.WiFi{KnownSSIDs: []string{"CorpWiFi"}}, false, 0},
		{"ssid desconocido", config.WiFi{KnownSSIDs: []string{"Otro"}}, true, 55},
		{"ssid oculto sin comparación", config.WiFi{KnownSSIDs: []string{"Otro"}}, false, 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			target := ap
			if tc.name == "ssid oculto sin comparación" {
				target.SSID = "(oculta)"
			}
			rogue, info, confidence := classifyRogue(target, tc.cfg)
			if rogue != tc.want || confidence < tc.confMin {
				t.Fatalf("rogue=%v confianza=%d info=%q; esperado rogue=%v conf>=%d",
					rogue, confidence, info, tc.want, tc.confMin)
			}
			if rogue && info == "" {
				t.Fatal("rogue sin explicación")
			}
		})
	}
}

func TestAggregateChannelsInterference(t *testing.T) {
	aps := []model.WifiAP{
		{BSSID: "00:00:00:00:00:01", SSID: "A", Channel: 1, Band: "2.4GHz"},
		{BSSID: "00:00:00:00:00:02", SSID: "B", Channel: 3, Band: "2.4GHz"},
		{BSSID: "00:00:00:00:00:03", SSID: "C", Channel: 11, Band: "2.4GHz"},
		{BSSID: "00:00:00:00:00:04", SSID: "D", Channel: 36, Band: "5GHz"},
	}
	clients := []model.WifiClient{
		{APBSSID: "00:00:00:00:00:01"},
		{APBSSID: "00:00:00:00:00:03"},
		{APBSSID: "00:00:00:00:00:03"},
	}
	channels := aggregateChannels(aps, clients)
	if len(channels) != 4 {
		t.Fatalf("esperados 4 canales, got %d: %+v", len(channels), channels)
	}
	byChannel := map[int]model.WifiChannel{}
	for _, ch := range channels {
		byChannel[ch.Channel] = ch
	}
	if byChannel[1].APs != 1 || byChannel[1].Clients != 1 || byChannel[1].Overlap != 1 {
		t.Fatalf("canal 1 incorrecto: %+v", byChannel[1])
	}
	if byChannel[3].Overlap != 1 || byChannel[3].Load != "baja" {
		t.Fatalf("canal 3 incorrecto: %+v", byChannel[3])
	}
	if byChannel[11].Clients != 2 || byChannel[11].Overlap != 0 {
		t.Fatalf("canal 11 incorrecto: %+v", byChannel[11])
	}
	if byChannel[36].Overlap != 0 || byChannel[36].Load != "baja" {
		t.Fatalf("canal 36 incorrecto: %+v", byChannel[36])
	}
	for _, ch := range channels {
		if ch.Suggest == "" {
			t.Fatalf("canal %d sin recomendación", ch.Channel)
		}
	}
}

func TestChannelLoadThresholds(t *testing.T) {
	if channelLoad(1, 0, 0) != "baja" || channelLoad(2, 6, 0) != "media" || channelLoad(4, 12, 3) != "alta" {
		t.Fatal("umbrales de carga incorrectos")
	}
}
