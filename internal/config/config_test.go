package config

import (
	"os"
	"path/filepath"
	"strings"
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

func TestLoadUsesSpecificGenerationAndEditingModels(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, ".goauthorllm")
	if err := os.WriteFile(configPath, []byte("base_url: http://file.example.com/v1\nmodel: default-model\ngeneration_model: generation-model\nediting_model: editing-model\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	oldwd, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.GenerationModel != "generation-model" || cfg.EditingModel != "editing-model" {
		t.Fatalf("unexpected specialized models: generation=%q editing=%q", cfg.GenerationModel, cfg.EditingModel)
	}
}

func TestLoadReadsEditBatchSizes(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".goauthorllm"), []byte("copy_edit_batch_size: 3\ndirected_edit_batch_size: 25\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	oldwd, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.CopyEditBatchSize != 3 || cfg.DirectedEditBatchSize != 25 {
		t.Fatalf("unexpected batch sizes: copy=%d directed=%d", cfg.CopyEditBatchSize, cfg.DirectedEditBatchSize)
	}
}

func TestLoadRejectsNonPositiveEditBatchSize(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".goauthorllm"), []byte("copy_edit_batch_size: 0\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	oldwd, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(oldwd) })
	if _, err := Load(nil); err == nil {
		t.Fatal("expected non-positive batch size to be rejected")
	}
}

func TestLoadSpecializedModelsFallBackToDefaultModel(t *testing.T) {
	dir := t.TempDir()
	oldwd, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(oldwd) })
	t.Setenv("GOAUTHORLLM_MODEL", "default-model")
	t.Setenv("GOAUTHORLLM_GENERATION_MODEL", "")
	t.Setenv("GOAUTHORLLM_EDITING_MODEL", "")

	cfg, err := Load(nil)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.GenerationModel != cfg.Model || cfg.EditingModel != cfg.Model {
		t.Fatalf("specialized models did not fall back: %#v", cfg)
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

func TestLoadValidatesNonInteractiveGeneration(t *testing.T) {
	dir := t.TempDir()
	oldwd, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	cfg, err := Load([]string{"--non-interactive", "--mode", "generate", "--submode", "continue", "draft.md"})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !cfg.NonInteractive || cfg.FilePath != "draft.md" || cfg.Mode != "generate" || cfg.Submode != "continue" {
		t.Fatalf("unexpected non-interactive config: %#v", cfg)
	}
}

func TestLoadRejectsIncompleteNonInteractiveEdit(t *testing.T) {
	dir := t.TempDir()
	oldwd, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(oldwd) })

	_, err := Load([]string{"--non-interactive", "--mode", "edit", "--submode", "copy", "draft.md"})
	if err == nil || !strings.Contains(err.Error(), "--approval") {
		t.Fatalf("expected missing approval error, got %v", err)
	}
	_, err = Load([]string{"--non-interactive", "--mode", "edit", "--submode", "directed", "--approval", "approve-all", "draft.md"})
	if err == nil || !strings.Contains(err.Error(), "edit-instructions") {
		t.Fatalf("expected missing instructions error, got %v", err)
	}
}

func TestLoadReadsOperationalValuesFromEnvironment(t *testing.T) {
	dir := t.TempDir()
	instructionsPath := filepath.Join(dir, "instructions.txt")
	if err := os.WriteFile(instructionsPath, []byte("Rewrite the introduction."), 0o644); err != nil {
		t.Fatalf("write instructions: %v", err)
	}
	oldwd, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(oldwd) })
	t.Setenv("GOAUTHORLLM_MODE", "edit")
	t.Setenv("GOAUTHORLLM_SUBMODE", "directed")
	t.Setenv("GOAUTHORLLM_APPROVAL", "llm-approved")
	t.Setenv("GOAUTHORLLM_EDIT_INSTRUCTIONS_FILE", instructionsPath)
	t.Setenv("GOAUTHORLLM_MAX_EDITS", "4")

	cfg, err := Load([]string{"--non-interactive", "draft.md"})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.EditInstructions != "Rewrite the introduction." || cfg.MaxEdits != 4 {
		t.Fatalf("unexpected environment-backed config: %#v", cfg)
	}
}

func TestLoadReadsPromptOverrideFiles(t *testing.T) {
	dir := t.TempDir()
	replacePath := filepath.Join(dir, "generate.txt")
	appendPath := filepath.Join(dir, "continue.txt")
	envPath := filepath.Join(dir, "edit.txt")
	if err := os.WriteFile(replacePath, []byte("Replacement system prompt."), 0o644); err != nil {
		t.Fatalf("write replacement: %v", err)
	}
	if err := os.WriteFile(appendPath, []byte("Additional continuation rule."), 0o644); err != nil {
		t.Fatalf("write append: %v", err)
	}
	if err := os.WriteFile(envPath, []byte("Environment edit prompt."), 0o644); err != nil {
		t.Fatalf("write environment prompt: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".goauthorllm"), []byte("generate_prompt:\n  replace_file: generate.txt\n"), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	oldwd, _ := os.Getwd()
	_ = os.Chdir(dir)
	t.Cleanup(func() { _ = os.Chdir(oldwd) })
	t.Setenv("GOAUTHORLLM_EDIT_PROMPT_FILE", envPath)

	cfg, err := Load([]string{"--prompt-append-file", "continue_prompt=" + appendPath})
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if cfg.MessageOverrides[prompts.GeneratePrompt].Replace != "Replacement system prompt." {
		t.Fatalf("unexpected replacement: %#v", cfg.MessageOverrides[prompts.GeneratePrompt])
	}
	if cfg.MessageOverrides[prompts.ContinuePrompt].Append != "Additional continuation rule." {
		t.Fatalf("unexpected append: %#v", cfg.MessageOverrides[prompts.ContinuePrompt])
	}
	if cfg.MessageOverrides[prompts.EditPrompt].Replace != "Environment edit prompt." {
		t.Fatalf("unexpected environment replacement: %#v", cfg.MessageOverrides[prompts.EditPrompt])
	}
}
