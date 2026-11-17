package fingerprint

import (
	"compress/gzip"
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

//go:embed oui.json.gz
var ouiEmbedded []byte

type ouiFile struct {
	Schema      int               `json:"schema"`
	GeneratedAt string            `json:"generated_at"`
	Source      string            `json:"source"`
	Count       int               `json:"count"`
	Entries     map[string]string `json:"entries"`
}

var (
	ouiOnce   sync.Once
	ouiMu     sync.RWMutex
	ouiTable  map[string]string
	ouiMeta   ouiFile
	ouiOrigin = "embed"
)

var virtualOUIs = map[string]string{
	"02:42":    "Docker",
	"0a:58":    "Kubernetes (Calico)",
	"52:54:00": "QEMU/KVM",
	"00:15:5d": "Microsoft Hyper-V",
	"00:50:56": "VMware",
	"00:0c:29": "VMware",
	"00:05:69": "VMware",
	"08:00:27": "VirtualBox",
	"00:16:3e": "Xen",
	"00:03:ff": "Microsoft Virtual",
	"00:1c:42": "Parallels",
	"00:1c:14": "VMware",
}

func loadOUI() {
	ouiOnce.Do(func() {
		table, meta, err := decodeOUI(ouiEmbedded)
		if err != nil {
			table = map[string]string{}
			meta = ouiFile{}
		}
		ouiTable = table
		ouiMeta = meta
	})
}

func decodeOUI(raw []byte) (map[string]string, ouiFile, error) {
	var meta ouiFile
	zr, err := gzip.NewReader(strings.NewReader(string(raw)))
	if err != nil {
		// Allow plain JSON files as well (external overrides).
		if err := json.Unmarshal(raw, &meta); err != nil {
			return nil, meta, err
		}
	} else {
		defer zr.Close()
		if err := json.NewDecoder(zr).Decode(&meta); err != nil {
			return nil, meta, err
		}
	}
	if meta.Entries == nil {
		meta.Entries = map[string]string{}
	}
	clean := make(map[string]string, len(meta.Entries))
	for k, v := range meta.Entries {
		clean[strings.ToUpper(strings.ReplaceAll(k, "-", ":"))] = v
	}
	return clean, meta, nil
}

// InitOUI optionally overlays a user-updated OUI database file.
func InitOUI(extraPath string) error {
	loadOUI()
	if extraPath == "" {
		return nil
	}
	f, err := os.Open(extraPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()
	info, _ := f.Stat()
	raw := make([]byte, info.Size())
	if _, err := f.Read(raw); err != nil {
		return err
	}
	table, meta, err := decodeOUI(raw)
	if err != nil {
		return fmt.Errorf("OUI %s: %w", extraPath, err)
	}
	ouiMu.Lock()
	if ouiTable == nil {
		loadOUI()
	}
	for k, v := range table {
		ouiTable[k] = v
	}
	ouiMeta = meta
	ouiOrigin = filepath.Base(extraPath)
	ouiMu.Unlock()
	return nil
}

// OUIStats describes the loaded vendor database.
type OUIStats struct {
	Count       int    `json:"count"`
	GeneratedAt string `json:"generated_at"`
	Source      string `json:"source"`
	Origin      string `json:"origin"`
}

// Stats returns metadata about the vendor database.
func Stats() OUIStats {
	loadOUI()
	ouiMu.RLock()
	defer ouiMu.RUnlock()
	return OUIStats{Count: len(ouiTable), GeneratedAt: ouiMeta.GeneratedAt, Source: ouiMeta.Source, Origin: ouiOrigin}
}

// NormalizeMAC returns a lower-case colon-separated MAC, or the trimmed input
// when it cannot be parsed.
func NormalizeMAC(mac string) string {
	clean := strings.ToLower(strings.TrimSpace(mac))
	clean = strings.NewReplacer(":", "", "-", "", ".", "", " ", "").Replace(clean)
	if len(clean) != 12 {
		return strings.ToLower(strings.TrimSpace(mac))
	}
	for _, c := range clean {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return strings.ToLower(strings.TrimSpace(mac))
		}
	}
	return strings.Join([]string{
		clean[0:2], clean[2:4], clean[4:6], clean[6:8], clean[8:10], clean[10:12],
	}, ":")
}

// IsRandomMAC reports whether the MAC has the locally-administered bit set.
func IsRandomMAC(mac string) bool {
	n := NormalizeMAC(mac)
	if len(n) < 2 {
		return false
	}
	b, err := parseHexByte(n[:2])
	if err != nil {
		return false
	}
	return b&0x02 != 0
}

func parseHexByte(s string) (byte, error) {
	var b byte
	for i := 0; i < len(s); i++ {
		var v byte
		switch c := s[i]; {
		case c >= '0' && c <= '9':
			v = c - '0'
		case c >= 'a' && c <= 'f':
			v = c - 'a' + 10
		case c >= 'A' && c <= 'F':
			v = c - 'A' + 10
		default:
			return 0, fmt.Errorf("hex inválido")
		}
		b = b*16 + v
	}
	return b, nil
}

// Vendor resolves a MAC prefix to an organisation name. Locally-administered
// MACs are not looked up unless they belong to a known virtual platform.
func Vendor(mac string) (string, bool) {
	loadOUI()
	n := NormalizeMAC(mac)
	if len(n) != 17 {
		return "", false
	}
	prefix := n[:8]
	ouiMu.RLock()
	if v, ok := virtualOUIs[prefix]; ok {
		ouiMu.RUnlock()
		return v, true
	}
	v, ok := ouiTable[strings.ToUpper(prefix)]
	ouiMu.RUnlock()
	if ok && v != "" {
		return v, true
	}
	if IsRandomMAC(n) {
		return "", false
	}
	return "", false
}

// IsVirtualPlatform reports whether the MAC belongs to a hypervisor or
// container bridge.
func IsVirtualPlatform(mac string) bool {
	n := NormalizeMAC(mac)
	if len(n) != 17 {
		return false
	}
	_, ok := virtualOUIs[n[:8]]
	return ok
}
