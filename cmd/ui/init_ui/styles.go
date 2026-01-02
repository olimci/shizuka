package init_ui

import (
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

type uiStyles struct {
	title             lipgloss.Style
	section           lipgloss.Style
	label             lipgloss.Style
	blockFocused      lipgloss.Style
	blockUnfocused    lipgloss.Style
	submit            lipgloss.Style
	submitFocused     lipgloss.Style
	listTitle         lipgloss.Style
	listSelectedTitle lipgloss.Style
	listSelectedDesc  lipgloss.Style
	listNormalTitle   lipgloss.Style
	listNormalDesc    lipgloss.Style
}

func initStyles() uiStyles {
	colors := catppuccinMocha()
	return uiStyles{
		title:   lipgloss.NewStyle().Bold(true).Foreground(colors.accent).MarginLeft(2),
		section: lipgloss.NewStyle().Bold(true).Foreground(colors.text),
		label: lipgloss.NewStyle().
			Bold(true).
			Foreground(colors.text),
		blockFocused: lipgloss.NewStyle().
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(colors.accent).
			PaddingLeft(1),
		blockUnfocused: lipgloss.NewStyle().PaddingLeft(2),
		submit: lipgloss.NewStyle().
			Bold(true).
			Foreground(colors.base).
			Background(colors.green).
			MarginLeft(2).
			PaddingLeft(1).
			PaddingRight(1),
		submitFocused: lipgloss.NewStyle().
			Bold(true).
			Foreground(colors.green).
			Background(colors.base).
			Underline(true).
			MarginLeft(2).
			PaddingLeft(1).
			PaddingRight(1),
		listTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(colors.accent),
		listSelectedTitle: lipgloss.NewStyle().
			Bold(true).
			Foreground(colors.text).
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(colors.accent).
			PaddingLeft(1),
		listSelectedDesc: lipgloss.NewStyle().
			Foreground(colors.subtext).
			Border(lipgloss.NormalBorder(), false, false, false, true).
			BorderForeground(colors.accent).
			PaddingLeft(1),
		listNormalTitle: lipgloss.NewStyle().
			Foreground(colors.text).
			PaddingLeft(2),
		listNormalDesc: lipgloss.NewStyle().
			Foreground(colors.muted).
			PaddingLeft(2),
	}
}

type uiColors struct {
	text    lipgloss.Color
	subtext lipgloss.Color
	muted   lipgloss.Color
	accent  lipgloss.Color
	input   lipgloss.Color
	base    lipgloss.Color
	green   lipgloss.Color
}

func catppuccinMocha() uiColors {
	return uiColors{
		text:    lipgloss.Color("#cdd6f4"),
		subtext: lipgloss.Color("#bac2de"),
		muted:   lipgloss.Color("#a6adc8"),
		accent:  lipgloss.Color("#cba6f7"),
		input:   lipgloss.Color("#89b4fa"),
		base:    lipgloss.Color("#1e1e2e"),
		green:   lipgloss.Color("#a6e3a1"),
	}
}

func applyTextInputStyles(input *textinput.Model) {
	colors := catppuccinMocha()
	input.TextStyle = lipgloss.NewStyle().Foreground(colors.input)
	input.PlaceholderStyle = lipgloss.NewStyle().Foreground(colors.muted)
	input.PromptStyle = lipgloss.NewStyle().Foreground(colors.subtext)
	input.CursorStyle = lipgloss.NewStyle().Foreground(colors.input)
}
