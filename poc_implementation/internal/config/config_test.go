package config

import (
	"os"
	"path/filepath"
	"testing"

	"goauthorllm/internal/prompts"
)

func TestLoadReadsLocalMessageOverrides(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".goauthorllm")
	if err := os.WriteFile(configPath, []byte(`generate_prompt:
  append: |
    Prefer concise paragraphs.
edit_prompt:
  replace: |
    You are a direct copy editor.
continue_prompt:
  append: |
    Keep transitions tight.
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldwd)
	})

	t.Setenv("GOAUTHORLLM_BASE_URL", "http://example.com/v1")
	t.Setenv("GOAUTHORLLM_MODEL", "test-model")
	t.Setenv("GOAUTHORLLM_TIMEOUT", "1s")

	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if got := cfg.MessageOverrides[prompts.GeneratePrompt].Append; got != "Prefer concise paragraphs." {
		t.Fatalf("unexpected generate append: %q", got)
	}
	if got := cfg.MessageOverrides[prompts.EditPrompt].Replace; got != "You are a direct copy editor." {
		t.Fatalf("unexpected edit replace: %q", got)
	}
	if got := cfg.MessageOverrides[prompts.ContinuePrompt].Append; got != "Keep transitions tight." {
		t.Fatalf("unexpected continue append: %q", got)
	}
}

func TestLoadIgnoresMissingLocalConfig(t *testing.T) {
	dir := t.TempDir()

	oldwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(oldwd)
	})

	t.Setenv("GOAUTHORLLM_BASE_URL", "http://example.com/v1")
	t.Setenv("GOAUTHORLLM_MODEL", "test-model")
	t.Setenv("GOAUTHORLLM_TIMEOUT", "1s")

	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if len(cfg.MessageOverrides) != 0 {
		t.Fatalf("expected empty overrides, got %#v", cfg.MessageOverrides)
	}
}
