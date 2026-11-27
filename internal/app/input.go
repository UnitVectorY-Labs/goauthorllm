package app

import (
	"path/filepath"
	"time"

	ta "github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/UnitVectorY-Labs/goauthorllm/internal/config"
	"github.com/UnitVectorY-Labs/goauthorllm/internal/document"
	"github.com/UnitVectorY-Labs/goauthorllm/internal/engine"
)

// --- Keyboard Handling ---

func (m *Model) handleKey(msg tea.KeyMsg) (bool, tea.Cmd) {
	k := msg.String()

	// Global Quit
	if k == "ctrl+q" {
		if m.busy() {
			m.stopGeneration()
			m.stopEditPass()
		}
		if err := m.saveBeforeLeave(); err != nil {
			m.setStatus("Save failed on quit: "+err.Error(), "error")
			return true, nil
		}
		return true, tea.Quit
	}

	// Esc: stop active LLM work first, then close overlay, then blur prompt.
	if msg.Type == tea.KeyEsc {
		if m.gen.active || m.gen.planning {
			m.stopGeneration()
			return true, nil
		}
		if m.edit.requesting {
			m.stopEditPass()
			return true, nil
		}
		if m.overlay != overlayNone {
			m.overlay = overlayNone
			m.syncFocus()
			return true, nil
		}
		if m.view == viewWrite && m.writeFocus == writeFocusPrompt {
			m.writeFocus = writeFocusEditor
			m.syncFocus()
			return true, nil
		}
		if m.view == viewEdit && m.edit.focus == editFocusInstructions {
			m.edit.focus = editFocusReview
			m.syncFocus()
			return true, nil
		}
		return true, nil
	}

	// Allow live speed adjustment even while streaming!
	if m.gen.active && (k == "alt+-" || k == "alt+=" || k == "alt++") {
		if k == "alt+-" {
			m.cycleStreamSpeed(-1)
		} else {
			m.cycleStreamSpeed(1)
		}
		return true, nil
	}

	// Overlay-specific keys
	if m.overlay != overlayNone {
		return m.handleOverlayKey(msg)
	}

	// Global shortcuts
	switch k {
	case "f1", "alt+/":
		m.overlay = overlayHelp
		m.helpScroll = 0
		m.syncFocus()
		return true, nil
	case "alt+e":
		if m.lastError != "" {
			m.overlay = overlayError
			m.errorScroll = 0
			m.syncFocus()
			return true, nil
		}
	case "alt+m":
		if m.doc != nil && !m.busy() {
			m.overlay = overlayMeta
			m.syncFocus()
			return true, nil
		}
	case "alt+1":
		m.switchView(viewWrite)
		return true, nil
	case "alt+2":
		m.switchView(viewEdit)
		return true, nil
	case "alt+3":
		m.switchView(viewConfig)
		return true, nil
	case "alt+4", "ctrl+o":
		m.switchView(viewFiles)
		return true, nil
	case "ctrl+s":
		if m.view == viewConfig {
			m.saveAndApplyConfig()
			return true, nil
		}
		if m.doc != nil {
			if err := m.saveCurrentDocument(); err != nil {
				m.setStatus("Save failed: "+err.Error(), "error")
			} else {
				m.setStatus("Saved "+m.doc.LastSavedAt.Format("15:04:05"), "success")
			}
			return true, nil
		}
	}

	if m.busy() {
		return true, nil
	}

	switch m.view {
	case viewWrite:
		return m.handleWriteKey(msg)
	case viewEdit:
		return m.handleEditKey(msg)
	case viewConfig:
		return m.handleConfigKey(msg)
	case viewFiles:
		return m.handleFilesKey(msg)
	}
	return false, nil
}

func (m *Model) handleOverlayKey(msg tea.KeyMsg) (bool, tea.Cmd) {
	switch m.overlay {
	case overlayMeta:
		if msg.String() == "ctrl+s" {
			if err := m.saveCurrentDocument(); err != nil {
				m.setStatus("Save failed: "+err.Error(), "error")
			} else {
				m.setStatus("Saved front matter", "success")
			}
			return true, nil
		}
		return false, nil

	case overlayHelp:
		switch msg.Type {
		case tea.KeyUp:
			m.helpScroll = max(0, m.helpScroll-1)
		case tea.KeyDown:
			m.helpScroll++
		case tea.KeyPgUp:
			m.helpScroll = max(0, m.helpScroll-10)
		case tea.KeyPgDown:
			m.helpScroll += 10
		}
		return true, nil

	case overlayError:
		switch msg.Type {
		case tea.KeyUp:
			m.errorScroll = max(0, m.errorScroll-1)
		case tea.KeyDown:
			m.errorScroll++
		case tea.KeyPgUp:
			m.errorScroll = max(0, m.errorScroll-10)
		case tea.KeyPgDown:
			m.errorScroll += 10
		}
		return true, nil

	case overlayModelPicker:
		models := m.filteredModels()
		switch msg.Type {
		case tea.KeyUp:
			if m.modelPickerIndex > 0 {
				m.modelPickerIndex--
			}
			return true, nil
		case tea.KeyDown:
			if m.modelPickerIndex < len(models)-1 {
				m.modelPickerIndex++
			}
			return true, nil
		case tea.KeyEnter:
			if len(models) > 0 && m.modelPickerIndex < len(models) {
				m.pickModel(models[m.modelPickerIndex])
			}
			return true, nil
		}
		return false, nil
	}
	return false, nil
}

func (m *Model) handleWriteKey(msg tea.KeyMsg) (bool, tea.Cmd) {
	k := msg.String()
	switch k {
	case "tab", "shift+tab":
		if m.writeFocus == writeFocusEditor {
			m.promptOpen = true
			m.writeFocus = writeFocusPrompt
		} else {
			m.writeFocus = writeFocusEditor
		}
		m.resizeInputs()
		m.syncFocus()
		return true, nil
	case "ctrl+g":
		return true, m.startGeneration(0)
	case "ctrl+r":
		return true, m.startGeneration(1)
	case "alt+s":
		m.cycleScope(1)
		return true, nil
	case "alt+[":
		m.adjustRounds(-1)
		return true, nil
	case "alt+]":
		m.adjustRounds(1)
		return true, nil
	case "alt+-":
		m.cycleStreamSpeed(-1)
		return true, nil
	case "alt+=", "alt++":
		m.cycleStreamSpeed(1)
		return true, nil
	}

	if m.writeFocus == writeFocusPrompt {
		if k == "alt+up" || k == "alt+down" {
			m.recallInstruction("generation", &m.prompt, k == "alt+up")
			return true, nil
		}
		if msg.Type == tea.KeyEnter && !msg.Alt {
			return true, m.startGeneration(0)
		}
		if k == "ctrl+j" || (msg.Type == tea.KeyEnter && msg.Alt) {
			var cmd tea.Cmd
			m.prompt, cmd = m.prompt.Update(tea.KeyMsg{Type: tea.KeyEnter})
			return true, cmd
		}
		return false, nil
	}

	// Document editor shortcuts
	switch k {
	case "ctrl+z":
		m.undoBody()
		return true, nil
	case "alt+k":
		m.deleteToEnd()
		return true, nil
	case "pgup":
		scrollTextarea(&m.editor, -1, max(1, m.editor.Height()-1))
		return true, nil
	case "pgdown":
		scrollTextarea(&m.editor, 1, max(1, m.editor.Height()-1))
		return true, nil
	}
	return false, nil
}

func (m *Model) handleEditKey(msg tea.KeyMsg) (bool, tea.Cmd) {
	k := msg.String()
	switch k {
	case "tab", "shift+tab":
		if m.edit.focus == editFocusInstructions {
			if m.edit.subTab == editTabDocument {
				m.edit.focus = editFocusDocument
			} else {
				m.edit.focus = editFocusReview
			}
		} else {
			m.edit.focus = editFocusInstructions
		}
		m.syncFocus()
		return true, nil
	case "ctrl+g":
		return true, m.startEditPass(false)
	case "alt+a":
		if m.edit.current != nil {
			return true, m.approveAllRemaining()
		}
		return true, m.startEditPass(true)
	case "ctrl+a":
		return true, m.acceptCurrentEdit()
	case "ctrl+k":
		return true, m.skipCurrentEdit()
	case "ctrl+r":
		return true, m.startEditPass(false)
	}

	if m.edit.focus == editFocusInstructions {
		if k == "alt+up" || k == "alt+down" {
			m.recallInstruction("directed", &m.edit.instructions, k == "alt+up")
			return true, nil
		}
		if msg.Type == tea.KeyEnter && !msg.Alt {
			return true, m.startEditPass(false)
		}
		if k == "ctrl+j" || (msg.Type == tea.KeyEnter && msg.Alt) {
			var cmd tea.Cmd
			m.edit.instructions, cmd = m.edit.instructions.Update(tea.KeyMsg{Type: tea.KeyEnter})
			return true, cmd
		}
		return false, nil
	}

	if m.edit.focus == editFocusDocument {
		if k == "ctrl+z" {
			m.undoBody()
			return true, nil
		}
		return false, nil
	}

	// Focused on Diff/Log review pane: crisp single-key ergonomics!
	switch k {
	case "y":
		return true, m.acceptCurrentEdit()
	case "n":
		return true, m.skipCurrentEdit()
	case "a":
		return true, m.approveAllRemaining()
	case "r":
		return true, m.startEditPass(false)
	case "u", "ctrl+z":
		m.undoLastAppliedEdit()
		return true, nil
	case "up":
		m.edit.reviewScroll = max(0, m.edit.reviewScroll-1)
		return true, nil
	case "down":
		m.edit.reviewScroll++
		return true, nil
	case "pgup":
		m.edit.reviewScroll = max(0, m.edit.reviewScroll-10)
		return true, nil
	case "pgdown":
		m.edit.reviewScroll += 10
		return true, nil
	case "left":
		if m.edit.subTab == editTabLog && len(m.edit.history) > 0 {
			m.edit.logIndex = clampInt(m.edit.logIndex-1, 0, len(m.edit.history)-1)
			m.edit.reviewScroll = 0
		} else {
			m.edit.subTab = editSubTab((int(m.edit.subTab) + 2) % 3)
		}
		return true, nil
	case "right":
		if m.edit.subTab == editTabLog && len(m.edit.history) > 0 {
			m.edit.logIndex = clampInt(m.edit.logIndex+1, 0, len(m.edit.history)-1)
			m.edit.reviewScroll = 0
		} else {
			m.edit.subTab = editSubTab((int(m.edit.subTab) + 1) % 3)
		}
		return true, nil
	}
	return false, nil
}

func (m *Model) handleConfigKey(msg tea.KeyMsg) (bool, tea.Cmd) {
	k := msg.String()
	switch k {
	case "alt+y":
		m.toggleConfigRawMode(!m.cfgUI.rawMode)
		return true, nil
	case "alt+f":
		target := cfgFieldModel
		if m.cfgUI.activeField == cfgFieldGenModel || m.cfgUI.activeField == cfgFieldEditModel {
			target = m.cfgUI.activeField
		}
		return true, m.fetchModels(target)
	}

	if m.cfgUI.rawMode {
		return false, nil
	}

	switch k {
	case "tab":
		m.selectConfigField((m.cfgUI.activeField + 1) % cfgFieldCount)
		return true, nil
	case "shift+tab":
		m.selectConfigField((m.cfgUI.activeField - 1 + cfgFieldCount) % cfgFieldCount)
		return true, nil
	case "up":
		if m.cfgUI.activeField != cfgFieldRules && m.cfgUI.activeField != cfgFieldCopyEditRules {
			m.selectConfigField(max(0, m.cfgUI.activeField-1))
			return true, nil
		}
	case "down":
		if m.cfgUI.activeField != cfgFieldRules && m.cfgUI.activeField != cfgFieldCopyEditRules {
			m.selectConfigField(min(cfgFieldCount-1, m.cfgUI.activeField+1))
			return true, nil
		}
	case "left", "right", " ":
		if m.cfgUI.activeField == cfgFieldDefaultScope {
			idx := 0
			for i, sc := range config.ValidScopes {
				if sc == m.cfgUI.defaultScope {
					idx = i
					break
				}
			}
			delta := 1
			if k == "left" {
				delta = -1
			}
			m.cfgUI.defaultScope = config.ValidScopes[(idx+delta+len(config.ValidScopes))%len(config.ValidScopes)]
			return true, nil
		}
		if m.cfgUI.activeField == cfgFieldStructuredGen {
			m.cfgUI.structuredGen = !m.cfgUI.structuredGen
			return true, nil
		}
	}
	return false, nil
}

func (m *Model) handleFilesKey(msg tea.KeyMsg) (bool, tea.Cmd) {
	k := msg.String()
	switch k {
	case "ctrl+r":
		if err := m.refreshFiles(); err != nil {
			m.setStatus("Refresh failed: "+err.Error(), "error")
		} else {
			m.setStatus("Refreshed file list", "success")
		}
		return true, nil
	case "tab", "shift+tab":
		if m.files.focus == filesFocusList {
			m.files.focus = filesFocusInput
		} else {
			m.files.focus = filesFocusList
		}
		m.syncFocus()
		return true, nil
	case "up":
		if m.files.focus == filesFocusList && m.files.selected > 0 {
			m.files.selected--
			return true, nil
		}
	case "down":
		if m.files.focus == filesFocusList && m.files.selected < len(m.files.files)-1 {
			m.files.selected++
			return true, nil
		}
	case "enter":
		if m.files.focus == filesFocusInput {
			return true, m.openTypedFilename()
		}
		if len(m.files.files) > 0 && m.files.selected < len(m.files.files) {
			return true, m.openFileByName(m.files.files[m.files.selected])
		}
		return true, nil
	}
	return false, nil
}

func (m *Model) openFileByName(name string) tea.Cmd {
	if err := m.saveBeforeLeave(); err != nil {
		m.setStatus("Save current file failed: "+err.Error(), "error")
		return nil
	}
	path := m.resolvePath(name)
	if err := m.openDocument(path); err != nil {
		m.setStatus("Open failed: "+err.Error(), "error")
		return nil
	}
	m.view = viewWrite
	m.writeFocus = writeFocusEditor
	m.resizeInputs()
	m.syncFocus()
	m.setStatus("Opened "+filepath.Base(path), "success")
	return nil
}

func (m *Model) openTypedFilename() tea.Cmd {
	name := document.NormalizeMarkdownFilename(m.files.input.Value())
	if name == "" {
		m.setStatus("Type a markdown filename first", "warn")
		return nil
	}
	m.files.input.SetValue("")
	return m.openFileByName(name)
}

// --- Mouse Handling ---

func (m *Model) handleMouse(msg tea.MouseMsg) (bool, tea.Cmd) {
	// Hover tracking
	if msg.Action == tea.MouseActionMotion {
		if target, ok := m.hits.hit(msg.X, msg.Y); ok {
			if m.hoverID != target.ID || m.hoverIndex != target.Index {
				m.hoverID = target.ID
				m.hoverIndex = target.Index
				return true, nil
			}
			return false, nil
		}
		if m.hoverID != "" {
			m.hoverID = ""
			m.hoverIndex = -1
			return true, nil
		}
		return false, nil
	}

	// Scroll wheel — scrolls whichever pane the cursor is over!
	if isWheel(msg) {
		dir := 1
		if msg.Button == tea.MouseButtonWheelUp {
			dir = -1
		}
		return m.handleWheel(msg.X, msg.Y, dir), nil
	}

	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return false, nil
	}

	target, ok := m.hits.hit(msg.X, msg.Y)
	if !ok {
		return false, nil
	}

	return true, m.dispatchClick(target)
}

func isWheel(msg tea.MouseMsg) bool {
	return msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown
}

func (m *Model) handleWheel(x, y, dir int) bool {
	if m.gen.active {
		return true
	}
	pane := m.hits.paneAt(x, y)
	switch pane {
	case "editor":
		scrollTextarea(&m.editor, dir, 3)
		return true
	case "prompt":
		scrollTextarea(&m.prompt, dir, 3)
		return true
	case "edit-instr":
		scrollTextarea(&m.edit.instructions, dir, 2)
		return true
	case "review":
		m.edit.reviewScroll = max(0, m.edit.reviewScroll+dir*3)
		return true
	case "frontmatter":
		scrollTextarea(&m.frontMatter, dir, 3)
		return true
	case "help":
		m.helpScroll = max(0, m.helpScroll+dir*3)
		return true
	case "error":
		m.errorScroll = max(0, m.errorScroll+dir*3)
		return true
	case "cfg-rules":
		scrollTextarea(&m.cfgUI.rules, dir, 2)
		return true
	case "cfg-copy-rules":
		scrollTextarea(&m.cfgUI.copyEditRules, dir, 2)
		return true
	case "cfg-raw":
		scrollTextarea(&m.cfgUI.rawYAML, dir, 3)
		return true
	case "files-list":
		if len(m.files.files) > 0 {
			m.files.selected = clampInt(m.files.selected+dir, 0, len(m.files.files)-1)
		}
		return true
	}
	// Fallback: scroll active primary area
	if m.overlay == overlayHelp {
		m.helpScroll = max(0, m.helpScroll+dir*3)
		return true
	}
	if m.overlay == overlayError {
		m.errorScroll = max(0, m.errorScroll+dir*3)
		return true
	}
	if m.view == viewWrite {
		scrollTextarea(&m.editor, dir, 3)
		return true
	}
	if m.view == viewEdit && m.edit.subTab != editTabDocument {
		m.edit.reviewScroll = max(0, m.edit.reviewScroll+dir*3)
		return true
	}
	return false
}

func (m *Model) dispatchClick(t hitTarget) tea.Cmd {
	switch t.ID {
	case "close-overlay":
		m.overlay = overlayNone
		m.syncFocus()
	case "show-error":
		if m.lastError != "" {
			m.overlay = overlayError
			m.errorScroll = 0
			m.syncFocus()
		}
	case "help":
		if m.overlay == overlayHelp {
			m.overlay = overlayNone
		} else {
			m.overlay = overlayHelp
			m.helpScroll = 0
		}
		m.syncFocus()
	case "meta":
		if m.doc != nil && !m.busy() {
			m.overlay = overlayMeta
			m.syncFocus()
		}
	case "quit":
		if m.busy() {
			m.stopGeneration()
			m.stopEditPass()
		}
		if err := m.saveBeforeLeave(); err != nil {
			m.setStatus("Save failed: "+err.Error(), "error")
			return nil
		}
		return tea.Quit
	case "save":
		if m.doc != nil {
			if err := m.saveCurrentDocument(); err != nil {
				m.setStatus("Save failed: "+err.Error(), "error")
			} else {
				m.setStatus("Saved "+m.doc.LastSavedAt.Format("15:04:05"), "success")
			}
		}
	case "tab":
		m.switchView(workspaceView(t.Index))

	// Write view controls
	case "scope":
		scopes := []engine.Scope{engine.ScopeSentence, engine.ScopeParagraph, engine.ScopeSection, engine.ScopeFree}
		if t.Index >= 0 && t.Index < len(scopes) {
			m.gen.scope = scopes[t.Index]
			m.setStatus("Generation scope: "+m.gen.scope.Label(), "accent")
		}
	case "rounds-dec":
		m.adjustRounds(-1)
	case "rounds-inc":
		m.adjustRounds(1)
	case "speed":
		m.cycleStreamSpeed(1)
	case "generate":
		return m.startGeneration(0)
	case "plus-one":
		return m.startGeneration(1)
	case "stop-gen":
		m.stopGeneration()
	case "undo":
		m.undoBody()
	case "toggle-prompt":
		if m.isCompactHeight() {
			if m.writeFocus == writeFocusPrompt {
				m.writeFocus = writeFocusEditor
			} else {
				m.writeFocus = writeFocusPrompt
			}
		} else {
			m.promptOpen = !m.promptOpen
			if m.promptOpen {
				m.writeFocus = writeFocusPrompt
			} else {
				m.writeFocus = writeFocusEditor
			}
		}
		m.resizeInputs()
		m.syncFocus()
	case "pane-editor":
		if m.view == viewWrite {
			m.writeFocus = writeFocusEditor
		} else if m.view == viewEdit {
			m.edit.focus = editFocusDocument
		}
		m.syncFocus()
	case "pane-prompt":
		m.writeFocus = writeFocusPrompt
		m.syncFocus()

	// Edit view controls
	case "pane-edit-instr":
		m.edit.focus = editFocusInstructions
		m.syncFocus()
	case "pane-review":
		m.edit.focus = editFocusReview
		m.syncFocus()
	case "edit-tab":
		m.edit.subTab = editSubTab(t.Index)
		m.edit.reviewScroll = 0
		if m.edit.subTab == editTabDocument {
			m.edit.focus = editFocusDocument
		} else {
			m.edit.focus = editFocusReview
		}
		m.syncFocus()
	case "edit-review":
		return m.startEditPass(false)
	case "edit-approve-all":
		return m.startEditPass(true)
	case "stop-edit":
		m.stopEditPass()
	case "edit-accept":
		return m.acceptCurrentEdit()
	case "edit-skip":
		return m.skipCurrentEdit()
	case "edit-approve-rem":
		return m.approveAllRemaining()
	case "edit-undo":
		m.undoLastAppliedEdit()

	// Config view controls
	case "cfg-mode":
		m.toggleConfigRawMode(t.Index == 1)
	case "cfg-save":
		m.saveAndApplyConfig()
	case "cfg-reload":
		m.loadConfigIntoEditor()
		m.setStatus("Reloaded .goauthorllm", "info")
	case "cfg-fetch":
		return m.fetchModels(t.Index)
	case "cfg-field":
		m.selectConfigField(t.Index)
	case "cfg-scope":
		if t.Index >= 0 && t.Index < len(config.ValidScopes) {
			m.cfgUI.defaultScope = config.ValidScopes[t.Index]
			m.selectConfigField(cfgFieldDefaultScope)
		}
	case "cfg-struct":
		m.cfgUI.structuredGen = t.Index == 1
		m.selectConfigField(cfgFieldStructuredGen)
	case "model-pick":
		models := m.filteredModels()
		if t.Index >= 0 && t.Index < len(models) {
			m.pickModel(models[t.Index])
		}

	// Files view controls
	case "files-refresh":
		if err := m.refreshFiles(); err != nil {
			m.setStatus("Refresh failed: "+err.Error(), "error")
		} else {
			m.setStatus("Refreshed files", "success")
		}
	case "file-row":
		if t.Index >= 0 && t.Index < len(m.files.files) {
			m.files.selected = t.Index
			m.files.focus = filesFocusList
			return m.openFileByName(m.files.files[t.Index])
		}
	case "pane-file-input":
		m.files.focus = filesFocusInput
		m.syncFocus()
	}
	return nil
}

// --- Input Delegation ---

func (m *Model) delegateInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.overlay == overlayMeta {
		before := m.frontMatter.Value()
		var cmd tea.Cmd
		m.frontMatter, cmd = m.frontMatter.Update(msg)
		if before != m.frontMatter.Value() && m.doc != nil {
			m.doc.SetFrontMatter(m.frontMatter.Value())
			m.lastEditAt = time.Now()
		}
		return m, cmd
	}
	if m.overlay == overlayModelPicker {
		var cmd tea.Cmd
		m.modelPickerFilter, cmd = m.modelPickerFilter.Update(msg)
		m.modelPickerIndex = 0
		return m, cmd
	}
	if m.overlay != overlayNone {
		return m, nil
	}

	switch m.view {
	case viewWrite:
		if m.writeFocus == writeFocusPrompt {
			var cmd tea.Cmd
			m.prompt, cmd = m.prompt.Update(msg)
			return m, cmd
		}
		return m.updateDocumentEditor(msg)

	case viewEdit:
		switch m.edit.focus {
		case editFocusInstructions:
			var cmd tea.Cmd
			m.edit.instructions, cmd = m.edit.instructions.Update(msg)
			return m, cmd
		case editFocusDocument:
			return m.updateDocumentEditor(msg)
		}
		return m, nil

	case viewConfig:
		return m.updateConfigInputs(msg)

	case viewFiles:
		if m.files.focus == filesFocusInput {
			var cmd tea.Cmd
			m.files.input, cmd = m.files.input.Update(msg)
			return m, cmd
		}
	}
	return m, nil
}

func (m *Model) updateDocumentEditor(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.gen.active {
		return m, nil
	}
	before := m.editor.Value()
	var cmd tea.Cmd
	m.editor, cmd = m.editor.Update(msg)
	if before != m.editor.Value() && m.doc != nil {
		if time.Since(m.lastUndoEditAt) > 750*time.Millisecond {
			m.pushUndo(before)
		}
		m.lastUndoEditAt = time.Now()
		m.doc.SetBody(m.editor.Value())
		m.lastEditAt = time.Now()
	}
	return m, cmd
}

func (m *Model) updateConfigInputs(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	if m.cfgUI.rawMode {
		m.cfgUI.rawYAML, cmd = m.cfgUI.rawYAML.Update(msg)
		return m, cmd
	}
	switch m.cfgUI.activeField {
	case cfgFieldBaseURL:
		m.cfgUI.baseURL, cmd = m.cfgUI.baseURL.Update(msg)
	case cfgFieldModel:
		m.cfgUI.model, cmd = m.cfgUI.model.Update(msg)
	case cfgFieldGenModel:
		m.cfgUI.genModel, cmd = m.cfgUI.genModel.Update(msg)
	case cfgFieldEditModel:
		m.cfgUI.editModel, cmd = m.cfgUI.editModel.Update(msg)
	case cfgFieldStreamSpeed:
		m.cfgUI.streamSpeed, cmd = m.cfgUI.streamSpeed.Update(msg)
	case cfgFieldCopyBatch:
		m.cfgUI.copyBatch, cmd = m.cfgUI.copyBatch.Update(msg)
	case cfgFieldDirectedBatch:
		m.cfgUI.directedBatch, cmd = m.cfgUI.directedBatch.Update(msg)
	case cfgFieldRules:
		m.cfgUI.rules, cmd = m.cfgUI.rules.Update(msg)
	case cfgFieldCopyEditRules:
		m.cfgUI.copyEditRules, cmd = m.cfgUI.copyEditRules.Update(msg)
	}
	return m, cmd
}

func scrollTextarea(input *ta.Model, dir, steps int) {
	keyType := tea.KeyDown
	if dir < 0 {
		keyType = tea.KeyUp
	}
	for range max(1, steps) {
		*input, _ = input.Update(tea.KeyMsg{Type: keyType})
	}
}
