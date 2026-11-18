package store

import (
	"strings"
	"testing"

	"github.com/kr3s4l4/netdiag/internal/model"
)

func TestIntegrationsEncryptSecrets(t *testing.T) {
	st := newTestStore(t)
	it, err := st.CreateIntegration(&model.Integration{
		Name: "Elastic local", Type: "elasticsearch", Enabled: true,
		Config: map[string]any{"url": "http://127.0.0.1:9200", "api_key": "super-secret-key"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if it.Config["api_key"] != "super-secret-key" {
		t.Fatalf("la configuración devuelta debe estar descifrada: %+v", it.Config)
	}

	var enc []byte
	var hasSecret int
	if err := st.db.QueryRow(`SELECT config_enc, has_secret FROM integrations WHERE id = ?`, it.ID).Scan(&enc, &hasSecret); err != nil {
		t.Fatal(err)
	}
	if hasSecret == 0 {
		t.Fatal("has_secret debe estar activo")
	}
	if strings.Contains(string(enc), "super-secret-key") {
		t.Fatal("el secreto no debe aparecer en texto plano en la base de datos")
	}

	it.Config["api_key"] = "rotated-key"
	if err := st.UpdateIntegration(it); err != nil {
		t.Fatal(err)
	}
	got, err := st.GetIntegration(it.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Config["api_key"] != "rotated-key" {
		t.Fatalf("la rotación no se persistió: %+v", got.Config)
	}
	if err := st.DeleteIntegration(it.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetIntegration(it.ID); err == nil {
		t.Fatal("la integración eliminada no debe existir")
	}
}

func TestIntegrationTypesAndSecrets(t *testing.T) {
	if !ValidateIntegrationType("elasticsearch") || ValidateIntegrationType("carrier-pigeon") {
		t.Fatal("validación de tipos incorrecta")
	}
	if len(IntegrationTypes) != 10 {
		t.Fatalf("esperaba 10 tipos, hay %d", len(IntegrationTypes))
	}
	if !IntegrationHasSecret("telegram", map[string]any{"token": "abc"}) {
		t.Fatal("telegram con token debe tener secreto")
	}
	if IntegrationHasSecret("telegram", map[string]any{"token": ""}) {
		t.Fatal("un token vacío no cuenta como secreto")
	}
	if IntegrationHasSecret("syslog", map[string]any{"host": "127.0.0.1"}) {
		t.Fatal("syslog no tiene secretos")
	}
}
