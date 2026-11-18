// Command netdiag-release contains the offline release tooling. It is not part
// of the user-facing binary and never contacts the network.
package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kr3s4l4/netdiag/internal/update"
)

type signingMaterial struct {
	Seed  string `json:"seed"`
	KeyID string `json:"key_id"`
}

func loadMaterial(path string) ([]byte, []byte, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("no se pudo leer la clave de firma: %w", err)
	}
	var m signingMaterial
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, nil, errors.New("la clave de firma debe ser JSON con seed y key_id")
	}
	seed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(m.Seed))
	if err != nil || len(seed) != 32 {
		return nil, nil, errors.New("la semilla debe ser base64 de 32 bytes")
	}
	keyID, err := hex.DecodeString(strings.TrimSpace(m.KeyID))
	if err != nil || len(keyID) != 8 {
		return nil, nil, errors.New("key_id debe ser hexadecimal de 8 bytes")
	}
	return seed, keyID, nil
}

func publicKeyArgument(value string) string {
	if value == "" {
		return ""
	}
	if info, err := os.Stat(value); err == nil && !info.IsDir() && info.Size() <= 1<<16 {
		if raw, err := os.ReadFile(value); err == nil {
			return string(raw)
		}
	}
	return value
}

func usage() {
	fmt.Fprintln(os.Stderr, `Uso: netdiag-release <comando> [opciones]

Comandos:
  keygen          Genera material de firma SOLO PARA PRUEBAS (nunca para releases oficiales)
  sign            Firma un archivo con minisign (ED) y escribe su .minisig
  verify          Verifica un archivo y su .minisig
  manifest        Crea y firma el manifiesto F23
  verify-manifest Verifica un manifiesto F23 con la clave pública`)
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "keygen":
		err = keygenCmd(os.Args[2:])
	case "sign":
		err = signCmd(os.Args[2:])
	case "verify":
		err = verifyCmd(os.Args[2:])
	case "manifest":
		err = manifestCmd(os.Args[2:])
	case "verify-manifest":
		err = verifyManifestCmd(os.Args[2:])
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "netdiag-release:", err)
		os.Exit(1)
	}
}

func keygenCmd(args []string) error {
	fs := flag.NewFlagSet("keygen", flag.ExitOnError)
	dir := fs.String("out", ".", "directorio donde escribir signing.json y release.pub")
	if err := fs.Parse(args); err != nil {
		return err
	}
	seed := make([]byte, 32)
	if _, err := rand.Read(seed); err != nil {
		return err
	}
	keyID := []byte("NDTEST01")
	material := signingMaterial{
		Seed:  base64.StdEncoding.EncodeToString(seed),
		KeyID: hex.EncodeToString(keyID),
	}
	data, err := json.MarshalIndent(material, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(*dir, 0o700); err != nil {
		return err
	}
	keyPath := filepath.Join(*dir, "signing.json")
	pubPath := filepath.Join(*dir, "release.pub")
	if err := os.WriteFile(keyPath, append(data, '\n'), 0o600); err != nil {
		return err
	}
	pub, err := update.PublicKeyFromSeed(seed, keyID)
	if err != nil {
		return err
	}
	if err := os.WriteFile(pubPath, []byte("untrusted comment: test release key\n"+pub+"\n"), 0o644); err != nil {
		return err
	}
	fmt.Printf("material de prueba: %s (0600) y %s\n", keyPath, pubPath)
	return nil
}

func signCmd(args []string) error {
	fs := flag.NewFlagSet("sign", flag.ExitOnError)
	in := fs.String("in", "", "archivo a firmar")
	out := fs.String("out", "", "archivo .minisig de salida")
	keyPath := fs.String("seed-file", "", "JSON con seed y key_id")
	comment := fs.String("comment", "", "comentario de confianza")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *in == "" || *out == "" || *keyPath == "" {
		return errors.New("faltan --in, --out o --seed-file")
	}
	seed, keyID, err := loadMaterial(*keyPath)
	if err != nil {
		return err
	}
	data, err := os.ReadFile(*in)
	if err != nil {
		return err
	}
	sig, err := update.SignMinisign(data, seed, keyID, *comment)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o700); err != nil {
		return err
	}
	return os.WriteFile(*out, []byte(sig), 0o644)
}

func verifyCmd(args []string) error {
	fs := flag.NewFlagSet("verify", flag.ExitOnError)
	in := fs.String("in", "", "archivo verificado")
	sigPath := fs.String("sig", "", "archivo .minisig")
	key := fs.String("key", "", "clave pública (archivo .pub o base64)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *in == "" || *sigPath == "" || *key == "" {
		return errors.New("faltan --in, --sig o --key")
	}
	data, err := os.ReadFile(*in)
	if err != nil {
		return err
	}
	sig, err := os.ReadFile(*sigPath)
	if err != nil {
		return err
	}
	comment, err := update.VerifyMinisign(data, publicKeyArgument(*key), string(sig))
	if err != nil {
		return fmt.Errorf("VERIFICACIÓN FALLIDA: %w", err)
	}
	fmt.Printf("OK: %s (%s)\n", *in, comment)
	return nil
}

func manifestCmd(args []string) error {
	fs := flag.NewFlagSet("manifest", flag.ExitOnError)
	version := fs.String("version", "", "versión del release")
	url := fs.String("url", "", "URL HTTPS del binario")
	checksum := fs.String("checksum", "", "SHA256 del binario")
	notes := fs.String("notes", "", "notas de la release")
	releasedAt := fs.String("released-at", "", "fecha RFC3339 (por defecto, ahora)")
	keyPath := fs.String("seed-file", "", "JSON con seed y key_id")
	out := fs.String("out", "", "archivo manifest.json de salida")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *version == "" || *url == "" || *checksum == "" || *keyPath == "" || *out == "" {
		return errors.New("faltan --version, --url, --checksum, --seed-file o --out")
	}
	seed, keyID, err := loadMaterial(*keyPath)
	if err != nil {
		return err
	}
	when := time.Now().UTC().Truncate(time.Second)
	if *releasedAt != "" {
		when, err = time.Parse(time.RFC3339, *releasedAt)
		if err != nil {
			return fmt.Errorf("released-at no válido: %w", err)
		}
	}
	rel := &update.Release{
		Version:    *version,
		URL:        *url,
		Checksum:   *checksum,
		ReleasedAt: when,
		Notes:      *notes,
	}
	if err := update.SignRelease(rel, seed, keyID); err != nil {
		return err
	}
	data, err := json.MarshalIndent(rel, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(*out), 0o700); err != nil {
		return err
	}
	return os.WriteFile(*out, append(data, '\n'), 0o644)
}

func verifyManifestCmd(args []string) error {
	fs := flag.NewFlagSet("verify-manifest", flag.ExitOnError)
	in := fs.String("in", "", "manifiesto F23")
	key := fs.String("key", "", "clave pública (archivo .pub o base64)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *in == "" || *key == "" {
		return errors.New("faltan --in o --key")
	}
	data, err := os.ReadFile(*in)
	if err != nil {
		return err
	}
	rel, err := update.ParseManifest(data, publicKeyArgument(*key))
	if err != nil {
		return fmt.Errorf("VERIFICACIÓN FALLIDA: %w", err)
	}
	fmt.Printf("OK: manifiesto %s -> %s (%s)\n", rel.Version, rel.URL, rel.Checksum)
	return nil
}
