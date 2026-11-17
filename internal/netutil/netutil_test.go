package netutil

import (
	"net"
	"testing"

	"github.com/kr3s4l4/netdiag/internal/model"
)

func TestIsPrivate(t *testing.T) {
	cases := []struct {
		ip   string
		want bool
	}{
		{"192.168.1.1", true},
		{"10.255.0.1", true},
		{"172.16.5.4", true},
		{"172.32.0.1", false},
		{"127.0.0.1", true},
		{"169.254.1.1", true},
		{"100.64.0.1", true},
		{"100.128.0.1", false},
		{"8.8.8.8", false},
		{"2606:4700:4700::1111", false},
		{"::1", true},
	}
	for _, c := range cases {
		if got := IsPrivate(net.ParseIP(c.ip)); got != c.want {
			t.Errorf("IsPrivate(%s) = %v, se esperaba %v", c.ip, got, c.want)
		}
	}
}

func TestHostsInCIDR(t *testing.T) {
	hosts, err := HostsInCIDR("192.168.10.0/30", 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 2 || hosts[0].String() != "192.168.10.1" || hosts[1].String() != "192.168.10.2" {
		t.Fatalf("hosts inesperados: %v", hosts)
	}
	if _, err := HostsInCIDR("10.0.0.0/24", 10); err == nil {
		t.Fatal("se esperaba error al superar el límite")
	}
	if _, err := HostsInCIDR("2001:db8::/64", 100); err == nil {
		t.Fatal("IPv6 debería rechazarse")
	}
	if _, err := HostsInCIDR("no-es-cidr", 100); err == nil {
		t.Fatal("un CIDR inválido debería fallar")
	}
}

func TestCountHosts(t *testing.T) {
	n, err := CountHosts("192.168.1.0/24")
	if err != nil {
		t.Fatal(err)
	}
	if n != 254 {
		t.Fatalf("se esperaban 254 hosts, hay %d", n)
	}
	n, err = CountHosts("192.168.1.4/31")
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 {
		t.Fatalf("/31 debería tener 2 direcciones, hay %d", n)
	}
	if _, err := CountHosts("10.0.0.0/8"); err == nil {
		t.Fatal("un /8 debería rechazarse")
	}
}

func TestGatewayFor(t *testing.T) {
	ifaces := []model.SystemInterface{
		{Name: "eth0", Gateway: "192.168.1.1", Addrs: []model.InterfaceAddr{{IP: "192.168.1.50", Family: "ipv4", Private: true}}},
		{Name: "docker0", Addrs: []model.InterfaceAddr{{IP: "172.17.0.1", Family: "ipv4", Private: true}}},
	}
	if got := GatewayFor("eth0", ifaces); got != "192.168.1.1" {
		t.Fatalf("gateway inesperado: %q", got)
	}
	if got := GatewayFor("docker0", ifaces); got != "172.17.0.1" {
		t.Fatalf("una interfaz sin ruta debe usar su propia IP: %q", got)
	}
	if got := GatewayFor("wlan9", ifaces); got != "" {
		t.Fatalf("interfaz inexistente: %q", got)
	}
}
