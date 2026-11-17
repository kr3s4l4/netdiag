// Package cryptoutil provides small, dependency-free cryptography helpers:
// AES-256-GCM at-rest encryption and secure random tokens.
package cryptoutil

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Box encrypts and decrypts small blobs with a local AES-256-GCM key.
type Box struct {
	aead cipher.AEAD
}

// LoadOrCreate loads a 32-byte key from path, creating it with 0600
// permissions when missing. Parent directories are created with 0700.
func LoadOrCreate(path string) (*Box, error) {
	key, err := os.ReadFile(path)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			return nil, err
		}
		key = make([]byte, 32)
		if _, err := io.ReadFull(rand.Reader, key); err != nil {
			return nil, err
		}
		tmp := path + ".tmp"
		if err := os.WriteFile(tmp, key, 0o600); err != nil {
			return nil, err
		}
		if err := os.Rename(tmp, path); err != nil {
			return nil, err
		}
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("clave de cifrado inválida: se esperaban 32 bytes, hay %d", len(key))
	}
	return New(key)
}

// New builds a Box from a 32-byte key.
func New(key []byte) (*Box, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return &Box{aead: aead}, nil
}

// Encrypt returns nonce||ciphertext||tag.
func (b *Box) Encrypt(plain []byte) ([]byte, error) {
	nonce := make([]byte, b.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	return b.aead.Seal(nonce, nonce, plain, nil), nil
}

// Decrypt opens a blob produced by Encrypt.
func (b *Box) Decrypt(blob []byte) ([]byte, error) {
	ns := b.aead.NonceSize()
	if len(blob) < ns+16 {
		return nil, errors.New("blob cifrado demasiado corto")
	}
	return b.aead.Open(nil, blob[:ns], blob[ns:], nil)
}

// EncryptString is a convenience wrapper returning base64 text.
func (b *Box) EncryptString(s string) (string, error) {
	out, err := b.Encrypt([]byte(s))
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(out), nil
}

// DecryptString is the inverse of EncryptString.
func (b *Box) DecryptString(s string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return "", err
	}
	out, err := b.Decrypt(raw)
	if err != nil {
		return "", err
	}
	return string(out), nil
}

// Token returns a URL-safe random token with n bytes of entropy.
func Token(n int) string {
	if n <= 0 {
		n = 32
	}
	b := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		panic("cryptoutil: no hay entropía disponible: " + err.Error())
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// HexToken returns a hex-encoded random token.
func HexToken(n int) string {
	if n <= 0 {
		n = 32
	}
	b := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		panic("cryptoutil: no hay entropía disponible: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// HashToken hashes a bearer token for storage.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// ConstantTimeEqual compares two strings without leaking length-dependent
// timing beyond the length check itself.
func ConstantTimeEqual(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
