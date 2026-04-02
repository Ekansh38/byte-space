package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"byte-space/engine"
)

var (
	clientEngineStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("226")).
		Bold(true)

	engineTTYStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("51"))

	ttyProgramStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("213"))

	ttyClientStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("111"))

	stateChangeStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("196")).
		Bold(true)

	timestampStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))

	detailStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))

	borderStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("238"))

	hexStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("93"))
		
	bufferStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("228")).
		Bold(true)
		
	titleStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("196")).
		Bold(true)
)

type Model struct {
	events      chan engine.Event
	logLines    []string
	ttyState    *TTYState
	width       int
	height      int
}

type TTYState struct {
	Mode           string
	Echo           bool
	Buffer         string
	CursorPos      int
	ForegroundProg string
	SessionUser    string
}

func NewModel(events chan engine.Event) Model {
	return Model{
		events:    events,
		logLines:  make([]string, 0),
		ttyState:  &TTYState{Mode: "CANONICAL", Echo: true},
	}
}

type eventMsg engine.Event

func waitForEvent(ch chan engine.Event) tea.Cmd {
	return func() tea.Msg {
		return eventMsg(<-ch)
	}
}

func (m Model) Init() tea.Cmd {
	return waitForEvent(m.events)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case eventMsg:
		e := engine.Event(msg)
		m.updateState(e)
		logLine := formatEvent(e)
		
		// Only add non-empty lines
		if logLine != "" {
			m.logLines = append(m.logLines, logLine)
		}

		if len(m.logLines) > 1000 {
			m.logLines = m.logLines[1:]
		}

		return m, waitForEvent(m.events)

	case tea.KeyMsg:
		if msg.String() == "q" || msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m *Model) updateState(e engine.Event) {
	switch e.Type {
	case engine.EventEngineToTTY:
		if mode, ok := e.Data["canonical"].(bool); ok {
			if mode {
				m.ttyState.Mode = "CANONICAL"
			} else {
				m.ttyState.Mode = "RAW"
			}
		}
		if echo, ok := e.Data["echo"].(bool); ok {
			m.ttyState.Echo = echo
		}
	case engine.EventForegroundChanged:
		if prog, ok := e.Data["program"].(string); ok {
			m.ttyState.ForegroundProg = prog
		}
	case engine.EventSessionCreated:
		if user, ok := e.Data["user"].(string); ok {
			m.ttyState.SessionUser = user
		}
	}
}

func formatKey(key interface{}) string {
	keyStr := fmt.Sprintf("%v", key)
	
	switch keyStr {
	case "\r":
		return hexStyle.Render("CR")
	case "\n":
		return hexStyle.Render("LF")
	case "\x03":
		return hexStyle.Render("^C")
	case "\x7f":
		return hexStyle.Render("DEL")
	case "\x1b[A":
		return hexStyle.Render("↑")
	case "\x1b[B":
		return hexStyle.Render("↓")
	case "\x1b[C":
		return hexStyle.Render("→")
	case "\x1b[D":
		return hexStyle.Render("←")
	case " ":
		return hexStyle.Render("SPC")
	case "\t":
		return hexStyle.Render("TAB")
	default:
		if len(keyStr) == 1 && keyStr[0] >= 32 && keyStr[0] <= 126 {
			return hexStyle.Render(fmt.Sprintf("'%s'", keyStr))
		}
		return hexStyle.Render(fmt.Sprintf("%q", keyStr))
	}
}

func formatEvent(e engine.Event) string {
	timestamp := timestampStyle.Render(e.Timestamp.Format("15:04:05"))
	
	var eventType, details string
	
	switch e.Type {
	// Skip individual keystroke events
	case engine.EventClientToEngine:
		return ""
	case engine.EventEngineToTTY:
		return ""
		
	// Show command execution (collapsed journey)
	case engine.EventTTYToProgram:
		if cmd, ok := e.Data["cmd"]; ok {
			// Command executed - show full journey
			eventType = clientEngineStyle.Render("KEYSTROKE→CMD ")
			cmdStr := bufferStyle.Render(fmt.Sprintf("%q", cmd))
			prog := detailStyle.Render(fmt.Sprintf("→ %v", e.Data["prog"]))
			details = fmt.Sprintf("%s %s", cmdStr, prog)
		} else {
			// Raw mode keystroke - still skip
			return ""
		}
		
	case engine.EventTTYToClient:
		eventType = ttyClientStyle.Render("OUTPUT       ")
		output := fmt.Sprintf("%v", e.Data["output"])
		
		// Truncate and show escapes
		if len(output) > 40 {
			output = output[:40] + "..."
		}
		output = strings.ReplaceAll(output, "\n", "\\n")
		output = strings.ReplaceAll(output, "\r", "\\r")
		
		details = fmt.Sprintf("%q", output)
		
	case engine.EventTTYCreated:
		eventType = stateChangeStyle.Render("[ TTY_INIT ]   ")
		details = detailStyle.Render(fmt.Sprintf("tty=%v", e.Data["tty_id"]))
		
	case engine.EventTTYModeChanged:
		eventType = stateChangeStyle.Render("[ TTY_MODE ]   ")
		details = detailStyle.Render(fmt.Sprintf("%v echo=%v", e.Data["mode"], e.Data["echo"]))
		
	case engine.EventForegroundChanged:
		eventType = stateChangeStyle.Render("[ FOREGROUND ] ")
		details = detailStyle.Render(fmt.Sprintf("%v", e.Data["program"]))
		
	case engine.EventProgramStarted:
		eventType = stateChangeStyle.Render("[ PROG_START ] ")
		details = detailStyle.Render(fmt.Sprintf("%v", e.Data["program_id"]))
		
	case engine.EventProgramExited:
		eventType = stateChangeStyle.Render("[ PROG_EXIT ]  ")
		details = detailStyle.Render(fmt.Sprintf("%v", e.Data["program_id"]))
		
	case engine.EventSessionCreated:
		eventType = stateChangeStyle.Render("[ SESSION ]    ")
		details = detailStyle.Render(fmt.Sprintf("%v@%v", e.Data["user"], e.Data["computer"]))
		
	default:
		return ""
	}
	
	return fmt.Sprintf("%s │ %s │ %s", timestamp, eventType, details)
}

func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}
	
	// Calculate widths
	stateWidth := 35
	logWidth := m.width - stateWidth - 3
	
	// Build state panel (FIXED HEIGHT)
	statePanel := m.renderStatePanel(stateWidth)
	stateLines := strings.Split(statePanel, "\n")
	
	// Build log panel (SCROLLS)
	logPanel := m.renderLogPanel(logWidth)
	logLines := strings.Split(logPanel, "\n")
	
	// Combine with fixed state height
	var result strings.Builder
	
	for i := 0; i < m.height; i++ {
		// Left side (state) - fixed, doesn't scroll
		if i < len(stateLines) {
			line := stateLines[i]
			result.WriteString(line)
			padding := stateWidth - lipgloss.Width(line)
			if padding > 0 {
				result.WriteString(strings.Repeat(" ", padding))
			}
		} else {
			// After state panel ends, just show empty space
			result.WriteString(strings.Repeat(" ", stateWidth))
		}
		
		// Divider
		result.WriteString(" │ ")
		
		// Right side (log) - scrolls
		if i < len(logLines) {
			result.WriteString(logLines[i])
		}
		
		result.WriteString("\n")
	}
	
	return result.String()
}

func (m Model) renderStatePanel(width int) string {
	var s strings.Builder
	
	s.WriteString(titleStyle.Render("ENGINE STATE") + "\n")
	s.WriteString(strings.Repeat("─", width) + "\n\n")
	
	s.WriteString("MODE: " + bufferStyle.Render(m.ttyState.Mode) + "\n")
	
	echoStr := "OFF"
	if m.ttyState.Echo {
		echoStr = "ON"
	}
	s.WriteString("ECHO: " + bufferStyle.Render(echoStr) + "\n\n")
	
	if m.ttyState.Buffer == "" {
		s.WriteString("BUFFER: " + detailStyle.Render("(empty)") + "\n")
	} else {
		s.WriteString("BUFFER: " + bufferStyle.Render(fmt.Sprintf("%q", m.ttyState.Buffer)) + "\n")
	}
	s.WriteString(detailStyle.Render(fmt.Sprintf("  len=%d pos=%d", len(m.ttyState.Buffer), m.ttyState.CursorPos)) + "\n\n")
	
	if m.ttyState.ForegroundProg == "" {
		s.WriteString("FOREGROUND: " + detailStyle.Render("(none)") + "\n")
	} else {
		s.WriteString("FOREGROUND:\n  " + bufferStyle.Render(m.ttyState.ForegroundProg) + "\n")
	}
	s.WriteString("\n")
	
	if m.ttyState.SessionUser == "" {
		s.WriteString("SESSION: " + detailStyle.Render("(none)") + "\n")
	} else {
		s.WriteString("SESSION: " + bufferStyle.Render(m.ttyState.SessionUser) + "\n")
	}
	
	return s.String()
}

func (m Model) renderLogPanel(width int) string {
	var s strings.Builder
	
	s.WriteString(titleStyle.Render("EVENT LOG") + "\n")
	s.WriteString(strings.Repeat("─", width) + "\n")
	
	// Show last N lines that fit
	maxLogLines := m.height - 2
	start := 0
	if len(m.logLines) > maxLogLines {
		start = len(m.logLines) - maxLogLines
	}
	
	for i := start; i < len(m.logLines); i++ {
		s.WriteString(m.logLines[i] + "\n")
	}
	
	return s.String()
}
