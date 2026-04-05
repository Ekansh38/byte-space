package computer

import (
	"fmt"
	"io"
	"net"
	"strings"

	"byte-space/utils"
)

type GraphicsAPI struct {
	writer io.Writer
}

func NewGraphicsAPI(writer io.Writer) *GraphicsAPI {
	return &GraphicsAPI{writer: writer}
}

func (g *GraphicsAPI) Write(str string) (int, error) {
	if g == nil {
		return 0, fmt.Errorf("nil")
	}

	return g.writer.Write([]byte(str))
}

type TTYAPI struct {
	tty     *TTY
	program Program
}

func (t *TTYAPI) Read(done chan struct{}) (string, int) {
	return t.tty.Read(t.program, done)
}

func (t *TTYAPI) SetPasswdMode(mode bool) {
	t.tty.PasswdMode = mode
}

func (t *TTYAPI) SetSession(session *Session) {
	t.tty.Session = session
}

func (t *TTYAPI) GetTTYID() string {
	return t.tty.id
}

func (t *TTYAPI) GetSession() *Session {
	return t.tty.Session
}

type Program interface {
	// API's (aces based security)

	SetTTyAPI(api *TTYAPI)

	SetKernel(api *Kernel)

	// no acces to session, the only "state" it has acess to is from the kernel syscalls or the proc

	SetProcess(proc *Process)

	AddGraphicsAPI(api *GraphicsAPI)
	RemoveGraphicsAPI()

	// General

	ID() string
	Run(returnStatus chan int, params []string)
	HandleSignal(sig Signal)

}

type TTY struct {
	io.Writer
	networkAPI        NetworkAPI
	PasswdMode        bool
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

func NewTTY(c net.Conn, engine NetworkAPI, id string) *TTY {
	handsomeNewTTY := &TTY{
		ForegroundProgram: nil,
		Canonical:         true,
		Echo:              true,
		Buffer:            "",
		dataChannel:       make(chan string),
		Session:           nil,
		Connection:        c,
		networkAPI:        engine,
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
	t.networkAPI.PublishEvent(EventClientToEngine, map[string]interface{}{
		"key": keystroke,
		"tty": t.id,
	})

	t.networkAPI.PublishEvent(EventEngineToTTY, map[string]interface{}{
		"key":       keystroke,
		"canonical": t.Canonical,
		"echo":      t.Echo,
		"tty":       t.id,
	})

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
	t.networkAPI.PublishEvent(EventForegroundChanged, map[string]interface{}{
		"program": program.ID(),
		"tty_id":  t.id,
	})

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
			if strings.HasPrefix(receivedData, "\x1b[1;5") {
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
				if t.PasswdMode && receivedData != "\r" && receivedData != "\x7f" {
					ansiData = "*"
				}

				t.networkAPI.WriteToClient(t.Connection, ansiData, utils.Success)
			}

			if !t.Canonical {
				t.networkAPI.PublishEvent(EventTTYToProgram, map[string]interface{}{
					"key":    receivedData,
					"prog":   program.ID(),
					"tty_id": t.id,
				})
				return receivedData, utils.Success
			}

			switch receivedData {
			case "\r": // enter
				data := t.Buffer

				t.networkAPI.PublishEvent(EventTTYToProgram, map[string]interface{}{
					"cmd":    t.Buffer,
					"prog":   program.ID(),
					"tty_id": t.id,
				})

				t.CursorPosition = 0
				t.Buffer = ""
				t.networkAPI.PublishEvent(EventBufferChanged, map[string]interface{}{
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
							t.networkAPI.WriteToClient(t.Connection, output, utils.Success)
						}
					}
				}

			case "\x1b[A", "\x1b[B":
				continue
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

			default:
				index := t.CursorPosition
				r := []rune(receivedData)
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
						t.networkAPI.WriteToClient(t.Connection, output, utils.Success)
					}
				}

			}

			t.networkAPI.PublishEvent(EventBufferChanged, map[string]interface{}{
				"buffer": t.Buffer,
				"cursor": t.CursorPosition,
				"tty":    t.id,
			})

		case <-done:
			return "SIGINT", utils.Exit
		}
	}

	return "ERROR READING", utils.Error
}

func (t *TTY) Write(str []byte) (int, error) {
	t.networkAPI.PublishEvent(EventTTYToClient, map[string]interface{}{
		"output": string(str),
		"tty":    t.id,
	})

	t.networkAPI.WriteToClient(t.Connection, string(str), utils.Success)
	return len(str), nil
}
