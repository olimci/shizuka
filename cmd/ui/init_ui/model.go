package init_ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/olimci/shizuka/pkg/scaffold"
)

type Params struct {
	Template   *scaffold.Template
	Collection *scaffold.Collection
	Selected   string
	Target     string
	Force      bool
}

type Result struct {
	Template  *scaffold.Template
	Target    string
	Force     bool
	Variables map[string]any
	Cancelled bool
}

type step int

const (
	stepSelectTemplate step = iota
	stepVariables
)

type Model struct {
	step step

	template   *scaffold.Template
	collection *scaffold.Collection
	selected   string

	templateList list.Model
	varKeys      []string
	varInputs    []textinput.Model
	targetInput  textinput.Model
	force        bool
	focusIdx     int
	done         bool

	result Result
	err    error
}

func Run(ctx context.Context, params Params) (*Result, error) {
	model, err := NewModel(params)
	if err != nil {
		return nil, err
	}

	program := tea.NewProgram(model, tea.WithContext(ctx))
	final, err := program.Run()
	if err != nil {
		return nil, err
	}

	m, ok := final.(*Model)
	if !ok {
		return nil, fmt.Errorf("unexpected model type %T", final)
	}

	return &m.result, m.err
}

func NewModel(params Params) (*Model, error) {
	if params.Template == nil && params.Collection == nil {
		return nil, fmt.Errorf("no template or collection available")
	}

	m := &Model{
		template:   params.Template,
		collection: params.Collection,
		selected:   params.Selected,
		force:      params.Force,
		result: Result{
			Target: params.Target,
			Force:  params.Force,
		},
	}

	m.targetInput = textinput.New()
	m.targetInput.Placeholder = "path/to/site"
	m.targetInput.Prompt = ""
	m.targetInput.SetValue(params.Target)
	applyTextInputStyles(&m.targetInput)

	if m.template == nil && m.collection != nil {
		if params.Selected != "" {
			resolved, err := resolveTemplate(m.collection, params.Selected)
			if err != nil {
				return nil, err
			}
			m.template = resolved
		} else if m.collection.Config.Templates.Default != "" {
			m.selected = m.collection.Config.Templates.Default
		}
	}

	if m.template == nil && m.collection != nil {
		m.step = stepSelectTemplate
		m.templateList = buildTemplateList(m.collection)
		if m.selected != "" {
			selectTemplateInList(&m.templateList, m.selected)
		}
	} else {
		m.step = stepVariables
		m.setTemplate(m.template)
	}

	return m, nil
}

func (m *Model) Init() tea.Cmd {
	return textinput.Blink
}

func (m *Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		if m.step == stepSelectTemplate {
			m.templateList.SetSize(msg.Width, msg.Height-6)
		}
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			m.done = true
			m.result.Cancelled = true
			return m, tea.Quit
		}
	}

	switch m.step {
	case stepSelectTemplate:
		return m.updateSelectTemplate(msg)
	case stepVariables:
		return m.updateVariables(msg)
	default:
		return m, nil
	}
}

func (m *Model) View() string {
	if m.done {
		return ""
	}
	switch m.step {
	case stepSelectTemplate:
		return m.viewSelectTemplate()
	case stepVariables:
		return m.viewVariables()
	default:
		return ""
	}
}

func (m *Model) updateSelectTemplate(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	m.templateList, cmd = m.templateList.Update(msg)

	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "enter":
			item, ok := m.templateList.SelectedItem().(templateItem)
			if !ok || item.tmpl == nil {
				return m, nil
			}
			m.setTemplate(item.tmpl)
			m.step = stepVariables
			return m, nil
		}
	}

	return m, cmd
}

func (m *Model) updateVariables(msg tea.Msg) (tea.Model, tea.Cmd) {
	if key, ok := msg.(tea.KeyMsg); ok {
		switch key.String() {
		case "tab", "down":
			m.focusNext()
			return m, nil
		case "shift+tab", "up":
			m.focusPrev()
			return m, nil
		case "enter":
			switch {
			case m.focusIdx <= len(m.varInputs):
				m.focusNext()
				return m, nil
			case m.focusIdx == len(m.varInputs)+1:
				m.captureResult()
				m.done = true
				return m, tea.Quit
			}
		}
	}

	if input := m.focusedInput(); input != nil {
		var cmd tea.Cmd
		*input, cmd = input.Update(msg)
		return m, cmd
	}

	return m, nil
}

func (m *Model) viewSelectTemplate() string {
	return m.templateList.View()
}

func (m *Model) viewVariables() string {
	styles := initStyles()
	var b strings.Builder
	b.WriteString(styles.title.Render("Template setup:") + "\n\n")
	targetBlock := styles.section.Render("Target directory:") + "\n" + m.targetInput.View()
	if m.focusIdx == 0 {
		targetBlock = styles.blockFocused.Render(targetBlock)
	} else {
		targetBlock = styles.blockUnfocused.Render(targetBlock)
	}
	b.WriteString(targetBlock)
	b.WriteString("\n\n")

	if len(m.varInputs) > 0 {
		for i := range m.varInputs {
			def := m.template.Config.Variables[m.varKeys[i]]
			label := styles.label.Render(def.Name)
			block := label + "\n" + m.varInputs[i].View()
			if m.focusIdx == i+1 {
				block = styles.blockFocused.Render(block)
			} else {
				block = styles.blockUnfocused.Render(block)
			}
			b.WriteString(block)
			b.WriteString("\n\n")
		}
	}

	submitLine := "Create scaffold"
	if m.focusIdx == len(m.varInputs)+1 {
		submitLine = styles.submitFocused.Render(submitLine)
	} else {
		submitLine = styles.submit.Render(submitLine)
	}
	b.WriteString(submitLine)
	b.WriteString("\n")

	return b.String()
}

func (m *Model) setTemplate(tmpl *scaffold.Template) {
	m.template = tmpl
	m.varKeys = sortedVarKeys(tmpl)
	m.varInputs = make([]textinput.Model, len(m.varKeys))
	for i, key := range m.varKeys {
		def := tmpl.Config.Variables[key]
		input := textinput.New()
		input.Prompt = ""
		if def.Description != "" {
			input.Placeholder = def.Description
		}
		if def.Default != "" {
			input.SetValue(def.Default)
		}
		applyTextInputStyles(&input)
		m.varInputs[i] = input
	}

	m.focusIdx = 0
	m.targetInput.Focus()
	m.result.Template = tmpl
}

func (m *Model) captureResult() {
	m.result.Template = m.template
	m.result.Target = strings.TrimSpace(m.targetInput.Value())
	if m.result.Target == "" {
		m.result.Target = "."
	}

	vars := make(map[string]any, len(m.varKeys))
	for i, key := range m.varKeys {
		vars[key] = strings.TrimSpace(m.varInputs[i].Value())
	}
	m.result.Variables = vars
}

func (m *Model) focusedInput() *textinput.Model {
	if m.focusIdx == 0 {
		return &m.targetInput
	}
	if m.focusIdx >= 1 && m.focusIdx <= len(m.varInputs) {
		return &m.varInputs[m.focusIdx-1]
	}
	return nil
}

func (m *Model) focusNext() {
	m.focusIdx++
	max := len(m.varInputs) + 1
	if m.focusIdx > max {
		m.focusIdx = 0
	}
	m.syncFocus()
}

func (m *Model) focusPrev() {
	m.focusIdx--
	max := len(m.varInputs) + 1
	if m.focusIdx < 0 {
		m.focusIdx = max
	}
	m.syncFocus()
}

func (m *Model) syncFocus() {
	if m.focusIdx == 0 {
		m.targetInput.Focus()
	} else {
		m.targetInput.Blur()
	}

	for i := range m.varInputs {
		if m.focusIdx == i+1 {
			m.varInputs[i].Focus()
		} else {
			m.varInputs[i].Blur()
		}
	}
}
