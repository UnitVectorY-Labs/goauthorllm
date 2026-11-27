package app

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	ta "github.com/charmbracelet/bubbles/textarea"
	ti "github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/UnitVectorY-Labs/goauthorllm/internal/config"
	"github.com/UnitVectorY-Labs/goauthorllm/internal/engine"
	"github.com/UnitVectorY-Labs/goauthorllm/internal/llm"
)

const (
	cfgFieldBaseURL = iota
	cfgFieldModel
	cfgFieldGenModel
	cfgFieldEditModel
	cfgFieldStreamSpeed
	cfgFieldDefaultScope
	cfgFieldStructuredGen
	cfgFieldCopyBatch
	cfgFieldDirectedBatch
	cfgFieldRules
	cfgFieldCopyEditRules
	cfgFieldCount
)

type configEditorState struct {
	rawMode       bool
	activeField   int
	scroll        int
	baseURL       ti.Model
	model         ti.Model
	genModel      ti.Model
	editModel     ti.Model
	streamSpeed   ti.Model
	defaultScope  string
	structuredGen bool
	copyBatch     ti.Model
	directedBatch ti.Model
	rules         ta.Model
	copyEditRules ta.Model
	rawYAML       ta.Model
	existingFile  config.LocalFile
}

type modelsFetchedMsg struct {
	models []string
	err    error
}

func (m *Model) initConfigEditor() {
	m.cfgUI.baseURL = newStyledTextInput("http://localhost:11434/v1")
	m.cfgUI.model = newStyledTextInput("model-id")
	m.cfgUI.genModel = newStyledTextInput("(optional: defaults to Model)")
	m.cfgUI.editModel = newStyledTextInput("(optional: defaults to Model)")
	m.cfgUI.streamSpeed = newStyledTextInput("0 = max speed, e.g. 65")
	m.cfgUI.copyBatch = newStyledTextInput("5")
	m.cfgUI.directedBatch = newStyledTextInput("10")
	m.cfgUI.rules = newStyledTextarea("Project-wide writing rules for .goauthorllm (tone, audience, style, formatting)...", false)
	m.cfgUI.copyEditRules = newStyledTextarea("Custom default copy-editing rules (e.g., use Oxford comma, prefer active voice)...", false)
	m.cfgUI.rawYAML = newStyledTextarea("# .goauthorllm YAML configuration\n", true)

	m.loadConfigIntoEditor()
}

func (m *Model) loadConfigIntoEditor() {
	local, err := config.LoadLocal(m.cfg.ConfigPath)
	if err == nil {
		m.cfgUI.existingFile = local
	}
	snap := m.cfg.ToLocalFile(m.cfgUI.existingFile)

	m.cfgUI.baseURL.SetValue(snap.BaseURL)
	m.cfgUI.model.SetValue(snap.Model)
	m.cfgUI.genModel.SetValue(snap.GenerationModel)
	m.cfgUI.editModel.SetValue(snap.EditingModel)
	m.cfgUI.streamSpeed.SetValue(strconv.Itoa(m.cfg.StreamSpeed))
	m.cfgUI.defaultScope = m.cfg.DefaultScope
	m.cfgUI.structuredGen = m.cfg.StructuredGeneration
	m.cfgUI.copyBatch.SetValue(strconv.Itoa(m.cfg.CopyEditBatchSize))
	m.cfgUI.directedBatch.SetValue(strconv.Itoa(m.cfg.DirectedEditBatchSize))
	m.cfgUI.rules.SetValue(m.cfg.Rules)
	m.cfgUI.copyEditRules.SetValue(m.cfg.CopyEditRules)

	if rendered, err := config.RenderLocalYAML(snap); err == nil {
		m.cfgUI.rawYAML.SetValue(rendered)
	}
}

func (m *Model) blurConfigInputs() {
	m.cfgUI.baseURL.Blur()
	m.cfgUI.model.Blur()
	m.cfgUI.genModel.Blur()
	m.cfgUI.editModel.Blur()
	m.cfgUI.streamSpeed.Blur()
	m.cfgUI.copyBatch.Blur()
	m.cfgUI.directedBatch.Blur()
	m.cfgUI.rules.Blur()
	m.cfgUI.copyEditRules.Blur()
	m.cfgUI.rawYAML.Blur()
}

func (m *Model) focusActiveConfigField() {
	m.blurConfigInputs()
	if m.cfgUI.rawMode {
		m.cfgUI.rawYAML.Focus()
		return
	}
	switch m.cfgUI.activeField {
	case cfgFieldBaseURL:
		m.cfgUI.baseURL.Focus()
	case cfgFieldModel:
		m.cfgUI.model.Focus()
	case cfgFieldGenModel:
		m.cfgUI.genModel.Focus()
	case cfgFieldEditModel:
		m.cfgUI.editModel.Focus()
	case cfgFieldStreamSpeed:
		m.cfgUI.streamSpeed.Focus()
	case cfgFieldCopyBatch:
		m.cfgUI.copyBatch.Focus()
	case cfgFieldDirectedBatch:
		m.cfgUI.directedBatch.Focus()
	case cfgFieldRules:
		m.cfgUI.rules.Focus()
	case cfgFieldCopyEditRules:
		m.cfgUI.copyEditRules.Focus()
	}
}

func (m *Model) selectConfigField(field int) {
	m.cfgUI.activeField = clampInt(field, 0, cfgFieldCount-1)
	m.syncFocus()
}

func (m *Model) toggleConfigRawMode(raw bool) {
	if m.cfgUI.rawMode == raw {
		return
	}
	if raw {
		// Sync current form values into YAML before switching to raw view.
		if local, err := m.buildLocalFileFromForm(); err == nil {
			if rendered, err := config.RenderLocalYAML(local); err == nil {
				m.cfgUI.rawYAML.SetValue(rendered)
			}
		}
	} else {
		// Parse raw YAML back into form if valid.
		if local, err := config.ParseLocalYAML(m.cfgUI.rawYAML.Value(), config.LocalConfigFilename); err == nil {
			m.cfgUI.existingFile = local
			if local.BaseURL != "" {
				m.cfgUI.baseURL.SetValue(local.BaseURL)
			}
			if local.Model != "" {
				m.cfgUI.model.SetValue(local.Model)
			}
			m.cfgUI.genModel.SetValue(local.GenerationModel)
			m.cfgUI.editModel.SetValue(local.EditingModel)
			if local.StreamSpeed != nil {
				m.cfgUI.streamSpeed.SetValue(strconv.Itoa(*local.StreamSpeed))
			}
			if local.DefaultScope != "" {
				m.cfgUI.defaultScope = local.DefaultScope
			}
			if local.StructuredGeneration != nil {
				m.cfgUI.structuredGen = *local.StructuredGeneration
			}
			if local.CopyEditBatchSize != nil {
				m.cfgUI.copyBatch.SetValue(strconv.Itoa(*local.CopyEditBatchSize))
			}
			if local.DirectedEditBatchSize != nil {
				m.cfgUI.directedBatch.SetValue(strconv.Itoa(*local.DirectedEditBatchSize))
			}
			m.cfgUI.rules.SetValue(local.Rules)
			m.cfgUI.copyEditRules.SetValue(local.CopyEditRules)
		}
	}
	m.cfgUI.rawMode = raw
	m.resizeInputs()
	m.syncFocus()
}

func (m *Model) buildLocalFileFromForm() (config.LocalFile, error) {
	baseURL := strings.TrimSpace(m.cfgUI.baseURL.Value())
	if baseURL == "" {
		return config.LocalFile{}, fmt.Errorf("base_url cannot be empty")
	}
	modelID := strings.TrimSpace(m.cfgUI.model.Value())
	if modelID == "" {
		return config.LocalFile{}, fmt.Errorf("model cannot be empty")
	}
	speedVal := 0
	if raw := strings.TrimSpace(m.cfgUI.streamSpeed.Value()); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 {
			return config.LocalFile{}, fmt.Errorf("stream speed must be a non-negative integer")
		}
		speedVal = parsed
	}
	copyBatch := config.DefaultCopyEditBatchSize
	if raw := strings.TrimSpace(m.cfgUI.copyBatch.Value()); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			return config.LocalFile{}, fmt.Errorf("copy edit batch size must be a positive integer")
		}
		copyBatch = parsed
	}
	dirBatch := config.DefaultDirectedEditBatchSize
	if raw := strings.TrimSpace(m.cfgUI.directedBatch.Value()); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed <= 0 {
			return config.LocalFile{}, fmt.Errorf("directed edit batch size must be a positive integer")
		}
		dirBatch = parsed
	}

	local := m.cfgUI.existingFile
	local.BaseURL = baseURL
	local.Model = modelID
	local.GenerationModel = strings.TrimSpace(m.cfgUI.genModel.Value())
	local.EditingModel = strings.TrimSpace(m.cfgUI.editModel.Value())
	local.StreamSpeed = &speedVal
	local.DefaultScope = m.cfgUI.defaultScope
	structured := m.cfgUI.structuredGen
	local.StructuredGeneration = &structured
	local.CopyEditBatchSize = &copyBatch
	local.DirectedEditBatchSize = &dirBatch
	local.Rules = strings.TrimSpace(m.cfgUI.rules.Value())
	local.CopyEditRules = strings.TrimSpace(m.cfgUI.copyEditRules.Value())
	return local, nil
}

func (m *Model) saveAndApplyConfig() {
	var (
		local config.LocalFile
		err   error
	)
	if m.cfgUI.rawMode {
		local, err = config.ParseLocalYAML(m.cfgUI.rawYAML.Value(), config.LocalConfigFilename)
		if err != nil {
			m.setStatus("Config error: "+err.Error(), "error")
			return
		}
	} else {
		local, err = m.buildLocalFileFromForm()
		if err != nil {
			m.setStatus("Config error: "+err.Error(), "error")
			return
		}
	}

	if err := config.SaveLocal(m.cfg.ConfigPath, local); err != nil {
		m.setStatus("Failed to write .goauthorllm: "+err.Error(), "error")
		return
	}
	if err := m.cfg.ApplyLocal(local); err != nil {
		m.setStatus("Saved file, but reload failed: "+err.Error(), "error")
		return
	}
	m.gen.scope = engine.NormalizeScope(m.cfg.DefaultScope)
	m.cfgUI.existingFile = local
	if rendered, err := config.RenderLocalYAML(local); err == nil {
		m.cfgUI.rawYAML.SetValue(rendered)
	}
	m.rebuildClients()
	m.setStatus("Saved .goauthorllm & applied live!", "success")
}

// fetchModels queries GET <base_url>/models so the user can pick a model interactively.
func (m *Model) fetchModels(targetField int) tea.Cmd {
	baseURL := strings.TrimSpace(m.cfgUI.baseURL.Value())
	if baseURL == "" {
		baseURL = m.cfg.BaseURL
	}
	m.modelPickerTarget = targetField
	m.modelPickerBusy = true
	m.modelPickerItems = nil
	m.modelPickerIndex = 0
	m.modelPickerFilter.SetValue("")
	m.overlay = overlayModelPicker
	m.syncFocus()
	m.setStatus("Fetching models from "+baseURL+"...", "accent")

	client := llm.NewClient(baseURL, m.cfg.Model, m.cfg.APIKey, m.cfg.Timeout, m.cfg.UserAgent)
	return tea.Batch(m.spin.Tick, func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), m.cfg.Timeout)
		defer cancel()
		models, err := client.ListModels(ctx)
		return modelsFetchedMsg{models: models, err: err}
	})
}

func (m *Model) handleModelsFetched(msg modelsFetchedMsg) (tea.Model, tea.Cmd) {
	m.modelPickerBusy = false
	if msg.err != nil {
		m.overlay = overlayNone
		m.syncFocus()
		m.setStatus("Fetch models failed: "+msg.err.Error(), "error")
		return m, nil
	}
	if len(msg.models) == 0 {
		m.overlay = overlayNone
		m.syncFocus()
		m.setStatus("Endpoint returned no models", "warn")
		return m, nil
	}
	m.modelPickerItems = msg.models
	m.modelPickerIndex = 0
	m.setStatus(fmt.Sprintf("Loaded %d models · type to filter, Enter/click selects", len(msg.models)), "success")
	return m, nil
}

func (m *Model) filteredModels() []string {
	q := strings.ToLower(strings.TrimSpace(m.modelPickerFilter.Value()))
	if q == "" {
		return m.modelPickerItems
	}
	out := make([]string, 0, len(m.modelPickerItems))
	for _, id := range m.modelPickerItems {
		if strings.Contains(strings.ToLower(id), q) {
			out = append(out, id)
		}
	}
	return out
}

func (m *Model) pickModel(id string) {
	switch m.modelPickerTarget {
	case cfgFieldGenModel:
		m.cfgUI.genModel.SetValue(id)
	case cfgFieldEditModel:
		m.cfgUI.editModel.SetValue(id)
	default:
		m.cfgUI.model.SetValue(id)
	}
	m.overlay = overlayNone
	m.syncFocus()
	m.setStatus("Selected model "+id+" · Ctrl+S saves .goauthorllm", "success")
}
