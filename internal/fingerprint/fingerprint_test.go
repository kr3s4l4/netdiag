package fingerprint

import (
	"testing"

	"github.com/kr3s4l4/netdiag/internal/model"
)

func TestParsePorts(t *testing.T) {
	got, err := ParsePorts("80,22,8000-8002,443,22")
	if err != nil {
		t.Fatal(err)
	}
	want := []int{22, 80, 443, 8000, 8001, 8002}
	if len(got) != len(want) {
		t.Fatalf("longitud inesperada: %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("puertos inesperados: %v", got)
		}
	}
	for _, bad := range []string{"0", "70000", "10-5", "abc", "80-"} {
		if _, err := ParsePorts(bad); err == nil {
			t.Errorf("se esperaba error para %q", bad)
		}
	}
}

func TestProfiles(t *testing.T) {
	if len(ProfilePorts("quick", nil)) == 0 {
		t.Fatal("el perfil rápido está vacío")
	}
	full := ProfilePorts("full", nil)
	if len(full) < 1000 {
		t.Fatalf("el perfil completo debería cubrir los puertos privilegiados: %d", len(full))
	}
	custom := ProfilePorts("custom", []int{9000, 80, 80})
	if len(custom) != 2 || custom[0] != 80 || custom[1] != 9000 {
		t.Fatalf("puertos personalizados inesperados: %v", custom)
	}
	if ServiceName(443) != "https" {
		t.Fatalf("servicio inesperado para 443: %q", ServiceName(443))
	}
}

func TestMACCasingAndRandomDetection(t *testing.T) {
	if got := NormalizeMAC("AA-BB-CC-DD-EE-FF"); got != "aa:bb:cc:dd:ee:ff" {
		t.Fatalf("normalización inesperada: %q", got)
	}
	if IsRandomMAC("00:1a:2b:dd:ee:ff") {
		t.Fatal("una MAC global no debería ser aleatoria")
	}
	if !IsRandomMAC("02:11:22:33:44:55") {
		t.Fatal("la MAC localmente administrada debería detectarse como aleatoria")
	}
}

func TestVirtualVendor(t *testing.T) {
	v, ok := Vendor("52:54:00:12:34:56")
	if !ok || v == "" {
		t.Fatalf("no se detectó QEMU/KVM: %q %v", v, ok)
	}
	if !IsVirtualPlatform("00:50:56:aa:bb:cc") {
		t.Fatal("VMware no detectado como plataforma virtual")
	}
}

func TestClassifyVirtualAndServer(t *testing.T) {
	vm := Classify(Input{MAC: "52:54:00:12:34:56", Hostname: "web-01"})
	if vm.DeviceType != model.DevVirtual {
		t.Fatalf("se esperaba virtual, obtenido %q", vm.DeviceType)
	}
	if vm.Confidence == 0 || len(vm.Evidence) == 0 {
		t.Fatal("la clasificación debe aportar evidencia y confianza")
	}

	srv := Classify(Input{
		IP:       "10.0.0.10",
		Hostname: "srv-db",
		Ports: []model.Port{
			{Number: 22, Protocol: "tcp", State: "open", Service: "ssh"},
			{Number: 3306, Protocol: "tcp", State: "open", Service: "mysql"},
			{Number: 80, Protocol: "tcp", State: "open", Service: "http"},
		},
	})
	if srv.DeviceType == model.DevUnknown {
		t.Fatalf("un servidor con SSH, MySQL y HTTP no debería quedar como desconocido: %+v", srv)
	}
}

func TestOpenPorts(t *testing.T) {
	h := model.Host{Ports: []model.Port{
		{Number: 22, State: "open"},
		{Number: 23, State: "closed"},
		{Number: 80},
	}}
	got := OpenPorts(h)
	if len(got) != 2 {
		t.Fatalf("se esperaban 2 puertos abiertos: %+v", got)
	}
}
