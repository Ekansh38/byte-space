// VIBE CODED SLOP DEBUGGER
package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"byte-space/computer"
	"byte-space/engine"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
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

type ViewMode int

const (
	ViewFull ViewMode = iota
	ViewLogsOnly
	ViewStateOnly
	ViewConnectionsOnly
	ViewPermissions
)

type Model struct {
	events      chan computer.Event
	eng         *engine.Engine
	connections map[string]*ConnectionState
	selectedTTY string
	showAllLogs bool
	viewMode    ViewMode
	width       int
	height      int
	startTime   time.Time
	totalEvents int
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
	LogEvents      []computer.Event
	EventCount     int
	KeystrokeCount int
	BytesSent      int
	BytesReceived  int
}

const MaxLogLines = 1000

func NewModel(events chan computer.Event, eng *engine.Engine) Model {
	return Model{
		events:      events,
		eng:         eng,
		connections: make(map[string]*ConnectionState),
		showAllLogs: false,
		viewMode:    ViewFull,
		startTime:   time.Now(),
	}
}

type eventMsg computer.Event

func waitForEvent(ch chan computer.Event) tea.Cmd {
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
		e := computer.Event(msg)
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
		case "v":
			// Cycle view modes
			m.viewMode = (m.viewMode + 1) % 5
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

	currentIdx := -1
	for i, id := range ttyIDs {
		if id == m.selectedTTY {
			currentIdx = i
			break
		}
	}

	m.selectedTTY = ttyIDs[(currentIdx+1)%len(ttyIDs)]
}

func getTTYIDFromEvent(e computer.Event) string {
	if ttyID, ok := e.Data["tty_id"].(string); ok {
		return ttyID
	}
	if ttyID, ok := e.Data["tty"].(string); ok {
		return ttyID
	}
	return ""
}

func (m *Model) updateState(e computer.Event) {
	ttyID := getTTYIDFromEvent(e)
	if ttyID == "" {
		return
	}

	// Handle TTY close first
	if e.Type == computer.EventTTYClosed {
		delete(m.connections, ttyID)
		// Select another TTY if this was selected
		if m.selectedTTY == ttyID {
			m.selectedTTY = ""
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
			TTYID:        ttyID,
			Status:       "connecting",
			ConnectedAt:  time.Now(),
			LastActivity: time.Now(),
			LogEvents:    make([]computer.Event, 0),
			ProgramStack: make([]string, 0),
		}
		if m.selectedTTY == "" {
			m.selectedTTY = ttyID
		}
	}

	conn := m.connections[ttyID]
	conn.LastActivity = time.Now()

	// Update state based on event
	switch e.Type {
	case computer.EventTTYCreated:
		conn.Status = "connecting"

	case computer.EventSessionCreated:
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

	case computer.EventEngineToTTY:
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

	case computer.EventForegroundChanged:
		if prog, ok := e.Data["program"].(string); ok {
			conn.ForegroundProg = prog
		}

	case computer.EventProgramStarted:
		if progID, ok := e.Data["program_id"].(string); ok {
			conn.ProgramStack = append(conn.ProgramStack, progID)
		}

	case computer.EventProgramExited:
		if progID, ok := e.Data["program_id"].(string); ok {
			// Remove from stack
			for i, p := range conn.ProgramStack {
				if p == progID {
					conn.ProgramStack = append(conn.ProgramStack[:i], conn.ProgramStack[i+1:]...)
					break
				}
			}
		}

	case computer.EventTTYToClient:
		if output, ok := e.Data["output"].(string); ok {
			conn.BytesSent += len(output)
		}

	case computer.EventClientToEngine:
		if key, ok := e.Data["key"].(string); ok {
			conn.BytesReceived += len(key)
		}

	case computer.EventBufferChanged:
		if buffer, ok := e.Data["buffer"].(string); ok {
			conn.Buffer = buffer
		}
		if cursor, ok := e.Data["cursor"].(int); ok {
			conn.CursorPos = cursor
		}

	case computer.EventWorkingDirChanged:
		if dir, ok := e.Data["dir"].(string); ok {
			conn.WorkingDir = dir
		}
	}

	// Add to log (use non-compact format just for filtering)
	if formatEventWithTTY(e, ttyID, false) != "" {
		conn.LogEvents = append(conn.LogEvents, e)
		if len(conn.LogEvents) > MaxLogLines {
			conn.LogEvents = conn.LogEvents[len(conn.LogEvents)-MaxLogLines:]
		}
		conn.EventCount++
	}
}

// Smart truncation - show end of string with ellipsis at start
func truncateFromStart(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return "..."
	}
	return "..." + s[len(s)-(maxLen-3):]
}

// Regular truncation - show start with ellipsis at end
func truncateFromEnd(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return "..."
	}
	return s[:maxLen-3] + "..."
}

// truncateLogLine truncates a pre-ANSI-styled string to maxWidth visual columns.
// Unlike truncateFromEnd (byte-based), this skips ANSI escape sequences when
// counting width so the cut point is at the correct visible character.
func truncateLogLine(s string, maxWidth int) string {
	if lipgloss.Width(s) <= maxWidth {
		return s
	}
	if maxWidth <= 3 {
		return "..."
	}
	target := maxWidth - 3 // reserve 3 cols for "..."
	visual := 0
	i := 0
	for i < len(s) {
		if s[i] == '\x1b' && i+1 < len(s) && s[i+1] == '[' {
			// CSI escape sequence - skip to terminating letter
			j := i + 2
			for j < len(s) && !((s[j] >= 'A' && s[j] <= 'Z') || (s[j] >= 'a' && s[j] <= 'z')) {
				j++
			}
			if j < len(s) {
				j++
			}
			i = j
			continue
		}
		visual++
		if visual > target {
			return s[:i] + "\x1b[0m..."
		}
		_, size := utf8.DecodeRuneInString(s[i:])
		i += size
	}
	return s
}

func formatDuration(d time.Duration) string {
	d = d.Round(time.Second)

	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60

	if hours > 0 {
		return fmt.Sprintf("%dh%dm%ds", hours, minutes, seconds)
	} else if minutes > 0 {
		return fmt.Sprintf("%dm%ds", minutes, seconds)
	}
	return fmt.Sprintf("%ds", seconds)
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

func formatEventWithTTY(e computer.Event, ttyID string, compact bool) string {
	var tsFmt string
	if compact {
		tsFmt = "04:05.000" // MM:SS.mmm - drops the hour, saves 3 cols
	} else {
		tsFmt = "15:04:05.000"
	}
	timestamp := timestampStyle.Render(e.Timestamp.Format(tsFmt))
	ttyTag := ttyTagStyle.Render(fmt.Sprintf("[%s]", ttyID))

	var eventType, details string

	switch e.Type {
	case computer.EventEngineToTTY:
		eventType = keystrokeStyle.Render("KEYSTROKE ")
		key := formatKey(e.Data["key"])
		details = key

	case computer.EventClientToEngine:
		return "" // Skip

	case computer.EventTTYToProgram:
		if cmd, ok := e.Data["cmd"]; ok {
			eventType = executeStyle.Render("EXECUTE   ")
			cmdStr := fmt.Sprintf("%v", cmd)
			// Truncate command but keep it readable
			if len(cmdStr) > 25 {
				cmdStr = truncateFromEnd(cmdStr, 25)
			}
			cmdStr = bufferStyle.Render(fmt.Sprintf("%q", cmdStr))
			prog := detailStyle.Render(fmt.Sprintf("→ %v", e.Data["prog"]))
			details = fmt.Sprintf("%s %s", cmdStr, prog)
		} else {
			eventType = keystrokeStyle.Render("KEYSTROKE ")
			key := formatKey(e.Data["key"])
			prog := detailStyle.Render(fmt.Sprintf("→ %v", e.Data["prog"]))
			details = fmt.Sprintf("%s %s", key, prog)
		}

	case computer.EventTTYToClient:
		eventType = outputStyle.Render("OUTPUT    ")
		output := fmt.Sprintf("%v", e.Data["output"])

		// Truncate raw string first (before adding ANSI codes)
		displayOutput := output
		if len(displayOutput) > 30 {
			displayOutput = truncateFromEnd(displayOutput, 30)
		}

		// Replace special chars with colored labels
		displayOutput = strings.ReplaceAll(displayOutput, "\n", hexStyle.Render("\\n"))
		displayOutput = strings.ReplaceAll(displayOutput, "\r", hexStyle.Render("\\r"))
		displayOutput = strings.ReplaceAll(displayOutput, "\b", hexStyle.Render("\\b"))
		displayOutput = strings.ReplaceAll(displayOutput, "\x1b", hexStyle.Render("\\e"))

		byteCount := detailStyle.Render(fmt.Sprintf("(%db)", len(output)))
		details = fmt.Sprintf("%s %s", displayOutput, byteCount)

	case computer.EventTTYCreated:
		eventType = stateChangeStyle.Render("[TTY_INIT]")
		details = detailStyle.Render("canonical echo")

	case computer.EventTTYModeChanged:
		eventType = stateChangeStyle.Render("[TTY_MODE]")
		details = detailStyle.Render(fmt.Sprintf("%v echo=%v", e.Data["mode"], e.Data["echo"]))

	case computer.EventForegroundChanged:
		eventType = stateChangeStyle.Render("[FGRND_CH]")
		details = detailStyle.Render(fmt.Sprintf("→ %v", e.Data["program"]))

	case computer.EventProgramStarted:
		eventType = stateChangeStyle.Render("[PRG_STRT]")
		details = detailStyle.Render(fmt.Sprintf("%v spawned", e.Data["program_id"]))

	case computer.EventProgramExited:
		eventType = stateChangeStyle.Render("[PRG_EXIT]")
		progID := e.Data["program_id"]
		status := e.Data["status"]
		details = detailStyle.Render(fmt.Sprintf("%v exit=%v", progID, status))

	case computer.EventSessionCreated:
		eventType = stateChangeStyle.Render("[SESSION ]")
		user := e.Data["user"]
		computer := e.Data["computer"]
		details = detailStyle.Render(fmt.Sprintf("%v@%v", user, computer))

	case computer.EventBufferChanged:
		return "" // Don't show in log, only update state

	case computer.EventWorkingDirChanged:
		eventType = stateChangeStyle.Render("[CWD_CHG ]")
		details = detailStyle.Render(fmt.Sprintf("→ %v", e.Data["dir"]))

	default:
		return ""
	}

	return fmt.Sprintf("%s %s %s │ %s", timestamp, ttyTag, eventType, details)
}

func (m Model) View() string {
	if m.width == 0 {
		return "Loading..."
	}

	var content string
	switch m.viewMode {
	case ViewLogsOnly:
		content = m.renderFullScreenLog()
	case ViewStateOnly:
		content = m.renderFullScreenState()
	case ViewConnectionsOnly:
		content = m.renderFullScreenConnections()
	case ViewPermissions:
		content = m.renderPermissions(m.width - 4)
	default:
		content = m.renderThreeColumn()
	}

	// Pad to exactly m.height lines so bubbletea always erases the full
	// previous render when switching between views of different heights.
	lines := strings.Split(content, "\n")
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	for len(lines) < m.height {
		lines = append(lines, strings.Repeat(" ", m.width))
	}
	if len(lines) > m.height {
		lines = lines[:m.height]
	}
	return strings.Join(lines, "\n")
}

func (m Model) renderThreeColumn() string {
	// Three column layout
	leftWidth := 25
	middleWidth := 30
	rightWidth := m.width - leftWidth - middleWidth - 6

	leftPanel := m.renderConnectionList(leftWidth)
	middlePanel := m.renderSelectedState(middleWidth)
	rightPanel := m.renderLog(rightWidth, true, m.height-4)

	return m.combinePanels(leftPanel, middlePanel, rightPanel, leftWidth, middleWidth, rightWidth)
}

func (m Model) renderFullScreenLog() string {
	return m.renderLog(m.width-4, false, m.height)
}

func (m Model) renderFullScreenState() string {
	return m.renderSelectedState(m.width - 4)
}

func (m Model) renderFullScreenConnections() string {
	return m.renderConnectionList(m.width - 4)
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
			workDir := truncateFromEnd(conn.WorkingDir, width-4)
			s.WriteString(detailStyle.Render(fmt.Sprintf("  %s", workDir)) + "\n")
		}

		// Stats - SPLIT ACROSS TWO LINES
		uptime := time.Since(conn.ConnectedAt)
		s.WriteString(detailStyle.Render(fmt.Sprintf("  ↑%d ↓%d │ %dev",
			conn.BytesSent, conn.BytesReceived, conn.EventCount)) + "\n")
		s.WriteString(detailStyle.Render(fmt.Sprintf("  %s", formatDuration(uptime))) + "\n")

		s.WriteString("\n")
	}

	return s.String()
}

func (m Model) renderSelectedState(width int) string {
	var s strings.Builder

	if m.selectedTTY == "" {
		s.WriteString(titleStyle.Render("NO CONNECTION") + "\n")
		return s.String()
	}

	conn, ok := m.connections[m.selectedTTY]
	if !ok {
		s.WriteString(titleStyle.Render("NOT FOUND") + "\n")
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

	// Buffer - SMART TRUNCATION (show end)
	s.WriteString(labelStyle.Render("BUFFER: "))
	if conn.Buffer == "" {
		s.WriteString(detailStyle.Render("(empty)") + "\n")
	} else {
		// Show last N characters of buffer (most recent typing)
		maxBufDisplay := width - 10
		displayBuffer := truncateFromStart(conn.Buffer, maxBufDisplay)
		s.WriteString(bufferStyle.Render(fmt.Sprintf("%q", displayBuffer)) + "\n")
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
		sessionID := truncateFromEnd(conn.SessionID, 16)
		s.WriteString(detailStyle.Render(fmt.Sprintf("  sid=%s", sessionID)) + "\n")
		workDir := truncateFromEnd(conn.WorkingDir, width-8)
		s.WriteString(detailStyle.Render(fmt.Sprintf("  cwd=%s", workDir)) + "\n")
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
	s.WriteString(detailStyle.Render(fmt.Sprintf("  Uptime: %s", formatDuration(uptime))) + "\n")
	s.WriteString(detailStyle.Render(fmt.Sprintf("  Last: %s ago", formatDuration(lastActivity))) + "\n")

	return s.String()
}

type logEntry struct {
	event computer.Event
	ttyID string
}

func (m Model) renderLog(width int, compact bool, panelHeight int) string {
	var s strings.Builder

	if m.showAllLogs {
		s.WriteString(titleStyle.Render("EVENT LOG [ALL]") + "\n")
	} else {
		s.WriteString(titleStyle.Render(fmt.Sprintf("EVENT LOG [%s]", m.selectedTTY)) + "\n")
	}
	s.WriteString(borderStyle.Render(strings.Repeat("─", width)) + "\n")

	maxLines := panelHeight - 2 // 2 header lines (title + border)

	var entries []logEntry

	if m.showAllLogs {
		for _, ttyID := range m.getSortedTTYIDs() {
			if conn, ok := m.connections[ttyID]; ok {
				for _, e := range conn.LogEvents {
					entries = append(entries, logEntry{e, conn.TTYID})
				}
			}
		}
	} else {
		if conn, ok := m.connections[m.selectedTTY]; ok {
			for _, e := range conn.LogEvents {
				entries = append(entries, logEntry{e, conn.TTYID})
			}
		}
	}

	start := 0
	if len(entries) > maxLines {
		start = len(entries) - maxLines
	}

	for i := start; i < len(entries); i++ {
		line := formatEventWithTTY(entries[i].event, entries[i].ttyID, compact)
		if lipgloss.Width(line) > width {
			line = truncateLogLine(line, width)
		}
		s.WriteString(line + "\n")
	}

	return s.String()
}

// formatMode converts OwnerMode and OtherMode uint8 into a 6-char permission string.
// Layout: [owner rwx][other rwx] - no group (simplified 2-party system).
// Each triplet: bit 4 = r, bit 2 = w, bit 1 = x.
func formatMode(owner, other uint8) string {
	bit := func(m, mask uint8, c string) string {
		if m&mask != 0 {
			return c
		}
		return "-"
	}
	ownerStr := bit(owner, 4, "r") + bit(owner, 2, "w") + bit(owner, 1, "x")
	otherStr := bit(other, 4, "r") + bit(other, 2, "w") + bit(other, 1, "x")
	return ownerStr + otherStr
}

func (m Model) renderPermissions(width int) string {
	var s strings.Builder

	if m.selectedTTY == "" || m.eng == nil {
		s.WriteString(titleStyle.Render("PERMISSIONS") + "\n")
		s.WriteString(detailStyle.Render("No connection selected") + "\n")
		return s.String()
	}

	conn, ok := m.connections[m.selectedTTY]
	if !ok || conn.Computer == "" {
		s.WriteString(titleStyle.Render("PERMISSIONS") + "\n")
		s.WriteString(detailStyle.Render("No session / computer") + "\n")
		return s.String()
	}

	meta := m.eng.GetFsMetaData(conn.Computer)
	s.WriteString(titleStyle.Render(fmt.Sprintf("PERMISSIONS [%s]", conn.Computer)) + "\n")
	s.WriteString(borderStyle.Render(strings.Repeat("─", width)) + "\n")

	if len(meta) == 0 {
		s.WriteString(detailStyle.Render("  (no metadata)") + "\n")
		return s.String()
	}

	// Sort paths for stable display
	paths := make([]string, 0, len(meta))
	for p := range meta {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for _, p := range paths {
		entry := meta[p]
		modeStr := formatMode(entry.OwnerMode, entry.OtherMode)

		setuidMark := " "
		if entry.Setuid {
			setuidMark = "s"
		}

		line := fmt.Sprintf("%s%s  %-8s  %s", modeStr, setuidMark, entry.Owner, p)
		if lipgloss.Width(line) > width {
			line = truncateFromEnd(line, width)
		}
		s.WriteString(detailStyle.Render(line) + "\n")
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
	result.WriteString(borderStyle.Render("┌"+strings.Repeat("─", leftW)+"┬"+
		strings.Repeat("─", middleW)+"┬"+strings.Repeat("─", rightW)+"┐") + "\n")

	writeCell := func(i int, lines []string, w int) {
		if i < len(lines) {
			line := lines[i]
			lineW := lipgloss.Width(line)
			if lineW > w {
				line = truncateLogLine(line, w)
				lineW = w
			}
			result.WriteString(line)
			if pad := w - lineW; pad > 0 {
				result.WriteString(strings.Repeat(" ", pad))
			}
		} else {
			result.WriteString(strings.Repeat(" ", w))
		}
	}

	for i := 0; i < maxLines; i++ {
		result.WriteString(borderStyle.Render("│"))
		writeCell(i, leftLines, leftW)
		result.WriteString(borderStyle.Render("│"))
		writeCell(i, middleLines, middleW)
		result.WriteString(borderStyle.Render("│"))
		writeCell(i, rightLines, rightW)
		result.WriteString(borderStyle.Render("│") + "\n")
	}

	// Bottom border + controls
	result.WriteString(borderStyle.Render("├"+strings.Repeat("─", leftW)+"┴"+
		strings.Repeat("─", middleW)+"┴"+strings.Repeat("─", rightW)+"┤") + "\n")

	// Controls - SINGLE LINE
	viewModeStr := "full"
	switch m.viewMode {
	case ViewLogsOnly:
		viewModeStr = "logs"
	case ViewStateOnly:
		viewModeStr = "state"
	case ViewConnectionsOnly:
		viewModeStr = "conn"
	case ViewPermissions:
		viewModeStr = "perms"
	}

	logScope := "one"
	if m.showAllLogs {
		logScope = "all"
	}

	controls := fmt.Sprintf(" tab │ 1-9 │ v:%s │ a:%s │ q │ ev:%d │ %s ",
		viewModeStr, logScope, m.totalEvents, formatDuration(time.Since(m.startTime)))

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
