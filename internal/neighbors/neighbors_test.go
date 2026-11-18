package neighbors

import (
	"net"
	"testing"
)

func TestNormalizeCIDRs(t *testing.T) {
	got := normalizeCIDRs([]string{" 192.168.0.105/24 ", "10.0.0.0/8", "invalid", "2001:db8::/64"})
	if !got["192.168.0.0/24"] {
		t.Fatalf("falta la red local: %v", got)
	}
	if !got["10.0.0.0/24"] {
		t.Fatalf("la red /8 debe reducirse a /24: %v", got)
	}
	if len(got) != 2 {
		t.Fatalf("cantidad inesperada: %v", got)
	}
}

func TestCandidateGatewaysArePrivateOnly(t *testing.T) {
	local := map[string]bool{"192.168.0.0/24": true}
	got := candidateGateways(local, []string{"8.8.8.8", "192.168.5.20"})
	if len(got) == 0 {
		t.Fatal("no se generaron candidatos")
	}
	for _, ip := range got {
		if ip == "8.8.8.8" {
			t.Fatal("se incluyó una IP pública")
		}
		if ip == "192.168.0.1" || ip == "192.168.0.254" {
			t.Fatalf("se incluyó la red local: %s", ip)
		}
	}
	found := false
	for _, ip := range got {
		if ip == "192.168.5.1" || ip == "192.168.5.20" {
			found = true
		}
	}
	if !found {
		t.Fatalf("la semilla del inventario no se propagó: %v", got)
	}
}

func TestBelongsTo(t *testing.T) {
	prefixes := localPrefixes(map[string]bool{"192.168.0.0/24": true})
	if !belongsTo(parseIP("192.168.0.40"), prefixes) {
		t.Fatal("192.168.0.40 debería considerarse local")
	}
	if belongsTo(parseIP("192.168.1.1"), prefixes) {
		t.Fatal("192.168.1.1 no debería considerarse local")
	}
}

func parseIP(s string) net.IP { return net.ParseIP(s) }
