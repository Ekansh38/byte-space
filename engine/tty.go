package engine

import (
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
	Run(returnStatus chan int)
	HandleSignal(sig Signal)
	AddGraphicsAPI(api *GraphicsAPI)
	RemoveGraphicsAPI()
}

type TTY struct {
	io.Writer
	ForegroundProgram Program
	Canonical         bool
	Echo              bool
	Buffer            string
	dataChannel       chan string
	Session           *Session
	Connection        net.Conn
	// Echo & Canonical false is RAW mode
}

func NewTTY(c net.Conn) *TTY {
	handsomeNewTTY := &TTY{
		ForegroundProgram: nil,
		Canonical:         true,
		Echo:              true,
		Buffer:            "",
		dataChannel:       make(chan string),
		Session:           nil,
		Connection:        c,
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
					if len(t.Buffer) > 0 {
						ansiData = "\x7f"
					} else {
						ansiData = ""
					}
				} else if receivedData == "\x1b[C" || receivedData == "\x1b[A" || receivedData == "\x1b[B" || receivedData == "\x1b[D" {
					ansiData = ""
				}
				data := newIPCMessage(ansiData, utils.Success)
				writeToClient(t.Connection, data)
			}

			if !t.Canonical {
				return receivedData, utils.Success
			}

			switch receivedData {
			case "\r":
				data := t.Buffer
				t.Buffer = ""
				return data, utils.Success
			case "\x7f": // delete
				if len(t.Buffer) > 0 {
					t.Buffer = t.Buffer[:len(t.Buffer)-1]
				}
			case "\x1b[C", "\x1b[A", "\x1b[B", "\x1b[D": // arrow keys
				continue
			default:
				t.Buffer += receivedData
			}

		case <-done:
			return "SIGINT", utils.Exit
		}
	}

	return "ERROR READING", utils.Error
}

func (t *TTY) Write(str []byte) (int, error) {
	data := newIPCMessage(string(str), utils.Success)
	writeToClient(t.Connection, data)
	return len(str), nil
}
