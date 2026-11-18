package trace

import (
	"encoding/binary"
	"net"
	"syscall"
	"testing"
	"time"
)

func TestNormalize(t *testing.T) {
	o := Options{}
	o.Normalize()
	if o.MaxHops != 20 || o.Probes != 2 || o.Timeout <= 0 {
		t.Fatalf("defaults incorrectos: %+v", o)
	}

	o = Options{MaxHops: 500, Probes: 99, Timeout: -time.Second}
	o.Normalize()
	if o.MaxHops != 64 || o.Probes != 5 || o.Timeout <= 0 {
		t.Fatalf("límites incorrectos: %+v", o)
	}
}

func TestParseError(t *testing.T) {
	data := make([]byte, 16+16)
	binary.LittleEndian.PutUint32(data[0:4], uint32(syscall.ECONNREFUSED))
	data[4] = 2 // SO_EE_ORIGIN_ICMP
	data[5] = 3 // ICMP destination unreachable
	data[6] = 3 // port unreachable
	binary.NativeEndian.PutUint16(data[16:18], uint16(syscall.AF_INET))
	binary.NativeEndian.PutUint16(data[18:20], 33434)
	copy(data[20:24], net.IPv4(192, 168, 0, 1).To4())

	const cmsgLen = 16 + 32
	oob := make([]byte, cmsgLen)
	binary.NativeEndian.PutUint64(oob[0:8], uint64(cmsgLen))
	binary.NativeEndian.PutUint32(oob[8:12], uint32(syscall.SOL_IP))
	binary.NativeEndian.PutUint32(oob[12:16], uint32(syscall.IP_RECVERR))
	copy(oob[16:], data)

	ip, icmpType, icmpCode, err := parseError(oob)
	if err != nil {
		t.Fatalf("parseError: %v", err)
	}
	if got := ip.String(); got != "192.168.0.1" {
		t.Fatalf("IP inesperada: %s", got)
	}
	if icmpType != 3 || icmpCode != 3 {
		t.Fatalf("ICMP inesperado: %d/%d", icmpType, icmpCode)
	}
}

func TestParseErrorRejectsOtherOrigin(t *testing.T) {
	data := make([]byte, 32)
	data[4] = 9 // SO_EE_ORIGIN_NONE
	oob := make([]byte, 32)
	binary.NativeEndian.PutUint64(oob[0:8], 32)
	binary.NativeEndian.PutUint32(oob[8:12], uint32(syscall.SOL_IP))
	binary.NativeEndian.PutUint32(oob[12:16], uint32(syscall.IP_RECVERR))
	copy(oob[16:], data)
	if _, _, _, err := parseError(oob); err == nil {
		t.Fatal("se esperaba error para un origen desconocido")
	}
}
