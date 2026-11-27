package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	ta "github.com/charmbracelet/bubbles/textarea"
	ti "github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/UnitVectorY-Labs/goauthorllm/internal/config"
	"github.com/UnitVectorY-Labs/goauthorllm/internal/document"
	"github.com/UnitVectorY-Labs/goauthorllm/internal/engine"
	"github.com/UnitVectorY-Labs/goauthorllm/internal/history"
	"github.com/UnitVectorY-Labs/goauthorllm/internal/llm"
)

const (
	autosaveInterval  = 2 * time.Second
	autosaveIdleDelay = 1500 * time.Millisecond
)

// workspaceView identifies the four primary studio views.
type workspaceView int

const (
	viewWrite workspaceView = iota
	viewEdit
	viewConfig
	viewFiles
)

// overlayKind identifies optional modal overlays.
type overlayKind int

const (
	overlayNone overlayKind = iota
	overlayMenu
	overlayMeta
	overlayHelp
	overlayError
	overlayModelPicker
)

// writeFocus distinguishes focus inside the Write view.
type writeFocus int

const (
	writeFocusEditor writeFocus = iota
	writeFocusPrompt
	writeFocusControls
)

// editSubTab switches between Diff Review, Session Log, and Document in Edit view.
type editSubTab int

const (
	editTabDiff editSubTab = iota
	editTabLog
	editTabDocument
)

// editFocus distinguishes focus inside the Edit view.
type editFocus int

const (
	editFocusReview editFocus = iota
	editFocusInstructions
	editFocusDocument
)

// filesFocus distinguishes focus inside the Files view.
type filesFocus int

const (
	filesFocusList filesFocus = iota
	filesFocusInput
)

// instructionRecall browses saved SQLite prompt history per document.
type instructionRecall struct {
	path    string
	kind    string
	draft   string
	entries []string
	index   int
}

// generationState tracks single- and multi-round streaming generation + pacing.
type generationState struct {
	scope            engine.Scope
	rounds           int
	active           bool
	planning         bool
	runID            int
	currentRound     int
	totalRounds      int
	plan             []engine.PlanStep
	currentIntent    string
	currentJoinMode  document.JoinMode
	roundStarted     bool
	roundPrefixAdded bool
	roundAddedChars  int
	lastGuidance     string
	ch               <-chan llm.StreamEvent
	cancel           context.CancelFunc
	pacer            streamPacer
}

// editSessionState tracks copy-edit and directed-edit sessions.
type editSessionState struct {
	subTab        editSubTab
	focus         editFocus
	instructions  ta.Model
	requesting    bool
	approveAll    bool
	requestID     int
	cancel        context.CancelFunc
	current       *document.ResolvedEdit
	pending       []document.ResolvedEdit
	batchTotal    int
	batchPos      int
	moreRounds    bool
	assessment    string
	history       []engine.EditHistoryEntry
	logIndex      int
	reviewScroll  int
	appliedStacks []string // document snapshots for one-key undo in edit mode
}

// filesState tracks the markdown file browser.
type filesState struct {
	files    []string
	selected int
	scroll   int
	focus    filesFocus
	input    ti.Model
}

// Model is the top-level Bubble Tea model for the reimagined goauthorllm studio.
type Model struct {
	cfg              config.Config
	generationClient *llm.Client
	editingClient    *llm.Client
	cwd              string

	view    workspaceView
	overlay overlayKind

	doc         *document.Document
	editor      ta.Model
	frontMatter ta.Model
	prompt      ta.Model
	writeFocus  writeFocus
	promptOpen  bool // controls prompt expansion on compact heights

	gen    generationState
	edit   editSessionState
	cfgUI  configEditorState
	files  filesState

	undoBodies     []string
	lastUndoEditAt time.Time
	lastEditAt     time.Time
	recall         instructionRecall

	spin        spinner.Model
	width       int
	height      int
	hits        hitMap
	hoverID     string
	hoverIndex  int
	statusText  string
	statusLevel string
	lastError   string
	errorScroll int
	helpScroll  int
	menuIndex   int

	// Model picker modal state
	modelPickerTarget int // which config field receives the picked model
	modelPickerItems  []string
	modelPickerIndex  int
	modelPickerBusy   bool
	modelPickerFilter ti.Model
}

// NewModel initializes the studio Model with an optional starting document.
func NewModel(cfg config.Config, client *llm.Client) (Model, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return Model{}, err
	}

	spin := spinner.New()
	spin.Spinner = spinner.Dot
	spin.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(colorOrange))

	m := Model{
		cfg:         cfg,
		cwd:         cwd,
		spin:        spin,
		hits:        newHitMap(),
		hoverIndex:  -1,
		lastEditAt:  time.Now(),
		statusText:  "Ready",
		statusLevel: "info",
		promptOpen:  true,
		gen: generationState{
			scope:  engine.NormalizeScope(cfg.DefaultScope),
			rounds: max(1, cfg.Rounds),
		},
		recall: instructionRecall{index: -1},
	}

	if client != nil {
		m.generationClient = client
		m.editingClient = client
		if cfg.GenerationModel != "" && cfg.GenerationModel != client.Model() {
			m.generationClient = llm.NewClient(cfg.BaseURL, cfg.GenerationModel, cfg.APIKey, cfg.Timeout, cfg.UserAgent)
		}
		if cfg.EditingModel != "" && cfg.EditingModel != client.Model() {
			m.editingClient = llm.NewClient(cfg.BaseURL, cfg.EditingModel, cfg.APIKey, cfg.Timeout, cfg.UserAgent)
		}
	} else {
		m.rebuildClients()
	}

	m.editor = newStyledTextarea("Start writing markdown here, or press Ctrl+G to generate...", true)
	m.frontMatter = newStyledTextarea("title: My Document\nsystem_message: |\n  Document-specific guidance for the LLM...", false)
	m.prompt = newStyledTextarea("Optional guidance (leave blank to continue naturally) · Enter generates · Alt+Enter newline · Alt+↑/↓ history", false)
	m.edit.instructions = newStyledTextarea("Optional custom edit instructions (blank = copy edit using .goauthorllm rules) · Enter starts · Alt+↑/↓ history", false)

	m.files.input = newStyledTextInput("new-draft.md")
	m.modelPickerFilter = newStyledTextInput("Filter models...")

	m.initConfigEditor()

	if err := m.refreshFiles(); err != nil {
		return Model{}, err
	}

	if strings.TrimSpace(cfg.FilePath) != "" {
		path := m.resolvePath(cfg.FilePath)
		if err := m.openDocument(path); err != nil {
			return Model{}, err
		}
		m.view = viewWrite
		m.writeFocus = writeFocusEditor
		m.setStatus("Opened "+filepath.Base(path)+" · ^G generate · ^L length · ^N rounds · ^T view · ^P menu", "success")
	} else {
		m.view = viewFiles
		m.files.focus = filesFocusList
		m.setStatus("Select a file or type a new name · ^T switches view · ^P menu & help", "info")
	}

	m.syncFocus()
	return m, nil
}

func newStyledTextarea(placeholder string, lineNumbers bool) ta.Model {
	input := ta.New()
	input.Placeholder = placeholder
	input.ShowLineNumbers = lineNumbers
	input.CharLimit = 0
	input.MaxHeight = 0
	input.Prompt = ""
	input.FocusedStyle.Base = lipgloss.NewStyle().Foreground(lipgloss.Color(colorBody))
	input.BlurredStyle.Base = lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted))
	input.FocusedStyle.CursorLine = lipgloss.NewStyle().Background(lipgloss.Color("#161E2E"))
	input.BlurredStyle.CursorLine = lipgloss.NewStyle()
	input.FocusedStyle.LineNumber = lipgloss.NewStyle().Foreground(lipgloss.Color(colorDim))
	input.BlurredStyle.LineNumber = lipgloss.NewStyle().Foreground(lipgloss.Color("#475569"))
	input.FocusedStyle.CursorLineNumber = lipgloss.NewStyle().Foreground(lipgloss.Color(colorOrange)).Bold(true)
	input.BlurredStyle.CursorLineNumber = lipgloss.NewStyle().Foreground(lipgloss.Color(colorMuted))
	input.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color(colorDim))
	input.BlurredStyle.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color("#475569"))
	return input
}

func newStyledTextInput(placeholder string) ti.Model {
	in := ti.New()
	in.Placeholder = placeholder
	in.Prompt = ""
	in.CharLimit = 256
	in.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorText))
	in.PlaceholderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorDim))
	in.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(colorOrange))
	return in
}

func (m *Model) rebuildClients() {
	genModel := m.cfg.GenerationModel
	if strings.TrimSpace(genModel) == "" {
		genModel = m.cfg.Model
	}
	editModel := m.cfg.EditingModel
	if strings.TrimSpace(editModel) == "" {
		editModel = m.cfg.Model
	}
	m.generationClient = llm.NewClient(m.cfg.BaseURL, genModel, m.cfg.APIKey, m.cfg.Timeout, m.cfg.UserAgent)
	if editModel == genModel {
		m.editingClient = m.generationClient
	} else {
		m.editingClient = llm.NewClient(m.cfg.BaseURL, editModel, m.cfg.APIKey, m.cfg.Timeout, m.cfg.UserAgent)
	}
}

// Init starts cursor blink and autosave tick.
func (m Model) Init() tea.Cmd {
	return tea.Batch(ta.Blink, ti.Blink, autosaveTick())
}

// Update handles all Bubble Tea messages.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resizeInputs()
		return m, nil

	case autosaveTickMsg:
		if m.doc != nil && m.doc.Dirty && !m.busy() && time.Since(m.lastEditAt) >= autosaveIdleDelay {
			if err := m.saveCurrentDocument(); err != nil {
				m.setStatus("Autosave failed: "+err.Error(), "error")
			} else {
				m.setStatus("Autosaved "+m.doc.LastSavedAt.Format("15:04:05"), "success")
			}
		}
		return m, autosaveTick()

	case spinner.TickMsg:
		if !m.busy() && !m.modelPickerBusy {
			return m, nil
		}
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd

	case planDoneMsg:
		return m.handlePlanDone(msg)

	case streamChunkMsg:
		return m.handleStreamChunk(msg)

	case pacerTickMsg:
		return m.handlePacerTick(msg)

	case editBatchMsg:
		return m.handleEditBatchMsg(msg)

	case modelsFetchedMsg:
		return m.handleModelsFetched(msg)

	case tea.MouseMsg:
		if handled, cmd := m.handleMouse(msg); handled {
			return m, cmd
		}

	case tea.KeyMsg:
		if handled, cmd := m.handleKey(msg); handled {
			return m, cmd
		}
	}

	return m.delegateInput(msg)
}

func (m *Model) busy() bool {
	return m.gen.active || m.gen.planning || m.edit.requesting
}

func (m *Model) setStatus(text, level string) {
	m.statusText = text
	m.statusLevel = level
	if level == "error" {
		m.lastError = text
		m.errorScroll = 0
	}
}

func (m *Model) resolvePath(name string) string {
	if filepath.IsAbs(name) {
		return filepath.Clean(name)
	}
	return filepath.Join(m.cwd, name)
}

func (m *Model) openDocument(path string) error {
	doc, err := document.Load(path)
	if err != nil {
		return err
	}
	if m.doc == nil || m.doc.Path != doc.Path {
		m.undoBodies = nil
		m.edit.history = nil
		m.edit.current = nil
		m.edit.pending = nil
		m.edit.appliedStacks = nil
		m.edit.logIndex = 0
		m.recall = instructionRecall{index: -1}
	}
	m.doc = doc
	m.frontMatter.SetValue(doc.FrontMatter)
	m.editor.SetValue(doc.Body)
	m.editor.CursorEnd()
	m.lastEditAt = time.Now()
	return nil
}

func (m *Model) saveCurrentDocument() error {
	if m.doc == nil {
		return nil
	}
	m.doc.SetFrontMatter(m.frontMatter.Value())
	m.doc.SetBody(m.editor.Value())
	return m.doc.Save()
}

func (m *Model) saveBeforeLeave() error {
	if m.doc == nil || !m.doc.Dirty {
		return nil
	}
	return m.saveCurrentDocument()
}

func (m *Model) refreshFiles() error {
	files, err := document.ListMarkdownFiles(m.cwd)
	if err != nil {
		return err
	}
	m.files.files = files
	if len(files) == 0 {
		m.files.selected = 0
	} else if m.files.selected >= len(files) {
		m.files.selected = len(files) - 1
	}
	return nil
}

func (m *Model) activeModelName() string {
	if m.view == viewEdit && m.cfg.EditingModel != "" {
		return m.cfg.EditingModel
	}
	if m.view == viewWrite && m.cfg.GenerationModel != "" {
		return m.cfg.GenerationModel
	}
	return m.cfg.Model
}

func (m *Model) switchView(v workspaceView) {
	if m.busy() {
		m.setStatus("Stop the active request (Esc) before switching views", "warn")
		return
	}
	if (v == viewWrite || v == viewEdit) && m.doc == nil {
		m.view = viewFiles
		m.setStatus("Open or create a markdown file first", "warn")
		m.syncFocus()
		return
	}
	if v == viewConfig {
		m.loadConfigIntoEditor()
	}
	if v == viewFiles {
		_ = m.refreshFiles()
	}
	m.view = v
	m.overlay = overlayNone
	m.resizeInputs()
	m.syncFocus()
}

func (m *Model) syncFocus() {
	m.editor.Blur()
	m.frontMatter.Blur()
	m.prompt.Blur()
	m.edit.instructions.Blur()
	m.files.input.Blur()
	m.modelPickerFilter.Blur()
	m.blurConfigInputs()

	switch m.overlay {
	case overlayMeta:
		m.frontMatter.Focus()
		return
	case overlayModelPicker:
		m.modelPickerFilter.Focus()
		return
	case overlayHelp, overlayError, overlayMenu:
		return
	}

	switch m.view {
	case viewWrite:
		switch m.writeFocus {
		case writeFocusPrompt:
			m.prompt.Focus()
		case writeFocusEditor:
			m.editor.Focus()
		}
	case viewEdit:
		switch m.edit.focus {
		case editFocusInstructions:
			m.edit.instructions.Focus()
		case editFocusDocument:
			m.editor.Focus()
		}
	case viewConfig:
		m.focusActiveConfigField()
	case viewFiles:
		if m.files.focus == filesFocusInput {
			m.files.input.Focus()
		}
	}
}

// --- Undo, Delete-to-End, Instruction History ---

func (m *Model) pushUndo(body string) {
	if len(m.undoBodies) > 0 && m.undoBodies[len(m.undoBodies)-1] == body {
		return
	}
	m.undoBodies = append(m.undoBodies, body)
	if len(m.undoBodies) > 100 {
		m.undoBodies = append([]string(nil), m.undoBodies[len(m.undoBodies)-100:]...)
	}
	m.lastUndoEditAt = time.Time{}
}

func (m *Model) undoBody() {
	if m.doc == nil || len(m.undoBodies) == 0 {
		m.setStatus("Nothing to undo", "info")
		return
	}
	last := len(m.undoBodies) - 1
	restored := m.undoBodies[last]
	m.undoBodies = m.undoBodies[:last]
	m.doc.SetBody(restored)
	m.editor.SetValue(restored)
	m.editor, _ = m.editor.Update(tea.KeyMsg{Type: tea.KeyCtrlEnd})
	m.lastEditAt = time.Now()
	m.lastUndoEditAt = time.Time{}
	m.setStatus("Undid last change", "success")
}

func (m *Model) deleteToEnd() {
	if m.doc == nil {
		return
	}
	body := m.editor.Value()
	lines := strings.Split(body, "\n")
	line := clampInt(m.editor.Line(), 0, max(0, len(lines)-1))
	info := m.editor.LineInfo()
	col := info.StartColumn + info.ColumnOffset
	runes := []rune(lines[line])
	if col > len(runes) {
		col = len(runes)
	}
	lines[line] = string(runes[:col])
	updated := strings.Join(lines[:line+1], "\n")
	if updated == body {
		return
	}
	m.pushUndo(body)
	m.editor.SetValue(updated)
	m.editor, _ = m.editor.Update(tea.KeyMsg{Type: tea.KeyCtrlEnd})
	m.doc.SetBody(updated)
	m.lastEditAt = time.Now()
	m.setStatus("Deleted from cursor to end · Ctrl+Z restores", "success")
}

func (m *Model) persistInstruction(kind, value string) {
	if m.doc == nil || strings.TrimSpace(value) == "" {
		return
	}
	if err := history.Save(m.doc.Path, kind, strings.TrimSpace(value)); err != nil {
		m.setStatus("Could not save prompt history: "+err.Error(), "warn")
		return
	}
	m.recall = instructionRecall{index: -1}
}

func (m *Model) recallInstruction(kind string, input *ta.Model, older bool) {
	if m.doc == nil {
		return
	}
	r := &m.recall
	if r.path != m.doc.Path || r.kind != kind || r.entries == nil || (r.index >= 0 && input.Value() != r.entries[r.index]) {
		entries, err := history.Load(m.doc.Path, kind)
		if err != nil {
			m.setStatus("Could not load history: "+err.Error(), "error")
			return
		}
		*r = instructionRecall{
			path:    m.doc.Path,
			kind:    kind,
			draft:   input.Value(),
			entries: entries,
			index:   -1,
		}
	}
	if len(r.entries) == 0 {
		m.setStatus("No saved prompts for this file yet", "info")
		return
	}
	if r.index < 0 {
		r.draft = input.Value()
	}
	if older {
		r.index = min(r.index+1, len(r.entries)-1)
	} else {
		r.index = max(-1, r.index-1)
	}
	if r.index < 0 {
		input.SetValue(r.draft)
		m.setStatus("Restored current draft", "info")
	} else {
		input.SetValue(r.entries[r.index])
		m.setStatus(fmt.Sprintf("History %d/%d · Alt+↑/↓ browses", r.index+1, len(r.entries)), "info")
	}
	input.CursorEnd()
}

type autosaveTickMsg time.Time

func autosaveTick() tea.Cmd {
	return tea.Tick(autosaveInterval, func(t time.Time) tea.Msg {
		return autosaveTickMsg(t)
	})
}

func (m *Model) wordCount() int {
	if m.doc == nil {
		return 0
	}
	return len(strings.Fields(m.editor.Value()))
}
