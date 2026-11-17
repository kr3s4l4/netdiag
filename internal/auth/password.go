// Package auth implements local authentication: Argon2id password hashes,
// server-side sessions, CSRF tokens and login throttling.
package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
)

// Tunable Argon2id parameters. These are the OWASP-recommended baseline
// (64 MiB, 3 iterations, 4 lanes) and cost ~50ms on a modern CPU.
const (
	argonMemory  = 64 * 1024
	argonTime    = 3
	argonThreads = 4
	argonKeyLen  = 32
	argonSaltLen = 16
)

// MinPasswordLength is the minimum accepted password length.
const MinPasswordLength = 10

// ErrWeakPassword is returned when a password is too short.
var ErrWeakPassword = fmt.Errorf("la contraseña debe tener al menos %d caracteres", MinPasswordLength)

var b64 = base64.RawStdEncoding

// HashPassword produces a PHC-format Argon2id hash.
func HashPassword(password string) (string, error) {
	if len(password) < MinPasswordLength {
		return "", ErrWeakPassword
	}
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	key := argon2.IDKey([]byte(password), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf("$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, argonMemory, argonTime, argonThreads,
		b64.EncodeToString(salt), b64.EncodeToString(key)), nil
}

// VerifyPassword checks a password against a stored hash. Both Argon2id and
// legacy bcrypt hashes are accepted.
func VerifyPassword(hash, password string) bool {
	hash = strings.TrimSpace(hash)
	switch {
	case strings.HasPrefix(hash, "$argon2id$"):
		return verifyArgon2(hash, password)
	case strings.HasPrefix(hash, "$2a$"), strings.HasPrefix(hash, "$2b$"), strings.HasPrefix(hash, "$2y$"):
		return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
	default:
		return false
	}
}

func verifyArgon2(hash, password string) bool {
	parts := strings.Split(hash, "$")
	// "", argon2id, v=19, params, salt, key
	if len(parts) != 6 {
		return false
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil || version != argon2.Version {
		return false
	}
	var memory uint32
	var time uint32
	var threads uint8
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads); err != nil {
		return false
	}
	salt, err := b64.DecodeString(parts[4])
	if err != nil {
		return false
	}
	want, err := b64.DecodeString(parts[5])
	if err != nil || len(want) == 0 {
		return false
	}
	got := argon2.IDKey([]byte(password), salt, time, memory, threads, uint32(len(want)))
	return subtle.ConstantTimeCompare(got, want) == 1
}

// dummyHash is used to keep login timing uniform for unknown users.
var dummyHash = func() string {
	h, _ := HashPassword("netdiag-dummy-password-value")
	return h
}()

// BurnTime performs a throwaway verification so unknown usernames cost the
// same as known ones.
func BurnTime(password string) {
	_ = VerifyPassword(dummyHash, password)
}

// HashToken returns the hex SHA-256 of a raw session token. Session IDs are
// stored hashed, so a database leak cannot be replayed.
func HashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

// NewToken returns a cryptographically random URL-safe token.
func NewToken(bytes int) (string, error) {
	if bytes < 16 {
		bytes = 32
	}
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return b64.EncodeToString(buf), nil
}

// ValidatePassword applies the password policy.
func ValidatePassword(password string) error {
	if len(password) < MinPasswordLength {
		return ErrWeakPassword
	}
	var hasLetter, hasDigit bool
	for _, r := range password {
		switch {
		case r >= '0' && r <= '9':
			hasDigit = true
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'):
			hasLetter = true
		}
	}
	if !hasLetter || !hasDigit {
		return errors.New("la contraseña debe combinar letras y números")
	}
	return nil
}
