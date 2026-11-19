package container

import (
	"archive/zip"
	"bytes"
	"crypto/rand"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/kr3s4l4/netdiag/internal/update"
)

func signingMaterial(t *testing.T) (seed, keyID, publicKey []byte) {
	t.Helper()
	seed = make([]byte, 32)
	if _, err := rand.Read(seed); err != nil {
		t.Fatal(err)
	}
	keyID = []byte("NETDIAG1")
	pub, err := update.PublicKeyFromSeed(seed, keyID)
	if err != nil {
		t.Fatal(err)
	}
	return seed, keyID, []byte(pub)
}

func buildSample(t *testing.T, dir string) (string, string) {
	t.Helper()
	seed, keyID, public := signingMaterial(t)
	path := filepath.Join(dir, "prueba.ndc")
	err := Build(path, BuildOptions{
		Version:   "1.5.0",
		Kind:      "evidence",
		Producer:  "netdiag-tests",
		CreatedAt: time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC),
		Files: []InputFile{
			{Name: "reporte.pdf", Data: []byte("informe ejecutivo")},
			{Name: "diff.json", Data: []byte("{\"changes\":0}")},
		},
		SBOM:  []byte("{\"bomFormat\":\"CycloneDX\"}"),
		Seed:  seed,
		KeyID: keyID,
	})
	if err != nil {
		t.Fatal(err)
	}
	return path, string(public)
}

func TestBuildSiteKindRoundTrip(t *testing.T) {
	dir := t.TempDir()
	seed, keyID, public := signingMaterial(t)
	path := filepath.Join(dir, "sede.ndc")
	if err := Build(path, BuildOptions{
		Version:   "1.5.2",
		Kind:      "site",
		Producer:  "netdiag-tests",
		CreatedAt: time.Date(2026, 11, 19, 12, 0, 0, 0, time.UTC),
		Files: []InputFile{
			{Name: "site-bundle.json", Data: []byte(`{"site_id":1,"hosts":3}`)},
		},
		Seed:  seed,
		KeyID: keyID,
	}); err != nil {
		t.Fatal(err)
	}
	rep, err := Verify(path, string(public))
	if err != nil {
		t.Fatal(err)
	}
	if !rep.Valid || !rep.ContentOK || rep.Kind != "site" || rep.Version != "1.5.2" {
		t.Fatalf("contenedor site no válido: %+v", rep)
	}
	if len(rep.Files) != 1 || rep.Files[0].Path != "site-bundle.json" {
		t.Fatalf("payload inesperado: %+v", rep.Files)
	}
}

func rewriteEntry(t *testing.T, path, entryName string, replacement []byte) {
	t.Helper()
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range zr.File {
		data, err := readEntry(f, maxFileBytes)
		if err != nil {
			t.Fatal(err)
		}
		if f.Name == entryName {
			data = replacement
		}
		h := &zip.FileHeader{Name: f.Name, Method: zip.Deflate}
		h.SetModTime(fixedZipTime)
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestBuildInspectVerifyRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path, public := buildSample(t, dir)

	inspected, err := Inspect(path)
	if err != nil {
		t.Fatal(err)
	}
	if inspected.Signature.Status != "not_checked" || inspected.Valid {
		t.Fatalf("Inspect no debe afirmar que la firma es válida: %+v", inspected.Signature)
	}
	if len(inspected.Files) != 2 || !inspected.ContentOK {
		t.Fatalf("contenido inesperado: %+v", inspected.Files)
	}

	rep, err := Verify(path, public)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Signature.Status != "valid" || !rep.Valid || !rep.ContentOK {
		t.Fatalf("verificación falló: %+v", rep)
	}
	if rep.Version != "1.5.0" || rep.Kind != "evidence" || rep.Producer != "netdiag-tests" {
		t.Fatalf("manifiesto inesperado: %+v", rep)
	}
}

func TestContainerIsPlainZip(t *testing.T) {
	dir := t.TempDir()
	path, _ := buildSample(t, dir)
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatalf("un zip normal debería poder abrir el .ndc: %v", err)
	}
	defer zr.Close()
	want := map[string]bool{
		ManifestName: true, SignatureName: true, SBOMName: true,
		PayloadPrefix + "reporte.pdf": true, PayloadPrefix + "diff.json": true,
	}
	got := map[string]bool{}
	for _, f := range zr.File {
		got[f.Name] = true
	}
	for name := range want {
		if !got[name] {
			t.Errorf("falta la entrada %q para unzip; hay %v", name, got)
		}
	}
}

func TestVerifyDetectsPayloadTampering(t *testing.T) {
	dir := t.TempDir()
	path, public := buildSample(t, dir)
	rewriteEntry(t, path, PayloadPrefix+"reporte.pdf", []byte("informe manipulado"))

	rep, err := Verify(path, public)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Valid || rep.ContentOK || rep.Signature.Status != "valid" {
		t.Fatalf("se esperaba firma válida pero contenido modificado: %+v", rep)
	}
	joined := strings.Join(rep.Warnings, " ")
	if !strings.Contains(joined, "reporte.pdf") {
		t.Fatalf("el aviso no identifica el fichero: %v", rep.Warnings)
	}
}

func TestVerifyDetectsSignatureTampering(t *testing.T) {
	dir := t.TempDir()
	path, public := buildSample(t, dir)
	rewriteEntry(t, path, SignatureName, []byte("firma falsa\n"))

	rep, err := Verify(path, public)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Valid || rep.Signature.Status != "invalid" || rep.Signature.Problem == "" {
		t.Fatalf("se esperaba firma inválida con motivo: %+v", rep.Signature)
	}
}

func TestVerifyDetectsManifestTampering(t *testing.T) {
	dir := t.TempDir()
	path, public := buildSample(t, dir)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// Sustituye la versión dentro del ZIP manteniendo el JSON compacto y canónico.
	zr, _ := zip.NewReader(bytes.NewReader(raw), int64(len(raw)))
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range zr.File {
		data, _ := readEntry(f, maxFileBytes)
		if f.Name == ManifestName {
			data = bytes.Replace(data, []byte(`"version":"1.5.0"`), []byte(`"version":"9.9.9"`), 1)
		}
		h := &zip.FileHeader{Name: f.Name, Method: zip.Deflate}
		h.SetModTime(fixedZipTime)
		w, _ := zw.CreateHeader(h)
		_, _ = w.Write(data)
	}
	_ = zw.Close()
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}

	rep, err := Verify(path, public)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Valid || rep.Signature.Status != "invalid" {
		t.Fatalf("se esperaba rechazo del manifiesto modificado: %+v", rep.Signature)
	}
}

func TestVerifyDetectsSBOMTampering(t *testing.T) {
	dir := t.TempDir()
	path, public := buildSample(t, dir)
	rewriteEntry(t, path, SBOMName, []byte("{\"bomFormat\":\"evil\"}"))

	rep, err := Verify(path, public)
	if err != nil {
		t.Fatal(err)
	}
	if rep.Valid || rep.Signature.Status != "invalid" || !strings.Contains(rep.Signature.Problem, "SBOM") {
		t.Fatalf("se esperaba rechazo del SBOM modificado: %+v", rep.Signature)
	}
}

func TestVerifyRejectsWrongKey(t *testing.T) {
	dir := t.TempDir()
	path, _ := buildSample(t, dir)
	_, _, otherPublic := signingMaterial(t)

	rep, err := Verify(path, string(otherPublic))
	if err != nil {
		t.Fatal(err)
	}
	if rep.Valid || rep.Signature.Status != "invalid" {
		t.Fatalf("se aceptó una clave ajena: %+v", rep.Signature)
	}
}

func TestBuildRejectsUnsafeNames(t *testing.T) {
	dir := t.TempDir()
	seed, keyID, _ := signingMaterial(t)
	for _, name := range []string{"../salir", "/absoluto", "payload/doble", "a\\b", ""} {
		path := filepath.Join(dir, "malo.ndc")
		err := Build(path, BuildOptions{
			Version: "1.0.0", Kind: "site",
			Files: []InputFile{{Name: name, Data: []byte("x")}},
			Seed:  seed, KeyID: keyID,
		})
		if err == nil {
			t.Errorf("se aceptó el nombre no seguro %q", name)
		}
	}
}

func TestVerifyReportsMissingSignature(t *testing.T) {
	dir := t.TempDir()
	path, _ := buildSample(t, dir)
	// Elimina la entrada de firma reescribiendo el ZIP.
	zr, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range zr.File {
		if f.Name == SignatureName {
			continue
		}
		data, err := readEntry(f, maxFileBytes)
		if err != nil {
			t.Fatal(err)
		}
		h := &zip.FileHeader{Name: f.Name, Method: zip.Deflate}
		h.SetModTime(fixedZipTime)
		w, _ := zw.CreateHeader(h)
		_, _ = w.Write(data)
	}
	_ = zw.Close()
	_ = zr.Close()
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}

	rep, err := Verify(path, "clave-que-no-importa")
	if err != nil {
		t.Fatal(err)
	}
	if rep.Valid || rep.Signature.Status != "absent" {
		t.Fatalf("se esperaba firma ausente: %+v", rep.Signature)
	}
}
