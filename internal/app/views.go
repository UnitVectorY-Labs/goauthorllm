package app

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"

	"github.com/UnitVectorY-Labs/goauthorllm/internal/config"
	"github.com/UnitVectorY-Labs/goauthorllm/internal/engine"
)

func (m *Model) isCompactHeight() bool { return m.height < 23 }

func (m *Model) resizeInputs() {
	if m.width <= 0 || m.height <= 0 {
		return
	}
	innerW := max(16, m.width-6)

	// Overlays
	m.frontMatter.SetWidth(innerW)
	m.frontMatter.SetHeight(max(5, m.height-9))
	m.modelPickerFilter.Width = max(16, innerW-4)

	// Files
	m.files.input.Width = max(16, innerW-4)

	// Config
	fieldW := max(16, innerW-26)
	m.cfgUI.baseURL.Width = fieldW
	m.cfgUI.model.Width = max(12, fieldW-16)
	m.cfgUI.genModel.Width = max(12, fieldW-16)
	m.cfgUI.editModel.Width = max(12, fieldW-16)
	m.cfgUI.streamSpeed.Width = max(12, fieldW)
	m.cfgUI.copyBatch.Width = max(12, fieldW)
	m.cfgUI.directedBatch.Width = max(12, fieldW)
	ruleH := clampInt((m.height-20)/2, 2, 5)
	m.cfgUI.rules.SetWidth(innerW)
	m.cfgUI.rules.SetHeight(ruleH)
	m.cfgUI.copyEditRules.SetWidth(innerW)
	m.cfgUI.copyEditRules.SetHeight(ruleH)
	m.cfgUI.rawYAML.SetWidth(innerW)
	m.cfgUI.rawYAML.SetHeight(max(6, m.height-10))

	// Write view sizing: header (2) + controls (1-2) + status (1) + borders
	promptVisible := m.shouldShowPromptBox()
	if m.isCompactHeight() && m.writeFocus == writeFocusPrompt && promptVisible {
		// On short terminals, give the prompt full height when focused so typing is comfortable!
		m.prompt.SetWidth(innerW)
		m.prompt.SetHeight(max(4, m.height-10))
		m.editor.SetWidth(innerW)
		m.editor.SetHeight(max(4, m.height-10))
	} else if promptVisible {
		promptH := clampInt(m.height/6, 3, 5)
		editorH := max(4, m.height-12-promptH)
		m.editor.SetWidth(innerW)
		m.editor.SetHeight(editorH)
		m.prompt.SetWidth(innerW)
		m.prompt.SetHeight(promptH)
	} else {
		m.editor.SetWidth(innerW)
		m.editor.SetHeight(max(4, m.height-9))
		m.prompt.SetWidth(innerW)
		m.prompt.SetHeight(4)
	}

	// Edit view sizing
	instrH := 2
	if m.height >= 28 {
		instrH = 3
	}
	m.edit.instructions.SetWidth(innerW)
	m.edit.instructions.SetHeight(instrH)
}

func (m *Model) shouldShowPromptBox() bool {
	if m.isCompactHeight() {
		return m.writeFocus == writeFocusPrompt
	}
	return m.promptOpen
}

// View renders the active workspace or overlay and updates exact hitboxes.
func (m *Model) View() string {
	if m.width <= 0 || m.height <= 0 {
		return "Loading goauthorllm..."
	}
	m.resizeInputs()

	fb := newFrameBuilder()

	switch m.overlay {
	case overlayMeta:
		m.renderMetaOverlay(fb)
	case overlayHelp:
		m.renderHelpOverlay(fb)
	case overlayError:
		m.renderErrorOverlay(fb)
	case overlayModelPicker:
		m.renderModelPickerOverlay(fb)
	default:
		m.renderHeaderBar(fb)
		switch m.view {
		case viewWrite:
			m.renderWriteView(fb)
		case viewEdit:
			m.renderEditView(fb)
		case viewConfig:
			m.renderConfigView(fb)
		case viewFiles:
			m.renderFilesView(fb)
		}
		m.renderStatusFooter(fb)
	}

	m.hits = fb.hits

	lines := fb.lines
	if len(lines) < m.height {
		for len(lines) < m.height {
			lines = append(lines, "")
		}
	} else if len(lines) > m.height {
		lines = lines[:m.height]
	}
	for i := range lines {
		lines[i] = ansi.Truncate(lines[i], m.width, "")
	}
	return strings.Join(lines, "\n")
}

func (m *Model) isHovered(id string, idx int) bool {
	return m.hoverID == id && m.hoverIndex == idx
}

func (m *Model) renderHeaderBar(fb *frameBuilder) {
	items := []rowItem{
		{Content: styleBrand.Render("◆ goauthorllm"), Gap: 2},
		{ID: "tab", Index: int(viewWrite), Content: renderTab("1 Write", m.view == viewWrite, m.isHovered("tab", int(viewWrite))), Gap: 1},
		{ID: "tab", Index: int(viewEdit), Content: renderTab("2 Edit", m.view == viewEdit, m.isHovered("tab", int(viewEdit))), Gap: 1},
		{ID: "tab", Index: int(viewConfig), Content: renderTab("3 Config", m.view == viewConfig, m.isHovered("tab", int(viewConfig))), Gap: 1},
		{ID: "tab", Index: int(viewFiles), Content: renderTab("4 Files", m.view == viewFiles, m.isHovered("tab", int(viewFiles))), Gap: 2},
	}

	if m.doc != nil {
		name := truncate(filepath.Base(m.doc.Path), max(10, m.width/5))
		items = append(items, rowItem{Content: styleTitle.Render(name), Gap: 1})
		if m.doc.Dirty {
			items = append(items, rowItem{ID: "save", Index: 0, Content: renderBadge("● unsaved", colorAmberDark, colorAmberSoft), Gap: 1})
		} else {
			items = append(items, rowItem{ID: "save", Index: 0, Content: renderBadge("saved", colorGreenDark, colorGreenSoft), Gap: 1})
		}
		items = append(items, rowItem{ID: "meta", Index: 0, Content: renderSecondaryBtn("Meta", m.isHovered("meta", 0)), Gap: 1})
	}
	items = append(items,
		rowItem{ID: "help", Index: 0, Content: renderSecondaryBtn("?", m.isHovered("help", 0)), Gap: 1},
		rowItem{ID: "quit", Index: 0, Content: renderSecondaryBtn("Quit", m.isHovered("quit", 0)), Gap: 0},
	)
	fb.addRow(items, m.width)

	// Thin orange accent divider line.
	divider := lipgloss.NewStyle().Foreground(lipgloss.Color(colorOrangeDeep)).Render(strings.Repeat("─", max(1, m.width)))
	fb.addBlock(divider)
}

func (m *Model) renderWriteView(fb *frameBuilder) {
	// Control strip: Scope pills + Rounds + Speed + Generate/Stop + Prompt toggle
	var controls []rowItem
	scopes := []engine.Scope{engine.ScopeSentence, engine.ScopeParagraph, engine.ScopeSection, engine.ScopeFree}
	for i, sc := range scopes {
		controls = append(controls, rowItem{
			ID:      "scope",
			Index:   i,
			Content: renderPill(sc.Label(), m.gen.scope == sc, m.isHovered("scope", i)),
			Gap:     1,
		})
	}

	controls = append(controls,
		rowItem{ID: "rounds-dec", Index: 0, Content: renderSecondaryBtn("−", m.isHovered("rounds-dec", 0)), Gap: 0},
		rowItem{ID: "rounds-inc", Index: 0, Content: renderPill(fmt.Sprintf("×%d", m.gen.rounds), m.gen.rounds > 1, m.isHovered("rounds-inc", 0)), Gap: 0},
		rowItem{ID: "rounds-inc", Index: 1, Content: renderSecondaryBtn("+", m.isHovered("rounds-inc", 1)), Gap: 1},
		rowItem{ID: "speed", Index: 0, Content: renderPill("Speed: "+formatSpeedLabel(m.cfg.StreamSpeed), m.cfg.StreamSpeed > 0, m.isHovered("speed", 0)), Gap: 1},
	)

	if m.gen.active || m.gen.planning {
		controls = append(controls, rowItem{
			ID:      "stop-gen",
			Index:   0,
			Content: renderDangerBtn("■ Stop [Esc]", m.isHovered("stop-gen", 0)),
			Gap:     1,
		})
	} else {
		controls = append(controls,
			rowItem{ID: "generate", Index: 0, Content: renderPrimaryBtn("▶ Generate [Ctrl+G]", m.isHovered("generate", 0)), Gap: 1},
			rowItem{ID: "plus-one", Index: 0, Content: renderSecondaryBtn("+1 Round [Ctrl+R]", m.isHovered("plus-one", 0)), Gap: 1},
			rowItem{ID: "undo", Index: 0, Content: renderSecondaryBtn("↩ Undo", m.isHovered("undo", 0)), Gap: 1},
		)
	}

	promptBtnLabel := "✎ Prompt"
	if strings.TrimSpace(m.prompt.Value()) != "" {
		promptBtnLabel = "✎ Prompt ●"
	}
	controls = append(controls, rowItem{
		ID:      "toggle-prompt",
		Index:   0,
		Content: renderPill(promptBtnLabel, m.shouldShowPromptBox(), m.isHovered("toggle-prompt", 0)),
		Gap:     0,
	})
	fb.addRow(controls, m.width)

	// Main Document or Full-height Prompt on very compact terminals
	if m.isCompactHeight() && m.writeFocus == writeFocusPrompt {
		promptBox := renderBox(
			"Guidance Prompt",
			"Enter: generate · Alt+Enter: newline · Alt+↑/↓: history · Tab: document",
			m.prompt.View(),
			m.width,
			true,
			m.hoverID == "pane-prompt",
		)
		r := fb.addBlock(promptBox)
		fb.hits.setPane("prompt", r)
		fb.hits.add("pane-prompt", 0, r)
		return
	}

	docTitle := "Document"
	if m.gen.planning {
		docTitle = fmt.Sprintf("Document · %s Planning %d rounds...", m.spin.View(), m.gen.totalRounds)
	} else if m.gen.active {
		roundTxt := ""
		if m.gen.totalRounds > 1 {
			roundTxt = fmt.Sprintf("Round %d/%d ", m.gen.currentRound, m.gen.totalRounds)
		}
		intentTxt := ""
		if m.gen.currentIntent != "" {
			intentTxt = "— " + truncate(m.gen.currentIntent, 34) + " "
		}
		docTitle = fmt.Sprintf("Document · %s %s%s(%s)", m.spin.View(), roundTxt, intentTxt, formatSpeedLabel(m.cfg.StreamSpeed))
	}
	docMeta := fmt.Sprintf("%d words · Ctrl+Z undo · Alt+K trim end · Tab prompt", m.wordCount())
	docBox := renderBox(docTitle, docMeta, m.editor.View(), m.width, m.writeFocus == writeFocusEditor, m.hoverID == "pane-editor")
	edRect := fb.addBlock(docBox)
	fb.hits.setPane("editor", edRect)
	fb.hits.add("pane-editor", 0, edRect)

	if m.shouldShowPromptBox() {
		promptMeta := "Enter: generate · Alt+Enter: newline · Alt+↑/↓: history · Tab: editor"
		promptBox := renderBox("Guidance Prompt (optional)", promptMeta, m.prompt.View(), m.width, m.writeFocus == writeFocusPrompt, m.hoverID == "pane-prompt")
		prRect := fb.addBlock(promptBox)
		fb.hits.setPane("prompt", prRect)
		fb.hits.add("pane-prompt", 0, prRect)
	}
}

func (m *Model) renderEditView(fb *frameBuilder) {
	// Instructions input box right at the top — custom copy editing is effortless!
	instrTitle := "Edit Instructions (leave blank for copy edit using .goauthorllm rules)"
	instrMeta := "Enter: Review · Alt+Enter: newline · Alt+↑/↓: history"
	instrBox := renderBox(instrTitle, instrMeta, m.edit.instructions.View(), m.width, m.edit.focus == editFocusInstructions, m.hoverID == "pane-edit-instr")
	ir := fb.addBlock(instrBox)
	fb.hits.setPane("edit-instr", ir)
	fb.hits.add("pane-edit-instr", 0, ir)

	// Sub-tabs + primary run controls
	var bar []rowItem
	bar = append(bar,
		rowItem{ID: "edit-tab", Index: int(editTabDiff), Content: renderPill("Diff", m.edit.subTab == editTabDiff, m.isHovered("edit-tab", int(editTabDiff))), Gap: 1},
		rowItem{ID: "edit-tab", Index: int(editTabLog), Content: renderPill(fmt.Sprintf("Log (%d)", len(m.edit.history)), m.edit.subTab == editTabLog, m.isHovered("edit-tab", int(editTabLog))), Gap: 1},
		rowItem{ID: "edit-tab", Index: int(editTabDocument), Content: renderPill("Document", m.edit.subTab == editTabDocument, m.isHovered("edit-tab", int(editTabDocument))), Gap: 2},
	)

	if m.edit.requesting {
		bar = append(bar, rowItem{ID: "stop-edit", Index: 0, Content: renderDangerBtn("■ Stop [Esc]", m.isHovered("stop-edit", 0)), Gap: 1})
	} else {
		bar = append(bar,
			rowItem{ID: "edit-review", Index: 0, Content: renderPrimaryBtn("▶ Review Edits [Ctrl+G]", m.isHovered("edit-review", 0)), Gap: 1},
			rowItem{ID: "edit-approve-all", Index: 0, Content: renderSecondaryBtn("⚡ Approve All [Alt+A]", m.isHovered("edit-approve-all", 0)), Gap: 1},
		)
	}

	if m.edit.current != nil && !m.edit.requesting {
		bar = append(bar,
			rowItem{ID: "edit-accept", Index: 0, Content: renderSuccessBtn("✓ Accept [y]", m.isHovered("edit-accept", 0)), Gap: 1},
			rowItem{ID: "edit-skip", Index: 0, Content: renderSecondaryBtn("✗ Skip [n]", m.isHovered("edit-skip", 0)), Gap: 1},
			rowItem{ID: "edit-approve-rem", Index: 0, Content: renderSecondaryBtn("⚡ Approve Remaining [a]", m.isHovered("edit-approve-rem", 0)), Gap: 1},
		)
	}
	if len(m.edit.appliedStacks) > 0 && !m.edit.requesting {
		bar = append(bar, rowItem{ID: "edit-undo", Index: 0, Content: renderSecondaryBtn("↩ Undo [u]", m.isHovered("edit-undo", 0)), Gap: 0})
	}
	fb.addRow(bar, m.width)

	// Main pane height = remaining rows minus status footer (1) and box borders (3).
	remaining := max(4, m.height-fb.y()-4)

	switch m.edit.subTab {
	case editTabDocument:
		m.editor.SetHeight(remaining)
		docBox := renderBox("Document (Editable)", "Tab: instructions · Ctrl+S: save", m.editor.View(), m.width, m.edit.focus == editFocusDocument, m.hoverID == "pane-editor")
		dr := fb.addBlock(docBox)
		fb.hits.setPane("editor", dr)
		fb.hits.add("pane-editor", 0, dr)

	case editTabLog:
		body := m.windowLines(m.renderEditLogBody(), max(16, m.width-6), remaining, &m.edit.reviewScroll)
		box := renderBox("Session Edit Log", "←/→ entry · ↑/↓ scroll · Tab instructions", body, m.width, m.edit.focus == editFocusReview, false)
		lr := fb.addBlock(box)
		fb.hits.setPane("review", lr)
		fb.hits.add("pane-review", 0, lr)

	default:
		body := m.windowLines(m.renderDiffBody(), max(16, m.width-6), remaining, &m.edit.reviewScroll)
		box := renderBox("Suggested Edit", "y accept · n skip · a approve remaining · ↑/↓ scroll", body, m.width, m.edit.focus == editFocusReview, false)
		dr := fb.addBlock(box)
		fb.hits.setPane("review", dr)
		fb.hits.add("pane-review", 0, dr)
	}
}

func (m *Model) renderConfigView(fb *frameBuilder) {
	// Top action row for Config
	fb.addRow([]rowItem{
		{ID: "cfg-mode", Index: 0, Content: renderPill("Form", !m.cfgUI.rawMode, m.isHovered("cfg-mode", 0)), Gap: 1},
		{ID: "cfg-mode", Index: 1, Content: renderPill("Raw YAML", m.cfgUI.rawMode, m.isHovered("cfg-mode", 1)), Gap: 2},
		{ID: "cfg-save", Index: 0, Content: renderPrimaryBtn("✓ Save & Apply [Ctrl+S]", m.isHovered("cfg-save", 0)), Gap: 1},
		{ID: "cfg-fetch", Index: cfgFieldModel, Content: renderSecondaryBtn("⌕ Fetch Models [Alt+F]", m.isHovered("cfg-fetch", cfgFieldModel)), Gap: 1},
		{ID: "cfg-reload", Index: 0, Content: renderSecondaryBtn("↻ Reload", m.isHovered("cfg-reload", 0)), Gap: 1},
		{Content: styleDim.Render(truncate(m.cfg.ConfigPath, max(12, m.width-58))), Gap: 0},
	}, m.width)

	if m.cfgUI.rawMode {
		m.cfgUI.rawYAML.SetHeight(max(5, m.height-fb.y()-4))
		box := renderBox(".goauthorllm (Raw YAML)", "Ctrl+S: validate, save & apply · Alt+Y: form view", m.cfgUI.rawYAML.View(), m.width, true, false)
		r := fb.addBlock(box)
		fb.hits.setPane("cfg-raw", r)
		return
	}

	// Structured Form View with exact clickable rows
	labelStyle := func(field int) lipgloss.Style {
		if m.cfgUI.activeField == field {
			return lipgloss.NewStyle().Width(22).Bold(true).Foreground(lipgloss.Color(colorOrange))
		}
		return lipgloss.NewStyle().Width(22).Foreground(lipgloss.Color(colorMuted))
	}

	addInputRow := func(field int, label, view string, extraBtn *rowItem) {
		prefix := "  "
		if m.cfgUI.activeField == field {
			prefix = lipgloss.NewStyle().Foreground(lipgloss.Color(colorOrange)).Bold(true).Render("▸ ")
		}
		items := []rowItem{
			{ID: "cfg-field", Index: field, Content: prefix + labelStyle(field).Render(label), Gap: 1},
			{ID: "cfg-field", Index: field, Content: view, Gap: 1},
		}
		if extraBtn != nil {
			items = append(items, *extraBtn)
		}
		fb.addRow(items, m.width)
	}

	pickBtn := func(field int) *rowItem {
		return &rowItem{ID: "cfg-fetch", Index: field, Content: renderSecondaryBtn("Pick…", m.isHovered("cfg-fetch", field)), Gap: 0}
	}

	addInputRow(cfgFieldBaseURL, "Base URL", m.cfgUI.baseURL.View(), nil)
	addInputRow(cfgFieldModel, "Model", m.cfgUI.model.View(), pickBtn(cfgFieldModel))
	addInputRow(cfgFieldGenModel, "Generation Model", m.cfgUI.genModel.View(), pickBtn(cfgFieldGenModel))
	addInputRow(cfgFieldEditModel, "Editing Model", m.cfgUI.editModel.View(), pickBtn(cfgFieldEditModel))
	addInputRow(cfgFieldStreamSpeed, "Stream Speed (c/s)", m.cfgUI.streamSpeed.View(), nil)

	// Default scope toggle pills
	scopeRow := []rowItem{
		{ID: "cfg-field", Index: cfgFieldDefaultScope, Content: func() string {
			p := "  "
			if m.cfgUI.activeField == cfgFieldDefaultScope {
				p = lipgloss.NewStyle().Foreground(lipgloss.Color(colorOrange)).Bold(true).Render("▸ ")
			}
			return p + labelStyle(cfgFieldDefaultScope).Render("Default Scope")
		}(), Gap: 1},
	}
	for i, sc := range config.ValidScopes {
		scopeRow = append(scopeRow, rowItem{
			ID:      "cfg-scope",
			Index:   i,
			Content: renderPill(sc, m.cfgUI.defaultScope == sc, m.isHovered("cfg-scope", i)),
			Gap:     1,
		})
	}
	fb.addRow(scopeRow, m.width)

	// Structured generation toggle
	structPrefix := "  "
	if m.cfgUI.activeField == cfgFieldStructuredGen {
		structPrefix = lipgloss.NewStyle().Foreground(lipgloss.Color(colorOrange)).Bold(true).Render("▸ ")
	}
	fb.addRow([]rowItem{
		{ID: "cfg-field", Index: cfgFieldStructuredGen, Content: structPrefix + labelStyle(cfgFieldStructuredGen).Render("Structured Stream"), Gap: 1},
		{ID: "cfg-struct", Index: 1, Content: renderPill("Enabled", m.cfgUI.structuredGen, m.isHovered("cfg-struct", 1)), Gap: 1},
		{ID: "cfg-struct", Index: 0, Content: renderPill("Plain", !m.cfgUI.structuredGen, m.isHovered("cfg-struct", 0)), Gap: 1},
	}, m.width)

	addInputRow(cfgFieldCopyBatch, "Copy Edit Batch", m.cfgUI.copyBatch.View(), nil)
	addInputRow(cfgFieldDirectedBatch, "Directed Edit Batch", m.cfgUI.directedBatch.View(), nil)

	// Remaining height split between Writing Rules and Copy Edit Rules textareas
	avail := max(4, m.height-fb.y()-7)
	eachH := max(2, avail/2)
	m.cfgUI.rules.SetHeight(eachH)
	m.cfgUI.copyEditRules.SetHeight(max(2, avail-eachH))

	r1 := fb.addBlock(renderBox("Writing Rules (.goauthorllm `rules`)", "Applied to every generation", m.cfgUI.rules.View(), m.width, m.cfgUI.activeField == cfgFieldRules, m.isHovered("cfg-field", cfgFieldRules)))
	fb.hits.setPane("cfg-rules", r1)
	fb.hits.add("cfg-field", cfgFieldRules, r1)

	r2 := fb.addBlock(renderBox("Copy Edit Rules (.goauthorllm `copy_edit_rules`)", "Applied to default copy edits", m.cfgUI.copyEditRules.View(), m.width, m.cfgUI.activeField == cfgFieldCopyEditRules, m.isHovered("cfg-field", cfgFieldCopyEditRules)))
	fb.hits.setPane("cfg-copy-rules", r2)
	fb.hits.add("cfg-field", cfgFieldCopyEditRules, r2)
}

func (m *Model) renderFilesView(fb *frameBuilder) {
	fb.addRow([]rowItem{
		{Content: styleTitle.Render("Markdown Documents in ") + styleMuted.Render(truncate(m.cwd, max(12, m.width-38))), Gap: 2},
		{ID: "files-refresh", Index: 0, Content: renderSecondaryBtn("↻ Refresh [Ctrl+R]", m.isHovered("files-refresh", 0)), Gap: 0},
	}, m.width)

	// Reserve room for New File box (4 lines) + status bar (1 line)
	listRoom := max(3, m.height-fb.y()-7)
	if len(m.files.files) == 0 {
		fb.addBlock(renderBox("Documents", "No .md files in folder yet", styleMuted.Render("Type a filename below and press Enter to create a new markdown file."), m.width, m.files.focus == filesFocusList, false))
	} else {
		if m.files.selected < m.files.scroll {
			m.files.scroll = m.files.selected
		}
		if m.files.selected >= m.files.scroll+listRoom {
			m.files.scroll = m.files.selected - listRoom + 1
		}
		end := min(len(m.files.files), m.files.scroll+listRoom)

		// Render header row of list
		fb.addBlock(styleDim.Render(fmt.Sprintf("  %d file(s) · ↑/↓ select · Enter or click to open · Tab creates new file", len(m.files.files))))
		listStartY := fb.y()
		for i := m.files.scroll; i < end; i++ {
			name := m.files.files[i]
			selected := i == m.files.selected
			hovered := m.isHovered("file-row", i)
			active := m.doc != nil && filepath.Base(m.doc.Path) == name

			prefix := "   "
			rowStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colorBody))
			if hovered {
				rowStyle = rowStyle.Foreground(lipgloss.Color(colorOrangeSoft)).Background(lipgloss.Color(colorSurface))
			}
			if selected && m.files.focus == filesFocusList {
				prefix = lipgloss.NewStyle().Foreground(lipgloss.Color(colorOrange)).Bold(true).Render(" ▸ ")
				rowStyle = lipgloss.NewStyle().Foreground(lipgloss.Color(colorText)).Background(lipgloss.Color(colorOrangeDeep)).Bold(true)
			}
			label := prefix + name
			if active {
				label += "  (open)"
			}
			rowStr := rowStyle.Width(max(12, m.width-2)).Render(truncate(label, max(12, m.width-2)))
			y := fb.y()
			fb.lines = append(fb.lines, rowStr)
			fb.hits.add("file-row", i, rect{x1: 0, y1: y, x2: max(0, m.width-1), y2: y})
		}
		fb.hits.setPane("files-list", rect{x1: 0, y1: listStartY, x2: max(0, m.width-1), y2: max(listStartY, fb.y()-1)})
	}

	fb.addBlank()
	newBox := renderBox("Create or Open by Filename", "Enter: create & open · Tab: file list", m.files.input.View(), m.width, m.files.focus == filesFocusInput, m.hoverID == "pane-file-input")
	ir := fb.addBlock(newBox)
	fb.hits.add("pane-file-input", 0, ir)
}

func (m *Model) renderStatusFooter(fb *frameBuilder) {
	// Pin status footer to the bottom line.
	for fb.y() < m.height-1 {
		fb.addBlank()
	}

	left := m.statusText
	if m.busy() {
		left = m.spin.View() + " " + left
	}
	if m.statusLevel == "error" {
		left = "[Alt+E details] " + left
	}
	right := fmt.Sprintf("%s · %s", m.activeModelName(), m.cfg.BaseURL)
	rightRendered := styleDim.Render(truncate(right, max(12, m.width/2)))

	leftBudget := max(10, m.width-lipgloss.Width(rightRendered)-2)
	leftRendered := lipgloss.NewStyle().Foreground(statusColor(m.statusLevel)).Render(truncate(left, leftBudget))
	gap := max(1, m.width-lipgloss.Width(leftRendered)-lipgloss.Width(rightRendered))

	y := fb.y()
	fb.lines = append(fb.lines, leftRendered+strings.Repeat(" ", gap)+rightRendered)
	if m.lastError != "" {
		fb.hits.add("show-error", 0, rect{x1: 0, y1: y, x2: max(0, lipgloss.Width(leftRendered)-1), y2: y})
	}
}

// --- Overlays ---

func (m *Model) renderMetaOverlay(fb *frameBuilder) {
	fb.addRow([]rowItem{
		{Content: styleBrand.Render("◆ Document Instructions & Front Matter"), Gap: 2},
		{ID: "save", Index: 0, Content: renderPrimaryBtn("✓ Save [Ctrl+S]", m.isHovered("save", 0)), Gap: 1},
		{ID: "close-overlay", Index: 0, Content: renderSecondaryBtn("Close [Esc]", m.isHovered("close-overlay", 0)), Gap: 0},
	}, m.width)
	fb.addBlock(styleDim.Render("YAML front matter for this file. Set `system_message:` to steer generation & editing for this document."))
	box := renderBox("Front Matter", "Ctrl+S save · Esc close", m.frontMatter.View(), m.width, true, false)
	r := fb.addBlock(box)
	fb.hits.setPane("frontmatter", r)
	m.renderStatusFooter(fb)
}

func (m *Model) renderModelPickerOverlay(fb *frameBuilder) {
	fb.addRow([]rowItem{
		{Content: styleBrand.Render("◆ Pick Model from Endpoint"), Gap: 2},
		{ID: "close-overlay", Index: 0, Content: renderSecondaryBtn("Cancel [Esc]", m.isHovered("close-overlay", 0)), Gap: 0},
	}, m.width)
	fb.addBlock(renderBox("Filter", "Type to narrow · ↑/↓ select · Enter or click to choose", m.modelPickerFilter.View(), m.width, true, false))

	if m.modelPickerBusy {
		fb.addBlock(m.spin.View() + " Loading models from " + m.cfgUI.baseURL.Value() + "...")
		m.renderStatusFooter(fb)
		return
	}

	models := m.filteredModels()
	if len(models) == 0 {
		fb.addBlock(styleMuted.Render("No matching models."))
		m.renderStatusFooter(fb)
		return
	}

	room := max(3, m.height-fb.y()-2)
	m.modelPickerIndex = clampInt(m.modelPickerIndex, 0, len(models)-1)
	start := max(0, m.modelPickerIndex-room+1)
	end := min(len(models), start+room)
	for i := start; i < end; i++ {
		id := models[i]
		selected := i == m.modelPickerIndex
		hovered := m.isHovered("model-pick", i)
		prefix := "  "
		st := lipgloss.NewStyle().Foreground(lipgloss.Color(colorBody))
		if hovered {
			st = st.Foreground(lipgloss.Color(colorOrangeSoft)).Background(lipgloss.Color(colorSurface))
		}
		if selected {
			prefix = lipgloss.NewStyle().Foreground(lipgloss.Color(colorOrange)).Bold(true).Render("▸ ")
			st = lipgloss.NewStyle().Foreground(lipgloss.Color(colorText)).Background(lipgloss.Color(colorOrangeDeep)).Bold(true)
		}
		y := fb.y()
		fb.lines = append(fb.lines, st.Width(max(12, m.width-2)).Render(truncate(prefix+id, max(12, m.width-2))))
		fb.hits.add("model-pick", i, rect{x1: 0, y1: y, x2: max(0, m.width-1), y2: y})
	}
	m.renderStatusFooter(fb)
}

func (m *Model) renderHelpOverlay(fb *frameBuilder) {
	fb.addRow([]rowItem{
		{Content: styleBrand.Render("◆ goauthorllm — Keyboard & Mouse Guide"), Gap: 2},
		{ID: "close-overlay", Index: 0, Content: renderSecondaryBtn("Close [Esc]", m.isHovered("close-overlay", 0)), Gap: 0},
	}, m.width)

	helpText := strings.Join([]string{
		styleTitle.Render("Global Navigation"),
		"  Alt+1 / Alt+2 / Alt+3 / Alt+4   Switch Write / Edit / Config / Files (or click tabs)",
		"  Alt+M                           Open Document Instructions (YAML Front Matter)",
		"  Ctrl+S                          Save current document (or save .goauthorllm in Config)",
		"  Ctrl+O                          Switch to Files view",
		"  F1 or Alt+/                     Toggle this Help guide",
		"  Alt+E                           View full last error message",
		"  Esc                             Stop active generation/edit or close overlay",
		"  Ctrl+Q                          Save & quit",
		"",
		styleTitle.Render("Write & Generate"),
		"  Ctrl+G (or Enter in Prompt)     Generate content using selected scope & rounds",
		"  Alt+S (or click pills)          Switch scope: Sentence / Paragraph / Section / Free",
		"  Alt+[ / Alt+] (or click - / +)  Set generation rounds (×1 to ×20; auto-plans multi-round)",
		"  Alt+- / Alt+= (or click Speed)  Adjust live streaming speed (Max, 35, 65, 110, 180 c/s)",
		"  Esc (while streaming)           Stop immediately at exact visible character",
		"  Ctrl+R (or +1 Round)            Generate one more round seamlessly",
		"  Tab                             Toggle focus between Document and Guidance Prompt",
		"  Alt+Enter / Ctrl+J              Insert newline inside Guidance Prompt",
		"  Alt+↑ / Alt+↓                   Recall saved prompt history for this document",
		"  Ctrl+Z / Alt+K                  Undo last change / Delete from cursor to end",
		"",
		styleTitle.Render("Copy Edit & Custom Edit"),
		"  Ctrl+G (or Enter in Instr)      Scan & review edits one by one (blank = copy edit)",
		"  Alt+A                           Approve All validated edits automatically",
		"  y / n / a (in Diff view)        Accept / Skip / Approve All Remaining",
		"  u (or Ctrl+Z)                   Undo last applied edit",
		"  ← / →                           Browse session log entries",
		"",
		styleTitle.Render("Config (.goauthorllm)"),
		"  Alt+Y                           Toggle Form view vs Raw YAML view",
		"  Alt+F (or Pick…)                Fetch & pick models live from your endpoint",
		"  Ctrl+S                          Save .goauthorllm and hot-reload immediately",
		"",
		styleTitle.Render("Mouse Support"),
		"  Click any tab, scope pill, speed/rounds control, button, file row, or pane",
		"  Scroll wheel scrolls whichever pane your cursor is over",
	}, "\n")

	room := max(4, m.height-fb.y()-4)
	body := m.windowLines(helpText, max(20, m.width-6), room, &m.helpScroll)
	box := renderBox("Shortcuts", "↑/↓ scroll · Esc close", body, m.width, true, false)
	r := fb.addBlock(box)
	fb.hits.setPane("help", r)
	m.renderStatusFooter(fb)
}

func (m *Model) renderErrorOverlay(fb *frameBuilder) {
	fb.addRow([]rowItem{
		{Content: lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(colorRed)).Render("◆ Error Details"), Gap: 2},
		{ID: "close-overlay", Index: 0, Content: renderSecondaryBtn("Close [Esc]", m.isHovered("close-overlay", 0)), Gap: 0},
	}, m.width)

	room := max(4, m.height-fb.y()-4)
	body := m.windowLines(m.lastError, max(20, m.width-6), room, &m.errorScroll)
	box := renderBox("Last Error", "↑/↓ PgUp/PgDn scroll · Esc close", body, m.width, true, false)
	r := fb.addBlock(box)
	fb.hits.setPane("error", r)
	m.renderStatusFooter(fb)
}

func (m *Model) windowLines(body string, width, height int, offset *int) string {
	wrapped := ansi.Hardwrap(body, max(1, width), true)
	lines := strings.Split(wrapped, "\n")
	maxOff := max(0, len(lines)-height)
	*offset = clampInt(*offset, 0, maxOff)
	visible := append([]string(nil), lines[*offset:min(len(lines), *offset+height)]...)
	for len(visible) < height {
		visible = append(visible, "")
	}
	return strings.Join(visible, "\n")
}
