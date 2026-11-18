package update

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strings"
	"time"
)

// ManifestSchema identifies the signed update manifest format.
const ManifestSchema = "netdiag/update/v1"

// Release is the signed description of a downloadable binary. Checksum is the
// lowercase hexadecimal SHA256 of the binary.
type Release struct {
	Schema     string    `json:"schema"`
	Version    string    `json:"version"`
	URL        string    `json:"url"`
	Checksum   string    `json:"checksum"`
	ReleasedAt time.Time `json:"released_at"`
	Notes      string    `json:"notes,omitempty"`
	Signature  string    `json:"signature"`
}

// signedManifest is the exact, deterministic set of fields covered by the
// minisign signature. Extra manifest properties are rejected.
type signedManifest struct {
	Schema     string `json:"schema"`
	Version    string `json:"version"`
	URL        string `json:"url"`
	Checksum   string `json:"checksum"`
	ReleasedAt string `json:"released_at"`
	Notes      string `json:"notes,omitempty"`
}

func (r *Release) canonical() ([]byte, error) {
	return json.Marshal(signedManifest{
		Schema:     r.Schema,
		Version:    r.Version,
		URL:        r.URL,
		Checksum:   strings.ToLower(strings.TrimSpace(r.Checksum)),
		ReleasedAt: r.ReleasedAt.UTC().Format(time.RFC3339),
		Notes:      r.Notes,
	})
}

// SignRelease fills the signature field with a minisign signature over the
// canonical manifest. It is intended for release tooling and tests; verification
// never needs the secret key.
func SignRelease(r *Release, seed, keyID []byte) error {
	if r.ReleasedAt.IsZero() {
		r.ReleasedAt = time.Now().UTC().Truncate(time.Second)
	}
	r.Checksum = strings.ToLower(strings.TrimSpace(r.Checksum))
	r.Version = strings.TrimSpace(r.Version)
	r.Schema = ManifestSchema
	msg, err := r.canonical()
	if err != nil {
		return err
	}
	comment := "version:" + r.Version + "\tsha256:" + r.Checksum
	sig, err := SignMinisign(msg, seed, keyID, comment)
	if err != nil {
		return err
	}
	r.Signature = sig
	return nil
}

// ParseManifest validates and verifies a signed manifest using the configured
// minisign public key.
func ParseManifest(data []byte, publicKey string) (*Release, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var rel Release
	if err := dec.Decode(&rel); err != nil {
		return nil, fmt.Errorf("manifiesto JSON no válido: %w", err)
	}
	if err := validateRelease(&rel); err != nil {
		return nil, err
	}
	msg, err := rel.canonical()
	if err != nil {
		return nil, err
	}
	comment, err := VerifyMinisign(msg, publicKey, rel.Signature)
	if err != nil {
		return nil, fmt.Errorf("verificación del manifiesto fallida: %w", err)
	}
	for _, field := range strings.Fields(comment) {
		switch {
		case strings.HasPrefix(field, "version:") && strings.TrimPrefix(field, "version:") != rel.Version:
			return nil, errors.New("el comentario firmado indica otra versión")
		case strings.HasPrefix(field, "sha256:") && strings.ToLower(strings.TrimPrefix(field, "sha256:")) != strings.ToLower(rel.Checksum):
			return nil, errors.New("el comentario firmado indica otro checksum")
		}
	}
	return &rel, nil
}

func validateRelease(rel *Release) error {
	if rel.Schema != ManifestSchema {
		return fmt.Errorf("schema de manifiesto no soportado: %q", rel.Schema)
	}
	rel.Version = strings.TrimSpace(rel.Version)
	if !IsNewer(rel.Version, "0.0.0") {
		return errors.New("versión del manifiesto no válida")
	}
	rel.Checksum = strings.ToLower(strings.TrimSpace(rel.Checksum))
	raw, err := hex.DecodeString(rel.Checksum)
	if err != nil || len(raw) != 32 {
		return errors.New("checksum sha256 no válido")
	}
	if err := validateArtifactURL(rel.URL); err != nil {
		return fmt.Errorf("url de descarga no válida: %w", err)
	}
	if rel.ReleasedAt.IsZero() {
		return errors.New("released_at es obligatorio")
	}
	if len(rel.Notes) > 4096 {
		return errors.New("notes supera 4096 caracteres")
	}
	if strings.TrimSpace(rel.Signature) == "" || len(rel.Signature) > 8192 {
		return errors.New("firma del manifiesto ausente o demasiado larga")
	}
	return nil
}

// validateArtifactURL only permits HTTPS, except for explicit loopback URLs
// used by self-hosted mirrors and integration tests.
func validateArtifactURL(raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Hostname() == "" {
		return errors.New("URL no válida")
	}
	switch u.Scheme {
	case "https":
	case "http":
		if !isLoopbackHost(u.Hostname()) {
			return errors.New("HTTP solo está permitido en localhost")
		}
	default:
		return errors.New("se requiere HTTPS")
	}
	if u.User != nil {
		return errors.New("la URL no puede incluir credenciales")
	}
	return nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// readLimited reads at most max bytes and rejects longer responses.
func readLimited(r io.Reader, max int64) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(r, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, fmt.Errorf("respuesta demasiado grande (límite %d bytes)", max)
	}
	return data, nil
}
