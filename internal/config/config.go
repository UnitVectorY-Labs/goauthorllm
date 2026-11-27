package config

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/UnitVectorY-Labs/goauthorllm/internal/prompts"
)

const (
	// LocalConfigFilename is the default project configuration file in the working directory.
	LocalConfigFilename = ".goauthorllm"
	// DefaultBaseURL is the default OpenAI-compatible endpoint (local Ollama).
	DefaultBaseURL = "http://localhost:11434/v1"
	// DefaultModel is the default LLM model name.
	DefaultModel = "gemma3:4b"
	// DefaultScope is the default content generation granularity.
	DefaultScope = "paragraph"
	// DefaultCopyEditBatchSize is the default max suggestions per copy-edit pass.
	DefaultCopyEditBatchSize = 5
	// DefaultDirectedEditBatchSize is the default max suggestions per directed-edit pass.
	DefaultDirectedEditBatchSize = 10
	// DefaultStreamSpeed is 0 (max speed, unthrottled).
	DefaultStreamSpeed = 0
)

// ValidScopes lists all supported generation scopes.
var ValidScopes = []string{"sentence", "paragraph", "section", "free"}

// IsValidScope reports whether scope is a supported generation scope.
func IsValidScope(scope string) bool {
	switch strings.ToLower(strings.TrimSpace(scope)) {
	case "sentence", "paragraph", "section", "free":
		return true
	default:
		return false
	}
}

// Config holds all resolved runtime settings.
type Config struct {
	ConfigPath            string
	FilePath              string
	NonInteractive        bool
	Mode                  string
	Submode               string
	Approval              string
	Guidance              string
	EditInstructions      string
	Rounds                int
	MaxEdits              int
	BaseURL               string
	Model                 string
	GenerationModel       string
	EditingModel          string
	APIKey                string
	Timeout               time.Duration
	UserAgent             string
	StreamSpeed           int // characters per second; 0 means unthrottled (max speed)
	DefaultScope          string
	StructuredGeneration  bool
	Rules                 string
	CopyEditRules         string
	CopyEditBatchSize     int
	DirectedEditBatchSize int
	MessageOverrides      prompts.Overrides
}

// LocalFile models the on-disk .goauthorllm YAML configuration.
type LocalFile struct {
	BaseURL               string                      `yaml:"base_url,omitempty"`
	Model                 string                      `yaml:"model,omitempty"`
	GenerationModel       string                      `yaml:"generation_model,omitempty"`
	EditingModel          string                      `yaml:"editing_model,omitempty"`
	StreamSpeed           *int                        `yaml:"stream_chars_per_sec,omitempty"`
	DefaultScope          string                      `yaml:"default_scope,omitempty"`
	StructuredGeneration  *bool                       `yaml:"structured_generation,omitempty"`
	Rules                 string                      `yaml:"rules,omitempty"`
	RulesFile             string                      `yaml:"rules_file,omitempty"`
	CopyEditRules         string                      `yaml:"copy_edit_rules,omitempty"`
	CopyEditRulesFile     string                      `yaml:"copy_edit_rules_file,omitempty"`
	CopyEditBatchSize     *int                        `yaml:"copy_edit_batch_size,omitempty"`
	DirectedEditBatchSize *int                        `yaml:"directed_edit_batch_size,omitempty"`
	Mode                  string                      `yaml:"mode,omitempty"`
	Submode               string                      `yaml:"submode,omitempty"`
	Approval              string                      `yaml:"approval,omitempty"`
	Rounds                *int                        `yaml:"rounds,omitempty"`
	MaxEdits              *int                        `yaml:"max_edits,omitempty"`
	Guidance              string                      `yaml:"guidance,omitempty"`
	GuidanceFile          string                      `yaml:"guidance_file,omitempty"`
	EditInstructions      string                      `yaml:"edit_instructions,omitempty"`
	EditInstructionsFile  string                      `yaml:"edit_instructions_file,omitempty"`
	MessageOverrides      map[string]prompts.Override `yaml:",inline"`
}

type stringListFlag []string

func (values *stringListFlag) String() string { return strings.Join(*values, ",") }
func (values *stringListFlag) Set(value string) error {
	*values = append(*values, value)
	return nil
}

// Load parses command-line flags, environment variables, and .goauthorllm
// into a Config. Precedence: Flags > Environment > .goauthorllm > Defaults.
func Load(args []string) (Config, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return Config{}, err
	}
	configPath := filepath.Join(cwd, LocalConfigFilename)
	localCfg, err := LoadLocal(configPath)
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

	cfg := Config{ConfigPath: configPath}
	var (
		baseURLFlag               string
		modelFlag                 string
		generationModelFlag       string
		editingModelFlag          string
		streamSpeedFlag           int
		defaultScopeFlag          string
		structuredGenFlag         bool
		roundsFlag                int
		copyEditBatchSizeFlag     int
		directedEditBatchSizeFlag int
		maxEditsFlag              int
		modeFlag                  string
		submodeFlag               string
		approvalFlag              string
		guidanceFlag              string
		guidanceFileFlag          string
		editInstructionsFlag      string
		editInstructionsFileFlag  string
		promptFiles               stringListFlag
		promptAppendFiles         stringListFlag
	)

	fs := flag.NewFlagSet("goauthorllm", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&cfg.FilePath, "file", firstNonEmpty(os.Getenv("GOAUTHORLLM_FILE")), "markdown document path")
	fs.BoolVar(&cfg.NonInteractive, "non-interactive", false, "run once without launching the TUI")
	fs.StringVar(&modeFlag, "mode", "", "operation mode: generate or edit")
	fs.StringVar(&submodeFlag, "submode", "", "generate: sentence, paragraph, section, free (or continue, new-section); edit: copy or directed")
	fs.StringVar(&approvalFlag, "approval", "", "non-interactive edit approval: approve-all")
	fs.StringVar(&guidanceFlag, "guidance", "", "optional generation guidance")
	fs.StringVar(&guidanceFileFlag, "guidance-file", "", "path containing generation guidance")
	fs.StringVar(&editInstructionsFlag, "edit-instructions", "", "instructions for directed/custom editing")
	fs.StringVar(&editInstructionsFileFlag, "edit-instructions-file", "", "path containing directed edit instructions")
	fs.IntVar(&roundsFlag, "rounds", 1, "number of generation rounds to run")
	fs.IntVar(&maxEditsFlag, "max-edits", 0, "maximum edits to apply in this run (0 is unlimited)")
	fs.Var(&promptFiles, "prompt-file", "replace a prompt from NAME=PATH (repeatable)")
	fs.Var(&promptAppendFiles, "prompt-append-file", "append to a prompt from NAME=PATH (repeatable)")
	fs.StringVar(&baseURLFlag, "base-url", "", "OpenAI-compatible base URL")
	fs.StringVar(&modelFlag, "model", "", "LLM model name")
	fs.StringVar(&generationModelFlag, "generation-model", "", "model for generation requests (defaults to --model)")
	fs.StringVar(&editingModelFlag, "editing-model", "", "model for editing requests (defaults to --model)")
	fs.IntVar(&streamSpeedFlag, "stream-speed", DefaultStreamSpeed, "streaming display speed in characters/sec (0 = max speed)")
	fs.StringVar(&defaultScopeFlag, "scope", "", "default generation scope: sentence, paragraph, section, or free")
	fs.BoolVar(&structuredGenFlag, "structured-generation", true, "use structured JSON streaming for generation")
	fs.IntVar(&copyEditBatchSizeFlag, "copy-edit-batch-size", DefaultCopyEditBatchSize, "maximum copy-edit suggestions per batch")
	fs.IntVar(&directedEditBatchSizeFlag, "directed-edit-batch-size", DefaultDirectedEditBatchSize, "maximum directed-edit suggestions per batch")
	fs.StringVar(&cfg.APIKey, "api-key", firstNonEmpty(os.Getenv("GOAUTHORLLM_API_KEY"), os.Getenv("OPENAI_API_KEY")), "API key for the LLM endpoint")
	fs.DurationVar(&cfg.Timeout, "timeout", timeoutDefault, "non-streaming LLM request timeout")

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
	cfg.GenerationModel = resolveStringValue(
		providedFlags["generation-model"],
		generationModelFlag,
		os.Getenv("GOAUTHORLLM_GENERATION_MODEL"),
		localCfg.GenerationModel,
		cfg.Model,
	)
	cfg.EditingModel = resolveStringValue(
		providedFlags["editing-model"],
		editingModelFlag,
		os.Getenv("GOAUTHORLLM_EDITING_MODEL"),
		localCfg.EditingModel,
		cfg.Model,
	)
	cfg.StreamSpeed, err = resolveNonNegativeIntValue(
		providedFlags["stream-speed"],
		streamSpeedFlag,
		os.Getenv("GOAUTHORLLM_STREAM_SPEED"),
		localCfg.StreamSpeed,
		DefaultStreamSpeed,
		"stream speed",
	)
	if err != nil {
		return Config{}, err
	}
	cfg.DefaultScope = strings.ToLower(strings.TrimSpace(resolveStringValue(
		providedFlags["scope"],
		defaultScopeFlag,
		os.Getenv("GOAUTHORLLM_SCOPE"),
		localCfg.DefaultScope,
		DefaultScope,
	)))
	if !IsValidScope(cfg.DefaultScope) {
		return Config{}, fmt.Errorf("invalid scope %q: must be sentence, paragraph, section, or free", cfg.DefaultScope)
	}
	cfg.StructuredGeneration, err = resolveBoolValue(
		providedFlags["structured-generation"],
		structuredGenFlag,
		os.Getenv("GOAUTHORLLM_STRUCTURED_GENERATION"),
		localCfg.StructuredGeneration,
		true,
		"structured generation",
	)
	if err != nil {
		return Config{}, err
	}
	cfg.Rounds, err = resolvePositiveIntValue(
		providedFlags["rounds"],
		roundsFlag,
		os.Getenv("GOAUTHORLLM_ROUNDS"),
		localCfg.Rounds,
		1,
		"rounds",
	)
	if err != nil {
		return Config{}, err
	}
	cfg.CopyEditBatchSize, err = resolvePositiveIntValue(
		providedFlags["copy-edit-batch-size"],
		copyEditBatchSizeFlag,
		os.Getenv("GOAUTHORLLM_COPY_EDIT_BATCH_SIZE"),
		localCfg.CopyEditBatchSize,
		DefaultCopyEditBatchSize,
		"copy edit batch size",
	)
	if err != nil {
		return Config{}, err
	}
	cfg.DirectedEditBatchSize, err = resolvePositiveIntValue(
		providedFlags["directed-edit-batch-size"],
		directedEditBatchSizeFlag,
		os.Getenv("GOAUTHORLLM_DIRECTED_EDIT_BATCH_SIZE"),
		localCfg.DirectedEditBatchSize,
		DefaultDirectedEditBatchSize,
		"directed edit batch size",
	)
	if err != nil {
		return Config{}, err
	}
	cfg.MaxEdits, err = resolveNonNegativeIntValue(
		providedFlags["max-edits"],
		maxEditsFlag,
		os.Getenv("GOAUTHORLLM_MAX_EDITS"),
		localCfg.MaxEdits,
		0,
		"maximum edits",
	)
	if err != nil {
		return Config{}, err
	}
	cfg.Mode = resolveStringValue(providedFlags["mode"], modeFlag, os.Getenv("GOAUTHORLLM_MODE"), localCfg.Mode)
	cfg.Submode = resolveStringValue(providedFlags["submode"], submodeFlag, os.Getenv("GOAUTHORLLM_SUBMODE"), localCfg.Submode)
	cfg.Approval = resolveStringValue(providedFlags["approval"], approvalFlag, os.Getenv("GOAUTHORLLM_APPROVAL"), localCfg.Approval)
	cfg.Guidance, err = resolveTextValue(
		providedFlags["guidance"], guidanceFlag,
		providedFlags["guidance-file"], guidanceFileFlag,
		os.Getenv("GOAUTHORLLM_GUIDANCE"), os.Getenv("GOAUTHORLLM_GUIDANCE_FILE"),
		localCfg.Guidance, localCfg.GuidanceFile,
		"generation guidance",
	)
	if err != nil {
		return Config{}, err
	}
	cfg.EditInstructions, err = resolveTextValue(
		providedFlags["edit-instructions"], editInstructionsFlag,
		providedFlags["edit-instructions-file"], editInstructionsFileFlag,
		os.Getenv("GOAUTHORLLM_EDIT_INSTRUCTIONS"), os.Getenv("GOAUTHORLLM_EDIT_INSTRUCTIONS_FILE"),
		localCfg.EditInstructions, localCfg.EditInstructionsFile,
		"edit instructions",
	)
	if err != nil {
		return Config{}, err
	}
	cfg.Rules, err = resolveTextValue(
		false, "",
		false, "",
		os.Getenv("GOAUTHORLLM_RULES"), os.Getenv("GOAUTHORLLM_RULES_FILE"),
		localCfg.Rules, localCfg.RulesFile,
		"writing rules",
	)
	if err != nil {
		return Config{}, err
	}
	cfg.CopyEditRules, err = resolveTextValue(
		false, "",
		false, "",
		os.Getenv("GOAUTHORLLM_COPY_EDIT_RULES"), os.Getenv("GOAUTHORLLM_COPY_EDIT_RULES_FILE"),
		localCfg.CopyEditRules, localCfg.CopyEditRulesFile,
		"copy edit rules",
	)
	if err != nil {
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

	cfg.MessageOverrides, err = resolveMessageOverrides(localCfg.MessageOverrides, promptFiles, promptAppendFiles)
	if err != nil {
		return Config{}, err
	}
	if cfg.NonInteractive {
		fileProvidedOnCommandLine := providedFlags["file"] || fs.NArg() == 1
		if err := validateNonInteractive(cfg, fileProvidedOnCommandLine); err != nil {
			return Config{}, err
		}
	}

	return cfg, nil
}

// LoadLocal reads and parses a .goauthorllm YAML file.
// If the file does not exist, an empty LocalFile is returned without error.
func LoadLocal(path string) (LocalFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return LocalFile{}, nil
		}
		return LocalFile{}, err
	}
	return ParseLocalYAML(string(data), path)
}

// ParseLocalYAML parses YAML content into a validated LocalFile.
func ParseLocalYAML(content, label string) (LocalFile, error) {
	if strings.TrimSpace(content) == "" {
		return LocalFile{}, nil
	}
	var raw LocalFile
	if err := yaml.Unmarshal([]byte(content), &raw); err != nil {
		return LocalFile{}, fmt.Errorf("parse %s: %w", label, err)
	}

	raw.BaseURL = strings.TrimSpace(raw.BaseURL)
	raw.Model = strings.TrimSpace(raw.Model)
	raw.GenerationModel = strings.TrimSpace(raw.GenerationModel)
	raw.EditingModel = strings.TrimSpace(raw.EditingModel)
	raw.DefaultScope = strings.ToLower(strings.TrimSpace(raw.DefaultScope))
	raw.Rules = strings.TrimSpace(raw.Rules)
	raw.CopyEditRules = strings.TrimSpace(raw.CopyEditRules)
	raw.Mode = strings.TrimSpace(raw.Mode)
	raw.Submode = strings.TrimSpace(raw.Submode)
	raw.Approval = strings.TrimSpace(raw.Approval)

	if raw.DefaultScope != "" && !IsValidScope(raw.DefaultScope) {
		return LocalFile{}, fmt.Errorf("%s: invalid default_scope %q", label, raw.DefaultScope)
	}
	if raw.StreamSpeed != nil && *raw.StreamSpeed < 0 {
		return LocalFile{}, fmt.Errorf("%s: stream_chars_per_sec cannot be negative", label)
	}
	if raw.CopyEditBatchSize != nil && *raw.CopyEditBatchSize <= 0 {
		return LocalFile{}, fmt.Errorf("%s: copy_edit_batch_size must be positive", label)
	}
	if raw.DirectedEditBatchSize != nil && *raw.DirectedEditBatchSize <= 0 {
		return LocalFile{}, fmt.Errorf("%s: directed_edit_batch_size must be positive", label)
	}
	for key := range raw.MessageOverrides {
		if !prompts.Valid(prompts.Name(strings.TrimSpace(key))) {
			return LocalFile{}, fmt.Errorf("%s: unknown key or prompt name %q", label, key)
		}
	}
	return raw, nil
}

// RenderLocalYAML serializes a LocalFile into clean YAML.
func RenderLocalYAML(local LocalFile) (string, error) {
	cleaned := local
	if len(cleaned.MessageOverrides) > 0 {
		filtered := make(map[string]prompts.Override, len(cleaned.MessageOverrides))
		for k, v := range cleaned.MessageOverrides {
			if !v.IsZero() {
				filtered[strings.TrimSpace(k)] = v
			}
		}
		if len(filtered) == 0 {
			cleaned.MessageOverrides = nil
		} else {
			cleaned.MessageOverrides = filtered
		}
	}
	data, err := yaml.Marshal(cleaned)
	if err != nil {
		return "", err
	}
	text := string(data)
	if strings.TrimSpace(text) == "{}" {
		return "", nil
	}
	return text, nil
}

// SaveLocal writes a LocalFile to path cleanly as YAML.
func SaveLocal(path string, local LocalFile) error {
	rendered, err := RenderLocalYAML(local)
	if err != nil {
		return err
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, []byte(rendered), 0o644)
}

// ApplyLocal merges settings from an updated LocalFile into an active Config
// so in-app edits take effect immediately without restarting.
func (c *Config) ApplyLocal(local LocalFile) error {
	if local.BaseURL != "" {
		c.BaseURL = local.BaseURL
	}
	if local.Model != "" {
		c.Model = local.Model
	}
	if local.GenerationModel != "" {
		c.GenerationModel = local.GenerationModel
	} else {
		c.GenerationModel = c.Model
	}
	if local.EditingModel != "" {
		c.EditingModel = local.EditingModel
	} else {
		c.EditingModel = c.Model
	}
	if local.StreamSpeed != nil {
		c.StreamSpeed = *local.StreamSpeed
	}
	if local.DefaultScope != "" {
		c.DefaultScope = local.DefaultScope
	}
	if local.StructuredGeneration != nil {
		c.StructuredGeneration = *local.StructuredGeneration
	}
	if local.CopyEditBatchSize != nil {
		c.CopyEditBatchSize = *local.CopyEditBatchSize
	}
	if local.DirectedEditBatchSize != nil {
		c.DirectedEditBatchSize = *local.DirectedEditBatchSize
	}
	rules, err := resolveTextValue(false, "", false, "", "", "", local.Rules, local.RulesFile, "writing rules")
	if err != nil {
		return err
	}
	c.Rules = rules

	copyRules, err := resolveTextValue(false, "", false, "", "", "", local.CopyEditRules, local.CopyEditRulesFile, "copy edit rules")
	if err != nil {
		return err
	}
	c.CopyEditRules = copyRules

	overrides, err := resolveMessageOverrides(local.MessageOverrides, nil, nil)
	if err != nil {
		return err
	}
	c.MessageOverrides = overrides
	return nil
}

// ToLocalFile builds a LocalFile snapshot from the current runtime Config and
// existing prompt overrides on disk.
func (c Config) ToLocalFile(existing LocalFile) LocalFile {
	out := existing
	out.BaseURL = strings.TrimSpace(c.BaseURL)
	out.Model = strings.TrimSpace(c.Model)
	if strings.TrimSpace(c.GenerationModel) != "" && c.GenerationModel != c.Model {
		out.GenerationModel = strings.TrimSpace(c.GenerationModel)
	} else {
		out.GenerationModel = ""
	}
	if strings.TrimSpace(c.EditingModel) != "" && c.EditingModel != c.Model {
		out.EditingModel = strings.TrimSpace(c.EditingModel)
	} else {
		out.EditingModel = ""
	}
	speed := c.StreamSpeed
	out.StreamSpeed = &speed
	out.DefaultScope = c.DefaultScope
	structured := c.StructuredGeneration
	out.StructuredGeneration = &structured
	copyBatch := c.CopyEditBatchSize
	out.CopyEditBatchSize = &copyBatch
	dirBatch := c.DirectedEditBatchSize
	out.DirectedEditBatchSize = &dirBatch
	out.Rules = strings.TrimSpace(c.Rules)
	out.CopyEditRules = strings.TrimSpace(c.CopyEditRules)
	return out
}

func normalizeMessageOverrides(raw map[string]prompts.Override) prompts.Overrides {
	overrides := make(prompts.Overrides, len(raw))
	for key, override := range raw {
		name := prompts.Name(strings.TrimSpace(key))
		overrides[name] = prompts.Override{
			Replace:     strings.TrimSpace(override.Replace),
			Append:      strings.TrimSpace(override.Append),
			ReplaceFile: strings.TrimSpace(override.ReplaceFile),
			AppendFile:  strings.TrimSpace(override.AppendFile),
		}
	}
	return overrides
}

func resolveMessageOverrides(raw map[string]prompts.Override, replaceFiles, appendFiles []string) (prompts.Overrides, error) {
	overrides := normalizeMessageOverrides(raw)
	for name, override := range overrides {
		if !prompts.Valid(name) {
			return nil, fmt.Errorf("unknown prompt name %q", name)
		}
		if override.ReplaceFile != "" {
			text, err := readTextFile(override.ReplaceFile, fmt.Sprintf("%s replace file", name))
			if err != nil {
				return nil, err
			}
			override.Replace = text
		}
		if override.AppendFile != "" {
			text, err := readTextFile(override.AppendFile, fmt.Sprintf("%s append file", name))
			if err != nil {
				return nil, err
			}
			override.Append = text
		}
		overrides[name] = override
	}

	for _, name := range prompts.Names() {
		envPrefix := "GOAUTHORLLM_" + strings.ToUpper(string(name))
		override := overrides[name]
		if path := strings.TrimSpace(os.Getenv(envPrefix + "_FILE")); path != "" {
			text, err := readTextFile(path, fmt.Sprintf("%s file", name))
			if err != nil {
				return nil, err
			}
			override.Replace = text
		}
		if path := strings.TrimSpace(os.Getenv(envPrefix + "_APPEND_FILE")); path != "" {
			text, err := readTextFile(path, fmt.Sprintf("%s append file", name))
			if err != nil {
				return nil, err
			}
			override.Append = text
		}
		if !override.IsZero() {
			overrides[name] = override
		}
	}

	if err := applyPromptFileFlags(overrides, replaceFiles, false); err != nil {
		return nil, err
	}
	if err := applyPromptFileFlags(overrides, appendFiles, true); err != nil {
		return nil, err
	}
	return overrides, nil
}

func applyPromptFileFlags(overrides prompts.Overrides, values []string, appendText bool) error {
	for _, value := range values {
		rawName, path, ok := strings.Cut(value, "=")
		name := prompts.Name(strings.TrimSpace(rawName))
		path = strings.TrimSpace(path)
		if !ok || path == "" {
			return fmt.Errorf("prompt file must use NAME=PATH: %q", value)
		}
		if !prompts.Valid(name) {
			return fmt.Errorf("unknown prompt name %q", name)
		}
		text, err := readTextFile(path, fmt.Sprintf("%s prompt file", name))
		if err != nil {
			return err
		}
		override := overrides[name]
		if appendText {
			override.Append = text
		} else {
			override.Replace = text
		}
		overrides[name] = override
	}
	return nil
}

func resolveTextValue(inlineFlagProvided bool, inlineFlag string, fileFlagProvided bool, fileFlag, inlineEnv, fileEnv, inlineConfig, fileConfig, label string) (string, error) {
	if inlineFlagProvided && fileFlagProvided {
		return "", fmt.Errorf("%s cannot be provided both inline and by file", label)
	}
	if fileFlagProvided {
		return readTextFile(fileFlag, label+" file")
	}
	if inlineFlagProvided {
		return strings.TrimSpace(inlineFlag), nil
	}
	if strings.TrimSpace(fileEnv) != "" {
		return readTextFile(fileEnv, label+" file")
	}
	if inlineEnv != "" {
		return strings.TrimSpace(inlineEnv), nil
	}
	if strings.TrimSpace(fileConfig) != "" {
		return readTextFile(fileConfig, label+" file")
	}
	return strings.TrimSpace(inlineConfig), nil
}

func readTextFile(path, label string) (string, error) {
	data, err := os.ReadFile(strings.TrimSpace(path))
	if err != nil {
		return "", fmt.Errorf("read %s: %w", label, err)
	}
	return strings.TrimSpace(string(data)), nil
}

func validateNonInteractive(cfg Config, fileProvidedOnCommandLine bool) error {
	if !fileProvidedOnCommandLine || strings.TrimSpace(cfg.FilePath) == "" {
		return fmt.Errorf("a command-line --file or positional document path is required in non-interactive mode")
	}
	mode := strings.ToLower(strings.TrimSpace(cfg.Mode))
	submode := strings.ToLower(strings.TrimSpace(cfg.Submode))
	approval := strings.ToLower(strings.TrimSpace(cfg.Approval))
	switch mode {
	case "generate":
		switch submode {
		case "sentence", "paragraph", "section", "free", "continue", "new-section":
		default:
			return fmt.Errorf("generation --submode must be sentence, paragraph, section, free, continue, or new-section")
		}
	case "edit":
		if submode != "copy" && submode != "directed" {
			return fmt.Errorf("editing --submode must be copy or directed")
		}
		if approval != "approve-all" {
			return fmt.Errorf("editing --approval must be approve-all in non-interactive mode")
		}
		if submode == "directed" && strings.TrimSpace(cfg.EditInstructions) == "" {
			return fmt.Errorf("directed editing requires --edit-instructions or --edit-instructions-file")
		}
	default:
		return fmt.Errorf("--mode must be generate or edit in non-interactive mode")
	}
	return nil
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

func resolvePositiveIntValue(flagProvided bool, flagValue int, envValue string, fileValue *int, defaultValue int, label string) (int, error) {
	if flagProvided {
		if flagValue <= 0 {
			return 0, fmt.Errorf("%s must be greater than zero", label)
		}
		return flagValue, nil
	}
	if strings.TrimSpace(envValue) != "" {
		value, err := strconv.Atoi(strings.TrimSpace(envValue))
		if err != nil || value <= 0 {
			return 0, fmt.Errorf("invalid GOAUTHORLLM_%s: must be a positive integer", strings.ToUpper(strings.ReplaceAll(label, " ", "_")))
		}
		return value, nil
	}
	if fileValue != nil {
		if *fileValue <= 0 {
			return 0, fmt.Errorf("%s must be greater than zero", label)
		}
		return *fileValue, nil
	}
	return defaultValue, nil
}

func resolveNonNegativeIntValue(flagProvided bool, flagValue int, envValue string, fileValue *int, defaultValue int, label string) (int, error) {
	if flagProvided {
		if flagValue < 0 {
			return 0, fmt.Errorf("%s cannot be negative", label)
		}
		return flagValue, nil
	}
	if strings.TrimSpace(envValue) != "" {
		value, err := strconv.Atoi(strings.TrimSpace(envValue))
		if err != nil || value < 0 {
			return 0, fmt.Errorf("invalid GOAUTHORLLM_%s: must be a non-negative integer", strings.ToUpper(strings.ReplaceAll(label, " ", "_")))
		}
		return value, nil
	}
	if fileValue != nil {
		if *fileValue < 0 {
			return 0, fmt.Errorf("%s cannot be negative", label)
		}
		return *fileValue, nil
	}
	return defaultValue, nil
}

func resolveBoolValue(flagProvided bool, flagValue bool, envValue string, fileValue *bool, defaultValue bool, label string) (bool, error) {
	if flagProvided {
		return flagValue, nil
	}
	if strings.TrimSpace(envValue) != "" {
		value, err := strconv.ParseBool(strings.TrimSpace(envValue))
		if err != nil {
			return false, fmt.Errorf("invalid %s env value %q", label, envValue)
		}
		return value, nil
	}
	if fileValue != nil {
		return *fileValue, nil
	}
	return defaultValue, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
