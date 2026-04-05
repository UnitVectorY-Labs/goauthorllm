package prompts

import (
	"bytes"
	"embed"
	"fmt"
	"strings"
	"text/template"
)

type Name string

const (
	GeneratePrompt       Name = "generate_prompt"
	EditPrompt           Name = "edit_prompt"
	SectionContextPrompt Name = "section_context_prompt"
	ContinuePrompt       Name = "continue_prompt"
	NewSectionPrompt     Name = "new_section_prompt"
	UserGuidancePrompt   Name = "user_guidance_prompt"
	EditTaskPrompt       Name = "edit_task_prompt"
	EditHistoryPrompt    Name = "edit_history_prompt"
	EditFeedbackPrompt   Name = "edit_feedback_prompt"
)

type Override struct {
	Replace string `yaml:"replace"`
	Append  string `yaml:"append"`
}

type Overrides map[Name]Override

//go:embed assets/*.txt
var assetFS embed.FS

func Base(name Name) (string, error) {
	path := "assets/" + string(name) + ".txt"
	data, err := assetFS.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("unknown prompt name %q", name)
	}
	return strings.TrimSpace(string(data)), nil
}

func Build(name Name, overrides Overrides) (string, error) {
	base, err := Base(name)
	if err != nil {
		return "", err
	}
	if overrides == nil {
		return Apply(base, Override{}), nil
	}
	return Apply(base, overrides[name]), nil
}

func Render(name Name, overrides Overrides, data any) (string, error) {
	source, err := Build(name, overrides)
	if err != nil {
		return "", err
	}

	tmpl, err := template.New(string(name)).Option("missingkey=error").Parse(source)
	if err != nil {
		return "", fmt.Errorf("parse prompt %q: %w", name, err)
	}

	var out bytes.Buffer
	if err := tmpl.Execute(&out, data); err != nil {
		return "", fmt.Errorf("render prompt %q: %w", name, err)
	}
	return strings.TrimSpace(out.String()), nil
}

func Apply(base string, override Override) string {
	base = strings.TrimSpace(base)
	replace := strings.TrimSpace(override.Replace)
	appendText := strings.TrimSpace(override.Append)

	if replace != "" {
		if appendText == "" {
			return replace
		}
		return replace + "\n\n" + appendText
	}

	if appendText == "" {
		return base
	}
	if base == "" {
		return appendText
	}

	return base + "\n\n" + appendText
}
