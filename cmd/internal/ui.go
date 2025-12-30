package internal

import (
	"fmt"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/log"
)

type UI struct {
	interactive bool
}

func NewUI(interactive bool) *UI {
	return &UI{
		interactive: interactive,
	}
}

func (ui *UI) IsInteractive() bool {
	return ui.interactive
}

func (ui *UI) NewModel(baseURL string, buildRequests chan<- BuildRequest) tea.Model {
	return &model{
		baseURL:       baseURL,
		buildRequests: buildRequests,
		started:       time.Now(),
		maxLines:      14,
	}
}

func (ui *UI) LogEvent(message string) {
	if !ui.interactive {
		log.Print(message)
	}
}

func (ui *UI) BuildResultToMsg(result BuildResult) tea.Msg {
	return buildResultMsg{
		Reason:   result.Reason,
		Paths:    result.Paths,
		Duration: result.Duration,
		Error:    result.Error,
		Number:   result.Number,
	}
}

func (ui *UI) PrintMsg(msg tea.Msg) {
	switch m := msg.(type) {
	case logMsg:
		log.Print(string(m))
	case BuildStartedMsg:
		log.Printf("BUILD #%d start (%s)", m.Number, m.Reason)
	case buildResultMsg:
		if m.Error != nil {
			log.Printf("ERR  build #%d failed in %s (%s): %v", m.Number, m.Duration.Truncate(time.Millisecond), m.Reason, m.Error)
			if len(m.Paths) > 0 {
				log.Printf("     changes: %s", strings.Join(m.Paths, ", "))
			}
			return
		}
		log.Printf("OK   build #%d in %s (%s)", m.Number, m.Duration.Truncate(time.Millisecond), m.Reason)
		if len(m.Paths) > 0 {
			log.Printf("     changes: %s", strings.Join(m.Paths, ", "))
		}
	}
}

// Interactive UI model
type model struct {
	baseURL       string
	buildRequests chan<- BuildRequest
	started       time.Time
	maxLines      int

	buildCount  int
	building    bool
	lastReason  string
	lastDur     time.Duration
	lastErr     string
	lastChanged []string

	logs []string
}

type logMsg string

type buildResultMsg struct {
	Reason   string
	Paths    []string
	Duration time.Duration
	Error    error
	Number   int
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch x := msg.(type) {
	case tea.KeyMsg:
		switch x.String() {
		case "ctrl+c", "q":
			return m, tea.Quit
		case "r":
			select {
			case m.buildRequests <- BuildRequest{Reason: "manual rebuild", Paths: nil}:
				m.appendLog("queued rebuild: manual")
			default:
				m.appendLog("rebuild skipped: request queue full")
			}
			return m, nil
		case "c":
			m.logs = nil
			return m, nil
		}

	case logMsg:
		m.appendLog(string(x))
		return m, nil

	case BuildStartedMsg:
		m.building = true
		m.lastReason = x.Reason
		m.buildCount = x.Number
		m.lastErr = ""
		m.lastChanged = nil
		m.appendLog(fmt.Sprintf("build #%d started: %s", x.Number, x.Reason))
		return m, nil

	case buildResultMsg:
		m.building = false
		m.buildCount = x.Number
		m.lastReason = x.Reason
		m.lastDur = x.Duration
		m.lastChanged = x.Paths
		if x.Error != nil {
			m.lastErr = x.Error.Error()
			m.appendLog(fmt.Sprintf("ERR  build #%d in %s: %v", x.Number, x.Duration.Truncate(time.Millisecond), x.Error))
		} else {
			m.lastErr = ""
			m.appendLog(fmt.Sprintf("OK   build #%d in %s", x.Number, x.Duration.Truncate(time.Millisecond)))
		}
		if len(x.Paths) > 0 {
			m.appendLog("changes: " + strings.Join(x.Paths, ", "))
		}
		return m, nil
	}

	return m, nil
}

func (m model) View() string {
	status := "ready"
	if m.building {
		status = "building"
	}
	if m.lastErr != "" {
		status = "error"
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("shizuka dev  %s  %s\n", status, m.baseURL))

	if m.buildCount == 0 && !m.building {
		b.WriteString("last build: (none yet)\n")
	} else if m.lastErr != "" {
		b.WriteString(fmt.Sprintf("last build: ERR #%d in %s  reason: %s\n", m.buildCount, m.lastDur.Truncate(time.Millisecond), m.lastReason))
	} else {
		b.WriteString(fmt.Sprintf("last build: OK #%d in %s  reason: %s\n", m.buildCount, m.lastDur.Truncate(time.Millisecond), m.lastReason))
	}

	if len(m.lastChanged) > 0 {
		b.WriteString("changes:   " + strings.Join(m.lastChanged, ", ") + "\n")
	} else {
		b.WriteString("changes:   (none)\n")
	}

	b.WriteString("\n")
	for _, line := range m.logs {
		b.WriteString(line)
		b.WriteString("\n")
	}

	b.WriteString("\nkeys: r rebuild   c clear   q quit\n")
	return b.String()
}

func (m *model) appendLog(s string) {
	s = strings.TrimRight(s, "\n")
	if s == "" {
		return
	}
	m.logs = append(m.logs, s)
	if len(m.logs) > m.maxLines {
		m.logs = m.logs[len(m.logs)-m.maxLines:]
	}
}
