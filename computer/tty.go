package computer

// The TTY is not stored in the filesystem (no inode or /dev entry)
// This is to ensure my system doesn't take on unnecessary complexity

// The TTY is represented by a FD and is a regular I/O device managed by the kernel in RAM.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"strings"

	"byte-space/utils"
)


type TTY struct {
	io.Writer
	PasswdMode     bool
	id             string
	ForegroundPGID int // pgid = proces group id
	Canonical      bool
	Echo           bool
	Buffer         string
	CursorPosition int
	Width          int
	Height         int
	dataChannel    chan string
	Session        *Session
	Connection     net.Conn
	EventBus       *EventBus
	// Echo & Canonical false is RAW mode
}

func NewTTY(c net.Conn, eb *EventBus, id string) *TTY {
	handsomeNewTTY := &TTY{
		ForegroundPGID: -1,
		Canonical:      true,
		Echo:           true,
		Buffer:         "",
		dataChannel:    make(chan string),
		Session:        nil,
		Connection:     c,
		id:             id,
		EventBus:       eb,
	}

	return handsomeNewTTY
}

func (t *TTY) writeToClient(data string, status int) {
	jsonData, err := json.Marshal(NewIPCMessage(data, status))
	if err != nil {
		log.Fatalf("Error occurred during marshalling: %s", err.Error())
	}
	jsonData = append(jsonData, '\n')
	t.Connection.Write(jsonData)
}

type Signal int

const (
	SIGINT   Signal = iota
	SIGTSTP
	SIGQUIT
	SIGINFO
	SIGWINCH // terminal window size changed
)

func (t *TTY) HandleKeystroke(keystroke string) {
	t.EventBus.Publish(EventClientToEngine, map[string]interface{}{
		"key": keystroke,
		"tty": t.id,
	})

	t.EventBus.Publish(EventEngineToTTY, map[string]interface{}{
		"key":       keystroke,
		"canonical": t.Canonical,
		"echo":      t.Echo,
		"tty":       t.id,
	})

	switch keystroke {
	case "\x03": // ctrl-c
		if t.ForegroundPGID != -1 {
			var foregroundPrograms []*Process

			procs := t.Session.Computer.Kernel.GetProcs()
			for _, proc := range procs {
				if proc.PGID == t.ForegroundPGID {
					foregroundPrograms = append(foregroundPrograms, proc)
				}
			}
			for _, proc := range foregroundPrograms {
				proc.Program.HandleSignal(SIGINT)
			}
		}
	default:
		t.dataChannel <- keystroke
	}
}

func (t *TTY) SetWinsize(width, height int) {
	if width == t.Width && height == t.Height {
		return // no change, no SIGWINCH
	}
	t.Width = width
	t.Height = height
	// send SIGWINCH to all foreground processes
	if t.ForegroundPGID != -1 && t.Session != nil {
		procs := t.Session.Computer.Kernel.GetProcs()
		for _, proc := range procs {
			if proc.PGID == t.ForegroundPGID {
				proc.Program.HandleSignal(SIGWINCH)
			}
		}
	}
}

func (t *TTY) SetForegroundPGID(pgid int) {
	t.ForegroundPGID = pgid
	t.EventBus.Publish(EventForegroundChanged, map[string]interface{}{
		"tty_id": t.id,
	})
}

func (t *TTY) Read(proc *Process, ctx context.Context) (string, int) {
	// TODO, change canonical buffer to a []byte instead of string.
	// this is due to better performance
	if proc.PGID != t.ForegroundPGID {
		return "Err: You are not foreground program", utils.Error
	}

	var foregroundPrograms []*Process

	procs := t.Session.Computer.Kernel.procs
	for _, proc := range procs {
		if proc.PGID == t.ForegroundPGID {
			foregroundPrograms = append(foregroundPrograms, proc)
		}
	}
	for {
		select {
		case receivedData := <-t.dataChannel:
			if strings.HasPrefix(receivedData, "\x1b[1;5") { // incomplete key.
				continue
			}

			if len(receivedData) == 1 && receivedData[0] == ';' {
				continue
			}
			preCursor := t.CursorPosition
			if t.Echo {
				ansiData := receivedData

				if receivedData == "\x7f" { // delete into backspace BANAANSI
					if t.CursorPosition > 0 {
						ansiData = "\b \b"
						t.CursorPosition--
					} else {
						ansiData = ""
					}
				} else if receivedData == "\t" {
					ansiData = "    " // expand tab to 4 spaces visually
				} else if receivedData == "\x1b[A" || receivedData == "\x1b[B" || receivedData == "\x15" {
					ansiData = ""
				} else if receivedData == "\x1b[C" {
					if t.CursorPosition == len(t.Buffer) {
						ansiData = ""
					}
				} else if receivedData == "\x1b[D" {
					if t.CursorPosition == 0 {
						ansiData = ""
					}
				}
				if t.PasswdMode && receivedData != "\r" && receivedData != "\x7f" {
					ansiData = "*"
				}

				t.writeToClient(ansiData, utils.Success)
			}

			if !t.Canonical {
				for _, proc := range foregroundPrograms {
					t.EventBus.Publish(EventTTYToProgram, map[string]interface{}{
						"key":    receivedData,
						"prog":   proc.Program.ID(),
						"tty_id": t.id,
					})
				}
				return receivedData, utils.Success
			}

			switch receivedData {

			case "\r": // enter
				data := t.Buffer

				for _, proc := range foregroundPrograms {
					t.EventBus.Publish(EventTTYToProgram, map[string]interface{}{
						"cmd":    t.Buffer,
						"prog":   proc.Program.ID(),
						"tty_id": t.id,
					})
				}

				t.CursorPosition = 0
				t.Buffer = ""
				t.EventBus.Publish(EventBufferChanged, map[string]interface{}{
					"buffer": t.Buffer,
					"cursor": t.CursorPosition,
					"tty":    t.id,
				})

				return data, utils.Success
			case "\x7f": // delete
				runes := []rune(t.Buffer)

				if preCursor > 0 && t.CursorPosition < len(runes) {
					runes = append(runes[:t.CursorPosition], runes[t.CursorPosition+1:]...)
					t.Buffer = string(runes)

					if t.Echo && !t.PasswdMode {
						right := string(runes[t.CursorPosition:])
						if len(right) > 0 {
							output := right + " "
							output += fmt.Sprintf("\x1b[%dD", len(right)+1)
							t.writeToClient(output, utils.Success)
						}
					}
				}

			case "\x1b[C":
				if t.PasswdMode {
					continue
				}

				if t.CursorPosition != len(t.Buffer) {
					t.CursorPosition += 1
				}

			case "\x1b[D":
				if t.PasswdMode {
					continue
				}

				if t.CursorPosition != 0 {
					t.CursorPosition -= 1
				}
			case "\x1b[A", "\x1b[B", "\x1b\x7f", "\x1bw", "\x15", "\x02", "\x1b":
				continue
			default:

				index := t.CursorPosition
				data := receivedData
				if data == "\t" {
					data = "    " // expand tab to 4 spaces in buffer
				}

				r := []rune(data)

				runes := []rune(t.Buffer)
				runes = append(runes[:index], append(r, runes[index:]...)...)
				newStr := string(runes)
				t.Buffer = newStr
				t.CursorPosition += len(r)
				if t.Echo && !t.PasswdMode {
					right := string(runes[t.CursorPosition:])
					if len(right) > 0 {
						output := right + " "
						output += fmt.Sprintf("\x1b[%dD", len(right)+1)
						t.writeToClient(output, utils.Success)
					}
				}

			}

			t.EventBus.Publish(EventBufferChanged, map[string]interface{}{
				"buffer": t.Buffer,
				"cursor": t.CursorPosition,
				"tty":    t.id,
			})

		case <-ctx.Done():
			return "SIGINT", utils.Exit
		}
	}
}

func (t *TTY) Write(str []byte) (int, error) {
	t.EventBus.Publish(EventTTYToClient, map[string]interface{}{
		"output": string(str),
		"tty":    t.id,
	})

	t.writeToClient(string(str), utils.Success)
	return len(str), nil
}

func (t *TTY) BuffClear() {
	t.Buffer = ""
	t.CursorPosition = 0
	t.EventBus.Publish(EventBufferChanged, map[string]interface{}{
		"buffer": t.Buffer,
		"cursor": t.CursorPosition,
		"tty":    t.id,
	})
}
