package evidence

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/kr3s4l4/netdiag/internal/buildinfo"
	"github.com/kr3s4l4/netdiag/internal/container"
	"github.com/kr3s4l4/netdiag/internal/cryptoutil"
	"github.com/kr3s4l4/netdiag/internal/store"
	"github.com/kr3s4l4/netdiag/internal/update"
)

// UsageSchema identifies the anonymous, voluntary usage report.
const UsageSchema = "netdiag.usage/v1"

// usageBands buckets the number of managed hosts per site. Only the bucket is
// exported, never the exact count or any site identifier.
var usageBands = []struct {
	name string
	max  int
}{
	{name: "1-5", max: 5},
	{name: "6-10", max: 10},
	{name: "11-50", max: 50},
	{name: "51-200", max: 200},
	{name: "201+", max: 0},
}

// UsageReport contains only aggregate counters derived from the local store.
// It has no IPs, MACs, hostnames, site names or credentials, and it is never
// sent automatically.
type UsageReport struct {
	Schema           string         `json:"schema"`
	GeneratedAt      time.Time      `json:"generated_at"`
	Version          string         `json:"version"`
	InstalledDays    int            `json:"installed_days"`
	Scans            int            `json:"scans"`
	Sites            int            `json:"sites"`
	ManagedHostBands map[string]int `json:"managed_host_bands"`
}

// UsageOptions configures one voluntary evidence export.
type UsageOptions struct {
	SeedFile string
	KeyPub   string
	Out      string
	DataDir  string
	Version  string
	Now      time.Time
}

// BuildUsage writes and verifies a signed .ndc holding a UsageReport.
func BuildUsage(st *store.Store, opt UsageOptions) (*Entry, error) {
	if st == nil {
		return nil, errors.New("evidencia de uso sin almacén")
	}
	seedFile := strings.TrimSpace(opt.SeedFile)
	if seedFile == "" {
		return nil, errors.New("indica la clave de firma con --seed-file")
	}
	seed, keyID, err := cryptoutil.LoadSigningMaterial(seedFile, "")
	if err != nil {
		return nil, err
	}
	publicKey := cryptoutil.PublicKeyB64(opt.KeyPub)
	if publicKey == "" {
		publicKey, err = update.PublicKeyFromSeed(seed, keyID)
		if err != nil {
			return nil, err
		}
	}

	now := opt.Now.UTC()
	if now.IsZero() {
		now = time.Now().UTC()
	}
	version := strings.TrimSpace(opt.Version)
	if version == "" {
		version = buildinfo.Version
	}
	report, err := collectUsage(st, now, version)
	if err != nil {
		return nil, err
	}
	payload, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, err
	}

	out := strings.TrimSpace(opt.Out)
	if out == "" {
		dir := strings.TrimSpace(opt.DataDir)
		if dir == "" {
			return nil, errors.New("indica la salida con --out o un directorio de datos")
		}
		out = filepath.Join(dir, "netdiag-usage-"+now.Format("20060102-150405")+".ndc")
		for i := 1; ; i++ {
			if _, err := os.Stat(out); errors.Is(err, os.ErrNotExist) {
				break
			}
			out = filepath.Join(dir, fmt.Sprintf("netdiag-usage-%s-%d.ndc", now.Format("20060102-150405"), i))
		}
	}
	if dir := filepath.Dir(out); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			return nil, err
		}
	}

	if err := container.Build(out, container.BuildOptions{
		Version:  version,
		Kind:     "evidence",
		Producer: "netdiag-usage",
		Files:    []container.InputFile{{Name: "usage.json", Data: payload}},
		Seed:     seed,
		KeyID:    keyID,
	}); err != nil {
		return nil, fmt.Errorf("no se pudo construir el evidencia de uso: %w", err)
	}
	verified, err := container.Verify(out, publicKey)
	if err != nil {
		return nil, err
	}
	if !verified.Valid {
		problem := verified.Signature.Problem
		if problem == "" {
			problem = "contenido no íntegro"
		}
		return nil, fmt.Errorf("evidencia de uso no válida: %s", problem)
	}
	info, err := os.Stat(out)
	if err != nil {
		return nil, err
	}
	return &Entry{Path: out, Name: filepath.Base(out), Size: info.Size(), CreatedAt: now, Valid: true}, nil
}

func collectUsage(st *store.Store, now time.Time, version string) (*UsageReport, error) {
	installedDays := 0
	users, err := st.ListUsers()
	if err != nil {
		return nil, err
	}
	installed := now
	for _, u := range users {
		if u.CreatedAt.Before(installed) {
			installed = u.CreatedAt
		}
	}
	if installed.Before(now) {
		installedDays = int(now.Sub(installed).Hours() / 24)
	}

	scans, err := st.CountScansSince(time.Unix(0, 0))
	if err != nil {
		return nil, err
	}
	sites, err := st.ListSites()
	if err != nil {
		return nil, err
	}
	counts, err := st.CountManagedHostsBySiteSince(now.AddDate(0, 0, -30))
	if err != nil {
		return nil, err
	}
	bands := make(map[string]int, len(usageBands))
	for _, band := range usageBands {
		bands[band.name] = 0
	}
	for _, n := range counts {
		bands[bandForCount(n)]++
	}

	return &UsageReport{
		Schema:           UsageSchema,
		GeneratedAt:      now,
		Version:          version,
		InstalledDays:    installedDays,
		Scans:            scans,
		Sites:            len(sites),
		ManagedHostBands: bands,
	}, nil
}

func bandForCount(n int) string {
	for _, band := range usageBands {
		if band.max > 0 && n <= band.max {
			return band.name
		}
	}
	return "201+"
}
