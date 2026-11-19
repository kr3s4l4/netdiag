package cryptoutil

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

// LoadSigningMaterial reads the local JSON used by minisign-based features:
// {"seed":"<base64 32 bytes>","key_id":"<hex16>"}. A bare base64 seed file is
// accepted for compatibility. The seed is a secret and must stay outside the
// repository and out of logs.
func LoadSigningMaterial(path, keyIDOverride string) (seed, keyID []byte, err error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, nil, fmt.Errorf("no se pudo leer la clave de firma: %w", err)
	}
	if info.Mode().Perm()&0o077 != 0 {
		return nil, nil, errors.New("la clave de firma debe tener permisos 0600: sin acceso para grupo u otros")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("no se pudo leer la clave de firma: %w", err)
	}
	var material struct {
		Seed  string `json:"seed"`
		KeyID string `json:"key_id"`
	}
	encoded := strings.TrimSpace(string(raw))
	if jsonErr := json.Unmarshal(raw, &material); jsonErr != nil {
		material.Seed = encoded
	}
	seed, err = base64.StdEncoding.DecodeString(strings.TrimSpace(material.Seed))
	if err != nil || len(seed) != 32 {
		return nil, nil, errors.New("la semilla Ed25519 debe estar en base64 y tener 32 bytes")
	}
	id := material.KeyID
	if keyIDOverride != "" {
		id = keyIDOverride
	}
	keyID, err = hex.DecodeString(strings.TrimSpace(id))
	if err != nil || len(keyID) != 8 {
		return nil, nil, errors.New("el id de clave minisign debe tener 16 caracteres hexadecimales")
	}
	return seed, keyID, nil
}

// PublicKeyB64 accepts a bare base64 minisign key, a two-line key file, or the
// path to such a file, and returns only the encoded key. Public keys are not
// secret; this helper never reads private material.
func PublicKeyB64(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if info, err := os.Stat(value); err == nil && !info.IsDir() && info.Size() <= 1<<16 {
		if raw, err := os.ReadFile(value); err == nil {
			value = string(raw)
		}
	}
	if !strings.ContainsAny(value, "\r\n") {
		return value
	}
	for _, line := range strings.Split(value, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, "untrusted comment:") || strings.HasPrefix(line, "trusted comment:") {
			continue
		}
		return line
	}
	return ""
}
