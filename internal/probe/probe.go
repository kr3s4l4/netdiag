// Package probe builds portable observations from one network vantage point.
// A probe never sends data anywhere: it writes a JSON document that an
// operator may copy to the console and import manually.
package probe

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/kr3s4l4/netdiag/internal/model"
	"github.com/kr3s4l4/netdiag/internal/update"
)

// Schema identifies the portable probe document.
const Schema = "netdiag.probe/v1"

const signatureAlgorithm = "minisign-ed25519-prehash"

// Signature is a minisign signature embedded in the document. The signed bytes
// are the canonical JSON of the same document with this field set to null.
type Signature struct {
	Algorithm string `json:"algorithm"`
	KeyID     string `json:"key_id,omitempty"`
	Value     string `json:"value"`
	Comment   string `json:"comment,omitempty"`
}

// SourceStatus records whether a best-effort source produced data.
type SourceStatus struct {
	Available bool   `json:"available"`
	Note      string `json:"note,omitempty"`
}

// HostInfo describes the machine where the probe ran.
type HostInfo struct {
	Hostname   string                  `json:"hostname,omitempty"`
	OS         string                  `json:"os,omitempty"`
	Gateway    string                  `json:"gateway,omitempty"`
	Interfaces []model.SystemInterface `json:"interfaces,omitempty"`
	Networks   []model.Network         `json:"networks,omitempty"`
}

// Sources makes explicit which optional collectors were usable.
type Sources struct {
	ARP  SourceStatus `json:"arp"`
	SNMP SourceStatus `json:"snmp"`
	WiFi SourceStatus `json:"wifi"`
	VPN  SourceStatus `json:"vpn"`
}

// WiFiData holds passive 802.11 observations without packet payloads.
type WiFiData struct {
	APs     []model.WifiAP     `json:"aps"`
	Clients []model.WifiClient `json:"clients"`
}

// VPNData holds passive tunnel observations derived from the kernel.
type VPNData struct {
	Tunnels []model.VPNTunnel `json:"tunnels"`
}

// SNMPRecord binds one SNMP summary to the polled host.
type SNMPRecord struct {
	HostIP string            `json:"host_ip"`
	Data   model.SNMPSummary `json:"data"`
}

// SNMPDeepRecord binds a deep SNMP walk to the polled host.
type SNMPDeepRecord struct {
	HostIP string                 `json:"host_ip"`
	Data   model.SNMPDeepSnapshot `json:"data"`
}

// Document is the portable unit produced by `netdiag probe`.
type Document struct {
	Schema      string                 `json:"schema"`
	GeneratedAt time.Time              `json:"generated_at"`
	SiteID      int64                  `json:"site_id"`
	Producer    string                 `json:"producer,omitempty"`
	Host        HostInfo               `json:"host"`
	Sources     Sources                `json:"sources"`
	ARP         []model.ARPObservation `json:"arp"`
	Neighbors   []model.Neighbor       `json:"neighbors"`
	SNMP        []SNMPRecord           `json:"snmp"`
	SNMPDeep    []SNMPDeepRecord       `json:"snmp_deep"`
	WiFi        WiFiData               `json:"wifi"`
	VPN         VPNData                `json:"vpn"`
	Limitations []string               `json:"limitations,omitempty"`
	Signature   *Signature             `json:"signature,omitempty"`
}

// New returns an empty, valid probe document.
func New(siteID int64) *Document {
	return &Document{
		Schema:      Schema,
		GeneratedAt: time.Now().UTC(),
		SiteID:      siteID,
		ARP:         []model.ARPObservation{},
		Neighbors:   []model.Neighbor{},
		SNMP:        []SNMPRecord{},
		SNMPDeep:    []SNMPDeepRecord{},
		WiFi: WiFiData{
			APs:     []model.WifiAP{},
			Clients: []model.WifiClient{},
		},
		VPN: VPNData{Tunnels: []model.VPNTunnel{}},
		Limitations: []string{
			"Solo se observa lo alcanzable desde este equipo.",
			"LLDP/CDP dependen de SNMP y de que el dispositivo publique vecinos.",
			"WiFi requiere -tags pcap e interfaz en modo monitor; VPN se lee del kernel sin descifrar tráfico.",
			"Este archivo no se envía a ningún servicio y no contiene secretos.",
		},
	}
}

// Validate checks the envelope before signing or importing.
func (d *Document) Validate() error {
	if d == nil {
		return errors.New("sonda vacía")
	}
	if d.Schema != Schema {
		return fmt.Errorf("esquema de sonda no soportado %q; se esperaba %s", d.Schema, Schema)
	}
	if d.SiteID <= 0 {
		return errors.New("site_id de sonda no válido")
	}
	return nil
}

// Sign embeds a deterministic minisign signature over the document JSON.
func (d *Document) Sign(seed, keyID []byte, comment string) error {
	if err := d.Validate(); err != nil {
		return err
	}
	d.Signature = nil
	payload, err := json.Marshal(d)
	if err != nil {
		return err
	}
	sig, err := update.SignMinisign(payload, seed, keyID, comment)
	if err != nil {
		return err
	}
	d.Signature = &Signature{
		Algorithm: signatureAlgorithm,
		KeyID:     hex.EncodeToString(keyID),
		Value:     sig,
		Comment:   comment,
	}
	return nil
}

// Verify checks an embedded signature with the given minisign public key.
func (d *Document) Verify(publicKey string) error {
	if err := d.Validate(); err != nil {
		return err
	}
	if d.Signature == nil {
		return errors.New("la sonda no lleva firma")
	}
	if d.Signature.Algorithm != signatureAlgorithm {
		return fmt.Errorf("algoritmo de firma desconocido %q", d.Signature.Algorithm)
	}
	signed := *d
	signed.Signature = nil
	payload, err := json.Marshal(&signed)
	if err != nil {
		return err
	}
	if _, err := update.VerifyMinisign(payload, publicKey, d.Signature.Value); err != nil {
		return fmt.Errorf("firma de sonda no válida: %w", err)
	}
	return nil
}

// Load reads a probe document from disk.
func Load(path string) (*Document, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer la sonda: %w", err)
	}
	var d Document
	if err := json.Unmarshal(raw, &d); err != nil {
		return nil, fmt.Errorf("JSON de sonda no válido: %w", err)
	}
	if err := d.Validate(); err != nil {
		return nil, err
	}
	return &d, nil
}

// Save writes a probe document with private permissions.
func Save(path string, d *Document) error {
	if err := d.Validate(); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o600)
}
