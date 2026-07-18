package app

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	ta "github.com/charmbracelet/bubbles/textarea"
	ti "github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/UnitVectorY-Labs/goauthorllm/internal/config"
	"github.com/UnitVectorY-Labs/goauthorllm/internal/diff"
	"github.com/UnitVectorY-Labs/goauthorllm/internal/document"
	"github.com/UnitVectorY-Labs/goauthorllm/internal/llm"
	"github.com/UnitVectorY-Labs/goauthorllm/internal/prompts"
)

const autosaveInterval = 2 * time.Second
const autosaveIdleDelay = 1500 * time.Millisecond

const (
	colorOrange      = "#F97316"
	colorOrangeDark  = "#9A3412"
	colorOrangeLight = "#FFEDD5"
	colorBlue        = "#2563EB"
	colorBlueDark    = "#1E3A8A"
	colorBlueLight   = "#DBEAFE"
)

type generationMode int

const (
	modeContinue generationMode = iota
	modeNewSection
)

type workspaceMode int

const (
	workspaceGenerate workspaceMode = iota
	workspaceEdit
	workspaceDocument
)

type screenState int

const (
	screenChooser screenState = iota
	screenModePicker
	screenEditOptions
	screenApprovalPicker
	screenWorkspace
)

type editKind int

const (
	editKindCopy editKind = iota
	editKindDirected
)

type approvalMode int

const (
	approvalManual approvalMode = iota
	approvalAutomatic
	approvalAll
)

type focusTarget int

const (
	focusChooserList focusTarget = iota
	focusChooserInput
	focusModeGenerate
	focusModeEdit
	focusModeDocument
	focusEditDefault
	focusEditCustom
	focusEditInstructions
	focusApprovalManual
	focusApprovalAutomatic
	focusApprovalAll
	focusEditNext
	focusWorkspaceTabs
	focusHistoryPane
	focusFrontMatter
	focusEditor
	focusPrompt
	focusContinueButton
	focusNewSectionButton
	focusAcceptButton
	focusSkipButton
	focusRefreshButton
	focusSaveButton
	focusFilesButton
	focusMessageButton
	focusQuitButton
)

type buttonAction string

const (
	actionContinue           buttonAction = "continue"
	actionNewSection         buttonAction = "new-section"
	actionAcceptSuggestion   buttonAction = "accept-suggestion"
	actionSkipSuggestion     buttonAction = "skip-suggestion"
	actionRefreshSuggestion  buttonAction = "refresh-suggestion"
	actionSave               buttonAction = "save"
	actionFiles              buttonAction = "files"
	actionToggleMessage      buttonAction = "toggle-message"
	actionQuit               buttonAction = "quit"
	actionChooseSelected     buttonAction = "choose-selected"
	actionChooseTyped        buttonAction = "choose-typed"
	actionPickGenerate       buttonAction = "pick-generate"
	actionPickEdit           buttonAction = "pick-edit"
	actionPickDocument       buttonAction = "pick-document"
	actionPickDefaultEditor  buttonAction = "pick-default-editor"
	actionPickCustomEditor   buttonAction = "pick-custom-editor"
	actionEditOptionsNext    buttonAction = "edit-options-next"
	actionPickManualApproval buttonAction = "pick-manual-approval"
	actionPickAutoApproval   buttonAction = "pick-auto-approval"
	actionPickAllApproval    buttonAction = "pick-all-approval"
	actionShowHistory        buttonAction = "show-history"
	actionCancelBusy         buttonAction = "cancel-busy"
	actionRefreshFiles       buttonAction = "refresh-files"
	actionBack               buttonAction = "back"
)

type keyMap struct {
	focusNext  key.Binding
	focusPrev  key.Binding
	save       key.Binding
	continueOp key.Binding
	newSection key.Binding
	accept     key.Binding
	skip       key.Binding
	refresh    key.Binding
	files      key.Binding
	selectItem key.Binding
	back       key.Binding
	quit       key.Binding
	moveUp     key.Binding
	moveDown   key.Binding
	history    key.Binding
	message    key.Binding
}

type rect struct {
	x1, y1, x2, y2 int
}

func (r rect) contains(x, y int) bool {
	return x >= r.x1 && x <= r.x2 && y >= r.y1 && y <= r.y2
}

type buttonRegion struct {
	Action buttonAction
	Rect   rect
}

type fileRegion struct {
	Index int
	Rect  rect
}

type choiceRegion struct {
	Index int
	Rect  rect
}

type layoutState struct {
	frontMatter      rect
	editor           rect
	prompt           rect
	modeGenerate     rect
	modeEdit         rect
	modeDocument     rect
	editDefault      rect
	editCustom       rect
	editInstructions rect
	approvalManual   rect
	approvalAuto     rect
	approvalAll      rect
	buttons          []buttonRegion
	files            []fileRegion
	chooserInput     rect
	tabs             []rect
	choices          []choiceRegion
}

type choiceItem struct {
	title       string
	description string
	value       int
}

func (i choiceItem) Title() string       { return i.title }
func (i choiceItem) Description() string { return i.description }
func (i choiceItem) FilterValue() string { return i.title }

type chooserState struct {
	files    []string
	selected int
	input    ti.Model
}

type streamMsg struct {
	id    int
	event llm.StreamEvent
}

type editSuggestion struct {
	OldText         string `json:"old_text"`
	NewText         string `json:"new_text"`
	RemainingRounds int    `json:"remaining_rounds"`
}

func (e editSuggestion) empty() bool {
	return strings.TrimSpace(e.OldText) == "" && strings.TrimSpace(e.NewText) == ""
}

type editHistoryEntry struct {
	Action  string
	OldText string
	NewText string
}

type editSuggestionResult struct {
	Suggestion *editSuggestion
	Note       string
	Err        error
}

type editMsg struct {
	id     int
	result editSuggestionResult
}

type editApprovalMsg struct {
	id       int
	approved bool
	err      error
}

type editState struct {
	suggestion    *editSuggestion
	history       []editHistoryEntry
	requesting    bool
	requestID     int
	requestCancel context.CancelFunc
	reviewing     bool
	kind          editKind
	approval      approvalMode
	instructions  ta.Model
	showHistory   bool
	historyIndex  int
}

// Model is the top-level Bubble Tea model for goauthorllm.
type Model struct {
	cfg    config.Config
	client *llm.Client
	cwd    string

	screen     screenState
	screenPath []screenState
	focus      focusTarget
	mode       workspaceMode

	pendingPath string
	pendingName string

	doc             *document.Document
	frontMatter     ta.Model
	editor          ta.Model
	prompt          ta.Model
	chooser         chooserState
	editorChoices   list.Model
	approvalChoices list.Model
	modeChoices     list.Model
	workspaceTab    int

	spin spinner.Model
	keys keyMap

	width  int
	height int
	layout layoutState

	statusText      string
	statusLevel     string
	lastEditAt      time.Time
	showFrontMatter bool
	hover           focusTarget

	generating        bool
	generationID      int
	generationMode    generationMode
	generationStarted bool
	generationCh      <-chan llm.StreamEvent
	generationCancel  context.CancelFunc

	edit editState
}

// NewModel creates a fully initialized Model ready for Bubble Tea.
func NewModel(cfg config.Config, client *llm.Client) (Model, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return Model{}, err
	}

	spin := spinner.New()
	spin.Spinner = spinner.Dot
	spin.Style = lipgloss.NewStyle().Foreground(lipgloss.Color(colorOrange))

	m := Model{
		cfg:    cfg,
		client: client,
		cwd:    cwd,
		mode:   workspaceGenerate,
		spin:   spin,
		keys: keyMap{
			focusNext:  key.NewBinding(key.WithKeys("tab")),
			focusPrev:  key.NewBinding(key.WithKeys("shift+tab")),
			save:       key.NewBinding(key.WithKeys("ctrl+s")),
			continueOp: key.NewBinding(key.WithKeys("ctrl+g")),
			newSection: key.NewBinding(key.WithKeys("ctrl+n")),
			accept:     key.NewBinding(key.WithKeys("ctrl+a")),
			skip:       key.NewBinding(key.WithKeys("ctrl+k")),
			refresh:    key.NewBinding(key.WithKeys("ctrl+r")),
			files:      key.NewBinding(key.WithKeys("ctrl+o")),
			selectItem: key.NewBinding(key.WithKeys("enter")),
			back:       key.NewBinding(key.WithKeys("esc")),
			quit:       key.NewBinding(key.WithKeys("ctrl+q")),
			moveUp:     key.NewBinding(key.WithKeys("up")),
			moveDown:   key.NewBinding(key.WithKeys("down")),
			history:    key.NewBinding(key.WithKeys("alt+h")),
			message:    key.NewBinding(key.WithKeys("alt+m")),
		},
		statusText:  "Choose a document to begin",
		statusLevel: "info",
		lastEditAt:  time.Now(),
		hover:       focusTarget(-1),
	}

	m.frontMatter = newTextarea("File metadata and document-specific instructions...", false)
	m.frontMatter.SetHeight(5)
	m.frontMatter.FocusedStyle.CursorLine = lipgloss.NewStyle()
	m.frontMatter.BlurredStyle.CursorLine = lipgloss.NewStyle()

	m.editor = newTextarea("Write or edit markdown content here...", true)
	m.editor.Focus()

	m.prompt = newTextarea("Optional guidance for the next generated addition...", false)
	m.prompt.SetHeight(5)
	m.prompt.ShowLineNumbers = false
	m.edit.instructions = newTextarea("Describe the directed edit you want the model to make...", false)
	m.edit.instructions.ShowLineNumbers = false
	m.edit.instructions.SetHeight(6)
	m.editorChoices = newChoiceList("Editor", []list.Item{
		choiceItem{title: "Copy editor", description: "Find one high-priority mechanical correction at a time.", value: int(editKindCopy)},
		choiceItem{title: "Directed editor", description: "Follow custom instructions for a rewrite, removal, or other targeted change.", value: int(editKindDirected)},
	})
	m.approvalChoices = newChoiceList("Approval", []list.Item{
		choiceItem{title: "Manual review", description: "Show every suggestion and wait for your decision.", value: int(approvalManual)},
		choiceItem{title: "Automatic review", description: "Auto-apply only suggestions that pass a second safety check.", value: int(approvalAutomatic)},
		choiceItem{title: "Approve all", description: "Apply every valid suggestion until editing is complete.", value: int(approvalAll)},
	})
	m.modeChoices = newChoiceList("Mode", []list.Item{
		choiceItem{title: "View / edit document", description: "Open the document directly for reading and manual changes.", value: int(workspaceDocument)},
		choiceItem{title: "Generate", description: "Continue the current section or generate a new section.", value: int(workspaceGenerate)},
		choiceItem{title: "Edit with AI", description: "Review copy edits or carry out directed editing instructions.", value: int(workspaceEdit)},
	})

	m.chooser.input = ti.New()
	m.chooser.input.Placeholder = "draft.md"
	m.chooser.input.Prompt = ""
	m.chooser.input.CharLimit = 128
	m.chooser.input.Cursor.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("#F8FAFC"))
	m.chooser.input.TextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#F8FAFC"))
	m.chooser.input.PlaceholderStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#64748B"))

	if err := m.refreshChooser(); err != nil {
		return Model{}, err
	}

	if cfg.FilePath != "" {
		m.pendingPath = m.resolveDocumentPath(cfg.FilePath)
		m.pendingName = filepath.Base(m.pendingPath)
		m.screen = screenModePicker
		m.screenPath = []screenState{screenChooser}
		m.focus = focusModeDocument
		m.setStatus("Choose how to work with "+m.pendingName, "info")
	} else {
		m.screen = screenChooser
		m.focus = focusChooserList
	}

	m.syncFocus()
	return m, nil
}

func newChoiceList(title string, items []list.Item) list.Model {
	delegate := list.NewDefaultDelegate()
	delegate.SetHeight(2)
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.Foreground(lipgloss.Color(colorOrange)).BorderLeftForeground(lipgloss.Color(colorOrange))
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.Foreground(lipgloss.Color(colorBlueLight)).BorderLeftForeground(lipgloss.Color(colorOrange))
	choices := list.New(items, delegate, 80, 12)
	choices.Title = title
	choices.SetShowFilter(false)
	choices.SetShowHelp(false)
	choices.SetShowPagination(false)
	choices.SetShowStatusBar(false)
	choices.DisableQuitKeybindings()
	choices.Styles.Title = choices.Styles.Title.Foreground(lipgloss.Color("#F8FAFC")).Background(lipgloss.Color(colorOrangeDark))
	return choices
}

func newTextarea(placeholder string, lineNumbers bool) ta.Model {
	input := ta.New()
	input.Placeholder = placeholder
	input.ShowLineNumbers = lineNumbers
	input.CharLimit = 0
	input.Prompt = ""
	input.FocusedStyle.Base = lipgloss.NewStyle().Foreground(lipgloss.Color("#E5E7EB"))
	input.BlurredStyle.Base = lipgloss.NewStyle().Foreground(lipgloss.Color("#CBD5E1"))
	input.FocusedStyle.CursorLine = lipgloss.NewStyle().Background(lipgloss.Color("#111827"))
	input.BlurredStyle.CursorLine = lipgloss.NewStyle().Background(lipgloss.Color("#0F172A"))
	input.FocusedStyle.LineNumber = lipgloss.NewStyle().Foreground(lipgloss.Color(colorBlue))
	input.BlurredStyle.LineNumber = lipgloss.NewStyle().Foreground(lipgloss.Color("#475569"))
	input.FocusedStyle.CursorLineNumber = lipgloss.NewStyle().Foreground(lipgloss.Color(colorOrange))
	input.BlurredStyle.CursorLineNumber = lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8"))
	input.FocusedStyle.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color("#475569"))
	input.BlurredStyle.Placeholder = lipgloss.NewStyle().Foreground(lipgloss.Color("#334155"))
	return input
}

// Init returns the initial commands for the model.
func (m Model) Init() tea.Cmd {
	return tea.Batch(ta.Blink, ti.Blink, autosaveTick())
}

// Update processes a message and returns the updated model.
func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.resize()
		return m, nil

	case autosaveTickMsg:
		if m.screen == screenWorkspace && m.doc != nil && m.doc.Dirty && !m.busy() && time.Since(m.lastEditAt) >= autosaveIdleDelay {
			if err := m.saveCurrentDocument(); err != nil {
				m.setStatus("Autosave failed: "+err.Error(), "error")
			} else {
				m.setStatus("Autosaved "+formatTimestamp(m.doc.LastSavedAt), "success")
			}
		}
		return m, autosaveTick()

	case spinner.TickMsg:
		if !m.busy() {
			return m, nil
		}
		var cmd tea.Cmd
		m.spin, cmd = m.spin.Update(msg)
		return m, cmd

	case streamMsg:
		return m.handleStreamMsg(msg)

	case editMsg:
		return m.handleEditMsg(msg)

	case editApprovalMsg:
		return m.handleEditApprovalMsg(msg)
	}

	if mouseMsg, ok := msg.(tea.MouseMsg); ok {
		if handled, cmd := m.handleMouse(mouseMsg); handled {
			return m, cmd
		}
	}

	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		if handled, cmd := m.handleKey(keyMsg); handled {
			return m, cmd
		}
	}

	switch m.screen {
	case screenChooser:
		return m.updateChooserInputs(msg)
	case screenModePicker:
		var cmd tea.Cmd
		m.modeChoices, cmd = m.modeChoices.Update(msg)
		return m, cmd
	case screenEditOptions:
		if m.focus == focusEditInstructions {
			var cmd tea.Cmd
			m.edit.instructions, cmd = m.edit.instructions.Update(msg)
			return m, cmd
		}
		var cmd tea.Cmd
		m.editorChoices, cmd = m.editorChoices.Update(msg)
		return m, cmd
	case screenApprovalPicker:
		var cmd tea.Cmd
		m.approvalChoices, cmd = m.approvalChoices.Update(msg)
		return m, cmd
		return m, nil
	case screenWorkspace:
		return m.updateWorkspaceInputs(msg)
	default:
		return m, nil
	}
}

// View renders the current UI state.
func (m *Model) View() string {
	if m.width == 0 || m.height == 0 {
		return "Loading..."
	}

	var view string
	switch {
	case m.showFrontMatter:
		view = m.renderFrontMatterModal()
	case m.screen == screenChooser:
		view = m.renderChooser()
	case m.screen == screenModePicker:
		view = m.renderModePicker()
	case m.screen == screenEditOptions:
		view = m.renderEditOptions()
	case m.screen == screenApprovalPicker:
		view = m.renderApprovalPicker()
	default:
		view = m.renderWorkspace()
	}

	// Pad or trim to exactly m.height lines so the diff-based renderer
	// never sees a height change and avoids full-screen redraws.
	lines := strings.Split(view, "\n")
	if len(lines) < m.height {
		for len(lines) < m.height {
			lines = append(lines, "")
		}
	} else if len(lines) > m.height {
		lines = lines[:m.height]
	}

	return strings.Join(lines, "\n")
}

// --- Stream handling ---

func (m *Model) handleStreamMsg(msg streamMsg) (tea.Model, tea.Cmd) {
	if msg.id != m.generationID {
		return m, nil
	}
	if msg.event.Err != nil {
		m.generating = false
		m.generationCh = nil
		m.generationCancel = nil
		if msg.event.Err == context.Canceled {
			m.setStatus("Generation canceled", "muted")
		} else if m.generationStarted {
			m.setStatus("Generation interrupted, partial text kept", "error")
		} else {
			m.setStatus("Generation failed: "+msg.event.Err.Error(), "error")
		}
		return m, nil
	}
	if msg.event.Done {
		m.generating = false
		m.generationCh = nil
		m.generationCancel = nil
		if !m.generationStarted {
			m.setStatus("Generation returned no content", "error")
			return m, nil
		}
		if err := m.saveCurrentDocument(); err != nil {
			m.setStatus("Generated content but save failed: "+err.Error(), "error")
			return m, nil
		}
		m.prompt.SetValue("")
		m.setStatus(m.generationMode.label()+" complete and saved", "success")
		return m, nil
	}
	if msg.event.Delta != "" {
		m.applyGenerationDelta(msg.event.Delta)
	}
	return m, waitForStream(m.generationCh, m.generationID)
}

func (m *Model) handleEditMsg(msg editMsg) (tea.Model, tea.Cmd) {
	if msg.id != m.edit.requestID {
		return m, nil
	}

	m.edit.requesting = false
	m.edit.requestCancel = nil

	if msg.result.Err != nil {
		if msg.result.Err == context.Canceled {
			m.setStatus("Edit suggestion canceled", "muted")
		} else {
			m.setStatus("Edit suggestion failed: "+msg.result.Err.Error(), "error")
		}
		return m, nil
	}

	m.edit.suggestion = msg.result.Suggestion
	if msg.result.Suggestion == nil {
		m.setStatus(firstNonEmptyString(msg.result.Note, "No edit suggestion available"), "muted")
		return m, nil
	}
	m.workspaceTab = 0

	if m.edit.approval == approvalAll {
		suggestion := *msg.result.Suggestion
		if !m.applySuggestion(suggestion, "approved-all") {
			m.setStatus("Automatic edit could not be applied; requesting a repaired suggestion", "error")
			return m, m.requestEditSuggestion("Repairing an automatic edit suggestion...")
		}
		if suggestion.RemainingRounds <= 0 {
			m.setStatus("Applied final automatic edit; no further rounds are needed", "success")
			return m, nil
		}
		return m, m.requestEditSuggestion("Applied automatic edit and requesting the next suggestion...")
	}

	if m.edit.approval == approvalAutomatic {
		m.setStatus("Suggestion ready; checking whether it is safe to apply automatically...", "info")
		return m, m.requestEditApproval(*msg.result.Suggestion)
	}
	m.setStatus(firstNonEmptyString(msg.result.Note, "Suggestion ready for review"), "success")
	return m, nil
}

func (m *Model) handleEditApprovalMsg(msg editApprovalMsg) (tea.Model, tea.Cmd) {
	if msg.id != m.edit.requestID {
		return m, nil
	}
	m.edit.reviewing = false
	m.edit.requestCancel = nil
	if msg.err != nil {
		if msg.err == context.Canceled {
			m.setStatus("Automatic review canceled; suggestion kept for manual review", "muted")
		} else {
			m.setStatus("Automatic review failed; suggestion kept for manual review: "+msg.err.Error(), "error")
		}
		return m, nil
	}
	if !msg.approved {
		m.setStatus("Automatic review requires your decision; suggestion remains visible", "muted")
		return m, nil
	}
	if m.edit.suggestion == nil {
		return m, nil
	}
	suggestion := *m.edit.suggestion
	if !m.applySuggestion(suggestion, "auto-accepted") {
		m.setStatus("Approved suggestion became stale; requesting a replacement", "error")
		return m, m.requestEditSuggestion("Repairing a stale edit suggestion...")
	}
	if suggestion.RemainingRounds <= 0 {
		m.switchWorkspaceTab(1)
		m.edit.historyIndex = len(m.edit.history) - 1
		m.setStatus("Applied final automatically approved edit", "success")
		return m, nil
	}
	return m, m.requestEditSuggestion("Applied approved edit and requesting the next suggestion...")
}

// --- Mouse handling ---

func (m *Model) handleMouse(msg tea.MouseMsg) (bool, tea.Cmd) {
	if msg.Action == tea.MouseActionMotion {
		return m.updateHover(msg.X, msg.Y), nil
	}
	if isWheelMouse(msg) {
		if m.screen != screenWorkspace {
			return true, nil
		}
		// The textarea uses its editing cursor as its scroll position. While a
		// generation is streaming, moving that cursor would cause a subsequent
		// delta to be inserted into the document rather than appended to it.
		if m.generating {
			return true, nil
		}
		direction := 1
		if isWheelUp(msg) {
			direction = -1
		}
		if m.layout.editor.contains(msg.X, msg.Y) {
			m.focus = focusEditor
			m.syncFocus()
			m.scrollTextArea(&m.editor, direction, 3)
			return true, nil
		}
		if m.showFrontMatter && m.layout.frontMatter.contains(msg.X, msg.Y) {
			m.focus = focusFrontMatter
			m.syncFocus()
			m.scrollTextArea(&m.frontMatter, direction, 3)
			return true, nil
		}
		if m.mode == workspaceGenerate && m.layout.prompt.contains(msg.X, msg.Y) {
			m.focus = focusPrompt
			m.syncFocus()
			m.scrollTextArea(&m.prompt, direction, 3)
			return true, nil
		}
		switch m.focus {
		case focusFrontMatter:
			m.scrollTextArea(&m.frontMatter, direction, 3)
		case focusPrompt:
			m.scrollTextArea(&m.prompt, direction, 3)
		default:
			if m.focus != focusEditor {
				m.focus = focusEditor
				m.syncFocus()
			}
			m.scrollTextArea(&m.editor, direction, 3)
		}
		return true, nil
	}

	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return false, nil
	}

	if action, ok := m.buttonAt(msg.X, msg.Y); ok {
		return true, m.runAction(action)
	}

	switch m.screen {
	case screenChooser:
		if index, ok := m.fileAt(msg.X, msg.Y); ok {
			m.chooser.selected = index
			m.focus = focusChooserList
			m.syncFocus()
			if len(m.chooser.files) > 0 {
				m.setStatus("Selected "+m.chooser.files[index], "muted")
			}
			return true, nil
		}
		if m.layout.chooserInput.contains(msg.X, msg.Y) {
			m.focus = focusChooserInput
			m.syncFocus()
			return true, nil
		}
	case screenModePicker:
		if index, ok := m.choiceAt(msg.X, msg.Y); ok {
			m.modeChoices.Select(index)
			if item, itemOK := m.modeChoices.SelectedItem().(choiceItem); itemOK {
				switch workspaceMode(item.value) {
				case workspaceDocument:
					return true, m.runAction(actionPickDocument)
				case workspaceEdit:
					return true, m.runAction(actionPickEdit)
				default:
					return true, m.runAction(actionPickGenerate)
				}
			}
		}
	case screenEditOptions:
		if m.layout.editInstructions.contains(msg.X, msg.Y) && m.edit.kind == editKindDirected {
			m.focus = focusEditInstructions
			m.syncFocus()
			return true, nil
		}
		if index, ok := m.choiceAt(msg.X, msg.Y); ok {
			m.editorChoices.Select(index)
			if item, itemOK := m.editorChoices.SelectedItem().(choiceItem); itemOK && editKind(item.value) == editKindDirected {
				return true, m.runAction(actionPickCustomEditor)
			}
			return true, m.runAction(actionPickDefaultEditor)
		}
	case screenApprovalPicker:
		if index, ok := m.choiceAt(msg.X, msg.Y); ok {
			m.approvalChoices.Select(index)
			if item, itemOK := m.approvalChoices.SelectedItem().(choiceItem); itemOK {
				switch approvalMode(item.value) {
				case approvalAutomatic:
					return true, m.runAction(actionPickAutoApproval)
				case approvalAll:
					return true, m.runAction(actionPickAllApproval)
				default:
					return true, m.runAction(actionPickManualApproval)
				}
			}
		}
	case screenWorkspace:
		for index, tab := range m.layout.tabs {
			if tab.contains(msg.X, msg.Y) {
				m.switchWorkspaceTab(index)
				return true, nil
			}
		}
		if m.layout.frontMatter.contains(msg.X, msg.Y) && m.showFrontMatter {
			m.focus = focusFrontMatter
			m.syncFocus()
			return true, nil
		}
		if m.layout.editor.contains(msg.X, msg.Y) {
			m.focus = focusEditor
			m.syncFocus()
			return true, nil
		}
		if m.mode == workspaceGenerate && m.layout.prompt.contains(msg.X, msg.Y) {
			m.focus = focusPrompt
			m.syncFocus()
			return true, nil
		}
	}

	return false, nil
}

func (m *Model) updateHover(x, y int) bool {
	next := focusTarget(-1)
	switch {
	case m.showFrontMatter && m.layout.frontMatter.contains(x, y):
		next = focusFrontMatter
	case m.screen == screenChooser && m.layout.chooserInput.contains(x, y):
		next = focusChooserInput
	case m.screen == screenEditOptions && m.edit.kind == editKindDirected && m.layout.editInstructions.contains(x, y):
		next = focusEditInstructions
	case m.screen == screenWorkspace && m.mode == workspaceDocument && m.layout.editor.contains(x, y):
		next = focusEditor
	case m.screen == screenWorkspace && m.mode == workspaceGenerate && m.workspaceTab == 0 && m.layout.editor.contains(x, y):
		next = focusEditor
	case m.screen == screenWorkspace && m.mode == workspaceGenerate && m.workspaceTab == 1 && m.layout.prompt.contains(x, y):
		next = focusPrompt
	case m.screen == screenWorkspace && m.mode == workspaceEdit && m.workspaceTab == 2 && m.layout.editor.contains(x, y):
		next = focusEditor
	}
	if next == m.hover {
		return false
	}
	m.hover = next
	return true
}

// --- Key handling ---

func (m *Model) handleKey(msg tea.KeyMsg) (bool, tea.Cmd) {
	if isTextInputFocus(m.focus) && (msg.Type == tea.KeyEnter || msg.Type == tea.KeyCtrlM) {
		return false, nil
	}

	if direction, ok := m.rawWheelDirection(msg); ok {
		if m.generating {
			return true, nil
		}
		switch m.focus {
		case focusFrontMatter:
			m.scrollTextArea(&m.frontMatter, direction, 3)
		case focusPrompt:
			m.scrollTextArea(&m.prompt, direction, 3)
		default:
			m.scrollTextArea(&m.editor, direction, 3)
		}
		return true, nil
	}

	if m.busy() && key.Matches(msg, m.keys.back) {
		m.cancelBusyRequest()
		return true, nil
	}

	if key.Matches(msg, m.keys.quit) {
		if err := m.saveBeforeLeave("quit"); err != nil {
			m.setStatus(err.Error(), "error")
			return true, nil
		}
		return true, tea.Quit
	}
	if m.screen == screenWorkspace && key.Matches(msg, m.keys.message) {
		return true, m.runAction(actionToggleMessage)
	}
	if m.screen == screenWorkspace && m.mode == workspaceEdit && key.Matches(msg, m.keys.history) {
		return true, m.runAction(actionShowHistory)
	}

	if m.showFrontMatter && key.Matches(msg, m.keys.back) {
		return true, m.runAction(actionToggleMessage)
	}

	allowBusyNavigation := m.screen == screenWorkspace && (key.Matches(msg, m.keys.focusNext) || key.Matches(msg, m.keys.focusPrev) || (m.focus == focusWorkspaceTabs && (msg.Type == tea.KeyLeft || msg.Type == tea.KeyRight)) ||
		(m.focus == focusHistoryPane && (msg.Type == tea.KeyLeft || msg.Type == tea.KeyRight || msg.Type == tea.KeyPgUp || msg.Type == tea.KeyPgDown)))
	if m.busy() && !allowBusyNavigation {
		return true, nil
	}

	if key.Matches(msg, m.keys.back) {
		return true, m.runAction(actionBack)
	}

	switch m.screen {
	case screenChooser:
		if key.Matches(msg, m.keys.refresh) {
			return true, m.runAction(actionRefreshFiles)
		}
		if key.Matches(msg, m.keys.focusNext) || key.Matches(msg, m.keys.focusPrev) {
			if m.focus == focusChooserList {
				m.focus = focusChooserInput
			} else {
				m.focus = focusChooserList
			}
			m.syncFocus()
			return true, nil
		}
		if key.Matches(msg, m.keys.moveUp) && m.focus == focusChooserList {
			if len(m.chooser.files) > 0 && m.chooser.selected > 0 {
				m.chooser.selected--
			}
			return true, nil
		}
		if key.Matches(msg, m.keys.moveDown) && m.focus == focusChooserList {
			if len(m.chooser.files) > 0 && m.chooser.selected < len(m.chooser.files)-1 {
				m.chooser.selected++
			}
			return true, nil
		}
		if key.Matches(msg, m.keys.selectItem) {
			if m.focus == focusChooserInput && strings.TrimSpace(m.chooser.input.Value()) != "" {
				return true, m.runAction(actionChooseTyped)
			}
			return true, m.runAction(actionChooseSelected)
		}
		return false, nil

	case screenModePicker:
		if key.Matches(msg, m.keys.selectItem) {
			if item, ok := m.modeChoices.SelectedItem().(choiceItem); ok {
				switch workspaceMode(item.value) {
				case workspaceDocument:
					return true, m.runAction(actionPickDocument)
				case workspaceEdit:
					return true, m.runAction(actionPickEdit)
				default:
					return true, m.runAction(actionPickGenerate)
				}
			}
		}
		var cmd tea.Cmd
		m.modeChoices, cmd = m.modeChoices.Update(msg)
		return true, cmd

	case screenEditOptions:
		if key.Matches(msg, m.keys.focusNext) || key.Matches(msg, m.keys.focusPrev) {
			if m.edit.kind == editKindDirected && m.focus == focusEditInstructions {
				m.focus = focusEditNext
			} else if m.edit.kind == editKindDirected && m.focus == focusEditNext {
				m.focus = focusEditInstructions
			} else {
				m.focus = focusEditDefault
			}
			m.syncFocus()
			return true, nil
		}
		if key.Matches(msg, m.keys.selectItem) && m.focus == focusEditNext {
			return true, m.runAction(actionEditOptionsNext)
		}
		if m.focus == focusEditInstructions {
			return false, nil
		}
		if key.Matches(msg, m.keys.selectItem) {
			if item, ok := m.editorChoices.SelectedItem().(choiceItem); ok {
				if editKind(item.value) == editKindCopy {
					m.edit.kind = editKindCopy
					return true, m.runAction(actionEditOptionsNext)
				}
				return true, m.runAction(actionPickCustomEditor)
			}
		}
		var cmd tea.Cmd
		m.editorChoices, cmd = m.editorChoices.Update(msg)
		return true, cmd

	case screenApprovalPicker:
		if key.Matches(msg, m.keys.selectItem) {
			if item, ok := m.approvalChoices.SelectedItem().(choiceItem); ok {
				switch approvalMode(item.value) {
				case approvalAutomatic:
					return true, m.runAction(actionPickAutoApproval)
				case approvalAll:
					return true, m.runAction(actionPickAllApproval)
				default:
					return true, m.runAction(actionPickManualApproval)
				}
			}
		}
		var cmd tea.Cmd
		m.approvalChoices, cmd = m.approvalChoices.Update(msg)
		return true, cmd

	case screenWorkspace:
		if key.Matches(msg, m.keys.focusNext) || key.Matches(msg, m.keys.focusPrev) {
			delta := 1
			if key.Matches(msg, m.keys.focusPrev) {
				delta = -1
			}
			count := 1
			if m.mode == workspaceGenerate {
				count = 2
			} else if m.mode == workspaceEdit {
				count = 3
			}
			m.switchWorkspaceTab((m.workspaceTab + delta + count) % count)
			return true, nil
		}
		if m.focus == focusWorkspaceTabs && (msg.Type == tea.KeyLeft || msg.Type == tea.KeyRight) {
			delta := 1
			if msg.Type == tea.KeyLeft {
				delta = -1
			}
			count := 2
			if m.mode == workspaceEdit {
				count = 3
			}
			m.switchWorkspaceTab((m.workspaceTab + delta + count) % count)
			return true, nil
		}
		if m.focus == focusHistoryPane && (msg.Type == tea.KeyLeft || msg.Type == tea.KeyRight || msg.Type == tea.KeyPgUp || msg.Type == tea.KeyPgDown) {
			delta := 1
			if msg.Type == tea.KeyLeft || msg.Type == tea.KeyPgUp {
				delta = -1
			}
			m.edit.historyIndex = clamp(m.edit.historyIndex+delta, 0, max(0, len(m.edit.history)-1))
			return true, nil
		}
		if key.Matches(msg, m.keys.save) {
			return true, m.runAction(actionSave)
		}
		if key.Matches(msg, m.keys.files) {
			return true, m.runAction(actionFiles)
		}
		if key.Matches(msg, m.keys.selectItem) {
			if action, ok := actionForFocus(m.focus); ok {
				return true, m.runAction(action)
			}
		}
		if m.mode == workspaceGenerate {
			if key.Matches(msg, m.keys.continueOp) {
				return true, m.runAction(actionContinue)
			}
			if key.Matches(msg, m.keys.newSection) {
				return true, m.runAction(actionNewSection)
			}
		} else if m.mode == workspaceEdit {
			if key.Matches(msg, m.keys.accept) {
				return true, m.runAction(actionAcceptSuggestion)
			}
			if key.Matches(msg, m.keys.skip) {
				return true, m.runAction(actionSkipSuggestion)
			}
			if key.Matches(msg, m.keys.refresh) {
				return true, m.runAction(actionRefreshSuggestion)
			}
		}
	}

	return false, nil
}

// --- Input delegation ---

func (m *Model) updateChooserInputs(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.focus != focusChooserInput {
		return m, nil
	}
	before := m.chooser.input.Value()
	var cmd tea.Cmd
	m.chooser.input, cmd = m.chooser.input.Update(msg)
	if before != m.chooser.input.Value() {
		name := document.NormalizeMarkdownFilename(m.chooser.input.Value())
		if name == "" {
			m.setStatus("Type a document name to continue", "muted")
		} else {
			m.setStatus("Ready to use "+name, "muted")
		}
	}
	return m, cmd
}

func (m *Model) updateWorkspaceInputs(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch m.focus {
	case focusFrontMatter:
		before := m.frontMatter.Value()
		var cmd tea.Cmd
		m.frontMatter, cmd = m.frontMatter.Update(msg)
		if before != m.frontMatter.Value() {
			m.doc.SetFrontMatter(m.frontMatter.Value())
			m.lastEditAt = time.Now()
		}
		return m, cmd
	case focusPrompt:
		before := m.prompt.Value()
		var cmd tea.Cmd
		m.prompt, cmd = m.prompt.Update(msg)
		if before != m.prompt.Value() {
			m.setStatus("Generation guidance updated", "muted")
		}
		return m, cmd
	default:
		if _, ok := actionForFocus(m.focus); ok {
			return m, nil
		}
		if keyMsg, ok := msg.(tea.KeyMsg); ok {
			switch keyMsg.Type {
			case tea.KeyPgUp:
				m.pageEditor(-1)
				return m, nil
			case tea.KeyPgDown:
				m.pageEditor(1)
				return m, nil
			}
		}
		before := m.editor.Value()
		var cmd tea.Cmd
		m.editor, cmd = m.editor.Update(msg)
		if before != m.editor.Value() {
			m.doc.SetBody(m.editor.Value())
			m.lastEditAt = time.Now()
		}
		return m, cmd
	}
}

// --- Layout ---

func (m *Model) resize() {
	contentWidth := m.paneContentWidth()

	switch m.screen {
	case screenChooser:
		m.chooser.input.Width = max(20, contentWidth-2)
		return
	case screenModePicker:
		m.modeChoices.SetSize(max(30, m.width-2), clamp(m.height-8, 9, 16))
		return
	case screenEditOptions:
		m.edit.instructions.SetWidth(contentWidth)
		m.edit.instructions.SetHeight(clamp(m.height/4, 4, 8))
		m.editorChoices.SetSize(max(30, m.width-2), clamp(m.height/3, 7, 12))
		return
	case screenApprovalPicker:
		m.approvalChoices.SetSize(max(30, m.width-2), clamp(m.height-8, 9, 16))
		return
	case screenWorkspace:
	}

	if m.showFrontMatter {
		modalWidth := max(40, min(contentWidth, 96))
		m.frontMatter.SetWidth(modalWidth)
		m.frontMatter.SetHeight(max(8, m.height-10))
		return
	}

	headerHeight := lineCount(m.renderHeader())
	statusHeight := lineCount(m.renderStatusBar())

	switch m.mode {
	case workspaceDocument:
		buttonRows := buttonRowCount(m.width-2, []string{"Save [Ctrl+S]", "Files [Ctrl+O]", "Message [Alt+M]", "Quit [Ctrl+Q]"})
		fixedHeight := headerHeight + statusHeight + buttonRows + 5
		m.editor.SetWidth(contentWidth)
		m.editor.SetHeight(max(3, m.height-fixedHeight))
	case workspaceGenerate:
		buttonRows := buttonRowCount(m.width-2, []string{
			"Continue [Ctrl+G]",
			"New Section [Ctrl+N]",
			"Save [Ctrl+S]",
			"Files [Ctrl+O]",
			"Message",
			"Quit [Ctrl+Q]",
		})
		fixedHeight := headerHeight + statusHeight + buttonRows + 7
		available := max(3, m.height-fixedHeight)

		m.editor.SetWidth(contentWidth)
		m.editor.SetHeight(available)
		m.prompt.SetWidth(contentWidth)
		m.prompt.SetHeight(available)
	case workspaceEdit:
		buttonRows := buttonRowCount(m.width-2, []string{
			"Accept [Ctrl+A]",
			"Skip [Ctrl+K]",
			"Refresh [Ctrl+R]",
			"Save [Ctrl+S]",
			"Files [Ctrl+O]",
			"Message",
			"Quit [Ctrl+Q]",
		})
		fixedHeight := headerHeight + statusHeight + buttonRows + 7
		available := max(3, m.height-fixedHeight)

		m.editor.SetWidth(contentWidth)
		m.editor.SetHeight(available)
	}
}

func (m *Model) syncFocus() {
	m.chooser.input.Blur()
	m.frontMatter.Blur()
	m.editor.Blur()
	m.prompt.Blur()
	m.edit.instructions.Blur()

	if m.showFrontMatter {
		m.frontMatter.Focus()
		return
	}

	switch m.focus {
	case focusChooserInput:
		m.chooser.input.Focus()
	case focusFrontMatter:
		m.frontMatter.Focus()
	case focusEditor:
		m.editor.Focus()
	case focusPrompt:
		m.prompt.Focus()
	case focusEditInstructions:
		m.edit.instructions.Focus()
	}
}

func (m *Model) advanceWorkspaceFocus(delta int) {
	order := []focusTarget{focusWorkspaceTabs}
	if m.mode == workspaceGenerate {
		if m.workspaceTab == 0 {
			order = append(order, focusEditor)
		} else {
			order = append(order, focusPrompt)
		}
		order = append(order, focusContinueButton, focusNewSectionButton)
	} else {
		if m.workspaceTab == 1 {
			order = append(order, focusHistoryPane)
		} else if m.workspaceTab == 2 {
			order = append(order, focusEditor)
		}
		order = append(order, focusAcceptButton, focusSkipButton, focusRefreshButton)
	}
	order = append(order, focusSaveButton, focusFilesButton, focusMessageButton, focusQuitButton)

	index := 0
	for i, target := range order {
		if m.focus == target {
			index = i
			break
		}
	}
	index = (index + delta + len(order)) % len(order)
	m.focus = order[index]
	m.syncFocus()
}

func (m *Model) switchWorkspaceTab(index int) {
	m.workspaceTab = index
	if m.mode == workspaceGenerate {
		if index == 1 {
			m.focus = focusPrompt
		} else {
			m.focus = focusEditor
		}
	} else {
		switch index {
		case 1:
			m.focus = focusHistoryPane
		case 2:
			m.focus = focusEditor
		default:
			m.focus = focusWorkspaceTabs
		}
	}
	m.syncFocus()
	m.resize()
}

// --- Rendering ---

func (m *Model) renderWorkspace() string {
	if m.mode == workspaceEdit {
		return m.renderEditWorkspace()
	}
	if m.mode == workspaceDocument {
		return m.renderDocumentWorkspace()
	}
	return m.renderGenerateWorkspace()
}

func (m *Model) renderDocumentWorkspace() string {
	m.layout = layoutState{}
	var lines []string
	y := 0
	header := m.renderHeader()
	lines = append(lines, header, "")
	y += lineCount(header) + 1
	pane := m.renderPane("Document", m.editor.View(), m.focus == focusEditor, m.hover == focusEditor, m.doc != nil && m.doc.Dirty)
	m.layout.editor = rect{x1: 0, y1: y, x2: max(0, m.width-1), y2: y + lineCount(pane) - 1}
	lines = append(lines, pane, "")
	y += lineCount(pane) + 1
	status := m.renderStatusBar()
	lines = append(lines, status)
	y += lineCount(status)
	buttons := []actionButton{
		{Action: actionSave, Label: "Save [Ctrl+S]", Background: colorOrangeDark, Foreground: colorOrangeLight, Focus: focusSaveButton},
		{Action: actionFiles, Label: "Files [Ctrl+O]", Background: colorBlueDark, Foreground: colorBlueLight, Focus: focusFilesButton},
		{Action: actionToggleMessage, Label: "Message [Alt+M]", Background: colorBlue, Foreground: colorBlueLight, Focus: focusMessageButton},
		{Action: actionQuit, Label: "Quit [Ctrl+Q]", Background: "#3F3F46", Foreground: "#F4F4F5", Focus: focusQuitButton},
	}
	lines = append(lines, m.renderButtons(buttons, y))
	return strings.Join(lines, "\n")
}

func (m *Model) renderGenerateWorkspace() string {
	m.layout = layoutState{}
	var lines []string
	y := 0
	header := m.renderHeader()
	lines = append(lines, header, "")
	y += lineCount(header) + 1
	tabs := m.renderTabs([]string{"Document", "Guidance"}, y)
	lines = append(lines, tabs, "")
	y += lineCount(tabs) + 1
	if m.workspaceTab == 1 {
		promptMeta := lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8")).Render(promptHelpText())
		pane := m.renderPane("Generation Guidance", m.prompt.View()+"\n"+promptMeta, m.focus == focusPrompt, m.hover == focusPrompt, false)
		m.layout.prompt = rect{x1: 0, y1: y, x2: max(0, m.width-1), y2: y + lineCount(pane) - 1}
		lines = append(lines, pane, "")
		y += lineCount(pane) + 1
	} else {
		pane := m.renderPane("Document", m.editor.View(), m.focus == focusEditor, m.hover == focusEditor, m.doc != nil && m.doc.Dirty)
		m.layout.editor = rect{x1: 0, y1: y, x2: max(0, m.width-1), y2: y + lineCount(pane) - 1}
		lines = append(lines, pane, "")
		y += lineCount(pane) + 1
	}

	status := m.renderStatusBar()
	lines = append(lines, status)
	y += lineCount(status)

	var buttons []actionButton
	if m.generating {
		buttons = append(buttons, actionButton{Action: actionCancelBusy, Label: "Cancel [Esc]", Background: colorOrangeDark, Foreground: colorOrangeLight, Focus: focusTarget(-1)})
	} else {
		buttons = append(buttons,
			actionButton{Action: actionContinue, Label: "Continue [Ctrl+G]", Background: colorOrangeDark, Foreground: colorOrangeLight, Focus: focusContinueButton},
			actionButton{Action: actionNewSection, Label: "New Section [Ctrl+N]", Background: colorBlue, Foreground: colorBlueLight, Focus: focusNewSectionButton},
		)
		buttons = append(buttons,
			actionButton{Action: actionSave, Label: "Save [Ctrl+S]", Background: colorOrangeDark, Foreground: colorOrangeLight, Focus: focusSaveButton},
			actionButton{Action: actionFiles, Label: "Files [Ctrl+O]", Background: colorBlueDark, Foreground: colorBlueLight, Focus: focusFilesButton},
			actionButton{Action: actionToggleMessage, Label: "Message [Alt+M]", Background: colorBlue, Foreground: colorBlueLight, Focus: focusMessageButton},
		)
	}
	buttons = append(buttons, actionButton{Action: actionQuit, Label: "Quit [Ctrl+Q]", Background: "#3F3F46", Foreground: "#F4F4F5", Focus: focusQuitButton})
	lines = append(lines, m.renderButtons(buttons, y))

	return strings.Join(lines, "\n")
}

func (m *Model) renderEditWorkspace() string {
	m.layout = layoutState{}
	var lines []string
	y := 0
	header := m.renderHeader()
	lines = append(lines, header, "")
	y += lineCount(header) + 1
	tabs := m.renderTabs([]string{"Suggestion", fmt.Sprintf("History (%d)", len(m.edit.history)), "Document"}, y)
	lines = append(lines, tabs, "")
	y += lineCount(tabs) + 1
	var pane string
	switch m.workspaceTab {
	case 1:
		pane = m.renderPane("Edit History", m.renderEditHistoryBody(), m.focus == focusWorkspaceTabs, false, false)
	case 2:
		pane = m.renderPane("Document", m.editor.View(), m.focus == focusEditor, m.hover == focusEditor, m.doc != nil && m.doc.Dirty)
		m.layout.editor = rect{x1: 0, y1: y, x2: max(0, m.width-1), y2: y + lineCount(pane) - 1}
	default:
		pane = m.renderPane("Edit Suggestion", m.renderSuggestionBody(), suggestionFocused(m.focus) || m.focus == focusWorkspaceTabs, false, false)
		m.layout.prompt = rect{x1: 0, y1: y, x2: max(0, m.width-1), y2: y + lineCount(pane) - 1}
	}
	lines = append(lines, pane, "")
	y += lineCount(pane) + 1

	status := m.renderStatusBar()
	lines = append(lines, status)
	y += lineCount(status)

	var buttons []actionButton
	if m.edit.requesting || m.edit.reviewing {
		buttons = append(buttons,
			actionButton{Action: actionCancelBusy, Label: "Cancel [Esc]", Background: colorOrangeDark, Foreground: colorOrangeLight, Focus: focusTarget(-1)},
			actionButton{Action: actionShowHistory, Label: "History [Alt+H]", Background: colorBlueDark, Foreground: colorBlueLight, Focus: focusTarget(-1)},
		)
	} else {
		if m.edit.suggestion != nil {
			buttons = append(buttons,
				actionButton{Action: actionAcceptSuggestion, Label: "Accept [Ctrl+A]", Background: colorOrangeDark, Foreground: colorOrangeLight, Focus: focusAcceptButton},
				actionButton{Action: actionSkipSuggestion, Label: "Skip [Ctrl+K]", Background: colorBlueDark, Foreground: colorBlueLight, Focus: focusSkipButton},
			)
		}
		buttons = append(buttons, actionButton{Action: actionRefreshSuggestion, Label: "Refresh [Ctrl+R]", Background: colorBlue, Foreground: colorBlueLight, Focus: focusRefreshButton})
		buttons = append(buttons,
			actionButton{Action: actionShowHistory, Label: "History [Alt+H]", Background: colorBlueDark, Foreground: colorBlueLight, Focus: focusTarget(-1)},
			actionButton{Action: actionSave, Label: "Save [Ctrl+S]", Background: colorOrangeDark, Foreground: colorOrangeLight, Focus: focusSaveButton},
			actionButton{Action: actionFiles, Label: "Files [Ctrl+O]", Background: colorBlueDark, Foreground: colorBlueLight, Focus: focusFilesButton},
			actionButton{Action: actionToggleMessage, Label: "Message [Alt+M]", Background: colorBlue, Foreground: colorBlueLight, Focus: focusMessageButton},
		)
	}
	buttons = append(buttons, actionButton{Action: actionQuit, Label: "Quit [Ctrl+Q]", Background: "#3F3F46", Foreground: "#F4F4F5", Focus: focusQuitButton})
	lines = append(lines, m.renderButtons(buttons, y))

	return strings.Join(lines, "\n")
}

func (m *Model) renderTabs(labels []string, y int) string {
	var rendered []string
	x := 0
	for index, label := range labels {
		style := lipgloss.NewStyle().Padding(0, 2).Foreground(lipgloss.Color("#94A3B8")).Background(lipgloss.Color("#1E293B"))
		if m.workspaceTab == index {
			style = style.Foreground(lipgloss.Color("#F8FAFC")).Background(lipgloss.Color(colorOrangeDark)).Bold(true)
		}
		part := style.Render(label)
		width := lipgloss.Width(part)
		m.layout.tabs = append(m.layout.tabs, rect{x1: x, y1: y, x2: x + width - 1, y2: y})
		rendered = append(rendered, part)
		x += width + 1
	}
	row := strings.Join(rendered, " ")
	hint := lipgloss.NewStyle().Foreground(lipgloss.Color("#64748B")).Render("Tab / Shift+Tab to switch")
	if lipgloss.Width(row)+2+lipgloss.Width(hint) <= max(20, m.width-2) {
		row += "  " + hint
	}
	return row
}

func (m Model) renderSuggestionBody() string {
	var parts []string
	parts = append(parts, lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8")).Render(editHelpText()))

	switch {
	case m.edit.requesting:
		parts = append(parts, "", lipgloss.NewStyle().Foreground(lipgloss.Color("#CBD5E1")).Render("Requesting the next highest-priority fix..."))
	case m.edit.suggestion == nil:
		parts = append(parts, "", lipgloss.NewStyle().Foreground(lipgloss.Color("#CBD5E1")).Render("No active suggestion. Use Refresh to ask for another pass."))
	default:
		matchCount := 0
		if m.doc != nil {
			matchCount = document.MatchCount(m.doc.Body, m.edit.suggestion.OldText)
		}

		ops := diff.Diff(m.edit.suggestion.OldText, m.edit.suggestion.NewText)
		defaultStyle := lipgloss.NewStyle()
		deleteStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))
		insertStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#22C55E"))

		parts = append(parts,
			"",
			lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F8FAFC")).Render("Old"),
			diff.FormatOld(ops, defaultStyle, deleteStyle),
			"",
			lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F8FAFC")).Render("New"),
			diff.FormatNew(ops, defaultStyle, insertStyle),
			"",
			lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8")).Render(fmt.Sprintf("Exact match count in document: %d", matchCount)),
		)
		if m.edit.reviewing {
			parts = append(parts, "", lipgloss.NewStyle().Foreground(lipgloss.Color("#FACC15")).Render(m.spin.View()+" Checking automatic approval; this suggestion remains available for review."))
		}
	}
	return strings.Join(parts, "\n")
}

func (m Model) renderEditHistoryBody() string {
	if len(m.edit.history) == 0 {
		return "No edits have been applied or skipped in this session.\n\nUse ←/→ to move through entries once history is available."
	}
	index := clamp(m.edit.historyIndex, 0, len(m.edit.history)-1)
	entry := m.edit.history[index]
	ops := diff.Diff(entry.OldText, entry.NewText)
	defaultStyle := lipgloss.NewStyle()
	deleteStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444"))
	insertStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("#22C55E"))
	return strings.Join([]string{
		fmt.Sprintf("Edit %d of %d · %s", index+1, len(m.edit.history), entry.Action),
		"Use ←/→ to page through the session history.",
		"",
		lipgloss.NewStyle().Bold(true).Render("Old"),
		diff.FormatOld(ops, defaultStyle, deleteStyle),
		"",
		lipgloss.NewStyle().Bold(true).Render("New"),
		diff.FormatNew(ops, defaultStyle, insertStyle),
	}, "\n")
}

func suggestionFocused(target focusTarget) bool {
	switch target {
	case focusAcceptButton, focusSkipButton, focusRefreshButton:
		return true
	default:
		return false
	}
}

func (m *Model) renderFrontMatterModal() string {
	m.layout = layoutState{}

	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F8FAFC")).Render("Document Instructions")
	subtitle := lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8")).Render("Edit file metadata and document-specific system guidance. Esc closes this panel.")
	header := lipgloss.JoinVertical(lipgloss.Left, title, subtitle)

	pane := m.renderPane("Front Matter", m.frontMatter.View(), true, m.hover == focusFrontMatter, m.doc != nil && m.doc.Dirty)
	m.layout.frontMatter = rect{x1: 0, y1: lineCount(header) + 1, x2: max(0, m.width-1), y2: lineCount(header) + lineCount(pane)}
	status := m.renderStatusBar()
	buttons := []actionButton{
		{Action: actionSave, Label: "Save [Ctrl+S]", Background: "#7C2D12", Foreground: "#FFEDD5", Focus: focusSaveButton},
		{Action: actionToggleMessage, Label: "Close [Esc]", Background: "#0F766E", Foreground: "#CCFBF1", Focus: focusMessageButton},
		{Action: actionQuit, Label: "Quit [Ctrl+Q]", Background: "#3F3F46", Foreground: "#F4F4F5", Focus: focusQuitButton},
	}

	content := []string{header, "", pane, "", status}
	y := lineCount(header) + 2 + lineCount(pane) + 1 + lineCount(status)
	content = append(content, m.renderButtons(buttons, y))

	return strings.Join(content, "\n")
}

func (m *Model) renderChooser() string {
	m.layout = layoutState{}

	contentWidth := max(60, m.width-2)
	var lines []string
	y := 0

	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F8FAFC")).Render("Choose a document")
	subtitle := lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8")).Render("Select a markdown file in this folder or type a new name, then choose a mode.")
	lines = append(lines, title, subtitle, "")
	y += 3

	var fileLines []string
	if len(m.chooser.files) == 0 {
		fileLines = append(fileLines, lipgloss.NewStyle().Foreground(lipgloss.Color("#64748B")).Render("No markdown files found in this folder yet."))
	} else {
		for i, name := range m.chooser.files {
			prefix := "  "
			style := lipgloss.NewStyle().Foreground(lipgloss.Color("#CBD5E1"))
			if i == m.chooser.selected {
				prefix = "• "
				style = lipgloss.NewStyle().Foreground(lipgloss.Color("#F8FAFC")).Bold(true)
			}
			fileLines = append(fileLines, style.Render(prefix+name))
		}
	}

	fileBox := m.renderPane("Documents", strings.Join(fileLines, "\n"), m.focus == focusChooserList, false, false)
	lines = append(lines, fileBox)
	fileBoxHeight := lineCount(fileBox)
	for i := range m.chooser.files {
		m.layout.files = append(m.layout.files, fileRegion{
			Index: i,
			Rect: rect{
				x1: 2,
				y1: y + 2 + i,
				x2: min(contentWidth-2, m.width-1),
				y2: y + 2 + i,
			},
		})
	}
	y += fileBoxHeight

	lines = append(lines, "")
	y++

	inputBox := m.renderPane("New Document Name", m.chooser.input.View(), m.focus == focusChooserInput, m.hover == focusChooserInput, false)
	m.layout.chooserInput = rect{x1: 0, y1: y, x2: max(0, m.width-1), y2: y + lineCount(inputBox) - 1}
	lines = append(lines, inputBox)
	y += lineCount(inputBox)

	lines = append(lines, "")
	y++

	status := m.renderStatusBar()
	lines = append(lines, status)
	y += lineCount(status)

	buttons := []actionButton{
		{Action: actionChooseSelected, Label: "Use Selected", Background: colorOrangeDark, Foreground: colorOrangeLight, Focus: focusTarget(-1)},
		{Action: actionChooseTyped, Label: "Use Typed", Background: colorBlue, Foreground: colorBlueLight, Focus: focusTarget(-1)},
		{Action: actionRefreshFiles, Label: "Refresh [Ctrl+R]", Background: colorBlueDark, Foreground: colorBlueLight, Focus: focusTarget(-1)},
	}
	if len(m.screenPath) > 0 {
		buttons = append(buttons, actionButton{Action: actionBack, Label: "Back [Esc]", Background: colorBlueDark, Foreground: colorBlueLight, Focus: focusTarget(-1)})
	}
	buttons = append(buttons, actionButton{Action: actionQuit, Label: "Quit [Ctrl+Q]", Background: "#3F3F46", Foreground: "#F4F4F5", Focus: focusTarget(-1)})
	lines = append(lines, m.renderButtons(buttons, y))

	return strings.Join(lines, "\n")
}

func (m *Model) renderModePicker() string {
	m.layout = layoutState{}
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F8FAFC")).Render("Choose a Mode")
	subtitle := lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8")).Render("Pick how goauthorllm should work with " + truncateToWidth(m.pendingName, max(20, m.width-8)))
	content := []string{title, subtitle, "", m.modeChoices.View(), "", m.renderStatusBar()}
	m.setChoiceRegions(3, len(m.modeChoices.Items()))
	y := lineCount(strings.Join(content, "\n"))
	buttons := []actionButton{
		{Action: actionBack, Label: "Back [Esc]", Background: colorBlueDark, Foreground: colorBlueLight, Focus: focusTarget(-1)},
		{Action: actionQuit, Label: "Quit [Ctrl+Q]", Background: "#3F3F46", Foreground: "#F4F4F5", Focus: focusTarget(-1)},
	}
	content = append(content, m.renderButtons(buttons, y))
	return strings.Join(content, "\n")
}

func (m *Model) renderEditOptions() string {
	m.layout = layoutState{}
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F8FAFC")).Render("Choose an Editor")
	subtitle := lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8")).Render("Use ↑/↓ and Enter. Tab moves between custom instructions and Next when needed.")
	listView := m.editorChoices.View()
	m.setChoiceRegions(3, len(m.editorChoices.Items()))
	content := []string{title, subtitle, "", listView}
	if m.edit.kind == editKindDirected {
		instructions := m.renderPane("Custom Editing Instructions", m.edit.instructions.View(), m.focus == focusEditInstructions, m.hover == focusEditInstructions, false)
		instructionsY := lineCount(strings.Join(content, "\n")) + 1
		m.layout.editInstructions = rect{x1: 0, y1: instructionsY, x2: max(0, m.width-1), y2: instructionsY + lineCount(instructions) - 1}
		content = append(content, "", instructions)
	}
	content = append(content, "", m.renderStatusBar())
	y := lineCount(strings.Join(content, "\n"))
	buttons := []actionButton{
		{Action: actionEditOptionsNext, Label: "Next [Enter]", Background: colorOrangeDark, Foreground: colorOrangeLight, Focus: focusEditNext},
		{Action: actionBack, Label: "Back [Esc]", Background: colorBlueDark, Foreground: colorBlueLight, Focus: focusTarget(-1)},
	}
	return strings.Join(append(content, m.renderButtons(buttons, y)), "\n")
}

func (m *Model) renderApprovalPicker() string {
	m.layout = layoutState{}
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F8FAFC")).Render("Choose Approval Mode")
	subtitle := lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8")).Render("Use ↑/↓ and Enter to begin editing. Esc returns to editor selection.")
	content := []string{title, subtitle, "", m.approvalChoices.View(), "", m.renderStatusBar()}
	m.setChoiceRegions(3, len(m.approvalChoices.Items()))
	y := lineCount(strings.Join(content, "\n"))
	buttons := []actionButton{{Action: actionBack, Label: "Back [Esc]", Background: colorBlueDark, Foreground: colorBlueLight, Focus: focusTarget(-1)}}
	return strings.Join(append(content, m.renderButtons(buttons, y)), "\n")
}

func (m *Model) setChoiceRegions(listTop, count int) {
	for index := 0; index < count; index++ {
		y := listTop + 2 + index*3
		m.layout.choices = append(m.layout.choices, choiceRegion{Index: index, Rect: rect{x1: 0, y1: y, x2: max(0, m.width-1), y2: y + 1}})
	}
}

func (m Model) renderHeader() string {
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#F8FAFC")).Render("goauthorllm")
	fileName := ""
	if m.doc != nil {
		fileName = filepath.Base(m.doc.Path)
	} else if m.pendingName != "" {
		fileName = m.pendingName
	}
	filePart := lipgloss.NewStyle().Foreground(lipgloss.Color("#94A3B8")).Render(truncateToWidth(fileName, max(12, m.width/3)))

	modeBadge := badge(m.mode.label(), colorOrangeDark, colorOrangeLight)
	if m.screen == screenModePicker {
		modeBadge = badge("mode select", "#334155", "#E2E8F0")
	}
	if m.screen == screenChooser {
		modeBadge = badge("documents", "#334155", "#E2E8F0")
	}

	dirty := badge("saved", "#14532D", "#DCFCE7")
	if m.doc != nil && m.doc.Dirty {
		dirty = badge("unsaved", "#7C2D12", "#FFEDD5")
	}

	return truncateRenderedLine(lipgloss.JoinHorizontal(lipgloss.Top, title, "  ", filePart, "  ", modeBadge, "  ", dirty), max(20, m.width-2))
}

func (m Model) renderPane(title, body string, focused, hovered, dirty bool) string {
	borderColor := lipgloss.Color("#334155")
	titleColor := lipgloss.Color("#CBD5E1")
	if hovered {
		borderColor = lipgloss.Color(colorBlue)
		titleColor = lipgloss.Color(colorBlueLight)
	}
	if focused {
		borderColor = lipgloss.Color(colorOrange)
		titleColor = lipgloss.Color("#F8FAFC")
	}
	if focused && hovered {
		borderColor = lipgloss.Color(colorOrangeLight)
	}

	label := title
	if dirty {
		label += " *"
	}

	return lipgloss.NewStyle().
		Width(m.paneContentWidth()).
		Border(lipgloss.RoundedBorder()).
		BorderForeground(borderColor).
		Render(lipgloss.JoinVertical(
			lipgloss.Left,
			lipgloss.NewStyle().Bold(true).Underline(hovered).Foreground(titleColor).Render(label),
			body,
		))
}

func (m Model) renderStatusBar() string {
	status := m.statusText
	if m.busy() {
		status = m.spin.View() + " " + status
	}

	width := max(20, m.width-2)
	lines := []string{
		lipgloss.NewStyle().Foreground(statusForeground(m.statusLevel)).Render(truncateToWidth(status, width)),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#64748B")).Render(truncateToWidth("Endpoint "+m.cfg.BaseURL, width)),
		lipgloss.NewStyle().Foreground(lipgloss.Color("#64748B")).Render(truncateToWidth("Model "+m.cfg.Model, width)),
	}
	return strings.Join(lines, "\n")
}

type actionButton struct {
	Action     buttonAction
	Label      string
	Background string
	Foreground string
	Focus      focusTarget
}

func (m *Model) renderButtons(buttons []actionButton, y int) string {
	m.layout.buttons = nil

	var (
		rendered []string
		rowParts []string
		x        int
		row      int
		maxWidth = max(20, m.width-2)
	)
	for _, button := range buttons {
		label := " " + button.Label + " "
		labelWidth := lipgloss.Width(label)
		startX := x
		if len(rowParts) > 0 {
			startX++
		}
		if startX+labelWidth > maxWidth && len(rowParts) > 0 {
			rendered = append(rendered, strings.Join(rowParts, " "))
			rowParts = nil
			row++
			x = 0
			startX = 0
		}

		m.layout.buttons = append(m.layout.buttons, buttonRegion{
			Action: button.Action,
			Rect: rect{
				x1: startX,
				y1: y + row,
				x2: startX + labelWidth - 1,
				y2: y + row,
			},
		})

		rowParts = append(rowParts, lipgloss.NewStyle().
			Foreground(lipgloss.Color(button.Foreground)).
			Background(lipgloss.Color(button.Background)).
			Bold(true).
			Underline(m.focus == button.Focus).
			Padding(0, 1).
			Render(button.Label))

		x = startX + labelWidth
	}

	if len(rowParts) > 0 {
		rendered = append(rendered, strings.Join(rowParts, " "))
	}

	return strings.Join(rendered, "\n")
}

func (m *Model) buttonAt(x, y int) (buttonAction, bool) {
	for _, button := range m.layout.buttons {
		if button.Rect.contains(x, y) {
			return button.Action, true
		}
	}
	return "", false
}

func (m *Model) fileAt(x, y int) (int, bool) {
	for _, file := range m.layout.files {
		if file.Rect.contains(x, y) {
			return file.Index, true
		}
	}
	return 0, false
}

func (m *Model) choiceAt(x, y int) (int, bool) {
	for _, choice := range m.layout.choices {
		if choice.Rect.contains(x, y) {
			return choice.Index, true
		}
	}
	return 0, false
}

// --- Actions ---

func (m *Model) runAction(action buttonAction) tea.Cmd {
	switch action {
	case actionContinue:
		if m.mode != workspaceGenerate || m.doc == nil || m.generating {
			return nil
		}
		return m.startGeneration(modeContinue)

	case actionNewSection:
		if m.mode != workspaceGenerate || m.doc == nil || m.generating {
			return nil
		}
		return m.startGeneration(modeNewSection)

	case actionAcceptSuggestion:
		if m.mode != workspaceEdit || m.doc == nil || m.edit.requesting || m.edit.reviewing || m.edit.suggestion == nil {
			return nil
		}
		return m.acceptSuggestion()

	case actionSkipSuggestion:
		if m.mode != workspaceEdit || m.doc == nil || m.edit.requesting || m.edit.reviewing {
			return nil
		}
		if m.edit.suggestion != nil {
			m.appendEditHistory("skipped", *m.edit.suggestion)
		}
		m.edit.suggestion = nil
		return m.requestEditSuggestion("Requesting the next edit suggestion...")

	case actionRefreshSuggestion:
		if m.mode != workspaceEdit || m.doc == nil || m.edit.requesting || m.edit.reviewing {
			return nil
		}
		return m.requestEditSuggestion("Refreshing edit suggestion...")

	case actionSave:
		if m.doc == nil {
			return nil
		}
		if err := m.saveCurrentDocument(); err != nil {
			m.setStatus("Save failed: "+err.Error(), "error")
		} else {
			m.setStatus("Saved "+formatTimestamp(m.doc.LastSavedAt), "success")
		}
		return nil

	case actionFiles:
		if err := m.saveBeforeLeave("switch files"); err != nil {
			m.setStatus(err.Error(), "error")
			return nil
		}
		m.pushScreen(screenWorkspace)
		m.screen = screenChooser
		m.focus = focusChooserList
		m.syncFocus()
		m.resize()
		m.setStatus("Choose a document", "info")
		return nil

	case actionToggleMessage:
		m.showFrontMatter = !m.showFrontMatter
		if m.showFrontMatter {
			m.focus = focusFrontMatter
			m.setStatus("Document instructions shown", "muted")
		} else if m.focus == focusFrontMatter {
			m.focus = focusEditor
			m.setStatus("Document instructions hidden", "muted")
		}
		m.resize()
		m.syncFocus()
		return nil

	case actionChooseSelected:
		if len(m.chooser.files) == 0 {
			m.setStatus("No documents available. Type a new name to create one.", "error")
			return nil
		}
		m.pendingPath = m.resolveDocumentPath(m.chooser.files[m.chooser.selected])
		m.pendingName = filepath.Base(m.pendingPath)
		m.pushScreen(screenChooser)
		m.screen = screenModePicker
		m.focus = focusModeDocument
		m.modeChoices.Select(0)
		m.resize()
		m.setStatus("Choose how to work with "+m.pendingName, "info")
		return nil

	case actionChooseTyped:
		name := document.NormalizeMarkdownFilename(m.chooser.input.Value())
		if name == "" {
			m.setStatus("Type a markdown filename first", "error")
			return nil
		}
		m.pendingPath = m.resolveDocumentPath(name)
		m.pendingName = filepath.Base(m.pendingPath)
		m.pushScreen(screenChooser)
		m.screen = screenModePicker
		m.focus = focusModeDocument
		m.modeChoices.Select(0)
		m.resize()
		m.setStatus("Choose how to work with "+m.pendingName, "info")
		return nil

	case actionPickGenerate:
		return m.enterWorkspace(workspaceGenerate)

	case actionPickDocument:
		return m.enterWorkspace(workspaceDocument)

	case actionPickEdit:
		m.screen = screenEditOptions
		m.screenPath = append(m.screenPath, screenModePicker)
		m.edit.kind = editKindCopy
		m.edit.approval = approvalManual
		m.edit.suggestion = nil
		m.edit.history = nil
		m.edit.historyIndex = 0
		m.edit.requesting = false
		m.edit.reviewing = false
		m.edit.instructions.SetValue("")
		m.editorChoices.Select(0)
		m.approvalChoices.Select(0)
		m.focus = focusEditDefault
		m.resize()
		m.syncFocus()
		m.setStatus("Choose an editor, then choose how suggestions are approved", "info")
		return nil

	case actionPickDefaultEditor:
		m.edit.kind = editKindCopy
		m.focus = focusEditDefault
		m.syncFocus()
		return m.runAction(actionEditOptionsNext)

	case actionPickCustomEditor:
		m.edit.kind = editKindDirected
		m.focus = focusEditInstructions
		m.syncFocus()
		m.setStatus("Custom editor selected. Write instructions, then Tab to Next and press Enter.", "muted")
		return nil

	case actionEditOptionsNext:
		if m.edit.kind == editKindDirected && strings.TrimSpace(m.edit.instructions.Value()) == "" {
			m.setStatus("Add custom editing instructions before continuing", "error")
			return nil
		}
		m.screen = screenApprovalPicker
		m.screenPath = append(m.screenPath, screenEditOptions)
		m.focus = focusApprovalManual
		m.resize()
		m.syncFocus()
		m.setStatus("Choose an approval mode", "info")
		return nil

	case actionPickManualApproval:
		m.edit.approval = approvalManual
		return m.enterWorkspace(workspaceEdit)
	case actionPickAutoApproval:
		m.edit.approval = approvalAutomatic
		return m.enterWorkspace(workspaceEdit)
	case actionPickAllApproval:
		m.edit.approval = approvalAll
		return m.enterWorkspace(workspaceEdit)

	case actionShowHistory:
		m.switchWorkspaceTab(1)
		return nil

	case actionCancelBusy:
		m.cancelBusyRequest()
		return nil

	case actionRefreshFiles:
		if err := m.refreshChooser(); err != nil {
			m.setStatus("Refresh failed: "+err.Error(), "error")
		} else {
			m.setStatus("Document list refreshed", "success")
		}
		return nil

	case actionBack:
		return m.goBack()

	case actionQuit:
		if err := m.saveBeforeLeave("quit"); err != nil {
			m.setStatus(err.Error(), "error")
			return nil
		}
		return tea.Quit
	}

	return nil
}

func (m *Model) goBack() tea.Cmd {
	if err := m.saveBeforeLeave("go back"); err != nil {
		m.setStatus(err.Error(), "error")
		return nil
	}

	if len(m.screenPath) == 0 {
		return tea.Quit
	}

	previous := m.screenPath[len(m.screenPath)-1]
	m.screenPath = m.screenPath[:len(m.screenPath)-1]
	m.screen = previous

	switch previous {
	case screenChooser:
		m.focus = focusChooserList
		m.setStatus("Choose a document", "info")
	case screenModePicker:
		m.focus = focusModeDocument
		if m.pendingName != "" {
			m.setStatus("Choose how to work with "+m.pendingName, "info")
		}
	case screenEditOptions:
		m.focus = focusEditDefault
		m.setStatus("Choose an editor", "info")
	case screenApprovalPicker:
		m.focus = focusEditDefault
		m.setStatus("Choose an editor", "info")
	case screenWorkspace:
		m.focus = focusEditor
		m.setStatus("Returned to "+m.mode.label()+" mode", "muted")
	}

	m.showFrontMatter = false
	m.resize()
	m.syncFocus()
	return nil
}

func (m *Model) enterWorkspace(mode workspaceMode) tea.Cmd {
	if m.pendingPath == "" {
		m.setStatus("Choose a document first", "error")
		return nil
	}
	if err := m.openDocument(m.pendingPath); err != nil {
		m.setStatus("Open failed: "+err.Error(), "error")
		return nil
	}

	m.mode = mode
	m.screen = screenWorkspace
	m.screenPath = []screenState{screenChooser, screenModePicker}
	m.workspaceTab = 0
	if mode == workspaceEdit {
		m.focus = focusWorkspaceTabs
	} else {
		m.focus = focusEditor
	}
	m.showFrontMatter = false
	m.resize()
	m.syncFocus()
	m.setStatus("Opened "+filepath.Base(m.doc.Path)+" in "+mode.label()+" mode", "success")

	if mode == workspaceEdit {
		return m.requestEditSuggestion("Reviewing the document for the highest-priority fix...")
	}
	return nil
}

// --- Document operations ---

func (m *Model) openDocument(path string) error {
	doc, err := document.Load(path)
	if err != nil {
		return err
	}

	if strings.TrimSpace(doc.FrontMatter) == "" && strings.TrimSpace(doc.SystemMessage) == "" {
		stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		doc.SetFrontMatter(fmt.Sprintf("title: %s", stem))
	}

	m.doc = doc
	m.pendingPath = doc.Path
	m.pendingName = filepath.Base(doc.Path)
	m.frontMatter.SetValue(doc.FrontMatter)
	m.editor.SetValue(doc.Body)
	m.prompt.SetValue("")
	m.lastEditAt = time.Now()
	return nil
}

func (m *Model) refreshChooser() error {
	files, err := document.ListMarkdownFiles(m.cwd)
	if err != nil {
		return err
	}
	m.chooser.files = files
	if len(files) == 0 {
		m.chooser.selected = 0
		return nil
	}
	if m.chooser.selected >= len(files) {
		m.chooser.selected = len(files) - 1
	}
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

func (m *Model) saveBeforeLeave(action string) error {
	if m.doc == nil || m.busy() || !m.doc.Dirty {
		return nil
	}
	if err := m.saveCurrentDocument(); err != nil {
		return fmt.Errorf("save before %s failed: %w", action, err)
	}
	return nil
}

// --- Generation ---

func (m *Model) startGeneration(mode generationMode) tea.Cmd {
	if m.doc == nil || m.client == nil {
		return nil
	}

	m.doc.SetFrontMatter(m.frontMatter.Value())
	m.doc.SetBody(m.editor.Value())

	body := m.doc.Body
	systemMessage := m.doc.SystemMessage
	prompt := m.prompt.Value()
	overrides := m.cfg.MessageOverrides
	messages, err := buildGenerationMessages(body, systemMessage, prompt, mode, overrides)
	if err != nil {
		m.setStatus("Generation failed: "+err.Error(), "error")
		return nil
	}

	m.generating = true
	m.generationID++
	m.generationMode = mode
	m.generationStarted = false
	m.setStatus("Streaming "+mode.progressLabel()+"...", "info")

	m.editor.SetValue(m.doc.Body)
	m.editor, _ = m.editor.Update(tea.KeyMsg{Type: tea.KeyCtrlEnd})
	m.editor.Focus()
	m.focus = focusEditor
	m.syncFocus()

	id := m.generationID
	timeout := m.cfg.Timeout
	client := m.client

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	m.generationCancel = cancel
	events := make(chan llm.StreamEvent, 32)
	go func() {
		err := client.StreamChat(ctx, messages, func(event llm.StreamEvent) error {
			events <- event
			return nil
		})
		if err != nil {
			events <- llm.StreamEvent{Err: err}
		}
		close(events)
	}()

	m.generationCh = events
	return tea.Batch(m.spin.Tick, waitForStream(events, id))
}

func (m *Model) applyGenerationDelta(delta string) {
	if delta == "" {
		return
	}

	if !m.generationStarted {
		delta = normalizeGenerationStart(m.doc.Body, delta, m.generationMode)
		m.generationStarted = true
	}

	// Keep the document body as the source of truth while streaming. The
	// textarea cursor can be moved by user input between stream events, so
	// inserting directly into the textarea can corrupt the generated suffix.
	m.doc.SetBody(m.doc.Body + delta)
	m.editor.SetValue(m.doc.Body)
	m.editor, _ = m.editor.Update(tea.KeyMsg{Type: tea.KeyCtrlEnd})
	m.lastEditAt = time.Now()
}

// --- Edit suggestions ---

func (m *Model) requestEditSuggestion(status string) tea.Cmd {
	if m.doc == nil || m.client == nil {
		return nil
	}

	m.doc.SetFrontMatter(m.frontMatter.Value())
	m.doc.SetBody(m.editor.Value())
	m.edit.requesting = true
	m.edit.requestID++
	m.edit.suggestion = nil
	m.setStatus(status, "info")

	id := m.edit.requestID
	timeout := m.cfg.Timeout
	body := m.doc.Body
	systemMessage := m.doc.SystemMessage
	history := append([]editHistoryEntry(nil), m.edit.history...)
	kind := m.edit.kind
	instructions := m.edit.instructions.Value()
	client := m.client
	overrides := m.cfg.MessageOverrides

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	m.edit.requestCancel = cancel
	events := make(chan editSuggestionResult, 1)
	go func() {
		events <- fetchEditSuggestion(ctx, client, body, systemMessage, history, overrides, kind, instructions)
		close(events)
	}()
	return tea.Batch(m.spin.Tick, waitForEditSuggestion(events, id))
}

func (m *Model) requestEditApproval(suggestion editSuggestion) tea.Cmd {
	if m.doc == nil || m.client == nil {
		return nil
	}
	m.edit.reviewing = true
	id := m.edit.requestID
	body := m.doc.Body
	kind := m.edit.kind
	instructions := m.edit.instructions.Value()
	client := m.client
	overrides := m.cfg.MessageOverrides
	ctx, cancel := context.WithTimeout(context.Background(), m.cfg.Timeout)
	m.edit.requestCancel = cancel
	return tea.Batch(m.spin.Tick, func() tea.Msg {
		approved, err := approveEditSuggestion(ctx, client, body, suggestion, kind, instructions, overrides)
		return editApprovalMsg{id: id, approved: approved, err: err}
	})
}

func (m *Model) acceptSuggestion() tea.Cmd {
	suggestion := m.edit.suggestion
	if suggestion == nil {
		return nil
	}

	if !m.applySuggestion(*suggestion, "accepted") {
		m.setStatus("Suggestion no longer matches exactly one location", "error")
		return m.requestEditSuggestion("Refreshing after a stale edit suggestion...")
	}

	m.edit.suggestion = nil

	return m.requestEditSuggestion("Applied edit and requesting the next suggestion...")
}

func (m *Model) applySuggestion(suggestion editSuggestion, action string) bool {
	updatedBody, ok := document.ReplaceUnique(m.doc.Body, suggestion.OldText, suggestion.NewText)
	if !ok {
		return false
	}
	m.editor.SetValue(updatedBody)
	m.doc.SetBody(updatedBody)
	m.lastEditAt = time.Now()
	m.appendEditHistory(action, suggestion)
	m.edit.suggestion = nil
	if err := m.saveCurrentDocument(); err != nil {
		m.setStatus("Applied suggestion but save failed: "+err.Error(), "error")
	}
	return true
}

func (m *Model) appendEditHistory(action string, suggestion editSuggestion) {
	m.edit.history = append(m.edit.history, editHistoryEntry{
		Action:  action,
		OldText: suggestion.OldText,
		NewText: suggestion.NewText,
	})
	if len(m.edit.history) > 20 {
		m.edit.history = m.edit.history[len(m.edit.history)-20:]
	}
	m.edit.historyIndex = len(m.edit.history) - 1
}

// --- State helpers ---

func (m *Model) setStatus(text, level string) {
	m.statusText = text
	m.statusLevel = level
}

func (m *Model) busy() bool {
	return m.generating || m.edit.requesting || m.edit.reviewing
}

func (m *Model) cancelBusyRequest() {
	if m.generating && m.generationCancel != nil {
		m.generationCancel()
		return
	}
	if m.edit.requesting && m.edit.requestCancel != nil {
		m.edit.requestCancel()
		return
	}
	if m.edit.reviewing && m.edit.requestCancel != nil {
		m.edit.requestCancel()
	}
}

func (m *Model) pushScreen(screen screenState) {
	m.screenPath = append(m.screenPath, screen)
}

func (m *Model) resolveDocumentPath(name string) string {
	if filepath.IsAbs(name) {
		return filepath.Clean(name)
	}
	return filepath.Join(m.cwd, name)
}

func (m *Model) pageEditor(direction int) {
	steps := max(1, m.editor.Height()-1)
	m.scrollTextArea(&m.editor, direction, steps)
}

func (m *Model) scrollTextArea(input *ta.Model, direction, steps int) {
	keyType := tea.KeyDown
	if direction < 0 {
		keyType = tea.KeyUp
	}
	for range max(1, steps) {
		*input, _ = input.Update(tea.KeyMsg{Type: keyType})
	}
}

// --- Tick and channel commands ---

type autosaveTickMsg time.Time

func autosaveTick() tea.Cmd {
	return tea.Tick(autosaveInterval, func(t time.Time) tea.Msg {
		return autosaveTickMsg(t)
	})
}

func waitForStream(ch <-chan llm.StreamEvent, id int) tea.Cmd {
	return func() tea.Msg {
		if ch == nil {
			return nil
		}
		event, ok := <-ch
		if !ok {
			return streamMsg{id: id, event: llm.StreamEvent{Done: true}}
		}
		return streamMsg{id: id, event: event}
	}
}

func waitForEditSuggestion(ch <-chan editSuggestionResult, id int) tea.Cmd {
	return func() tea.Msg {
		if ch == nil {
			return nil
		}
		result, ok := <-ch
		if !ok {
			return editMsg{id: id}
		}
		return editMsg{id: id, result: result}
	}
}

// --- Message building ---

func buildGenerationMessages(body, fileSystemMessage, prompt string, mode generationMode, overrides prompts.Overrides) ([]llm.Message, error) {
	sections := document.SplitSections(body)
	systemPrompt, err := prompts.Render(prompts.GeneratePrompt, overrides, nil)
	if err != nil {
		return nil, fmt.Errorf("render %s: %w", prompts.GeneratePrompt, err)
	}
	messages := []llm.Message{systemPromptMessage(systemPrompt)}

	if strings.TrimSpace(fileSystemMessage) != "" {
		messages = append(messages, documentInstructionsMessage(fileSystemMessage))
	}

	for index, section := range sections {
		contextPrompt, err := prompts.Render(prompts.SectionContextPrompt, overrides, struct {
			Index int
			Total int
		}{
			Index: index + 1,
			Total: len(sections),
		})
		if err != nil {
			return nil, fmt.Errorf("render %s: %w", prompts.SectionContextPrompt, err)
		}
		messages = append(messages,
			contextInstructionsMessage(contextPrompt),
			contentMessage("assistant", section.Markdown),
		)
	}

	actionMsgs, err := actionMessages(mode, sections, prompt, overrides)
	if err != nil {
		return nil, err
	}
	messages = append(messages, actionMsgs...)
	return messages, nil
}

func actionMessages(mode generationMode, sections []document.Section, prompt string, overrides prompts.Overrides) ([]llm.Message, error) {
	var messages []llm.Message

	switch mode {
	case modeContinue:
		data := struct {
			SectionLabel string
			HasExcerpt   bool
		}{}
		if len(sections) == 0 {
			data.SectionLabel = "the start of the document"
			data.HasExcerpt = false
			continuePrompt, err := prompts.Render(prompts.ContinuePrompt, overrides, data)
			if err != nil {
				return nil, fmt.Errorf("render %s: %w", prompts.ContinuePrompt, err)
			}
			messages = append(messages, taskInstructionsMessage(continuePrompt))
		} else {
			lastSection := sections[len(sections)-1]
			lastHeading := lastSection.Heading
			if lastHeading == "" {
				lastHeading = "the current untitled section"
			}
			data.SectionLabel = lastHeading
			data.HasExcerpt = true
			continuePrompt, err := prompts.Render(prompts.ContinuePrompt, overrides, data)
			if err != nil {
				return nil, fmt.Errorf("render %s: %w", prompts.ContinuePrompt, err)
			}
			messages = append(messages,
				taskInstructionsMessage(continuePrompt),
				contentMessage("user", tailExcerpt(lastSection.Markdown, 1200)),
			)
		}
	case modeNewSection:
		newSectionPrompt, err := prompts.Render(prompts.NewSectionPrompt, overrides, nil)
		if err != nil {
			return nil, fmt.Errorf("render %s: %w", prompts.NewSectionPrompt, err)
		}
		messages = append(messages, taskInstructionsMessage(newSectionPrompt))
	}

	if strings.TrimSpace(prompt) != "" {
		userGuidancePrompt, err := prompts.Render(prompts.UserGuidancePrompt, overrides, struct {
			Prompt string
		}{
			Prompt: strings.TrimSpace(prompt),
		})
		if err != nil {
			return nil, fmt.Errorf("render %s: %w", prompts.UserGuidancePrompt, err)
		}
		messages = append(messages, userInstructionsMessage(userGuidancePrompt))
	}

	return messages, nil
}

func fetchEditSuggestion(ctx context.Context, client *llm.Client, body, fileSystemMessage string, history []editHistoryEntry, overrides prompts.Overrides, kind editKind, instructions string) editSuggestionResult {
	if strings.TrimSpace(body) == "" {
		return editSuggestionResult{Note: "The document is empty. Add content before requesting edits."}
	}
	doneNote := "No further high-priority copy edits suggested."
	suggestionNote := "Suggested one copy edit with a unique exact match"
	if kind == editKindDirected {
		doneNote = "The requested directed edit is complete."
		suggestionNote = "Suggested one directed edit with a unique exact match"
	}

	feedback := ""
	for range 3 {
		messages, err := buildEditMessagesWithOptions(body, fileSystemMessage, history, overrides, feedback, kind, instructions)
		if err != nil {
			return editSuggestionResult{Err: err}
		}
		raw, err := client.StructuredChat(ctx, messages, "copy_edit_suggestion", editSuggestionSchema)
		if err != nil {
			return editSuggestionResult{Err: err}
		}

		var suggestion editSuggestion
		if err := json.Unmarshal([]byte(raw), &suggestion); err != nil {
			feedback = "The previous response was not valid JSON for the requested schema. Return only the structured object."
			continue
		}
		if suggestion.empty() {
			return editSuggestionResult{Note: doneNote}
		}
		if suggestion.OldText == suggestion.NewText {
			feedback = "old_text and new_text must differ unless both are empty."
			continue
		}

		matchCount := document.MatchCount(body, suggestion.OldText)
		if matchCount != 1 {
			repaired, repairErr := repairEditSuggestion(ctx, client, body, suggestion, overrides)
			if repairErr != nil {
				return editSuggestionResult{Err: repairErr}
			}
			suggestion = repaired
			if suggestion.empty() {
				return editSuggestionResult{Note: "Could not safely repair an ambiguous edit suggestion."}
			}
			if document.MatchCount(body, suggestion.OldText) != 1 {
				feedback = fmt.Sprintf("The repaired old_text matched %d locations. Return a unique exact excerpt.", document.MatchCount(body, suggestion.OldText))
				continue
			}
		}

		return editSuggestionResult{
			Suggestion: &suggestion,
			Note:       suggestionNote,
		}
	}

	return editSuggestionResult{Err: fmt.Errorf("could not obtain a unique edit suggestion")}
}

func buildEditMessages(body, fileSystemMessage string, history []editHistoryEntry, overrides prompts.Overrides, feedback string) ([]llm.Message, error) {
	return buildEditMessagesWithOptions(body, fileSystemMessage, history, overrides, feedback, editKindCopy, "")
}

func buildEditMessagesWithOptions(body, fileSystemMessage string, history []editHistoryEntry, overrides prompts.Overrides, feedback string, kind editKind, instructions string) ([]llm.Message, error) {
	promptName := prompts.EditPrompt
	if kind == editKindDirected {
		promptName = prompts.DirectedEditPrompt
	}
	systemPrompt, err := prompts.Render(promptName, overrides, nil)
	if err != nil {
		return nil, fmt.Errorf("render %s: %w", promptName, err)
	}
	taskPrompt, err := prompts.Render(prompts.EditTaskPrompt, overrides, nil)
	if err != nil {
		return nil, fmt.Errorf("render %s: %w", prompts.EditTaskPrompt, err)
	}
	messages := []llm.Message{
		systemPromptMessage(systemPrompt),
		taskInstructionsMessage(taskPrompt),
		contentMessage("user", body),
	}
	if kind == editKindDirected {
		messages = append(messages[:1], append([]llm.Message{userInstructionsMessage("Author-directed editing instructions:\n" + strings.TrimSpace(instructions))}, messages[1:]...)...)
	}

	if strings.TrimSpace(fileSystemMessage) != "" {
		messages = append(messages[:1], append([]llm.Message{documentInstructionsMessage(fileSystemMessage)}, messages[1:]...)...)
	}

	if len(history) > 0 {
		historyPrompt, err := prompts.Render(prompts.EditHistoryPrompt, overrides, struct {
			History []editHistoryEntry
		}{
			History: history,
		})
		if err != nil {
			return nil, fmt.Errorf("render %s: %w", prompts.EditHistoryPrompt, err)
		}
		messages = append(messages, historyMessage(historyPrompt))
	}
	if strings.TrimSpace(feedback) != "" {
		feedbackPrompt, err := prompts.Render(prompts.EditFeedbackPrompt, overrides, struct {
			Feedback string
		}{
			Feedback: strings.TrimSpace(feedback),
		})
		if err != nil {
			return nil, fmt.Errorf("render %s: %w", prompts.EditFeedbackPrompt, err)
		}
		messages = append(messages, feedbackMessage(feedbackPrompt))
	}
	return messages, nil
}

func repairEditSuggestion(ctx context.Context, client *llm.Client, body string, suggestion editSuggestion, overrides prompts.Overrides) (editSuggestion, error) {
	prompt, err := prompts.Render(prompts.EditRepairPrompt, overrides, nil)
	if err != nil {
		return editSuggestion{}, err
	}
	raw, err := client.StructuredChat(ctx, []llm.Message{systemPromptMessage(prompt), taskInstructionsMessage(fmt.Sprintf("Invalid suggestion: old_text=%q new_text=%q", suggestion.OldText, suggestion.NewText)), contentMessage("user", body)}, "repaired_edit_suggestion", editSuggestionSchema)
	if err != nil {
		return editSuggestion{}, err
	}
	var repaired editSuggestion
	if err := json.Unmarshal([]byte(raw), &repaired); err != nil {
		return editSuggestion{}, err
	}
	return repaired, nil
}

func approveEditSuggestion(ctx context.Context, client *llm.Client, body string, suggestion editSuggestion, kind editKind, instructions string, overrides prompts.Overrides) (bool, error) {
	prompt, err := prompts.Render(prompts.EditApprovalPrompt, overrides, nil)
	if err != nil {
		return false, err
	}
	mode := "copy editing"
	if kind == editKindDirected {
		mode = "directed editing; author instructions: " + strings.TrimSpace(instructions)
	}
	raw, err := client.StructuredChat(ctx, []llm.Message{systemPromptMessage(prompt), taskInstructionsMessage(fmt.Sprintf("Editing mode: %s\nProposed replacement: old_text=%q new_text=%q", mode, suggestion.OldText, suggestion.NewText)), contentMessage("user", body)}, "edit_auto_approval", editApprovalSchema)
	if err != nil {
		return false, err
	}
	var result struct {
		Approve bool `json:"approve"`
	}
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		return false, err
	}
	return result.Approve, nil
}

// --- Message constructors ---

func contentMessage(role, content string) llm.Message {
	return llm.Message{Role: role, Name: "content", Content: content}
}

func systemPromptMessage(content string) llm.Message {
	return llm.Message{Role: "system", Name: "system_prompt", Content: content}
}

func documentInstructionsMessage(content string) llm.Message {
	return llm.Message{Role: "user", Name: "document_instructions", Content: strings.TrimSpace(content)}
}

func contextInstructionsMessage(content string) llm.Message {
	return llm.Message{Role: "user", Name: "context_instructions", Content: content}
}

func taskInstructionsMessage(content string) llm.Message {
	return llm.Message{Role: "user", Name: "task_instructions", Content: content}
}

func userInstructionsMessage(content string) llm.Message {
	return llm.Message{Role: "user", Name: "user_instructions", Content: content}
}

func historyMessage(content string) llm.Message {
	return llm.Message{Role: "user", Name: "session_history", Content: content}
}

func feedbackMessage(content string) llm.Message {
	return llm.Message{Role: "user", Name: "validation_feedback", Content: content}
}

var editSuggestionSchema = json.RawMessage(`{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "old_text": {
      "type": "string"
    },
    "new_text": {
      "type": "string"
    },
    "remaining_rounds": {
      "type": "integer",
      "minimum": 0
    }
  },
  "required": ["old_text", "new_text", "remaining_rounds"]
}`)

var editApprovalSchema = json.RawMessage(`{
  "type": "object",
  "additionalProperties": false,
  "properties": {"approve": {"type": "boolean"}},
  "required": ["approve"]
}`)

// --- Text normalization ---

func tailExcerpt(value string, maxRunes int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= maxRunes {
		return string(runes)
	}
	return string(runes[len(runes)-maxRunes:])
}

func normalizeGenerationStart(body, delta string, mode generationMode) string {
	if delta == "" {
		return ""
	}

	prefix := generationPrefix(body, delta, mode)
	switch mode {
	case modeNewSection:
		return prefix + strings.TrimLeft(delta, " \t\r\n")
	default:
		return prefix + delta
	}
}

func generationPrefix(body, delta string, mode generationMode) string {
	switch mode {
	case modeContinue:
		return continuationBoundary(body, delta)
	case modeNewSection:
		return newSectionBoundary(body)
	default:
		return ""
	}
}

func continuationBoundary(body, delta string) string {
	if strings.TrimSpace(body) == "" || delta == "" {
		return ""
	}

	first, ok := firstNonWhitespaceRune(delta)
	if !ok {
		return ""
	}

	trimmedBody := strings.TrimRight(body, " \t")
	if trimmedBody == "" || strings.HasSuffix(body, "\n") || strings.HasSuffix(body, " ") || strings.HasSuffix(body, "\t") {
		return ""
	}

	lastLn := lastLine(trimmedBody)
	if looksLikeMarkdownHeading(lastLn) {
		return "\n\n"
	}

	last, ok := lastNonWhitespaceRune(trimmedBody)
	if !ok {
		return ""
	}

	if shouldInsertSpace(last, first) {
		return " "
	}

	return ""
}

func newSectionBoundary(body string) string {
	if strings.TrimSpace(body) == "" {
		return ""
	}

	trailingNewlines := 0
	for i := len(body) - 1; i >= 0 && body[i] == '\n'; i-- {
		trailingNewlines++
	}

	switch {
	case trailingNewlines >= 2:
		return ""
	case trailingNewlines == 1:
		return "\n"
	default:
		return "\n\n"
	}
}

func firstNonWhitespaceRune(value string) (rune, bool) {
	for _, r := range value {
		if !unicode.IsSpace(r) {
			return r, true
		}
	}
	return 0, false
}

func lastNonWhitespaceRune(value string) (rune, bool) {
	runes := []rune(value)
	for i := len(runes) - 1; i >= 0; i-- {
		if !unicode.IsSpace(runes[i]) {
			return runes[i], true
		}
	}
	return 0, false
}

func lastLine(value string) string {
	if index := strings.LastIndex(value, "\n"); index >= 0 {
		return strings.TrimSpace(value[index+1:])
	}
	return strings.TrimSpace(value)
}

func looksLikeMarkdownHeading(line string) bool {
	if line == "" {
		return false
	}
	trimmed := strings.TrimLeft(line, " ")
	if len(trimmed) < 2 || trimmed[0] != '#' {
		return false
	}

	hashes := 0
	for hashes < len(trimmed) && hashes < 6 && trimmed[hashes] == '#' {
		hashes++
	}
	return hashes > 0 && hashes < len(trimmed) && trimmed[hashes] == ' '
}

func shouldInsertSpace(last, first rune) bool {
	if isSpaceJoinRune(last) || isNoLeadingSpaceRune(first) {
		return false
	}
	return isWordJoinRune(last) || isSentenceJoinRune(last)
}

func isSpaceJoinRune(r rune) bool {
	switch r {
	case '(', '[', '{', '/', '\\', '-', '\u2014', '\u2013', '#':
		return true
	default:
		return unicode.IsSpace(r)
	}
}

func isNoLeadingSpaceRune(r rune) bool {
	switch r {
	case '.', ',', '!', '?', ';', ':', ')', ']', '}', '%':
		return true
	default:
		return false
	}
}

func isWordJoinRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsNumber(r)
}

func isSentenceJoinRune(r rune) bool {
	switch r {
	case '.', ',', '!', '?', ';', ':', ')', ']', '}', '"', '\'':
		return true
	default:
		return false
	}
}

// --- Utility ---

func (m Model) paneContentWidth() int {
	return max(20, m.width-6)
}

func truncateToWidth(value string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	if width == 1 {
		return string(runes[:1])
	}
	return string(runes[:width-1]) + "…"
}

func truncateRenderedLine(value string, width int) string {
	if width <= 0 {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= width {
		return value
	}
	if width == 1 {
		return string(runes[:1])
	}
	return string(runes[:width-1]) + "…"
}

func isWheelMouse(msg tea.MouseMsg) bool {
	return msg.Button == tea.MouseButtonWheelUp ||
		msg.Button == tea.MouseButtonWheelDown ||
		msg.Type == tea.MouseWheelUp ||
		msg.Type == tea.MouseWheelDown
}

func isWheelUp(msg tea.MouseMsg) bool {
	return msg.Button == tea.MouseButtonWheelUp || msg.Type == tea.MouseWheelUp
}

func (m *Model) rawWheelDirection(msg tea.KeyMsg) (int, bool) {
	if msg.Type != tea.KeyRunes {
		return 0, false
	}

	value := string(msg.Runes)
	switch {
	case strings.HasPrefix(value, "<64;"), strings.HasPrefix(value, "[<64;"):
		return -1, true
	case strings.HasPrefix(value, "<65;"), strings.HasPrefix(value, "[<65;"):
		return 1, true
	default:
		return 0, false
	}
}

func isTextInputFocus(target focusTarget) bool {
	switch target {
	case focusEditor, focusPrompt, focusFrontMatter:
		return true
	default:
		return false
	}
}

func buttonRowCount(width int, labels []string) int {
	if len(labels) == 0 {
		return 0
	}
	rows := 1
	lineWidth := 0
	maxW := max(20, width)
	for _, label := range labels {
		buttonWidth := lipgloss.Width(" " + label + " ")
		startX := lineWidth
		if lineWidth > 0 {
			startX++
		}
		if startX+buttonWidth > maxW && lineWidth > 0 {
			rows++
			lineWidth = buttonWidth
			continue
		}
		lineWidth = startX + buttonWidth
	}
	return rows
}

func badge(text, background, foreground string) string {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color(foreground)).
		Background(lipgloss.Color(background)).
		Padding(0, 1).
		Render(text)
}

func statusForeground(level string) lipgloss.Color {
	switch level {
	case "error":
		return lipgloss.Color("#FCA5A5")
	case "success":
		return lipgloss.Color("#86EFAC")
	case "muted":
		return lipgloss.Color("#94A3B8")
	default:
		return lipgloss.Color("#E2E8F0")
	}
}

func formatTimestamp(t time.Time) string {
	if t.IsZero() {
		return "just now"
	}
	return t.Format("3:04:05 PM")
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func lineCount(value string) int {
	if value == "" {
		return 1
	}
	return strings.Count(value, "\n") + 1
}

func promptHelpText() string {
	return "Prompt is optional. Continue extends the current section. New Section starts the next heading."
}

func editHelpText() string {
	return "Edit mode requests one exact replacement at a time. Accept applies it. Skip asks for the next suggestion. Automatic modes continue safely and History shows this session's edits."
}

func wrappedLineCount(value string, width int) int {
	if width <= 0 {
		return lineCount(value)
	}
	return lineCount(lipgloss.NewStyle().Width(width).Render(value))
}

func clamp(value, low, high int) int {
	if value < low {
		return low
	}
	if value > high {
		return high
	}
	return value
}

func (m generationMode) label() string {
	switch m {
	case modeContinue:
		return "Continue"
	case modeNewSection:
		return "New section"
	default:
		return "Generation"
	}
}

func (m generationMode) progressLabel() string {
	switch m {
	case modeContinue:
		return "continuation"
	case modeNewSection:
		return "new section"
	default:
		return "generation"
	}
}

func (m workspaceMode) label() string {
	switch m {
	case workspaceEdit:
		return "edit"
	case workspaceDocument:
		return "document"
	default:
		return "generate"
	}
}

func actionForFocus(target focusTarget) (buttonAction, bool) {
	switch target {
	case focusContinueButton:
		return actionContinue, true
	case focusNewSectionButton:
		return actionNewSection, true
	case focusAcceptButton:
		return actionAcceptSuggestion, true
	case focusSkipButton:
		return actionSkipSuggestion, true
	case focusRefreshButton:
		return actionRefreshSuggestion, true
	case focusSaveButton:
		return actionSave, true
	case focusFilesButton:
		return actionFiles, true
	case focusMessageButton:
		return actionToggleMessage, true
	case focusQuitButton:
		return actionQuit, true
	default:
		return "", false
	}
}
