package update

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/blake2b"
)

const (
	minisignPublicAlg = "Ed" // public keys always use the legacy tag
	minisignHashedAlg = "ED" // hashed signatures: Ed25519 over BLAKE2b-512
)

// VerifyMinisign checks a minisign signature over message using a base64
// public key (bare key or full two-line public-key file). It accepts only the
// modern prehashed "ED" algorithm and verifies both the file signature and
// the trusted-comment signature. The authenticated trusted comment is returned
// so callers can bind extra metadata such as the release version.
func VerifyMinisign(message []byte, publicKey, signature string) (string, error) {
	pubRaw, err := decodeMinisignPublicKey(publicKey)
	if err != nil {
		return "", err
	}
	var pk ed25519.PublicKey = ed25519.PublicKey(pubRaw[10:42])
	keyID := pubRaw[2:10]

	body, comment, global, err := parseMinisignSignature(signature)
	if err != nil {
		return "", err
	}
	if string(body[:2]) != minisignHashedAlg {
		return "", errors.New("se requiere una firma minisign prehashed (ED)")
	}
	if !bytes.Equal(body[2:10], keyID) {
		return "", errors.New("el id de clave de la firma no coincide con la clave pública")
	}
	sig := body[10:74]

	h, err := blake2b.New512(nil)
	if err != nil {
		return "", err
	}
	_, _ = h.Write(message)
	if !ed25519.Verify(pk, h.Sum(nil), sig) {
		return "", errors.New("firma minisign no válida")
	}

	globalMessage := make([]byte, 0, len(sig)+len(comment))
	globalMessage = append(globalMessage, sig...)
	globalMessage = append(globalMessage, []byte(comment)...)
	if !ed25519.Verify(pk, globalMessage, global) {
		return "", errors.New("firma del comentario de confianza no válida")
	}
	return comment, nil
}

func decodeMinisignPublicKey(s string) ([]byte, error) {
	s = bareBase64Key(strings.TrimSpace(s))
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("clave pública minisign no válida: %w", err)
	}
	if len(raw) != 42 {
		return nil, errors.New("clave pública minisign de longitud incorrecta")
	}
	if string(raw[:2]) != minisignPublicAlg {
		return nil, errors.New("algoritmo de clave pública minisign no compatible")
	}
	return raw, nil
}

func bareBase64Key(s string) string {
	if !strings.ContainsAny(s, "\r\n") {
		return strings.TrimSpace(s)
	}
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(strings.TrimSuffix(line, "\r"))
		if line == "" || strings.HasPrefix(line, "untrusted comment:") || strings.HasPrefix(line, "trusted comment:") {
			continue
		}
		return line
	}
	return ""
}

// parseMinisignSignature accepts the canonical four-line signature file:
// untrusted comment, base64(alg||keyID||sig), trusted comment, base64(global).
func parseMinisignSignature(s string) (body []byte, comment string, global []byte, err error) {
	lines := make([]string, 0, 4)
	for _, line := range strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) != 4 {
		return nil, "", nil, errors.New("firma minisign incompleta")
	}
	if !strings.HasPrefix(lines[0], "untrusted comment:") {
		return nil, "", nil, errors.New("comentario no fiable ausente en la firma")
	}
	if !strings.HasPrefix(lines[2], "trusted comment: ") {
		return nil, "", nil, errors.New("comentario de confianza ausente en la firma")
	}
	body, err = base64.StdEncoding.DecodeString(lines[1])
	if err != nil || len(body) != 74 {
		return nil, "", nil, errors.New("cuerpo de firma minisign no válido")
	}
	global, err = base64.StdEncoding.DecodeString(lines[3])
	if err != nil || len(global) != 64 {
		return nil, "", nil, errors.New("firma global minisign no válida")
	}
	return body, strings.TrimPrefix(lines[2], "trusted comment: "), global, nil
}

// SignMinisign is used by the release tooling and by tests to produce the
// prehashed signature format verified by VerifyMinisign. seed must be a
// 64-byte Ed25519 private key and keyID an 8-byte identifier.
func SignMinisign(message []byte, seed, keyID []byte, trustedComment string) (string, error) {
	if len(seed) != ed25519.SeedSize {
		return "", errors.New("la semilla Ed25519 debe tener 32 bytes")
	}
	if len(keyID) != 8 {
		return "", errors.New("el id de clave debe tener 8 bytes")
	}
	priv := ed25519.NewKeyFromSeed(seed)

	h, err := blake2b.New512(nil)
	if err != nil {
		return "", err
	}
	_, _ = h.Write(message)
	sig := ed25519.Sign(priv, h.Sum(nil))

	body := make([]byte, 0, 74)
	body = append(body, []byte(minisignHashedAlg)...)
	body = append(body, keyID...)
	body = append(body, sig...)
	globalMessage := append(append([]byte{}, sig...), []byte(trustedComment)...)
	global := ed25519.Sign(priv, globalMessage)

	return fmt.Sprintf("untrusted comment: firma NetDiag\n%s\ntrusted comment: %s\n%s\n",
		base64.StdEncoding.EncodeToString(body),
		trustedComment,
		base64.StdEncoding.EncodeToString(global)), nil
}

// PublicKeyFromSeed returns the minisign base64 public key for an Ed25519 seed.
func PublicKeyFromSeed(seed, keyID []byte) (string, error) {
	if len(seed) != ed25519.SeedSize {
		return "", errors.New("la semilla Ed25519 debe tener 32 bytes")
	}
	if len(keyID) != 8 {
		return "", errors.New("el id de clave debe tener 8 bytes")
	}
	pub := ed25519.NewKeyFromSeed(seed).Public().(ed25519.PublicKey)
	raw := make([]byte, 0, 42)
	raw = append(raw, []byte(minisignPublicAlg)...)
	raw = append(raw, keyID...)
	raw = append(raw, pub...)
	return base64.StdEncoding.EncodeToString(raw), nil
}
