package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/kr3s4l4/netdiag/internal/model"
)

const integrationColumns = `id, name, type, enabled, config_enc, has_secret, created_at, updated_at, last_test_at, last_error`

func (s *Store) scanIntegration(r rowScanner) (*model.Integration, error) {
	var (
		it        model.Integration
		enabled   int
		hasSecret int
		enc       []byte
		created   int64
		updated   int64
		lastTest  sql.NullInt64
	)
	if err := r.Scan(&it.ID, &it.Name, &it.Type, &enabled, &enc, &hasSecret, &created, &updated, &lastTest, &it.LastError); err != nil {
		return nil, err
	}
	it.Enabled = enabled != 0
	it.HasSecret = hasSecret != 0
	it.CreatedAt = fromMS(created)
	it.UpdatedAt = fromMS(updated)
	it.LastTestAt = fromMSNull(lastTest)
	it.Config = map[string]any{}
	if s.box != nil && len(enc) > 0 {
		plain, err := s.box.Decrypt(enc)
		if err != nil {
			return nil, fmt.Errorf("integración %d: no se pudo descifrar la configuración: %w", it.ID, err)
		}
		_ = json.Unmarshal(plain, &it.Config)
	}
	return &it, nil
}

// ListIntegrations returns all integrations with decrypted configuration.
// Callers exposing this data must mask secret fields first.
func (s *Store) ListIntegrations() ([]model.Integration, error) {
	rows, err := s.db.Query(`SELECT ` + integrationColumns + ` FROM integrations ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Integration{}
	for rows.Next() {
		it, err := s.scanIntegration(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *it)
	}
	return out, rows.Err()
}

// GetIntegration returns one integration by id.
func (s *Store) GetIntegration(id int64) (*model.Integration, error) {
	it, err := s.scanIntegration(s.db.QueryRow(`SELECT `+integrationColumns+` FROM integrations WHERE id = ?`, id))
	if err == sql.ErrNoRows {
		return nil, ErrNotFound
	}
	return it, err
}

// IntegrationHasSecret reports whether any secret field has a real value.
func IntegrationHasSecret(typ string, cfg map[string]any) bool {
	for _, f := range IntegrationSecretFields(typ) {
		if v, ok := cfg[f]; ok {
			if str, _ := v.(string); str != "" {
				return true
			}
		}
	}
	return false
}

func (s *Store) encryptIntegrationConfig(cfg map[string]any) ([]byte, error) {
	if s.box == nil {
		return nil, fmt.Errorf("no hay clave de cifrado configurada")
	}
	plain, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}
	return s.box.Encrypt(plain)
}

// CreateIntegration encrypts and stores an integration.
func (s *Store) CreateIntegration(it *model.Integration) (*model.Integration, error) {
	if it.Config == nil {
		it.Config = map[string]any{}
	}
	enc, err := s.encryptIntegrationConfig(it.Config)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	res, err := s.db.Exec(`INSERT INTO integrations(name,type,enabled,config_enc,has_secret,created_at,updated_at,last_error)
		VALUES(?,?,?,?,?,?,?,'')`, it.Name, it.Type, boolInt(it.Enabled), enc,
		boolInt(IntegrationHasSecret(it.Type, it.Config)), now.UnixMilli(), now.UnixMilli())
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	return s.GetIntegration(id)
}

// UpdateIntegration replaces an integration definition.
func (s *Store) UpdateIntegration(it *model.Integration) error {
	if it.Config == nil {
		it.Config = map[string]any{}
	}
	enc, err := s.encryptIntegrationConfig(it.Config)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`UPDATE integrations SET name=?, type=?, enabled=?, config_enc=?, has_secret=?, updated_at=? WHERE id=?`,
		it.Name, it.Type, boolInt(it.Enabled), enc,
		boolInt(IntegrationHasSecret(it.Type, it.Config)), nowMS(), it.ID)
	return err
}

// DeleteIntegration removes an integration definition.
func (s *Store) DeleteIntegration(id int64) error {
	_, err := s.db.Exec(`DELETE FROM integrations WHERE id = ?`, id)
	return err
}

// MarkIntegrationTest records the outcome of a test delivery.
func (s *Store) MarkIntegrationTest(id int64, errMsg string) error {
	_, err := s.db.Exec(`UPDATE integrations SET last_test_at=?, last_error=? WHERE id=?`, nowMS(), errMsg, id)
	return err
}
