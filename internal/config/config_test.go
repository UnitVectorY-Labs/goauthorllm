package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/UnitVectorY-Labs/goauthorllm/internal/prompts"
)

func TestLoadReadsLocalMessageOverrides(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".goauthorllm")
	if err := os.WriteFile(configPath, []byte("generate_prompt:\n  append: |\n    Prefer concise paragraphs.\nedit_prompt:\n  replace: |\n    You are a direct copy editor.\ncontinue_prompt:\n  append: |\n    Keep transitions tight.\n"), 0o644); err != nil {
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

func TestLoadReadsBaseURLAndModelFromLocalConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".goauthorllm")
	if err := os.WriteFile(configPath, []byte("base_url: http://file.example.com/v1\nmodel: file-model\ngenerate_prompt:\n  append: |\n    Prefer concise paragraphs.\n"), 0o644); err != nil {
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

	t.Setenv("GOAUTHORLLM_BASE_URL", "")
	t.Setenv("GOAUTHORLLM_MODEL", "")
	t.Setenv("OPENAI_BASE_URL", "")
	t.Setenv("OPENAI_MODEL", "")

	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if cfg.BaseURL != "http://file.example.com/v1" {
		t.Fatalf("unexpected base URL: %q", cfg.BaseURL)
	}
	if cfg.Model != "file-model" {
		t.Fatalf("unexpected model: %q", cfg.Model)
	}
	if got := cfg.MessageOverrides[prompts.GeneratePrompt].Append; got != "Prefer concise paragraphs." {
		t.Fatalf("unexpected generate append: %q", got)
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

func TestLoadUsesDefaults(t *testing.T) {
	dir := t.TempDir()
	oldwd, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	// Clear any env vars that would override defaults
	t.Setenv("GOAUTHORLLM_BASE_URL", "")
	t.Setenv("GOAUTHORLLM_MODEL", "")
	t.Setenv("OPENAI_BASE_URL", "")
	t.Setenv("OPENAI_MODEL", "")

	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if cfg.BaseURL != DefaultBaseURL {
		t.Fatalf("unexpected base URL: %q", cfg.BaseURL)
	}
	if cfg.Model != DefaultModel {
		t.Fatalf("unexpected model: %q", cfg.Model)
	}
}

func TestLoadEnvOverridesLocalConfig(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".goauthorllm")
	if err := os.WriteFile(configPath, []byte("base_url: http://file.example.com/v1\nmodel: file-model\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	oldwd, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	t.Setenv("GOAUTHORLLM_BASE_URL", "http://env.example.com/v1")
	t.Setenv("GOAUTHORLLM_MODEL", "env-model")

	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if cfg.BaseURL != "http://env.example.com/v1" {
		t.Fatalf("expected env base URL, got %q", cfg.BaseURL)
	}
	if cfg.Model != "env-model" {
		t.Fatalf("expected env model, got %q", cfg.Model)
	}
}

func TestLoadFlagsOverrideEnv(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".goauthorllm")
	if err := os.WriteFile(configPath, []byte("base_url: http://file.example.com/v1\nmodel: file-model\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	oldwd, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	t.Setenv("GOAUTHORLLM_BASE_URL", "http://env.example.com/v1")
	t.Setenv("GOAUTHORLLM_MODEL", "env-model")

	cfg, err := Load([]string{"--base-url", "http://flag.example.com/v1", "--model", "flag-model"})
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if cfg.BaseURL != "http://flag.example.com/v1" {
		t.Fatalf("expected flag base URL, got %q", cfg.BaseURL)
	}
	if cfg.Model != "flag-model" {
		t.Fatalf("expected flag model, got %q", cfg.Model)
	}
}

func TestLoadPositionalArgument(t *testing.T) {
	dir := t.TempDir()
	oldwd, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	cfg, err := Load([]string{"draft.md"})
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if cfg.FilePath != "draft.md" {
		t.Fatalf("unexpected file path: %q", cfg.FilePath)
	}
}

func TestLoadRejectsMultiplePositionalArgs(t *testing.T) {
	dir := t.TempDir()
	oldwd, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	_, err := Load([]string{"a.md", "b.md"})
	if err == nil {
		t.Fatal("expected error for multiple positional arguments")
	}
}
