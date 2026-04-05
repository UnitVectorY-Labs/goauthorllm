package config

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/UnitVectorY-Labs/goauthorllm/internal/prompts"
)

const (
	// DefaultBaseURL is the default OpenAI-compatible endpoint (local Ollama).
	DefaultBaseURL = "http://localhost:11434/v1"
	// DefaultModel is the default LLM model name.
	DefaultModel = "gemma3:4b"
)

// Config holds all runtime settings.
type Config struct {
	FilePath         string
	BaseURL          string
	Model            string
	APIKey           string
	Timeout          time.Duration
	MessageOverrides prompts.Overrides
}

// Load parses command-line flags and environment variables into a Config.
// Environment variables are consulted for defaults; flags take precedence.
func Load(args []string) (Config, error) {
	timeoutDefault := 90 * time.Second
	if raw := firstNonEmpty(os.Getenv("GOAUTHORLLM_TIMEOUT")); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil {
			return Config{}, fmt.Errorf("invalid GOAUTHORLLM_TIMEOUT: %w", err)
		}
		timeoutDefault = parsed
	}

	cfg := Config{}
	fs := flag.NewFlagSet("goauthorllm", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&cfg.FilePath, "file", firstNonEmpty(os.Getenv("GOAUTHORLLM_FILE")), "markdown document path")
	fs.StringVar(&cfg.BaseURL, "base-url", firstNonEmpty(os.Getenv("GOAUTHORLLM_BASE_URL"), os.Getenv("OPENAI_BASE_URL"), DefaultBaseURL), "OpenAI-compatible base URL")
	fs.StringVar(&cfg.Model, "model", firstNonEmpty(os.Getenv("GOAUTHORLLM_MODEL"), os.Getenv("OPENAI_MODEL"), DefaultModel), "LLM model name")
	fs.StringVar(&cfg.APIKey, "api-key", firstNonEmpty(os.Getenv("GOAUTHORLLM_API_KEY"), os.Getenv("OPENAI_API_KEY")), "API key for the LLM endpoint")
	fs.DurationVar(&cfg.Timeout, "timeout", timeoutDefault, "request timeout")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	switch fs.NArg() {
	case 0:
	case 1:
		cfg.FilePath = fs.Arg(0)
	default:
		return Config{}, fmt.Errorf("expected at most one positional argument for the document path")
	}

	if cfg.BaseURL == "" {
		return Config{}, fmt.Errorf("base URL is required")
	}
	if cfg.Model == "" {
		return Config{}, fmt.Errorf("model is required")
	}
	if cfg.Timeout <= 0 {
		return Config{}, fmt.Errorf("timeout must be greater than zero")
	}

	messageOverrides, err := loadLocalMessageOverrides(".goauthorllm")
	if err != nil {
		return Config{}, err
	}
	cfg.MessageOverrides = messageOverrides

	return cfg, nil
}

func loadLocalMessageOverrides(path string) (prompts.Overrides, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return prompts.Overrides{}, nil
		}
		return prompts.Overrides{}, err
	}

	var raw map[string]prompts.Override
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return prompts.Overrides{}, fmt.Errorf("parse %s: %w", path, err)
	}

	return normalizeMessageOverrides(raw), nil
}

func normalizeMessageOverrides(raw map[string]prompts.Override) prompts.Overrides {
	overrides := make(prompts.Overrides, len(raw))
	for key, override := range raw {
		name := prompts.Name(strings.TrimSpace(key))
		overrides[name] = prompts.Override{
			Replace: strings.TrimSpace(override.Replace),
			Append:  strings.TrimSpace(override.Append),
		}
	}
	return overrides
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
