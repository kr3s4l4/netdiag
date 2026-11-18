package vpn

import (
	"os"
	"path/filepath"
	"testing"
)

func writeFixture(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func fixtureRoot(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFixture(t, root, "net/udp", `   sl  local_address rem_address   st tx_queue:rx_queue tr:tm->when  retrnsmt   uid  timeout inode
    0: 0100007F:CA6C 0201A8C0:CA6C 07 00000000:00000000 00:00000000 00000000  1000        0 1234 1 0000000000000000 100 0 0 10 0
    1: 0100007F:0035 00000000:0000 07 00000000:00000000 00:00000000 00000000  1000        0 1235 1 0000000000000000 100 0 0 10 0
`)
	writeFixture(t, root, "net/tcp", `   sl  local_address rem_address   st tx_queue:rx_queue tr:tm->when  retrnsmt   uid  timeout inode
    0: 0100007F:04AA 0201A8C0:04AA 01 00000000:00000000 00:00000000 00000000  1000        0 1236 1 0000000000000000 100 0 0 10 0
    1: 0100007F:01BB 00000000:0000 01 00000000:00000000 00:00000000 00000000  1000        0 1237 1 0000000000000000 100 0 0 10 0
`)
	writeFixture(t, root, "net/dev", `Inter-|   Receive                                                |  Transmit
 face |bytes    packets errs drop fifo frame compressed multicast|bytes    packets errs drop fifo colls carrier compressed
    lo: 100       2    0    0    0     0          0         0   100       2    0    0    0       0          0
   wg0: 5000     40    0    0    0     0          0         0  9000      50    0    0    0       0          0
`)
	writeFixture(t, root, "sys/class/net/wg0/operstate", "up\n")
	return root
}

func TestObserveDetectsWireGuardOpenVPNAndIgnoresNoise(t *testing.T) {
	d := Detector{Root: fixtureRoot(t)}
	obs := d.Observe()
	if len(obs) != 3 {
		t.Fatalf("esperadas 3 observaciones, got %d: %+v", len(obs), obs)
	}
	var wg, ov *Observation
	for i := range obs {
		if obs[i].Type == "wireguard" && obs[i].LocalPort == 51820 {
			wg = &obs[i]
		}
		if obs[i].Type == "openvpn" && obs[i].LocalPort == 1194 {
			ov = &obs[i]
		}
	}
	if wg == nil || wg.Confidence < 90 {
		t.Fatalf("WireGuard no detectado: %+v", obs)
	}
	if wg.LocalIP != "127.0.0.1" || wg.RemoteIP != "192.168.1.2" {
		t.Fatalf("direcciones WireGuard incorrectas: %+v", *wg)
	}
	if ov == nil || ov.State != "active" {
		t.Fatalf("OpenVPN TCP no detectado: %+v", obs)
	}
	foundCounters := false
	for _, o := range obs {
		if o.BytesRecv == 5000 && o.BytesSent == 9000 {
			foundCounters = true
		}
	}
	if !foundCounters {
		t.Fatal("contadores de la interfaz wg0 no propagados")
	}
}

func TestParseProcAddr(t *testing.T) {
	ip, port, ok := parseProcAddr("0100007F:04AA", false)
	if !ok || ip != "127.0.0.1" || port != 1194 {
		t.Fatalf("IPv4: %s %d %v", ip, port, ok)
	}
	if _, _, ok := parseProcAddr("bad", false); ok {
		t.Fatal("dirección inválida aceptada")
	}
}

func TestMergeObservationsSumsCounters(t *testing.T) {
	a := Observation{Type: "wireguard", LocalIP: "1.1.1.1", LocalPort: 51820, BytesRecv: 10, BytesSent: 20, Confidence: 90}
	b := a
	b.BytesRecv, b.BytesSent, b.Confidence = 5, 7, 95
	merged := mergeObservations(a, b)
	if merged.BytesRecv != 15 || merged.BytesSent != 27 || merged.Confidence != 95 {
		t.Fatalf("fusión incorrecta: %+v", merged)
	}
}
