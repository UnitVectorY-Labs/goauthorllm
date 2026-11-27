package config

import (
	"path/filepath"
	"testing"
)

func TestSaveAndLoadLocalRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, LocalConfigFilename)

	speed := 75
	structured := true
	copyBatch := 4
	dirBatch := 8
	orig := LocalFile{
		BaseURL:               "http://localhost:8080/v1",
		Model:                 "local-model",
		GenerationModel:       "gen-model",
		EditingModel:          "edit-model",
		StreamSpeed:           &speed,
		DefaultScope:          "section",
		StructuredGeneration:  &structured,
		Rules:                 "Keep tone crisp and technical.",
		CopyEditRules:         "Use Oxford comma and active voice.",
		CopyEditBatchSize:     &copyBatch,
		DirectedEditBatchSize: &dirBatch,
	}

	if err := SaveLocal(path, orig); err != nil {
		t.Fatalf("save local: %v", err)
	}

	loaded, err := LoadLocal(path)
	if err != nil {
		t.Fatalf("load local: %v", err)
	}

	if loaded.BaseURL != orig.BaseURL || loaded.Model != orig.Model {
		t.Fatalf("endpoint mismatch: %#v", loaded)
	}
	if loaded.StreamSpeed == nil || *loaded.StreamSpeed != 75 {
		t.Fatalf("stream speed mismatch: %#v", loaded.StreamSpeed)
	}
	if loaded.DefaultScope != "section" {
		t.Fatalf("default scope mismatch: %q", loaded.DefaultScope)
	}
	if loaded.Rules != orig.Rules || loaded.CopyEditRules != orig.CopyEditRules {
		t.Fatalf("rules mismatch: rules=%q copy=%q", loaded.Rules, loaded.CopyEditRules)
	}

	var cfg Config
	if err := cfg.ApplyLocal(loaded); err != nil {
		t.Fatalf("apply local: %v", err)
	}
	if cfg.StreamSpeed != 75 || cfg.DefaultScope != "section" || cfg.Rules != orig.Rules || cfg.CopyEditRules != orig.CopyEditRules {
		t.Fatalf("unexpected applied config: %#v", cfg)
	}
}
