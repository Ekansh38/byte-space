package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"byte-space/engine"
)

var (
	// Arrow/flow colors
	clientEngineStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("226")).  // Bright yellow
		Bold(true)

	engineTTYStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("51"))  // Bright cyan

	ttyProgramStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("213"))  // Bright pink

	programTTYStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("83"))  // Bright green

	ttyClientStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("111"))  // Light blue

	// Event type colors
	stateChangeStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("196")).  // Red
		Bold(true)

	timestampStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("240"))  // Dark gray

	detailStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("252"))  // Light gray

	borderStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("238")).
		Bold(true)

	hexStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("93"))  // Purple
)

type Model struct {
	events   chan engine.Event
	logLines []string
	width    int
	height   int
}

func NewModel(events chan engine.Event) Model {
	return Model{
		events:   events,
		logLines: make([]string, 0),
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
		logLine := formatEvent(e)
		m.logLines = append(m.logLines, logLine)

		if len(m.logLines) > 500 {
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

func formatKey(key interface{}) string {
	keyStr := fmt.Sprintf("%v", key)
	
	// Show special keys
	switch keyStr {
	case "\r":
		return hexStyle.Render("CR") + detailStyle.Render(" (0x0d)")
	case "\n":
		return hexStyle.Render("LF") + detailStyle.Render(" (0x0a)")
	case "\x03":
		return hexStyle.Render("^C") + detailStyle.Render(" (SIGINT)")
	case "\x04":
		return hexStyle.Render("^D") + detailStyle.Render(" (EOF)")
	case "\x7f":
		return hexStyle.Render("BS") + detailStyle.Render(" (backspace)")
	case "\x1b[A":
		return hexStyle.Render("↑") + detailStyle.Render(" (up)")
	case "\x1b[B":
		return hexStyle.Render("↓") + detailStyle.Render(" (down)")
	case "\x1b[C":
		return hexStyle.Render("→") + detailStyle.Render(" (right)")
	case "\x1b[D":
		return hexStyle.Render("←") + detailStyle.Render(" (left)")
	default:
		// Show printable + hex
		if len(keyStr) == 1 && keyStr[0] >= 32 && keyStr[0] <= 126 {
			hex := fmt.Sprintf("0x%02x", keyStr[0])
			return hexStyle.Render(keyStr) + detailStyle.Render(fmt.Sprintf(" (%s)", hex))
		}
		return hexStyle.Render(fmt.Sprintf("%q", keyStr))
	}
}

func truncate(s string, max int) string {
	if len(s) > max {
		return s[:max] + detailStyle.Render("...")
	}
	return s
}

func formatEvent(e engine.Event) string {
	timestamp := timestampStyle.Render(e.Timestamp.Format("15:04:05.000"))
	
	var eventType, details string
	
	switch e.Type {
	case engine.EventClientToEngine:
		eventType = clientEngineStyle.Render("CLIENT→ENGINE")
		key := formatKey(e.Data["key"])
		tty := detailStyle.Render(fmt.Sprintf("tty=%v", e.Data["tty"]))
		details = fmt.Sprintf("%s %s", key, tty)
		
	case engine.EventEngineToTTY:
		eventType = engineTTYStyle.Render("ENGINE→TTY   ")
		key := formatKey(e.Data["key"])
		
		mode := "raw"
		if canonical, ok := e.Data["canonical"].(bool); ok && canonical {
			mode = "canonical"
		}
		echo := "echo=off"
		if echoOn, ok := e.Data["echo"].(bool); ok && echoOn {
			echo = "echo=on"
		}
		
		modeInfo := detailStyle.Render(fmt.Sprintf("mode=%s %s", mode, echo))
		tty := detailStyle.Render(fmt.Sprintf("tty=%v", e.Data["tty"]))
		details = fmt.Sprintf("%s %s %s", key, modeInfo, tty)
		
	case engine.EventTTYToProgram:
		eventType = ttyProgramStyle.Render("TTY→PROGRAM  ")
		prog := detailStyle.Render(fmt.Sprintf("prog=%v", e.Data["prog"]))
		
		if cmd, ok := e.Data["cmd"]; ok {
			cmdStr := hexStyle.Render(fmt.Sprintf("%q", cmd))
			bufLen := detailStyle.Render(fmt.Sprintf("len=%d", len(fmt.Sprintf("%v", cmd))))
			details = fmt.Sprintf("cmd=%s %s %s", cmdStr, bufLen, prog)
		} else {
			key := formatKey(e.Data["key"])
			details = fmt.Sprintf("key=%s %s", key, prog)
		}
		
	case engine.EventProgramToTTY:
		eventType = programTTYStyle.Render("PROGRAM→TTY  ")
		output := fmt.Sprintf("%v", e.Data["output"])
		prog := detailStyle.Render(fmt.Sprintf("prog=%v", e.Data["prog"]))
		
		// Show output with escapes visible
		displayOutput := truncate(output, 60)
		displayOutput = strings.ReplaceAll(displayOutput, "\n", hexStyle.Render("\\n"))
		displayOutput = strings.ReplaceAll(displayOutput, "\r", hexStyle.Render("\\r"))
		
		byteCount := detailStyle.Render(fmt.Sprintf("bytes=%d", len(output)))
		details = fmt.Sprintf("data=%q %s %s", displayOutput, byteCount, prog)
		
	case engine.EventTTYToClient:
		eventType = ttyClientStyle.Render("TTY→CLIENT   ")
		output := fmt.Sprintf("%v", e.Data["output"])
		tty := detailStyle.Render(fmt.Sprintf("tty=%v", e.Data["tty"]))
		
		// Show output with escapes visible
		displayOutput := truncate(output, 50)
		displayOutput = strings.ReplaceAll(displayOutput, "\n", hexStyle.Render("\\n"))
		displayOutput = strings.ReplaceAll(displayOutput, "\r", hexStyle.Render("\\r"))
		
		byteCount := detailStyle.Render(fmt.Sprintf("bytes=%d", len(output)))
		details = fmt.Sprintf("ansi=%q %s %s", displayOutput, byteCount, tty)
		
	case engine.EventTTYCreated:
		eventType = stateChangeStyle.Render("[ TTY_INIT ]   ")
		ttyID := e.Data["tty_id"]
		details = detailStyle.Render(fmt.Sprintf("created tty=%v canonical=true echo=true", ttyID))
		
	case engine.EventTTYModeChanged:
		eventType = stateChangeStyle.Render("[ TTY_MODE ]   ")
		tty := e.Data["tty_id"]
		mode := e.Data["mode"]
		echo := e.Data["echo"]
		details = detailStyle.Render(fmt.Sprintf("tty=%v mode=%v echo=%v", tty, mode, echo))
		
	case engine.EventForegroundChanged:
		eventType = stateChangeStyle.Render("[ FOREGROUND ] ")
		prog := e.Data["program"]
		tty := e.Data["tty_id"]
		details = detailStyle.Render(fmt.Sprintf("tty=%v → prog=%v (has input control)", tty, prog))
		
	case engine.EventProgramStarted:
		eventType = stateChangeStyle.Render("[ PROG_START ] ")
		progID := e.Data["program_id"]
		progType := e.Data["type"]
		details = detailStyle.Render(fmt.Sprintf("spawned %v id=%v", progType, progID))
		
	case engine.EventProgramExited:
		eventType = stateChangeStyle.Render("[ PROG_EXIT ]  ")
		progID := e.Data["program_id"]
		status := e.Data["status"]
		runtime := e.Data["runtime"]
		details = detailStyle.Render(fmt.Sprintf("terminated id=%v status=%v runtime=%v", progID, status, runtime))
		
	case engine.EventSessionCreated:
		eventType = stateChangeStyle.Render("[ SESSION ]    ")
		sessionID := e.Data["session_id"]
		user := e.Data["user"]
		computer := e.Data["computer"]
		details = detailStyle.Render(fmt.Sprintf("established session=%v user=%v@%v", sessionID, user, computer))
		
	default:
		eventType = lipgloss.NewStyle().
			Foreground(lipgloss.Color("248")).
			Render(fmt.Sprintf("[ %-10s ]", string(e.Type)))
		details = detailStyle.Render(fmt.Sprintf("%v", e.Data))
	}
	
	return fmt.Sprintf("%s │ %s │ %s", timestamp, eventType, details)
}

func (m Model) View() string {
	if m.width == 0 {
		return "Initializing..."
	}
	
	// Top border with stats
	stats := detailStyle.Render(fmt.Sprintf(
		"  events:%d  |  buffer:%d/500  ",
		len(m.logLines),
		len(m.logLines),
	))
	
	topBorder := borderStyle.Render("┌" + strings.Repeat("─", m.width-2) + "┐")
	statLine := borderStyle.Render("│") + stats + 
		borderStyle.Render(strings.Repeat(" ", m.width-len(stats)-2)) + 
		borderStyle.Render("│")
	divider := borderStyle.Render("├" + strings.Repeat("─", m.width-2) + "┤")
	
	// Event log
	maxLines := m.height - 6
	if maxLines < 1 {
		maxLines = 20
	}

	start := 0
	if len(m.logLines) > maxLines {
		start = len(m.logLines) - maxLines
	}

	var logs strings.Builder
	for i := start; i < len(m.logLines); i++ {
		logs.WriteString(borderStyle.Render("│") + " " + m.logLines[i])
		// Pad to width
		lineLen := lipgloss.Width(m.logLines[i])
		padding := m.width - lineLen - 4
		if padding > 0 {
			logs.WriteString(strings.Repeat(" ", padding))
		}
		logs.WriteString(borderStyle.Render("│") + "\n")
	}

	// Bottom border
	bottomDivider := borderStyle.Render("├" + strings.Repeat("─", m.width-2) + "┤")
	
	controls := detailStyle.Render("  q:quit  |  ctrl+c:exit  ")
	controlLine := borderStyle.Render("│") + controls +
		borderStyle.Render(strings.Repeat(" ", m.width-len(controls)-2)) +
		borderStyle.Render("│")
	
	bottomBorder := borderStyle.Render("└" + strings.Repeat("─", m.width-2) + "┘")

	return topBorder + "\n" +
		statLine + "\n" +
		divider + "\n" +
		logs.String() +
		bottomDivider + "\n" +
		controlLine + "\n" +
		bottomBorder
}
