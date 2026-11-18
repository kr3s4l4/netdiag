// Package container implements the open, signed .ndc container format.
//
// An .ndc file is a regular ZIP archive: manifest.json describes every file in
// payload/, signature.minisig is a minisign signature over the canonical
// manifest, and sbom.cdx.json is optional. Anyone can inspect it with unzip;
// NetDiag is only needed to verify the signature and the digests.
package container

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/kr3s4l4/netdiag/internal/update"
)

const (
	// Schema is the only supported container manifest version.
	Schema = "netdiag/container/v1"

	ManifestName  = "manifest.json"
	SignatureName = "signature.minisig"
	SBOMName      = "sbom.cdx.json"
	PayloadPrefix = "payload/"
)

const (
	maxManifestBytes = 1 << 20 // 1 MiB
	maxSignatureSize = 8 << 10 // 8 KiB
	maxFileBytes     = 1 << 30 // 1 GiB per payload file
	maxFiles         = 10000
	maxNameLen       = 512
)

var fixedZipTime = time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)

// FileEntry is one authenticated file inside the container.
type FileEntry struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

// Manifest is the signed description of the container contents.
type Manifest struct {
	Schema    string      `json:"schema"`
	Version   string      `json:"version"`
	Kind      string      `json:"kind"` // release | evidence | site
	CreatedAt string      `json:"created_at"`
	Files     []FileEntry `json:"files"`
	Producer  string      `json:"producer"`
}

// InputFile is a file to include under payload/ when building a container.
type InputFile struct {
	Name string // relative name inside payload/, e.g. "netdiag"
	Path string // source path on disk
	Data []byte // alternative to Path, useful for generated reports
}

// BuildOptions configures Build. Seed and KeyID are the minisign signing
// material; they are required only for issuing containers, never for verifying.
type BuildOptions struct {
	Version   string
	Kind      string
	Producer  string
	Files     []InputFile
	SBOM      []byte
	Seed      []byte
	KeyID     []byte
	CreatedAt time.Time
}

// canonicalManifest is the exact field order covered by the signature.
type canonicalManifest struct {
	Schema    string      `json:"schema"`
	Version   string      `json:"version"`
	Kind      string      `json:"kind"`
	CreatedAt string      `json:"created_at"`
	Files     []FileEntry `json:"files"`
	Producer  string      `json:"producer"`
}

// CanonicalManifest returns the deterministic byte sequence that is signed and
// stored verbatim as manifest.json.
func CanonicalManifest(m *Manifest) ([]byte, error) {
	if m == nil {
		return nil, errors.New("manifiesto vacío")
	}
	files := append([]FileEntry(nil), m.Files...)
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	return json.Marshal(canonicalManifest{
		Schema:    m.Schema,
		Version:   m.Version,
		Kind:      m.Kind,
		CreatedAt: m.CreatedAt,
		Files:     files,
		Producer:  m.Producer,
	})
}

// PayloadDigest is a stable digest over the ordered list of file digests and
// names. It is bound into the trusted minisign comment as sha256:<digest>.
func PayloadDigest(m *Manifest) string {
	h := sha256.New()
	files := append([]FileEntry(nil), m.Files...)
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	for _, f := range files {
		fmt.Fprintf(h, "%s  %s\n", strings.ToLower(f.SHA256), f.Path)
	}
	return hex.EncodeToString(h.Sum(nil))
}

func validKind(kind string) bool {
	switch kind {
	case "release", "evidence", "site":
		return true
	default:
		return false
	}
}

func validName(name string) bool {
	if name == "" || len(name) > maxNameLen {
		return false
	}
	if strings.HasPrefix(name, "/") || strings.HasPrefix(name, PayloadPrefix) ||
		strings.Contains(name, "\\") || strings.Contains(name, "..") {
		return false
	}
	for _, r := range name {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	return true
}

// Build writes a signed .ndc container to dst atomically.
func Build(dst string, opts BuildOptions) error {
	if strings.TrimSpace(opts.Version) == "" {
		return errors.New("falta la versión del contenedor")
	}
	if !validKind(opts.Kind) {
		return fmt.Errorf("tipo de contenedor no válido %q; use release, evidence o site", opts.Kind)
	}
	if len(opts.Files) == 0 {
		return errors.New("el contenedor no tiene ficheros")
	}
	if len(opts.Files) > maxFiles {
		return errors.New("demasiados ficheros")
	}
	if err := validateSigningKey(opts.Seed, opts.KeyID); err != nil {
		return err
	}
	if len(opts.SBOM) > maxManifestBytes {
		return errors.New("el SBOM es demasiado grande")
	}

	created := opts.CreatedAt
	if created.IsZero() {
		created = time.Now().UTC()
	}
	m := &Manifest{
		Schema:    Schema,
		Version:   strings.TrimSpace(opts.Version),
		Kind:      opts.Kind,
		CreatedAt: created.UTC().Truncate(time.Second).Format(time.RFC3339),
		Producer:  strings.TrimSpace(opts.Producer),
		Files:     make([]FileEntry, 0, len(opts.Files)),
	}
	if m.Producer == "" {
		m.Producer = "netdiag"
	}

	contents := make(map[string][]byte, len(opts.Files))
	seen := make(map[string]bool, len(opts.Files))
	for _, in := range opts.Files {
		if !validName(in.Name) {
			return fmt.Errorf("nombre de fichero no válido %q", in.Name)
		}
		if seen[in.Name] {
			return fmt.Errorf("fichero duplicado %q", in.Name)
		}
		seen[in.Name] = true
		data := in.Data
		if data == nil {
			raw, err := os.ReadFile(in.Path)
			if err != nil {
				return fmt.Errorf("no se pudo leer %s: %w", in.Path, err)
			}
			data = raw
		}
		if int64(len(data)) > maxFileBytes {
			return fmt.Errorf("el fichero %s supera el tamaño máximo", in.Name)
		}
		sum := sha256.Sum256(data)
		m.Files = append(m.Files, FileEntry{
			Path:   in.Name,
			SHA256: hex.EncodeToString(sum[:]),
			Size:   int64(len(data)),
		})
		contents[in.Name] = data
	}

	canonical, err := CanonicalManifest(m)
	if err != nil {
		return err
	}
	comment := "version:" + m.Version + "\tsha256:" + PayloadDigest(m)
	if len(opts.SBOM) > 0 {
		sum := sha256.Sum256(opts.SBOM)
		comment += "\tsbom:" + hex.EncodeToString(sum[:])
	}
	sig, err := update.SignMinisign(canonical, opts.Seed, opts.KeyID, comment)
	if err != nil {
		return err
	}

	dir := filepath.Dir(dst)
	tmp, err := os.CreateTemp(dir, ".netdiag-*.ndc")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if err := writeContainer(tmp, canonical, contents, opts.SBOM, sig); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, dst)
}

func validateSigningKey(seed, keyID []byte) error {
	if len(seed) != 32 {
		return errors.New("la semilla Ed25519 debe tener 32 bytes")
	}
	if len(keyID) != 8 {
		return errors.New("el id de clave minisign debe tener 8 bytes")
	}
	return nil
}

func writeContainer(w io.Writer, canonical []byte, contents map[string][]byte, sbom []byte, sig string) error {
	zw := zip.NewWriter(w)
	add := func(name string, data []byte) error {
		h := &zip.FileHeader{Name: name, Method: zip.Deflate}
		h.SetModTime(fixedZipTime)
		e, err := zw.CreateHeader(h)
		if err != nil {
			return err
		}
		_, err = e.Write(data)
		return err
	}
	if err := add(ManifestName, canonical); err != nil {
		zw.Close()
		return err
	}
	names := make([]string, 0, len(contents))
	for name := range contents {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		if err := add(PayloadPrefix+name, contents[name]); err != nil {
			zw.Close()
			return err
		}
	}
	if len(sbom) > 0 {
		if err := add(SBOMName, sbom); err != nil {
			zw.Close()
			return err
		}
	}
	if err := add(SignatureName, []byte(sig)); err != nil {
		zw.Close()
		return err
	}
	return zw.Close()
}

// FileReport is the result of checking one payload file against the manifest.
type FileReport struct {
	Path    string `json:"path"`
	Size    int64  `json:"size"`
	SHA256  string `json:"sha256"`
	Matches bool   `json:"matches"`
}

// SignatureReport describes the signature state without ever trusting it in
// Inspect. Status is one of not_checked, valid, invalid or absent.
type SignatureReport struct {
	Present bool   `json:"present"`
	Status  string `json:"status"`
	Comment string `json:"comment,omitempty"`
	Problem string `json:"problem,omitempty"`
}

// Report is the structured result of Inspect and Verify.
type Report struct {
	File          string          `json:"file"`
	Schema        string          `json:"schema"`
	Version       string          `json:"version"`
	Kind          string          `json:"kind"`
	CreatedAt     string          `json:"created_at"`
	Producer      string          `json:"producer"`
	PayloadSHA256 string          `json:"payload_sha256"`
	Files         []FileReport    `json:"files"`
	Signature     SignatureReport `json:"signature"`
	ContentOK     bool            `json:"content_ok"`
	Valid         bool            `json:"valid"`
	Warnings      []string        `json:"warnings"`
}

func readEntry(f *zip.File, max int64) ([]byte, error) {
	if f.UncompressedSize64 > uint64(max) {
		return nil, fmt.Errorf("%s supera el tamaño máximo", f.Name)
	}
	rc, err := f.Open()
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, fmt.Errorf("%s supera el tamaño máximo", f.Name)
	}
	return data, nil
}

func parseManifest(raw []byte) (*Manifest, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	var m Manifest
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("manifest.json no válido: %w", err)
	}
	if m.Schema != Schema {
		return nil, fmt.Errorf("esquema de contenedor no soportado %q", m.Schema)
	}
	m.Version = strings.TrimSpace(m.Version)
	if m.Version == "" || len(m.Version) > 128 {
		return nil, errors.New("versión del contenedor no válida")
	}
	if !validKind(m.Kind) {
		return nil, fmt.Errorf("tipo de contenedor no válido %q", m.Kind)
	}
	m.Producer = strings.TrimSpace(m.Producer)
	if len(m.Producer) > 256 {
		return nil, errors.New("productor demasiado largo")
	}
	t, err := time.Parse(time.RFC3339, m.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("created_at no es RFC3339 válido: %w", err)
	}
	m.CreatedAt = t.UTC().Truncate(time.Second).Format(time.RFC3339)
	if len(m.Files) == 0 || len(m.Files) > maxFiles {
		return nil, errors.New("número de ficheros del manifiesto no válido")
	}
	seen := make(map[string]bool, len(m.Files))
	for i := range m.Files {
		f := &m.Files[i]
		if !validName(f.Path) {
			return nil, fmt.Errorf("ruta de fichero no válida %q", f.Path)
		}
		if seen[f.Path] {
			return nil, fmt.Errorf("fichero duplicado en el manifiesto %q", f.Path)
		}
		seen[f.Path] = true
		f.SHA256 = strings.ToLower(strings.TrimSpace(f.SHA256))
		rawSum, err := hex.DecodeString(f.SHA256)
		if err != nil || len(rawSum) != sha256.Size {
			return nil, fmt.Errorf("sha256 no válido para %q", f.Path)
		}
		if f.Size < 0 || f.Size > maxFileBytes {
			return nil, fmt.Errorf("tamaño no válido para %q", f.Path)
		}
	}
	return &m, nil
}

// open reads and structurally validates a container. It never checks the
// signature: Inspect deliberately shows contents even when they are untrusted.
func open(path string) (*Report, []byte, string, []byte, error) {
	zr, err := zip.OpenReader(path)
	if err != nil {
		return nil, nil, "", nil, fmt.Errorf("no se pudo abrir el contenedor: %w", err)
	}
	defer zr.Close()
	if len(zr.File) == 0 || len(zr.File) > maxFiles+4 {
		return nil, nil, "", nil, errors.New("número de entradas ZIP no válido")
	}

	entries := make(map[string]*zip.File, len(zr.File))
	for _, f := range zr.File {
		name := f.Name
		if strings.HasPrefix(name, PayloadPrefix) {
			if !validName(strings.TrimPrefix(name, PayloadPrefix)) {
				return nil, nil, "", nil, fmt.Errorf("ruta ZIP no válida %q", name)
			}
		} else if name != ManifestName && name != SignatureName && name != SBOMName {
			return nil, nil, "", nil, fmt.Errorf("entrada ZIP inesperada %q", name)
		}
		if _, exists := entries[name]; exists {
			return nil, nil, "", nil, fmt.Errorf("entrada ZIP duplicada %q", name)
		}
		entries[name] = f
	}

	manifestFile, ok := entries[ManifestName]
	if !ok {
		return nil, nil, "", nil, errors.New("falta manifest.json en el contenedor")
	}
	rawManifest, err := readEntry(manifestFile, maxManifestBytes)
	if err != nil {
		return nil, nil, "", nil, err
	}
	m, err := parseManifest(rawManifest)
	if err != nil {
		return nil, nil, "", nil, err
	}
	canonical, err := CanonicalManifest(m)
	if err != nil {
		return nil, nil, "", nil, err
	}
	if !bytes.Equal(canonical, rawManifest) {
		return nil, nil, "", nil, errors.New("manifest.json no está en forma canónica; pudo ser modificado")
	}

	var sigText string
	var sbom []byte
	if f, ok := entries[SignatureName]; ok {
		sig, err := readEntry(f, maxSignatureSize)
		if err != nil {
			return nil, nil, "", nil, err
		}
		sigText = string(sig)
	}
	if f, ok := entries[SBOMName]; ok {
		sbom, err = readEntry(f, maxManifestBytes)
		if err != nil {
			return nil, nil, "", nil, err
		}
	}

	rep := &Report{
		File:          path,
		Schema:        m.Schema,
		Version:       m.Version,
		Kind:          m.Kind,
		CreatedAt:     m.CreatedAt,
		Producer:      m.Producer,
		PayloadSHA256: PayloadDigest(m),
		Files:         make([]FileReport, 0, len(m.Files)),
		Signature:     SignatureReport{Present: sigText != "", Status: "not_checked"},
		Warnings:      []string{},
	}
	if !rep.Signature.Present {
		rep.Signature.Status = "absent"
		rep.Warnings = append(rep.Warnings, "signature.minisig no está presente; el contenido no tiene procedencia verificable")
	}

	for _, want := range m.Files {
		entry, ok := entries[PayloadPrefix+want.Path]
		if !ok {
			rep.Files = append(rep.Files, FileReport{Path: want.Path, SHA256: want.SHA256, Size: want.Size, Matches: false})
			rep.Warnings = append(rep.Warnings, "falta el fichero "+want.Path+" en payload/")
			continue
		}
		data, err := readEntry(entry, maxFileBytes)
		if err != nil {
			return nil, nil, "", nil, err
		}
		sum := sha256.Sum256(data)
		actual := hex.EncodeToString(sum[:])
		matches := actual == want.SHA256 && int64(len(data)) == want.Size
		rep.Files = append(rep.Files, FileReport{
			Path: want.Path, SHA256: actual, Size: int64(len(data)), Matches: matches,
		})
		if !matches {
			rep.Warnings = append(rep.Warnings, "el fichero "+want.Path+" no coincide con el sha256 o el tamaño del manifiesto")
		}
	}
	rep.ContentOK = allPayloadMatch(rep.Files)
	return rep, canonical, sigText, sbom, nil
}

func allPayloadMatch(files []FileReport) bool {
	for _, f := range files {
		if !f.Matches {
			return false
		}
	}
	return true
}

// Inspect shows the container contents and checks digests, but does not verify
// the signature. It is safe to run on untrusted files.
func Inspect(path string) (*Report, error) {
	rep, _, _, _, err := open(path)
	if err != nil {
		return nil, err
	}
	rep.Warnings = append(rep.Warnings, "firma no verificada; use netdiag container verify --key <pub> antes de confiar en el contenido")
	return rep, nil
}

// Verify checks the minisign signature, the trusted comment and every payload
// digest. It returns a report even on failure so the CLI can explain the reason.
func Verify(path, publicKey string) (*Report, error) {
	rep, canonical, sigText, sbom, err := open(path)
	if err != nil {
		return nil, err
	}
	if !rep.Signature.Present {
		return rep, nil
	}
	comment, err := update.VerifyMinisign(canonical, publicKey, sigText)
	if err != nil {
		rep.Signature.Status = "invalid"
		rep.Signature.Problem = err.Error()
		return rep, nil
	}
	rep.Signature.Comment = comment
	fields := make(map[string]string)
	for _, field := range strings.Fields(comment) {
		if i := strings.Index(field, ":"); i > 0 {
			fields[field[:i]] = field[i+1:]
		}
	}
	if fields["version"] != rep.Version {
		rep.Signature.Status = "invalid"
		rep.Signature.Problem = "el comentario firmado indica otra versión"
		return rep, nil
	}
	if strings.ToLower(fields["sha256"]) != rep.PayloadSHA256 {
		rep.Signature.Status = "invalid"
		rep.Signature.Problem = "el comentario firmado indica otro sha256 del contenido"
		return rep, nil
	}
	if len(sbom) > 0 {
		sum := sha256.Sum256(sbom)
		if strings.ToLower(fields["sbom"]) != hex.EncodeToString(sum[:]) {
			rep.Signature.Status = "invalid"
			rep.Signature.Problem = "el comentario firmado indica otro sha256 del SBOM"
			return rep, nil
		}
	}
	rep.Signature.Status = "valid"
	rep.Valid = rep.ContentOK
	if !rep.ContentOK {
		rep.Warnings = append(rep.Warnings, "la firma es válida, pero el contenido del contenedor no coincide con el manifiesto")
	}
	return rep, nil
}
