package engine

import (
	"fmt"
	"io"
	"net"

	"byte-space/utils"
)

type GraphicsAPI struct {
	writer io.Writer
}

func NewGraphicsAPI(writer io.Writer) *GraphicsAPI {
	return &GraphicsAPI{writer: writer}
}

func (g *GraphicsAPI) Write(str string) (int, error) {
	return g.writer.Write([]byte(str))
}

type Program interface {
	ID() string
	Run(returnStatus chan int, params []string)
	HandleSignal(sig Signal)
	AddGraphicsAPI(api *GraphicsAPI)
	RemoveGraphicsAPI()
}

type TTY struct {
	io.Writer
	engine            *Engine
	id                string
	ForegroundProgram Program
	Canonical         bool
	Echo              bool
	Buffer            string
	CursorPosition    int
	dataChannel       chan string
	Session           *Session
	Connection        net.Conn
	// Echo & Canonical false is RAW mode
}

func NewTTY(c net.Conn, engine *Engine, id string) *TTY {
	handsomeNewTTY := &TTY{
		ForegroundProgram: nil,
		Canonical:         true,
		Echo:              true,
		Buffer:            "",
		dataChannel:       make(chan string),
		Session:           nil,
		Connection:        c,
		engine:            engine,
		id:                id,
	}

	return handsomeNewTTY
}

type Signal int

const (
	SIGINT Signal = iota
	SIGTSTP
	SIGQUIT
	SIGINFO
)

func (t *TTY) HandleKeystroke(keystroke string) {
	if t.engine != nil && t.engine.EventBus != nil {
		t.engine.EventBus.Publish(EventEngineToTTY, map[string]interface{}{
			"key":       keystroke,
			"canonical": t.Canonical,
			"tty":       t.id,
		})
	}

	switch keystroke {
	case "\x03": // ctrl-c
		if t.ForegroundProgram != nil {
			t.ForegroundProgram.HandleSignal(SIGINT)
		}
	default:
		t.dataChannel <- keystroke
	}
}

func (t *TTY) SetForegroundProcess(program Program) (string, int) {
	if t.engine != nil && t.engine.EventBus != nil {
		t.engine.EventBus.Publish(EventForegroundChanged, map[string]interface{}{
			"program": program.ID(),
			"tty_id":  t.id,
		})
	}

	// GraphicsAPI is only for the ForegroundProgram so that they can write to TTY
	if program.ID() != "" {
		if t.ForegroundProgram != nil {
			t.ForegroundProgram.RemoveGraphicsAPI()
		}

		t.ForegroundProgram = program
		program.AddGraphicsAPI(NewGraphicsAPI(t))
		return "Successfully set foreground program", utils.Success
	} else {
		return "Invalid program ID", utils.Error
	}
}

func (t *TTY) Read(program Program, done chan struct{}) (string, int) {
	if program != t.ForegroundProgram {
		return "Err: You are not foreground program", utils.Error
	}
	for {
		select {
		case receivedData := <-t.dataChannel:
			if t.Echo {
				ansiData := receivedData

				if receivedData == "\x7f" { // delete -> backspace ANSI
					if t.CursorPosition > 0 {
						ansiData = "\b \b"
						t.CursorPosition--
					} else {
						ansiData = ""
					}
				} else if receivedData == "\x1b[A" || receivedData == "\x1b[B" {
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

				data := newIPCMessage(ansiData, utils.Success)

				writeToClient(t.Connection, data)
			}

			if !t.Canonical {
				if t.engine != nil && t.engine.EventBus != nil {
					t.engine.EventBus.Publish(EventTTYToProgram, map[string]interface{}{
						"key":  receivedData,
						"prog": program.ID(),
					})
				}
				return receivedData, utils.Success
			}

			switch receivedData {
			case "\r":
				data := t.Buffer

				if t.engine != nil && t.engine.EventBus != nil {
					t.engine.EventBus.Publish(EventTTYToProgram, map[string]interface{}{
						"cmd":  t.Buffer,
						"prog": program.ID(),
					})
				}

				t.CursorPosition = 0
				t.Buffer = ""

				return data, utils.Success
			case "\x7f": // delete
				runes := []rune(t.Buffer)

				if t.CursorPosition < len(runes) {
					runes = append(runes[:t.CursorPosition], runes[t.CursorPosition+1:]...)
					t.Buffer = string(runes)

					if t.Echo {
						right := string(runes[t.CursorPosition:])
						output := right + " "
						output += fmt.Sprintf("\x1b[%dD", len(right)+1)
						writeToClient(t.Connection, newIPCMessage(output, utils.Success))
					}
				}
			case "\x1b[A", "\x1b[B":
				continue
			case "\x1b[C":
				if t.CursorPosition != len(t.Buffer) {
					t.CursorPosition += 1
				}
			case "\x1b[D":
				if t.CursorPosition != 0 {
					t.CursorPosition -= 1
				}

			default:
				index := t.CursorPosition
				r := []rune(receivedData)
				runes := []rune(t.Buffer)
				runes = append(runes[:index], append(r, runes[index:]...)...)
				newStr := string(runes)
				t.Buffer = newStr
				t.CursorPosition += len(r)
				if t.Echo {
					right := string(runes[t.CursorPosition:])
					output := right + " "
					output += fmt.Sprintf("\x1b[%dD", len(right)+1)
					writeToClient(t.Connection, newIPCMessage(output, utils.Success))
				}

			}

		case <-done:
			return "SIGINT", utils.Exit
		}
	}

	return "ERROR READING", utils.Error
}

func (t *TTY) Write(str []byte) (int, error) {
	if t.engine != nil && t.engine.EventBus != nil {
		t.engine.EventBus.Publish(EventTTYToClient, map[string]interface{}{
			"output": string(str),
			"tty":    t.id,
		})
	}
	data := newIPCMessage(string(str), utils.Success)
	writeToClient(t.Connection, data)
	return len(str), nil
}
