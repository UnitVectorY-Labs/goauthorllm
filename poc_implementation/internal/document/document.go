package document

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

var headingPattern = regexp.MustCompile(`^\s{0,3}#{1,6}\s+\S`)

type Document struct {
	Path          string
	FrontMatter   string
	SystemMessage string
	Body          string
	Dirty         bool
	LastSavedAt   time.Time
}

type Section struct {
	Heading  string
	Content  string
	Markdown string
}

func Load(path string) (*Document, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Document{Path: path}, nil
		}
		return nil, err
	}

	frontMatter, systemMessage, body := parse(string(content))
	info, statErr := os.Stat(path)

	doc := &Document{
		Path:          path,
		FrontMatter:   frontMatter,
		SystemMessage: systemMessage,
		Body:          body,
		Dirty:         false,
	}

	if statErr == nil {
		doc.LastSavedAt = info.ModTime()
	}

	return doc, nil
}

func (d *Document) SetBody(body string) {
	if d.Body == body {
		return
	}
	d.Body = body
	d.Dirty = true
}

func (d *Document) SetFrontMatter(frontMatter string) {
	normalized := strings.Trim(frontMatter, "\n")
	if d.FrontMatter == normalized {
		return
	}
	d.FrontMatter = normalized
	d.SystemMessage = extractSystemMessage(normalized)
	d.Dirty = true
}

func (d *Document) Save() error {
	if err := os.MkdirAll(filepath.Dir(d.Path), 0o755); err != nil {
		return err
	}

	content := render(d.FrontMatter, d.Body)
	if err := os.WriteFile(d.Path, []byte(content), 0o644); err != nil {
		return err
	}

	d.Dirty = false
	d.LastSavedAt = time.Now()
	return nil
}

func SplitSections(body string) []Section {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	if strings.TrimSpace(body) == "" {
		return nil
	}

	lines := strings.Split(body, "\n")
	var (
		sections []Section
		current  []string
		heading  string
	)

	flush := func() {
		markdown := strings.Trim(strings.Join(current, "\n"), "\n")
		if markdown == "" {
			current = nil
			heading = ""
			return
		}

		contentLines := current
		if heading != "" && len(current) > 1 {
			contentLines = current[1:]
		}
		if heading != "" && len(current) == 1 {
			contentLines = nil
		}

		sections = append(sections, Section{
			Heading:  heading,
			Content:  strings.Trim(strings.Join(contentLines, "\n"), "\n"),
			Markdown: markdown,
		})
		current = nil
		heading = ""
	}

	for _, line := range lines {
		if headingPattern.MatchString(line) {
			if len(current) > 0 {
				flush()
			}
			heading = strings.TrimSpace(line)
			current = []string{line}
			continue
		}

		current = append(current, line)
	}

	if len(current) > 0 {
		flush()
	}

	return sections
}

func ListMarkdownFiles(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	files := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if ext == ".md" || ext == ".markdown" {
			files = append(files, name)
		}
	}

	sort.Strings(files)
	return files, nil
}

func NormalizeMarkdownFilename(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}
	if filepath.Ext(name) == "" {
		name += ".md"
	}
	return filepath.Base(name)
}

func AppendContinuation(body, addition string) string {
	addition = strings.TrimLeft(addition, "\n")
	if strings.TrimSpace(addition) == "" {
		return body
	}
	if strings.TrimSpace(body) == "" {
		return addition
	}
	if strings.HasSuffix(body, "\n") {
		return body + addition
	}
	return body + "\n" + addition
}

func AppendNewSection(body, addition string) string {
	addition = strings.Trim(addition, "\n")
	if strings.TrimSpace(addition) == "" {
		return body
	}
	if strings.TrimSpace(body) == "" {
		return addition
	}
	if strings.HasSuffix(body, "\n\n") {
		return body + addition
	}
	if strings.HasSuffix(body, "\n") {
		return body + "\n" + addition
	}
	return body + "\n\n" + addition
}

func MatchCount(body string, needle string) int {
	if needle == "" {
		return 0
	}
	return strings.Count(body, needle)
}

func ReplaceUnique(body string, oldText string, newText string) (string, bool) {
	if MatchCount(body, oldText) != 1 {
		return body, false
	}

	index := strings.Index(body, oldText)
	if index == -1 {
		return body, false
	}

	return body[:index] + newText + body[index+len(oldText):], true
}

func parse(content string) (frontMatter string, systemMessage string, body string) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(content, "---\n") {
		return "", "", content
	}

	rest := strings.TrimPrefix(content, "---\n")
	end := strings.Index(rest, "\n---\n")
	if end == -1 {
		return "", "", content
	}

	frontMatter = strings.TrimRight(rest[:end], "\n")
	body = strings.TrimLeft(rest[end+5:], "\n")

	systemMessage = extractSystemMessage(frontMatter)

	return frontMatter, systemMessage, body
}

func render(frontMatter string, body string) string {
	body = strings.TrimLeft(body, "\n")
	if strings.TrimSpace(frontMatter) == "" {
		if body == "" {
			return ""
		}
		if strings.HasSuffix(body, "\n") {
			return body
		}
		return body + "\n"
	}

	rendered := "---\n" + strings.TrimRight(frontMatter, "\n") + "\n---\n"
	if body != "" {
		rendered += "\n" + body
		if !strings.HasSuffix(rendered, "\n") {
			rendered += "\n"
		}
	}
	return rendered
}

func extractSystemMessage(frontMatter string) string {
	var metadata struct {
		SystemMessage string `yaml:"system_message"`
	}
	if err := yaml.Unmarshal([]byte(frontMatter), &metadata); err == nil {
		return strings.TrimSpace(metadata.SystemMessage)
	}
	return ""
}
