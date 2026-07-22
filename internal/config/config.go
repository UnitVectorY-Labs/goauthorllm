package config

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strconv"
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
	// DefaultCopyEditBatchSize preserves the original one-at-a-time copy-edit flow.
	DefaultCopyEditBatchSize = 1
	// DefaultDirectedEditBatchSize preserves the original directed-edit batch size.
	DefaultDirectedEditBatchSize = 10
)

// Config holds all runtime settings.
type Config struct {
	FilePath              string
	NonInteractive        bool
	Mode                  string
	Submode               string
	Approval              string
	Guidance              string
	EditInstructions      string
	MaxEdits              int
	BaseURL               string
	Model                 string
	GenerationModel       string
	EditingModel          string
	APIKey                string
	Timeout               time.Duration
	CopyEditBatchSize     int
	DirectedEditBatchSize int
	MessageOverrides      prompts.Overrides
}

type localConfigFile struct {
	Mode                  string                      `yaml:"mode"`
	Submode               string                      `yaml:"submode"`
	Approval              string                      `yaml:"approval"`
	Guidance              string                      `yaml:"guidance"`
	GuidanceFile          string                      `yaml:"guidance_file"`
	EditInstructions      string                      `yaml:"edit_instructions"`
	EditInstructionsFile  string                      `yaml:"edit_instructions_file"`
	MaxEdits              *int                        `yaml:"max_edits"`
	BaseURL               string                      `yaml:"base_url"`
	Model                 string                      `yaml:"model"`
	GenerationModel       string                      `yaml:"generation_model"`
	EditingModel          string                      `yaml:"editing_model"`
	CopyEditBatchSize     *int                        `yaml:"copy_edit_batch_size"`
	DirectedEditBatchSize *int                        `yaml:"directed_edit_batch_size"`
	MessageOverrides      map[string]prompts.Override `yaml:",inline"`
}

type stringListFlag []string

func (values *stringListFlag) String() string { return strings.Join(*values, ",") }
func (values *stringListFlag) Set(value string) error {
	*values = append(*values, value)
	return nil
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
	var generationModelFlag string
	var editingModelFlag string
	var copyEditBatchSizeFlag int
	var directedEditBatchSizeFlag int
	var maxEditsFlag int
	var modeFlag string
	var submodeFlag string
	var approvalFlag string
	var guidanceFlag string
	var guidanceFileFlag string
	var editInstructionsFlag string
	var editInstructionsFileFlag string
	var promptFiles stringListFlag
	var promptAppendFiles stringListFlag
	fs := flag.NewFlagSet("goauthorllm", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&cfg.FilePath, "file", firstNonEmpty(os.Getenv("GOAUTHORLLM_FILE")), "markdown document path")
	fs.BoolVar(&cfg.NonInteractive, "non-interactive", false, "run once without launching the TUI")
	fs.StringVar(&modeFlag, "mode", "", "operation mode: generate or edit")
	fs.StringVar(&submodeFlag, "submode", "", "generate: continue or new-section; edit: copy or directed")
	fs.StringVar(&approvalFlag, "approval", "", "non-interactive edit approval: approve-all or llm-approved")
	fs.StringVar(&guidanceFlag, "guidance", "", "optional generation guidance")
	fs.StringVar(&guidanceFileFlag, "guidance-file", "", "path containing generation guidance")
	fs.StringVar(&editInstructionsFlag, "edit-instructions", "", "instructions for directed editing")
	fs.StringVar(&editInstructionsFileFlag, "edit-instructions-file", "", "path containing directed edit instructions")
	fs.IntVar(&maxEditsFlag, "max-edits", 0, "maximum edits to apply in this run (0 is unlimited)")
	fs.Var(&promptFiles, "prompt-file", "replace a prompt from NAME=PATH (repeatable)")
	fs.Var(&promptAppendFiles, "prompt-append-file", "append to a prompt from NAME=PATH (repeatable)")
	fs.StringVar(&baseURLFlag, "base-url", "", "OpenAI-compatible base URL")
	fs.StringVar(&modelFlag, "model", "", "LLM model name")
	fs.StringVar(&generationModelFlag, "generation-model", "", "model for generation requests (defaults to --model)")
	fs.StringVar(&editingModelFlag, "editing-model", "", "model for editing requests (defaults to --model)")
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
	cfg.CopyEditBatchSize, err = resolvePositiveIntValue(providedFlags["copy-edit-batch-size"], copyEditBatchSizeFlag, os.Getenv("GOAUTHORLLM_COPY_EDIT_BATCH_SIZE"), localCfg.CopyEditBatchSize, DefaultCopyEditBatchSize, "copy edit batch size")
	if err != nil {
		return Config{}, err
	}
	cfg.DirectedEditBatchSize, err = resolvePositiveIntValue(providedFlags["directed-edit-batch-size"], directedEditBatchSizeFlag, os.Getenv("GOAUTHORLLM_DIRECTED_EDIT_BATCH_SIZE"), localCfg.DirectedEditBatchSize, DefaultDirectedEditBatchSize, "directed edit batch size")
	if err != nil {
		return Config{}, err
	}
	cfg.MaxEdits, err = resolveNonNegativeIntValue(providedFlags["max-edits"], maxEditsFlag, os.Getenv("GOAUTHORLLM_MAX_EDITS"), localCfg.MaxEdits, 0, "maximum edits")
	if err != nil {
		return Config{}, err
	}
	cfg.Mode = resolveStringValue(providedFlags["mode"], modeFlag, os.Getenv("GOAUTHORLLM_MODE"), localCfg.Mode)
	cfg.Submode = resolveStringValue(providedFlags["submode"], submodeFlag, os.Getenv("GOAUTHORLLM_SUBMODE"), localCfg.Submode)
	cfg.Approval = resolveStringValue(providedFlags["approval"], approvalFlag, os.Getenv("GOAUTHORLLM_APPROVAL"), localCfg.Approval)
	cfg.Guidance, err = resolveTextValue(providedFlags["guidance"], guidanceFlag, providedFlags["guidance-file"], guidanceFileFlag, os.Getenv("GOAUTHORLLM_GUIDANCE"), os.Getenv("GOAUTHORLLM_GUIDANCE_FILE"), localCfg.Guidance, localCfg.GuidanceFile, "generation guidance")
	if err != nil {
		return Config{}, err
	}
	cfg.EditInstructions, err = resolveTextValue(providedFlags["edit-instructions"], editInstructionsFlag, providedFlags["edit-instructions-file"], editInstructionsFileFlag, os.Getenv("GOAUTHORLLM_EDIT_INSTRUCTIONS"), os.Getenv("GOAUTHORLLM_EDIT_INSTRUCTIONS_FILE"), localCfg.EditInstructions, localCfg.EditInstructionsFile, "edit instructions")
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
	raw.GenerationModel = strings.TrimSpace(raw.GenerationModel)
	raw.EditingModel = strings.TrimSpace(raw.EditingModel)
	raw.Mode = strings.TrimSpace(raw.Mode)
	raw.Submode = strings.TrimSpace(raw.Submode)
	raw.Approval = strings.TrimSpace(raw.Approval)

	return raw, nil
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
		if override != (prompts.Override{}) {
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
		if submode != "continue" && submode != "new-section" {
			return fmt.Errorf("generation --submode must be continue or new-section")
		}
	case "edit":
		if submode != "copy" && submode != "directed" {
			return fmt.Errorf("editing --submode must be copy or directed")
		}
		if approval != "approve-all" && approval != "llm-approved" {
			return fmt.Errorf("editing --approval must be approve-all or llm-approved; manual approval is not available in non-interactive mode")
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

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
