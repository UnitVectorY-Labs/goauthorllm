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

type localConfigFile struct {
	BaseURL          string                      `yaml:"base_url"`
	Model            string                      `yaml:"model"`
	MessageOverrides map[string]prompts.Override `yaml:",inline"`
}

// Load parses command-line flags, environment variables, and .goauthorllm
// into a Config. Flags take precedence over environment variables, which take
// precedence over .goauthorllm values.
func Load(args []string) (Config, error) {
	localCfg, err := loadLocalConfig(".goauthorllm")
	if err != nil {
		return Config{}, err
	}

	timeoutDefault := 90 * time.Second
	if raw := firstNonEmpty(os.Getenv("GOAUTHORLLM_TIMEOUT")); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil {
			return Config{}, fmt.Errorf("invalid GOAUTHORLLM_TIMEOUT: %w", err)
		}
		timeoutDefault = parsed
	}

	cfg := Config{}
	var baseURLFlag string
	var modelFlag string
	fs := flag.NewFlagSet("goauthorllm", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&cfg.FilePath, "file", firstNonEmpty(os.Getenv("GOAUTHORLLM_FILE")), "markdown document path")
	fs.StringVar(&baseURLFlag, "base-url", "", "OpenAI-compatible base URL")
	fs.StringVar(&modelFlag, "model", "", "LLM model name")
	fs.StringVar(&cfg.APIKey, "api-key", firstNonEmpty(os.Getenv("GOAUTHORLLM_API_KEY"), os.Getenv("OPENAI_API_KEY")), "API key for the LLM endpoint")
	fs.DurationVar(&cfg.Timeout, "timeout", timeoutDefault, "request timeout")

	if err := fs.Parse(args); err != nil {
		return Config{}, err
	}

	providedFlags := visitedFlags(fs)
	cfg.BaseURL = resolveStringValue(
		providedFlags["base-url"],
		baseURLFlag,
		os.Getenv("GOAUTHORLLM_BASE_URL"),
		os.Getenv("OPENAI_BASE_URL"),
		localCfg.BaseURL,
		DefaultBaseURL,
	)
	cfg.Model = resolveStringValue(
		providedFlags["model"],
		modelFlag,
		os.Getenv("GOAUTHORLLM_MODEL"),
		os.Getenv("OPENAI_MODEL"),
		localCfg.Model,
		DefaultModel,
	)

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

	cfg.MessageOverrides = normalizeMessageOverrides(localCfg.MessageOverrides)

	return cfg, nil
}

func loadLocalConfig(path string) (localConfigFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return localConfigFile{}, nil
		}
		return localConfigFile{}, err
	}

	var raw localConfigFile
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return localConfigFile{}, fmt.Errorf("parse %s: %w", path, err)
	}

	raw.BaseURL = strings.TrimSpace(raw.BaseURL)
	raw.Model = strings.TrimSpace(raw.Model)

	return raw, nil
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

func visitedFlags(fs *flag.FlagSet) map[string]bool {
	flags := make(map[string]bool)
	fs.Visit(func(f *flag.Flag) {
		flags[f.Name] = true
	})
	return flags
}

func resolveStringValue(flagProvided bool, flagValue string, values ...string) string {
	if flagProvided {
		return flagValue
	}
	return firstNonEmpty(values...)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
