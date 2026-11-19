// Package security prepares local CRA runbook drafts from already signed
// release evidence. It makes no network calls, sends no mail and creates no
// legal obligation: every artefact is marked as a dry run.
package security

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/kr3s4l4/netdiag/internal/cryptoutil"
	"github.com/kr3s4l4/netdiag/internal/update"
)

// DryRunT0 is the deterministic timestamp used when the operator does not
// supply one. A fixed value keeps repeated drafts byte-identical.
const DryRunT0 = "2026-01-01T00:00:00Z"

var (
	signedFiles = []string{"manifest.json", "sbom.cdx.json", "provenance.intoto.jsonl"}
	incidentRE  = regexp.MustCompile(`^[A-Za-z0-9._-]{1,64}$`)
)

// RunbookOptions configures one local dry run.
type RunbookOptions struct {
	Dir      string // directory holding the signed release evidence
	KeyPub   string // release public key (path, base64 or .pub file)
	Out      string // output directory, created with 0700
	Incident string // synthetic incident ID
	T0       string // RFC3339 instant; empty uses DryRunT0
	Version  string // product version; empty reads it from manifest.json
}

// RunbookResult describes the generated drafts.
type RunbookResult struct {
	Incident         string   `json:"incident"`
	T0               string   `json:"t0"`
	Deadline24h      string   `json:"deadline_24h"`
	Deadline72h      string   `json:"deadline_72h"`
	Version          string   `json:"version"`
	ManifestSHA256   string   `json:"manifest_sha256"`
	SBOMSHA256       string   `json:"sbom_sha256"`
	ProvenanceSHA256 string   `json:"provenance_sha256"`
	SBOMComponents   int      `json:"sbom_components"`
	Files            []string `json:"files,omitempty"`
}

// BuildDryRun verifies the detached signatures of the release evidence and
// writes deterministic 24 h / 72 h drafts. Nothing leaves this machine.
func BuildDryRun(opts RunbookOptions) (*RunbookResult, error) {
	dir := strings.TrimSpace(opts.Dir)
	out := strings.TrimSpace(opts.Out)
	if dir == "" || out == "" {
		return nil, errors.New("indica --dir con la evidencia firmada y --out de salida")
	}
	keyPub := strings.TrimSpace(opts.KeyPub)
	if keyPub == "" {
		keyPub = filepath.Join(dir, "release.pub")
	}
	publicKey := cryptoutil.PublicKeyB64(keyPub)
	if publicKey == "" {
		return nil, errors.New("no se pudo leer la clave pública de release")
	}

	incident := strings.TrimSpace(opts.Incident)
	if incident == "" {
		incident = "INC-DRYRUN-2026-001"
	}
	if !incidentRE.MatchString(incident) {
		return nil, errors.New("el ID de incidente solo admite letras, números, punto, guion y guion bajo")
	}
	t0 := strings.TrimSpace(opts.T0)
	if t0 == "" {
		t0 = DryRunT0
	}
	start, err := time.Parse(time.RFC3339, t0)
	if err != nil {
		return nil, fmt.Errorf("T0 no es RFC3339: %w", err)
	}
	start = start.UTC()
	t0 = start.Format(time.RFC3339)

	contents := make(map[string][]byte, len(signedFiles))
	hashes := make(map[string]string, len(signedFiles))
	for _, name := range signedFiles {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			return nil, fmt.Errorf("no se pudo leer %s: %w", name, err)
		}
		sig, err := os.ReadFile(filepath.Join(dir, name+".minisig"))
		if err != nil {
			return nil, fmt.Errorf("no se pudo leer la firma de %s: %w", name, err)
		}
		if _, err := update.VerifyMinisign(data, publicKey, string(sig)); err != nil {
			return nil, fmt.Errorf("firma no válida de %s: %w", name, err)
		}
		sum := sha256.Sum256(data)
		contents[name] = data
		hashes[name] = hex.EncodeToString(sum[:])
	}

	version := strings.TrimSpace(opts.Version)
	if version == "" {
		var manifest struct {
			Version string `json:"version"`
		}
		if err := json.Unmarshal(contents["manifest.json"], &manifest); err != nil {
			return nil, fmt.Errorf("manifest.json no válido: %w", err)
		}
		version = strings.TrimSpace(manifest.Version)
	}
	if version == "" {
		return nil, errors.New("el manifiesto firmado no indica versión; usa --version")
	}

	components, err := countSBOMComponents(contents["sbom.cdx.json"])
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(out, 0o700); err != nil {
		return nil, err
	}
	deadline24 := start.Add(24 * time.Hour).Format(time.RFC3339)
	deadline72 := start.Add(72 * time.Hour).Format(time.RFC3339)
	result := &RunbookResult{
		Incident:         incident,
		T0:               t0,
		Deadline24h:      deadline24,
		Deadline72h:      deadline72,
		Version:          version,
		ManifestSHA256:   hashes["manifest.json"],
		SBOMSHA256:       hashes["sbom.cdx.json"],
		ProvenanceSHA256: hashes["provenance.intoto.jsonl"],
		SBOMComponents:   components,
	}

	drafts := []struct {
		name     string
		template string
	}{
		{"aviso-24h-dryrun.md", draft24},
		{"aviso-72h-dryrun.md", draft72},
	}
	for _, d := range drafts {
		body := fmt.Sprintf(d.template, incident, t0, deadline24, deadline72,
			version, hashes["sbom.cdx.json"], components,
			hashes["provenance.intoto.jsonl"], hashes["manifest.json"])
		path := filepath.Join(out, d.name)
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			return nil, err
		}
		result.Files = append(result.Files, path)
	}

	// The summary must not depend on the chosen output directory.
	summaryData := *result
	summaryData.Files = nil
	summary, err := json.MarshalIndent(summaryData, "", "  ")
	if err != nil {
		return nil, err
	}
	summaryPath := filepath.Join(out, "runbook-dryrun.json")
	if err := os.WriteFile(summaryPath, append(summary, '\n'), 0o600); err != nil {
		return nil, err
	}
	result.Files = append(result.Files, summaryPath)

	markerPath := filepath.Join(out, "NO-ENVIADO.txt")
	marker := "BORRADOR DE ENSAYO. NO enviar a ENISA, CSIRT, clientes ni ningún tercero.\n" +
		"F41 real sigue bloqueado: faltan credenciales SRP y release correctiva.\n"
	if err := os.WriteFile(markerPath, []byte(marker), 0o600); err != nil {
		return nil, err
	}
	result.Files = append(result.Files, markerPath)
	return result, nil
}

func countSBOMComponents(data []byte) (int, error) {
	var doc struct {
		Components []struct {
			Name    string `json:"name"`
			Version string `json:"version"`
		} `json:"components"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return 0, fmt.Errorf("sbom.cdx.json no válido: %w", err)
	}
	seen := make(map[string]struct{}, len(doc.Components))
	for _, c := range doc.Components {
		key := c.Name + "@" + c.Version
		seen[strings.ToLower(key)] = struct{}{}
	}
	keys := make([]string, 0, len(seen))
	for k := range seen {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return len(keys), nil
}
