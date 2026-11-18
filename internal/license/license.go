// Package license implements offline, cryptographically signed licences.
//
// A licence is a JSON file stored at <data_dir>/license.json. Its signature
// covers every field except "signature" itself, serialised in the fixed order
// of the payload struct below. Verification is entirely local: NetDiag never
// contacts a licensing server and never phones home.
package license

import (
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Schema is the only accepted licence format version.
const Schema = "netdiag-license/v1"

// PublicKeyBase64 is the Ed25519 public key of the licence issuer. It is
// deliberately separate from the minisign key used for release artefacts.
const PublicKeyBase64 = "pNc3VNWpOKBid1TQsm6rF52MqvBxyNdvPJxvqSNha/s="

// License is the content of a signed licence file.
type License struct {
	Schema          string   `json:"schema"`
	LicenseID       string   `json:"license_id"`
	Customer        string   `json:"customer"`
	Edition         string   `json:"edition"`
	MaxManagedHosts int      `json:"max_managed_hosts"`
	MaxSites        int      `json:"max_sites"`
	RetentionDays   int      `json:"retention_days"`
	ExpiresAt       string   `json:"expires_at"`
	Capabilities    []string `json:"capabilities"`
	SupportLevel    string   `json:"support_level"`
	Signature       string   `json:"signature"`
}

// payload is the exact byte sequence covered by the signature. Field order is
// part of the format; do not reorder it without bumping Schema.
type payload struct {
	Schema          string   `json:"schema"`
	LicenseID       string   `json:"license_id"`
	Customer        string   `json:"customer"`
	Edition         string   `json:"edition"`
	MaxManagedHosts int      `json:"max_managed_hosts"`
	MaxSites        int      `json:"max_sites"`
	RetentionDays   int      `json:"retention_days"`
	ExpiresAt       string   `json:"expires_at"`
	Capabilities    []string `json:"capabilities"`
	SupportLevel    string   `json:"support_level"`
}

// Limits summarises the contracted band. Zero means "no limit" for hosts and
// sites; retention is always expressed in days.
type Limits struct {
	MaxManagedHosts int `json:"max_managed_hosts"`
	MaxSites        int `json:"max_sites"`
	RetentionDays   int `json:"retention_days"`
}

// Status is the safe representation returned by the API. It never includes the
// signature or the customer name.
type Status struct {
	Status       string   `json:"status"`
	Edition      string   `json:"edition"`
	LicenseID    string   `json:"license_id,omitempty"`
	ExpiresAt    string   `json:"expires_at"`
	Limits       Limits   `json:"limits"`
	Capabilities []string `json:"capabilities"`
	SupportLevel string   `json:"support_level"`
	ManagedHosts int      `json:"managed_hosts"`
	Sites        int      `json:"sites"`
	Warnings     []string `json:"warnings"`
}

func (l *License) canonical() ([]byte, error) {
	return json.Marshal(payload{
		Schema:          l.Schema,
		LicenseID:       l.LicenseID,
		Customer:        l.Customer,
		Edition:         l.Edition,
		MaxManagedHosts: l.MaxManagedHosts,
		MaxSites:        l.MaxSites,
		RetentionDays:   l.RetentionDays,
		ExpiresAt:       l.ExpiresAt,
		Capabilities:    l.Capabilities,
		SupportLevel:    l.SupportLevel,
	})
}

// Validate checks the structural fields of a licence without verifying its
// signature.
func Validate(l *License) error {
	if l == nil {
		return errors.New("licencia vacía")
	}
	if l.Schema != Schema {
		return fmt.Errorf("esquema de licencia desconocido %q; se esperaba %q", l.Schema, Schema)
	}
	if strings.TrimSpace(l.LicenseID) == "" {
		return errors.New("falta license_id")
	}
	switch l.Edition {
	case "community", "pro", "enterprise":
	default:
		return fmt.Errorf("edición de licencia desconocida %q", l.Edition)
	}
	if l.MaxManagedHosts <= 0 || l.MaxSites <= 0 || l.RetentionDays <= 0 {
		return errors.New("los límites de la licencia deben ser positivos")
	}
	if _, err := time.Parse(time.RFC3339, l.ExpiresAt); err != nil {
		return fmt.Errorf("expires_at no es una fecha RFC3339 válida: %w", err)
	}
	return nil
}

// Sign fills the signature field using a base64-encoded 32-byte Ed25519 seed.
// It is intended for the offline issuance tool, never for the running server.
func Sign(l *License, seedBase64 string) error {
	if err := Validate(l); err != nil {
		return err
	}
	seed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(seedBase64))
	if err != nil {
		return fmt.Errorf("semilla de firma no válida: %w", err)
	}
	if len(seed) != ed25519.SeedSize {
		return fmt.Errorf("la semilla debe tener %d bytes", ed25519.SeedSize)
	}
	data, err := l.canonical()
	if err != nil {
		return err
	}
	key := ed25519.NewKeyFromSeed(seed)
	l.Signature = base64.StdEncoding.EncodeToString(ed25519.Sign(key, data))
	return nil
}

// Verify checks the signature and the structure of a licence against a
// base64-encoded Ed25519 public key.
func Verify(l *License, publicKeyBase64 string) error {
	if l == nil {
		return errors.New("licencia vacía")
	}
	pub, err := decodePublicKey(publicKeyBase64)
	if err != nil {
		return err
	}
	sig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(l.Signature))
	if err != nil || len(sig) != ed25519.SignatureSize {
		return errors.New("firma de licencia no válida")
	}
	data, err := l.canonical()
	if err != nil {
		return err
	}
	if !ed25519.Verify(pub, data, sig) {
		return errors.New("la firma de la licencia no coincide con la clave pública")
	}
	return Validate(l)
}

func decodePublicKey(value string) (ed25519.PublicKey, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, errors.New("falta la clave pública de licencias")
	}
	raw, err := base64.StdEncoding.DecodeString(value)
	if err != nil {
		return nil, fmt.Errorf("clave pública de licencias no válida: %w", err)
	}
	if len(raw) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("la clave pública debe tener %d bytes", ed25519.PublicKeySize)
	}
	return ed25519.PublicKey(raw), nil
}

// Manager loads, activates and reports the local licence. A missing file means
// Community edition; an invalid file is reported but never blocks the product.
type Manager struct {
	mu      sync.RWMutex
	path    string
	pub     ed25519.PublicKey
	current *License
	problem string
}

// New builds a manager using the issuer key compiled into the binary.
func New(dataDir string) (*Manager, error) {
	return NewWithKey(dataDir, PublicKeyBase64)
}

// NewWithKey builds a manager with an explicit public key. It is used by tests
// and by private deployments that issue their own licences.
func NewWithKey(dataDir, publicKeyBase64 string) (*Manager, error) {
	pub, err := decodePublicKey(publicKeyBase64)
	if err != nil {
		return nil, err
	}
	m := &Manager{path: filepath.Join(dataDir, "license.json"), pub: pub}
	if err := m.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		m.problem = err.Error()
	}
	return m, nil
}

// Path returns the location of the local licence file.
func (m *Manager) Path() string { return m.path }

// Load re-reads the licence file. A missing file selects Community silently;
// a malformed or badly signed file is remembered as a warning.
func (m *Manager) Load() error {
	data, err := os.ReadFile(m.path)
	if err != nil {
		m.mu.Lock()
		m.current = nil
		if errors.Is(err, os.ErrNotExist) {
			m.problem = ""
		} else {
			m.problem = err.Error()
		}
		m.mu.Unlock()
		return err
	}
	var l License
	if err := json.Unmarshal(data, &l); err != nil {
		m.setProblem(nil, fmt.Sprintf("no se pudo leer license.json: %v", err))
		return fmt.Errorf("license.json no es JSON válido: %w", err)
	}
	if err := Verify(&l, base64.StdEncoding.EncodeToString(m.pub)); err != nil {
		m.setProblem(nil, err.Error())
		return err
	}
	m.mu.Lock()
	m.current = &l
	m.problem = ""
	m.mu.Unlock()
	return nil
}

func (m *Manager) setProblem(l *License, problem string) {
	m.mu.Lock()
	m.current = l
	m.problem = problem
	m.mu.Unlock()
}

// Activate verifies a licence file and installs it as the local licence.
func (m *Manager) Activate(src string) (*License, error) {
	data, err := os.ReadFile(src)
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer la licencia: %w", err)
	}
	var l License
	if err := json.Unmarshal(data, &l); err != nil {
		return nil, fmt.Errorf("la licencia no es JSON válido: %w", err)
	}
	if err := Verify(&l, base64.StdEncoding.EncodeToString(m.pub)); err != nil {
		return nil, err
	}
	if err := writeAtomic(m.path, data, 0o600); err != nil {
		return nil, fmt.Errorf("no se pudo guardar la licencia: %w", err)
	}
	if err := m.Load(); err != nil {
		return nil, err
	}
	return &l, nil
}

// Clear removes the local licence and returns the product to Community.
func (m *Manager) Clear() error {
	err := os.Remove(m.path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	m.mu.Lock()
	m.current = nil
	m.problem = ""
	m.mu.Unlock()
	return nil
}

// Current returns a copy of the active licence, or nil for Community.
func (m *Manager) Current() *License {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.current == nil {
		return nil
	}
	copy := *m.current
	copy.Capabilities = append([]string(nil), m.current.Capabilities...)
	return &copy
}

// Problem returns the reason why the installed licence is not valid.
func (m *Manager) Problem() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.problem
}

// Status builds the safe API/UI representation. Over-limit and expiry are
// warnings: they never disable reading, scanning or data access.
func (m *Manager) Status(now time.Time, managedHosts, sites int) Status {
	m.mu.RLock()
	defer m.mu.RUnlock()
	st := Status{
		Status:       "community",
		Edition:      "community",
		Capabilities: []string{},
		Limits:       Limits{RetentionDays: 7},
		Warnings:     []string{},
		ManagedHosts: managedHosts,
		Sites:        sites,
	}
	if m.problem != "" {
		st.Status = "invalid"
		st.Warnings = append(st.Warnings, "La licencia instalada no es válida: "+m.problem+". Se aplican las capacidades Community y se conservan todos los datos.")
		return st
	}
	if m.current == nil {
		return st
	}
	l := m.current
	st.Status = "active"
	st.Edition = l.Edition
	st.LicenseID = l.LicenseID
	st.ExpiresAt = l.ExpiresAt
	st.Limits = Limits{
		MaxManagedHosts: l.MaxManagedHosts,
		MaxSites:        l.MaxSites,
		RetentionDays:   l.RetentionDays,
	}
	st.Capabilities = append([]string{}, l.Capabilities...)
	st.SupportLevel = l.SupportLevel

	expiry, err := time.Parse(time.RFC3339, l.ExpiresAt)
	if err == nil && now.After(expiry) {
		st.Status = "expired"
		st.Warnings = append(st.Warnings, "La licencia caducó el "+expiry.Format("2006-01-02")+". Siguen disponibles la lectura y los datos; las funciones de pago requieren renovación.")
	}
	if l.MaxManagedHosts > 0 && managedHosts > l.MaxManagedHosts {
		st.Warnings = append(st.Warnings, fmt.Sprintf("Hay %d hosts gestionados y la banda contratada es %d. No se bloquea ninguna función.", managedHosts, l.MaxManagedHosts))
	}
	if l.MaxSites > 0 && sites > l.MaxSites {
		st.Warnings = append(st.Warnings, fmt.Sprintf("Hay %d sedes y la licencia cubre %d. No se bloquea ninguna función.", sites, l.MaxSites))
	}
	return st
}

func writeAtomic(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".license-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
