package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"byte-space/engine"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	// Connection status colors
	activeStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00ff00")).
		Bold(true)

	connectingStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ffff00"))

	closedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ff0000"))

	selectedStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00ffff")).
		Bold(true).
		Background(lipgloss.Color("#1a1a1a"))

	// Event type colors
	keystrokeStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#4275ff"))

	executeStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ff00ff")).
		Bold(true)

	outputStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00ff88"))

	stateChangeStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ff4444")).
		Bold(true)

	timestampStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#666666"))

	detailStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#aaaaaa"))

	hexStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ff66ff"))

	bufferStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ffff00")).
		Bold(true)

	titleStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ff0000")).
		Bold(true).
		Underline(true)

	labelStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#888888"))

	valueStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#ffffff")).
		Bold(true)

	borderStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#444444"))

	ttyTagStyle = lipgloss.NewStyle().
		Foreground(lipgloss.Color("#00aaff")).
		Bold(true)
)

type Model struct {
	events       chan engine.Event
	connections  map[string]*ConnectionState
	selectedTTY  string
	showAllLogs  bool
	width        int
	height       int
	startTime    time.Time
	totalEvents  int
}

type ConnectionState struct {
	TTYID          string
	SessionID      string
	SessionUser    string
	Computer       string
	WorkingDir     string
	Mode           string
	Echo           bool
	Buffer         string
	CursorPos      int
	ForegroundProg string
	ProgramStack   []string
	Status         string
	ConnectedAt    time.Time
	LastActivity   time.Time
	LogLines       []string
	EventCount     int
	KeystrokeCount int
	BytesSent      int
	BytesReceived  int
}

func NewModel(events chan engine.Event) Model {
	return Model{
		events:      events,
		connections: make(map[string]*ConnectionState),
		showAllLogs: false,
		startTime:   time.Now(),
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
		m.totalEvents++

		return m, waitForEvent(m.events)

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		case "a":
			m.showAllLogs = !m.showAllLogs
		case "1", "2", "3", "4", "5", "6", "7", "8", "9":
			// Jump to TTY by number
			idx := int(msg.String()[0] - '0')
			ttyIDs := m.getSortedTTYIDs()
			if idx > 0 && idx <= len(ttyIDs) {
				m.selectedTTY = ttyIDs[idx-1]
			}
		case "tab":
			m.cycleConnection()
		}
	}

	return m, nil
}

func (m Model) getSortedTTYIDs() []string {
	ttyIDs := make([]string, 0, len(m.connections))
	for id := range m.connections {
		ttyIDs = append(ttyIDs, id)
	}
	sort.Strings(ttyIDs)
	return ttyIDs
}

func (m *Model) cycleConnection() {
	if len(m.connections) == 0 {
		return
	}

	ttyIDs := m.getSortedTTYIDs()

	// Find current index
	currentIdx := -1
	for i, id := range ttyIDs {
		if id == m.selectedTTY {
			currentIdx = i
			break
		}
	}

	// Cycle to next
	nextIdx := (currentIdx + 1) % len(ttyIDs)
	m.selectedTTY = ttyIDs[nextIdx]
}

func getTTYIDFromEvent(e engine.Event) string {
	if ttyID, ok := e.Data["tty_id"].(string); ok {
		return ttyID
	}
	if ttyID, ok := e.Data["tty"].(string); ok {
		return ttyID
	}
	return ""
}

func (m *Model) updateState(e engine.Event) {
	ttyID := getTTYIDFromEvent(e)
	if ttyID == "" {
		return
	}

	// Handle TTY close first
	if e.Type == engine.EventTTYClosed {
		delete(m.connections, ttyID)
		// Select another TTY if this was selected
		if m.selectedTTY == ttyID {
			for id := range m.connections {
				m.selectedTTY = id
				break
			}
		}
		return
	}

	// Create connection if doesn't exist
	if _, ok := m.connections[ttyID]; !ok {
		m.connections[ttyID] = &ConnectionState{
			TTYID:         ttyID,
			Status:        "connecting",
			ConnectedAt:   time.Now(),
			LastActivity:  time.Now(),
			LogLines:      make([]string, 0),
			ProgramStack:  make([]string, 0),
		}
		if m.selectedTTY == "" {
			m.selectedTTY = ttyID
		}
	}

	conn := m.connections[ttyID]
	conn.LastActivity = time.Now()

	// Update state based on event
	switch e.Type {
	case engine.EventTTYCreated:
		conn.Status = "connecting"

	case engine.EventSessionCreated:
		if user, ok := e.Data["user"].(string); ok {
			conn.SessionUser = user
		}
		if computer, ok := e.Data["computer"].(string); ok {
			conn.Computer = computer
		}
		if sessionID, ok := e.Data["session_id"].(string); ok {
			conn.SessionID = sessionID
		}
		conn.Status = "active"

		// Set working dir
		if conn.SessionUser == "root" {
			conn.WorkingDir = "/root"
		} else {
			conn.WorkingDir = "/home/" + conn.SessionUser
		}

	case engine.EventEngineToTTY:
		if mode, ok := e.Data["canonical"].(bool); ok {
			if mode {
				conn.Mode = "CANONICAL"
			} else {
				conn.Mode = "RAW"
			}
		}
		if echo, ok := e.Data["echo"].(bool); ok {
			conn.Echo = echo
		}
		conn.KeystrokeCount++

	case engine.EventForegroundChanged:
		if prog, ok := e.Data["program"].(string); ok {
			conn.ForegroundProg = prog
		}

	case engine.EventProgramStarted:
		if progID, ok := e.Data["program_id"].(string); ok {
			conn.ProgramStack = append(conn.ProgramStack, progID)
		}

	case engine.EventProgramExited:
		if progID, ok := e.Data["program_id"].(string); ok {
			// Remove from stack
			for i, p := range conn.ProgramStack {
				if p == progID {
					conn.ProgramStack = append(conn.ProgramStack[:i], conn.ProgramStack[i+1:]...)
					break
				}
			}
		}

	case engine.EventTTYToClient:
		if output, ok := e.Data["output"].(string); ok {
			conn.BytesSent += len(output)
		}

	case engine.EventClientToEngine:
		if key, ok := e.Data["key"].(string); ok {
			conn.BytesReceived += len(key)
		}

	case engine.EventBufferChanged:
		if buffer, ok := e.Data["buffer"].(string); ok {
			conn.Buffer = buffer
		}
		if cursor, ok := e.Data["cursor"].(int); ok {
			conn.CursorPos = cursor
		}
	}

	// Add to log
	logLine := formatEventWithTTY(e, ttyID)
	if logLine != "" {
		conn.LogLines = append(conn.LogLines, logLine)
		if len(conn.LogLines) > 500 {
			conn.LogLines = conn.LogLines[1:]
		}
		conn.EventCount++
	}
}

func formatKey(key interface{}) string {
	keyStr := fmt.Sprintf("%v", key)

	switch keyStr {
	case "\r":
		return hexStyle.Render("CR") + detailStyle.Render("(0x0d)")
	case "\n":
		return hexStyle.Render("LF") + detailStyle.Render("(0x0a)")
	case "\x03":
		return hexStyle.Render("^C") + detailStyle.Render("(SIGINT)")
	case "\x7f":
		return hexStyle.Render("DEL") + detailStyle.Render("(0x7f)")
	case "\x1b[A":
		return hexStyle.Render("↑") + detailStyle.Render("(up)")
	case "\x1b[B":
		return hexStyle.Render("↓") + detailStyle.Render("(down)")
	case "\x1b[C":
		return hexStyle.Render("→") + detailStyle.Render("(right)")
	case "\x1b[D":
		return hexStyle.Render("←") + detailStyle.Render("(left)")
	case " ":
		return hexStyle.Render("SPC") + detailStyle.Render("(0x20)")
	case "\t":
		return hexStyle.Render("TAB") + detailStyle.Render("(0x09)")
	default:
		if len(keyStr) == 1 && keyStr[0] >= 32 && keyStr[0] <= 126 {
			hex := fmt.Sprintf("0x%02x", keyStr[0])
			return hexStyle.Render(fmt.Sprintf("'%s'", keyStr)) + detailStyle.Render(fmt.Sprintf("(%s)", hex))
		}
		return hexStyle.Render(fmt.Sprintf("%q", keyStr))
	}
}

func formatEventWithTTY(e engine.Event, ttyID string) string {
	timestamp := timestampStyle.Render(e.Timestamp.Format("15:04:05.000"))
	ttyTag := ttyTagStyle.Render(fmt.Sprintf("[%s]", ttyID))

	var eventType, details string

	switch e.Type {
	case engine.EventEngineToTTY:
		eventType = keystrokeStyle.Render("KEYSTROKE ")
		key := formatKey(e.Data["key"])
		details = key

	case engine.EventClientToEngine:
		return "" // Skip

	case engine.EventTTYToProgram:
		if cmd, ok := e.Data["cmd"]; ok {
			eventType = executeStyle.Render("EXECUTE   ")
			cmdStr := bufferStyle.Render(fmt.Sprintf("%q", cmd))
			prog := detailStyle.Render(fmt.Sprintf("→ %v", e.Data["prog"]))
			bufLen := detailStyle.Render(fmt.Sprintf("len=%d", len(fmt.Sprintf("%v", cmd))))
			details = fmt.Sprintf("%s %s %s", cmdStr, bufLen, prog)
		} else {
			eventType = keystrokeStyle.Render("KEYSTROKE ")
			key := formatKey(e.Data["key"])
			prog := detailStyle.Render(fmt.Sprintf("→ %v", e.Data["prog"]))
			details = fmt.Sprintf("%s %s", key, prog)
		}

	case engine.EventTTYToClient:
		eventType = outputStyle.Render("OUTPUT    ")
		output := fmt.Sprintf("%v", e.Data["output"])

		displayOutput := output
		if len(displayOutput) > 35 {
			displayOutput = displayOutput[:35] + "..."
		}
		displayOutput = strings.ReplaceAll(displayOutput, "\n", hexStyle.Render("\\n"))
		displayOutput = strings.ReplaceAll(displayOutput, "\r", hexStyle.Render("\\r"))
		displayOutput = strings.ReplaceAll(displayOutput, "\b", hexStyle.Render("\\b"))

		byteCount := detailStyle.Render(fmt.Sprintf("(%d bytes)", len(output)))
		details = fmt.Sprintf("%q %s", displayOutput, byteCount)

	case engine.EventTTYCreated:
		eventType = stateChangeStyle.Render("[TTY_INIT]")
		details = detailStyle.Render("canonical=true echo=true")

	case engine.EventTTYModeChanged:
		eventType = stateChangeStyle.Render("[TTY_MODE]")
		details = detailStyle.Render(fmt.Sprintf("mode=%v echo=%v", e.Data["mode"], e.Data["echo"]))

	case engine.EventForegroundChanged:
		eventType = stateChangeStyle.Render("[FGRND_CH]")
		details = detailStyle.Render(fmt.Sprintf("→ %v", e.Data["program"]))

	case engine.EventProgramStarted:
		eventType = stateChangeStyle.Render("[PRG_STRT]")
		details = detailStyle.Render(fmt.Sprintf("%v spawned", e.Data["program_id"]))

	case engine.EventProgramExited:
		eventType = stateChangeStyle.Render("[PRG_EXIT]")
		progID := e.Data["program_id"]
		status := e.Data["status"]
		details = detailStyle.Render(fmt.Sprintf("%v exited status=%v", progID, status))

	case engine.EventSessionCreated:
		eventType = stateChangeStyle.Render("[SESSION ]")
		details = detailStyle.Render(fmt.Sprintf("%v@%v session=%v", e.Data["user"], e.Data["computer"], e.Data["session_id"]))

	case engine.EventBufferChanged:
		return "" // Don't show in log, only update state

	default:
		return ""
	}

	return fmt.Sprintf("%s %s %s │ %s", timestamp, ttyTag, eventType, details)
}

func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	// Three column layout
	leftWidth := 25
	middleWidth := 25
	rightWidth := m.width - leftWidth - middleWidth - 6

	leftPanel := m.renderConnectionList(leftWidth)
	middlePanel := m.renderSelectedState(middleWidth)
	rightPanel := m.renderLog(rightWidth)

	return m.combinePanels(leftPanel, middlePanel, rightPanel, leftWidth, middleWidth, rightWidth)
}

func (m Model) renderConnectionList(width int) string {
	var s strings.Builder

	s.WriteString(titleStyle.Render("CONNECTIONS") + "\n")
	s.WriteString(borderStyle.Render(strings.Repeat("─", width)) + "\n\n")

	if len(m.connections) == 0 {
		s.WriteString(detailStyle.Render("No connections\n"))
		return s.String()
	}

	ttyIDs := m.getSortedTTYIDs()

	for _, ttyID := range ttyIDs {
		conn := m.connections[ttyID]

		// Status indicator
		var statusIcon string
		var statusColor lipgloss.Style
		switch conn.Status {
		case "active":
			statusIcon = "●"
			statusColor = activeStyle
		case "connecting":
			statusIcon = "○"
			statusColor = connectingStyle
		case "closed":
			statusIcon = "✕"
			statusColor = closedStyle
		}

		// Highlight selected
		ttyLine := fmt.Sprintf("%s %s", statusIcon, ttyID)
		if ttyID == m.selectedTTY {
			ttyLine = selectedStyle.Render(ttyLine)
		} else {
			ttyLine = statusColor.Render(ttyLine)
		}

		s.WriteString(ttyLine + "\n")

		// Connection details
		if conn.SessionUser != "" {
			userInfo := fmt.Sprintf("  %s@%s", conn.SessionUser, conn.Computer)
			s.WriteString(detailStyle.Render(userInfo) + "\n")
		} else {
			s.WriteString(detailStyle.Render("  (no session)") + "\n")
		}

		if conn.ForegroundProg != "" {
			progInfo := fmt.Sprintf("  %s", conn.ForegroundProg)
			s.WriteString(valueStyle.Render(progInfo) + "\n")
		}

		if conn.WorkingDir != "" {
			s.WriteString(detailStyle.Render(fmt.Sprintf("  %s", conn.WorkingDir)) + "\n")
		}

		// Stats
		uptime := time.Since(conn.ConnectedAt).Round(time.Second)
		s.WriteString(detailStyle.Render(fmt.Sprintf("  ↑%d ↓%d │ %d evts │ %v",
			conn.BytesSent, conn.BytesReceived, conn.EventCount, uptime)) + "\n")

		s.WriteString("\n")
	}

	return s.String()
}

func (m Model) renderSelectedState(width int) string {
	var s strings.Builder

	if m.selectedTTY == "" {
		s.WriteString(titleStyle.Render("NO CONNECTION SELECTED") + "\n")
		return s.String()
	}

	conn, ok := m.connections[m.selectedTTY]
	if !ok {
		s.WriteString(titleStyle.Render("CONNECTION NOT FOUND") + "\n")
		return s.String()
	}

	s.WriteString(titleStyle.Render(fmt.Sprintf("STATE: %s", m.selectedTTY)) + "\n")
	s.WriteString(borderStyle.Render(strings.Repeat("─", width)) + "\n\n")

	// TTY Mode
	s.WriteString(labelStyle.Render("MODE: ") + valueStyle.Render(conn.Mode) + "\n")

	// Echo
	echoStr := "OFF"
	if conn.Echo {
		echoStr = "ON"
	}
	s.WriteString(labelStyle.Render("ECHO: ") + valueStyle.Render(echoStr) + "\n\n")

	// Buffer
	s.WriteString(labelStyle.Render("BUFFER: "))
	if conn.Buffer == "" {
		s.WriteString(detailStyle.Render("(empty)") + "\n")
	} else {
		s.WriteString(bufferStyle.Render(fmt.Sprintf("%q", conn.Buffer)) + "\n")
	}
	s.WriteString(detailStyle.Render(fmt.Sprintf("  len=%d cursor=%d", len(conn.Buffer), conn.CursorPos)) + "\n\n")

	// Foreground program
	s.WriteString(labelStyle.Render("FOREGROUND: "))
	if conn.ForegroundProg == "" {
		s.WriteString(detailStyle.Render("(none)") + "\n\n")
	} else {
		s.WriteString(valueStyle.Render(conn.ForegroundProg) + "\n\n")
	}

	// Program stack
	s.WriteString(labelStyle.Render("PROGRAM STACK:") + "\n")
	if len(conn.ProgramStack) == 0 {
		s.WriteString(detailStyle.Render("  (empty)") + "\n")
	} else {
		for i := len(conn.ProgramStack) - 1; i >= 0; i-- {
			prog := conn.ProgramStack[i]
			if prog == conn.ForegroundProg {
				s.WriteString(valueStyle.Render(fmt.Sprintf("  [FG] %s", prog)) + "\n")
			} else {
				s.WriteString(detailStyle.Render(fmt.Sprintf("  [BG] %s", prog)) + "\n")
			}
		}
	}
	s.WriteString("\n")

	// Session
	s.WriteString(labelStyle.Render("SESSION:") + "\n")
	if conn.SessionUser == "" {
		s.WriteString(detailStyle.Render("  (none)") + "\n")
	} else {
		s.WriteString(valueStyle.Render(fmt.Sprintf("  %s@%s", conn.SessionUser, conn.Computer)) + "\n")
		s.WriteString(detailStyle.Render(fmt.Sprintf("  session=%s", conn.SessionID)) + "\n")
		s.WriteString(detailStyle.Render(fmt.Sprintf("  cwd=%s", conn.WorkingDir)) + "\n")
	}
	s.WriteString("\n")

	// Stats
	s.WriteString(labelStyle.Render("STATISTICS:") + "\n")
	s.WriteString(detailStyle.Render(fmt.Sprintf("  Events: %d", conn.EventCount)) + "\n")
	s.WriteString(detailStyle.Render(fmt.Sprintf("  Keystrokes: %d", conn.KeystrokeCount)) + "\n")
	s.WriteString(detailStyle.Render(fmt.Sprintf("  Sent: %d bytes", conn.BytesSent)) + "\n")
	s.WriteString(detailStyle.Render(fmt.Sprintf("  Received: %d bytes", conn.BytesReceived)) + "\n")

	uptime := time.Since(conn.ConnectedAt)
	lastActivity := time.Since(conn.LastActivity)
	s.WriteString(detailStyle.Render(fmt.Sprintf("  Uptime: %v", uptime.Round(time.Second))) + "\n")
	s.WriteString(detailStyle.Render(fmt.Sprintf("  Last activity: %v ago", lastActivity.Round(time.Second))) + "\n")

	return s.String()
}

func (m Model) renderLog(width int) string {
	var s strings.Builder

	if m.showAllLogs {
		s.WriteString(titleStyle.Render("EVENT LOG [ALL]") + "\n")
	} else {
		s.WriteString(titleStyle.Render(fmt.Sprintf("EVENT LOG [%s]", m.selectedTTY)) + "\n")
	}
	s.WriteString(borderStyle.Render(strings.Repeat("─", width)) + "\n")

	maxLines := m.height - 3

	var logLines []string

	if m.showAllLogs {
		// Collect all logs from all connections
		for _, ttyID := range m.getSortedTTYIDs() {
			if conn, ok := m.connections[ttyID]; ok {
				logLines = append(logLines, conn.LogLines...)
			}
		}
	} else {
		// Show selected TTY only
		if conn, ok := m.connections[m.selectedTTY]; ok {
			logLines = conn.LogLines
		}
	}

	start := 0
	if len(logLines) > maxLines {
		start = len(logLines) - maxLines
	}

	for i := start; i < len(logLines); i++ {
		s.WriteString(logLines[i] + "\n")
	}

	return s.String()
}

func (m Model) combinePanels(left, middle, right string, leftW, middleW, rightW int) string {
	leftLines := strings.Split(left, "\n")
	middleLines := strings.Split(middle, "\n")
	rightLines := strings.Split(right, "\n")

	maxLines := m.height - 4
	if maxLines < 1 {
		maxLines = 40
	}

	var result strings.Builder

	// Top border
	totalWidth := leftW + middleW + rightW + 2
	result.WriteString(borderStyle.Render("┌" + strings.Repeat("─", leftW) + "┬" +
		strings.Repeat("─", middleW) + "┬" + strings.Repeat("─", rightW) + "┐") + "\n")

	for i := 0; i < maxLines; i++ {
		result.WriteString(borderStyle.Render("│"))

		// Left panel
		if i < len(leftLines) {
			line := leftLines[i]
			result.WriteString(line)
			padding := leftW - lipgloss.Width(line)
			if padding > 0 {
				result.WriteString(strings.Repeat(" ", padding))
			}
		} else {
			result.WriteString(strings.Repeat(" ", leftW))
		}

		result.WriteString(borderStyle.Render("│"))

		// Middle panel
		if i < len(middleLines) {
			line := middleLines[i]
			result.WriteString(line)
			padding := middleW - lipgloss.Width(line)
			if padding > 0 {
				result.WriteString(strings.Repeat(" ", padding))
			}
		} else {
			result.WriteString(strings.Repeat(" ", middleW))
		}

		result.WriteString(borderStyle.Render("│"))

		// Right panel
		if i < len(rightLines) {
			line := rightLines[i]
			result.WriteString(line)
			padding := rightW - lipgloss.Width(line)
			if padding > 0 {
				result.WriteString(strings.Repeat(" ", padding))
			}
		} else {
			result.WriteString(strings.Repeat(" ", rightW))
		}

		result.WriteString(borderStyle.Render("│") + "\n")
	}

	// Bottom border + controls
	result.WriteString(borderStyle.Render("├" + strings.Repeat("─", leftW) + "┴" +
		strings.Repeat("─", middleW) + "┴" + strings.Repeat("─", rightW) + "┤") + "\n")

	// Controls - SINGLE LINE
	controls := fmt.Sprintf(
		" tab:cycle │ 1-9:select │ a:all logs(%s) │ q:quit │ events:%d │ uptime:%v ",
		func() string {
			if m.showAllLogs {
				return "ON"
			}
			return "OFF"
		}(),
		m.totalEvents,
		time.Since(m.startTime).Round(time.Second),
	)

	controlLine := borderStyle.Render("│") + detailStyle.Render(controls)
	padding := totalWidth - lipgloss.Width(controls)
	if padding > 0 {
		controlLine += strings.Repeat(" ", padding)
	}
	controlLine += borderStyle.Render("│")

	result.WriteString(controlLine + "\n")
	result.WriteString(borderStyle.Render("└" + strings.Repeat("─", totalWidth) + "┘"))

	return result.String()
}
