package demo

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Active reports whether dataDir contains a valid synthetic-data marker.
// It is used by the UI, the API and the report exporter to label demo data.
func Active(dataDir string) bool {
	if dataDir == "" {
		return false
	}
	raw, err := os.ReadFile(filepath.Join(dataDir, MarkerName))
	if err != nil {
		return false
	}
	var m struct {
		Demo bool `json:"demo"`
	}
	return json.Unmarshal(raw, &m) == nil && m.Demo
}
