// Package wifi parses passively captured 802.11 frames into access points,
// clients and channel load. Capture itself lives in the pcap-tagged files;
// parsing is pure so it can be tested without libpcap.
package wifi

import (
	"errors"
	"strings"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// ErrCaptureUnavailable is returned when no monitor interface is configured or
// the portable build does not include raw 802.11 capture support.
var ErrCaptureUnavailable = errors.New("captura 802.11 no disponible: configura una interfaz en modo monitor y usa el binario -tags pcap")

// APObservation is a single beacon/probe-response observation.
type APObservation struct {
	BSSID      string
	SSID       string
	Channel    int
	Band       string
	Encryption string
	SignalDBM  int
	Beacons    int64
}

// ClientObservation is a single station observation from a data frame.
type ClientObservation struct {
	MAC       string
	APBSSID   string
	SignalDBM int
}

// BandForChannel classifies an 802.11 channel.
func BandForChannel(ch int) string {
	switch {
	case ch >= 1 && ch <= 14:
		return "2.4GHz"
	case ch >= 36 && ch <= 196:
		return "5GHz"
	case ch >= 1 && ch <= 233:
		// 6 GHz channels overlap the 2.4 GHz range; frequencies above 5000
		// are distinguished by the caller through channel >= 1 in 6GHz band
		// only when radiotap reports a >5900 MHz frequency.
		return "6GHz"
	default:
		return ""
	}
}

// FrequencyToChannel converts a radiotap frequency (MHz) to a channel number.
func FrequencyToChannel(mhz int) int {
	switch {
	case mhz >= 2412 && mhz <= 2484:
		if mhz == 2484 {
			return 14
		}
		return (mhz - 2407) / 5
	case mhz >= 5160 && mhz <= 5895:
		return (mhz - 5000) / 5
	case mhz >= 5955 && mhz <= 7115:
		return (mhz - 5950) / 5
	default:
		return 0
	}
}

// bandFromFrequency prefers the radiotap frequency when available because
// 6 GHz channel numbers collide with 2.4 GHz ones.
func bandFromFrequency(mhz int) string {
	switch {
	case mhz >= 5955 && mhz <= 7115:
		return "6GHz"
	case mhz >= 5160 && mhz <= 5895:
		return "5GHz"
	case mhz >= 2412 && mhz <= 2484:
		return "2.4GHz"
	default:
		return ""
	}
}

// ParsePacket extracts an AP or client observation from one frame. It returns
// (nil, nil) for irrelevant frames.
func ParsePacket(pkt gopacket.Packet) (*APObservation, *ClientObservation) {
	dot, ok := pkt.Layer(layers.LayerTypeDot11).(*layers.Dot11)
	if !ok || len(dot.Address2) != 6 {
		return nil, nil
	}
	signal := 0
	freq := 0
	if rt, ok := pkt.Layer(layers.LayerTypeRadioTap).(*layers.RadioTap); ok {
		signal = int(rt.DBMAntennaSignal)
		freq = int(rt.ChannelFrequency)
	}

	switch dot.Type.MainType() {
	case layers.Dot11TypeMgmt:
		if dot.Type != layers.Dot11TypeMgmtBeacon && dot.Type != layers.Dot11TypeMgmtProbeResp {
			return nil, nil
		}
		bssid := normalizeMAC(dot.Address2)
		if bssid == "" || bssid == "ff:ff:ff:ff:ff:ff" {
			return nil, nil
		}
		ap := &APObservation{
			BSSID:      bssid,
			SignalDBM:  signal,
			Beacons:    1,
			Encryption: "abierta",
		}
		channel := FrequencyToChannel(freq)
		band := bandFromFrequency(freq)
		privacy := false
		rsn, wpa := false, false
		for _, lyr := range pkt.Layers() {
			ie, ok := lyr.(*layers.Dot11InformationElement)
			if !ok {
				continue
			}
			switch ie.ID {
			case layers.Dot11InformationElementIDSSID:
				ap.SSID = sanitizeSSID(string(ie.Info))
			case layers.Dot11InformationElementIDDSSet:
				if len(ie.Info) > 0 {
					channel = int(ie.Info[0])
				}
			case layers.Dot11InformationElementIDRSNInfo:
				rsn = true
			case layers.Dot11InformationElementIDVendor:
				// gopacket splits the 4-byte OUI/type prefix into OUI for ID 221;
				// hand-built layers may instead keep it at the start of Info.
				vendor := append(append([]byte{}, ie.OUI...), ie.Info...)
				if len(vendor) >= 4 && vendor[0] == 0x00 && vendor[1] == 0x0f && vendor[2] == 0x4c && vendor[3] == 0x01 {
					wpa = true
				}
			}
		}
		if beacon, ok := pkt.Layer(layers.LayerTypeDot11MgmtBeacon).(*layers.Dot11MgmtBeacon); ok {
			privacy = beacon.Flags&0x0010 != 0
		} else if probe, ok := pkt.Layer(layers.LayerTypeDot11MgmtProbeResp).(*layers.Dot11MgmtProbeResp); ok {
			privacy = probe.Flags&0x0010 != 0
		}
		if rsn {
			ap.Encryption = "WPA2"
		} else if wpa {
			ap.Encryption = "WPA"
		} else if privacy {
			ap.Encryption = "WEP"
		}
		ap.Channel = channel
		ap.Band = band
		if ap.Band == "" {
			ap.Band = BandForChannel(channel)
		}
		return ap, nil

	case layers.Dot11TypeData:
		// Only frames with three addresses reveal the AP via Address3.
		if len(dot.Address3) != 6 || isZeroMAC(dot.Address3) {
			return nil, nil
		}
		client := ""
		apBSSID := normalizeMAC(dot.Address3)
		switch {
		case dot.Flags.FromDS() && !dot.Flags.ToDS():
			client = normalizeMAC(dot.Address1)
		case dot.Flags.ToDS() && !dot.Flags.FromDS():
			client = normalizeMAC(dot.Address2)
		default:
			return nil, nil
		}
		if client == "" || client == "ff:ff:ff:ff:ff:ff" || apBSSID == "" {
			return nil, nil
		}
		return nil, &ClientObservation{MAC: client, APBSSID: apBSSID, SignalDBM: signal}
	}
	return nil, nil
}

func normalizeMAC(b []byte) string {
	if len(b) != 6 {
		return ""
	}
	parts := make([]string, 6)
	for i, v := range b {
		const hex = "0123456789abcdef"
		parts[i] = string([]byte{hex[v>>4], hex[v&0x0f]})
	}
	return strings.Join(parts, ":")
}

func isZeroMAC(b []byte) bool {
	for _, v := range b {
		if v != 0 {
			return false
		}
	}
	return true
}

func sanitizeSSID(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, s)
	if len(s) > 32 {
		s = s[:32]
	}
	if s == "" {
		return "(oculta)"
	}
	return s
}
