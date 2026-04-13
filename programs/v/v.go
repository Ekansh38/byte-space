package v

import (
	"context"
	"fmt"
	"log"
	"strings"

	"byte-space/computer"
	"byte-space/utils"
)

type VEdit struct { // Not nvim, not vim, not even vi, just v (a prequel, to vi)
	id         string
	Kernel     *computer.Kernel
	proc       *computer.Process
	termHeight int
	termWdith  int
	cursorRow  int
	cursorCol  int
	statusBar  int
	buffer     [][]byte
	yOffset    int
	mode       int
	didWrite   bool
}

func New(pid int) computer.Program { return &VEdit{id: fmt.Sprintf("v-%d", pid)} }

func (p *VEdit) SetProcess(proc *computer.Process) { p.proc = proc }
func (p *VEdit) SetKernel(k *computer.Kernel)      { p.Kernel = k }
func (p *VEdit) ID() string               { return p.id }

func (p *VEdit) HandleSignal(sig computer.Signal) {
	if sig == computer.SIGINT {
		p.Kernel.Ioctl(p.proc, 0, computer.TIOCBUFFCLEAR, nil)
		p.Kernel.Ioctl(p.proc, 0, computer.TIOCRAW, false)
		p.Kernel.Write(p.proc, 1, []byte("\033[H\033[2J"))
		p.Kernel.Write(p.proc, 1, []byte("\n(SIGINT), quitting!\n"))
		p.proc.CtxCancel()

	} else if sig == computer.SIGWINCH {
		var ws computer.Winsize
		p.Kernel.Ioctl(p.proc, 0, computer.TIOCGWINSZ, &ws)
		p.termHeight = ws.Height
		p.termWdith = ws.Width
		p.vDraw()

		// tell run, later tho TODO

	}
}

func (p *VEdit) Run(ctx context.Context, returnStatus chan int, params []string) {
	p.didWrite = false
	if len(params) != 2 {
		p.Kernel.Write(p.proc, 1, []byte("\nUsage: v <path>\n"))
		returnStatus <- utils.Error
		return
	}

	target := params[1]
	content, err := p.Kernel.ReadFile(p.proc, target)
	if err != nil {
		p.Kernel.Write(p.proc, 1, []byte(fmt.Sprintf("\nError reading file: %s\n", err)))
		returnStatus <- utils.Error
		return
	}

	if err := p.Kernel.Ioctl(p.proc, 0, computer.TIOCRAW, true); err != nil {
		p.Kernel.Write(p.proc, 1, []byte(fmt.Sprintf("\nError setting raw p.mode: %s\n", err)))
		returnStatus <- utils.Error
		return
	}

	bufferTemp := strings.Split(string(content), "\n")
	p.buffer = make([][]byte, len(bufferTemp))

	for i := range bufferTemp {
		p.buffer[i] = []byte(bufferTemp[i])
	}
	// now p.buffer is of type [][]byte and it stores each line as a new element in the array.

	p.cursorCol = 1
	p.cursorRow = 1

	var ws computer.Winsize
	p.Kernel.Ioctl(p.proc, 0, computer.TIOCGWINSZ, &ws)
	p.termHeight = ws.Height - 1 // for status bar
	p.statusBar = p.termHeight + 1
	p.termWdith = ws.Width

	log.Printf("WIDTH: %d", p.termWdith)

	p.yOffset = 0

	p.mode = 0 // 0 = normal mode, 1 = insert mode

	p.vDraw()

	for {
		keystroke, status := p.Kernel.Read(p.proc, 0, ctx)
		switch status {
		case utils.Success:
			if keystroke == "\x13" {
				// convert via adding newlines to each split
				var actualBuffer []byte
				for i := range p.buffer {
					actualBuffer = append(actualBuffer, []byte(string(p.buffer[i])+"\n")...)
				}
				err := p.Kernel.WriteFile(p.proc, target, actualBuffer) // with newlines and all
				if err != nil {
					p.Kernel.Ioctl(p.proc, 0, computer.TIOCBUFFCLEAR, nil)
					p.Kernel.Ioctl(p.proc, 0, computer.TIOCRAW, false)
					p.Kernel.Write(p.proc, 1, []byte("ERROR WRITING TO FILE!"))
					p.proc.CtxCancel()
				}
				p.didWrite = true

				p.vDraw()

			}

			if p.mode == 0 { // Normal mode
				if keystroke == "\x1b" {
					p.mode = 0
					p.didWrite = false
					p.vDraw()

					// bro i gotta leave this here,
					// my old here:
					// p.vDraw()
					// p.mode = 0

					// NOOOO! NOOO!
					continue
				}
				if keystroke == "l" {
					if p.cursorCol-1 < len(p.buffer[p.cursorRow-1]) {
						p.cursorCol += 1
					}
				} else if keystroke == "h" {
					if p.cursorCol-1 >= 0 {
						p.cursorCol -= 1
					}
				} else if keystroke == "i" {
					p.mode = 1
					p.didWrite = false
					p.vDraw()
					continue
				}

				if keystroke == "j" {
					// +0 means +1, -1 means regular
					if p.cursorRow < len(p.buffer)-1 {

						if len(p.buffer[p.cursorRow+0]) < p.cursorCol-1 {
							p.cursorCol = len(p.buffer[p.cursorRow+0])+1 // cuz len is not index!!! oioiiooioioi
						}

						p.cursorRow++
					}
				}

				if keystroke == "k" {
					if p.cursorRow > 1 {
						// -2 = -1
						if len(p.buffer[p.cursorRow-2]) < p.cursorCol-1 {
							p.cursorCol = len(p.buffer[p.cursorRow-2])+1
						}

						p.cursorRow--
					}
				}
			} else if p.mode == 1 { // Insert mode
				if keystroke == "\x1b" {
					p.mode = 0
					p.didWrite = false
					p.vDraw()

					// bro i gotta leave this here,
					// my old here:
					// p.vDraw()
					// p.mode = 0

					// NOOOO! NOOO!
					continue
				}

				// insertion logic
				switch keystroke {
				case "\x1b\x7f", "\x1bw", "\x15", "\x02", "\x1b", "\x1b[A", "\x1b[B", "\x1b[C", "\x1b[D":
					continue
				case "\r": // enter
					row := p.cursorRow - 1
					col := p.cursorCol - 1

					line := p.buffer[row]

					left := line[:col]
					right := line[col:]

					p.buffer[row] = left

					p.buffer = append(
						p.buffer[:row+1],
						append([][]byte{right}, p.buffer[row+1:]...)...,
					)

					p.cursorRow++
					p.cursorCol = 1
				case "\x7f": // delete
					runes := []rune(string(p.buffer[p.cursorRow-1]))

					if p.cursorCol > 1 {
						// normal delete
						idx := p.cursorCol - 1
						runes = append(runes[:idx-1], runes[idx:]...)
						p.buffer[p.cursorRow-1] = []byte(string(runes))
						p.cursorCol--

					} else if p.cursorRow > 1 {
						// merge with previous line
						prev := p.buffer[p.cursorRow-2]
						curr := p.buffer[p.cursorRow-1]

						newLine := append(prev, curr...)

						p.buffer[p.cursorRow-2] = newLine
						p.buffer = append(p.buffer[:p.cursorRow-1], p.buffer[p.cursorRow:]...)

						p.cursorRow--
						p.cursorCol = len(prev) + 1
					}

				default:

					index := p.cursorCol - 1 // minus 1 because its an index
					if index < 0 {
						index = 0
					}
					data := keystroke
					if data == "\t" {
						data = "    " // expand tab to 4 spaces in buffer
					}

					r := []rune(data)

					runes := []rune(string(p.buffer[p.cursorRow-1]))
					runes = append(runes[:index], append(r, runes[index:]...)...)
					newStr := string(runes)
					p.buffer[p.cursorRow-1] = []byte(newStr)
					p.cursorCol += len(r)
				}

			}

			// REDRAW
			p.vDraw()

		case utils.Exit:
			// exiting on ctrl-c is correct behaviour in V!
			returnStatus <- utils.Success
			return
		}
	}
}

func (p *VEdit) vDraw() {
	p.Kernel.Write(p.proc, 1, []byte("\033[H\033[2J"))
	var drawBuf []byte

	for i := range p.termHeight {
		if p.yOffset+i < len(p.buffer) {
			drawBuf = append(drawBuf, p.buffer[p.yOffset+i]...)
			if p.yOffset+i == len(p.buffer)-1 {
				continue
			} else {
				drawBuf = append(drawBuf, []byte("\n")...)
			}
		} else {
			drawBuf = append(drawBuf, []byte("~")...)
			drawBuf = append(drawBuf, []byte("\n")...)
		}
	}
	p.Kernel.Write(p.proc, 1, drawBuf)
	temp := fmt.Sprintf("\033[%d;%dH", p.statusBar, 1)
	p.Kernel.Write(p.proc, 1, []byte(temp))

	modeString := ""
	if p.didWrite {
		modeString = "WRITTEN TO FILE"
	} else if p.mode == 0 {
		modeString = "NORMAL"
	} else if p.mode == 1 {
		modeString = "INSERT"
	}

	content := fmt.Sprintf(" %s ", modeString)
	padding := p.termWdith - len(content)

	if padding < 0 {
		padding = 0
	}

	status := content + strings.Repeat(" ", padding)
	statusValue := statusColor(modeString, status)

	p.Kernel.Write(p.proc, 1, []byte(statusValue))

	log.Printf("ROW: %d COL: %d", p.cursorRow, p.cursorCol)
	cursorVal := fmt.Sprintf("\033[%d;%dH", p.cursorRow, p.cursorCol)
	p.Kernel.Write(p.proc, 1, []byte(cursorVal))
}

func statusColor(mode string, status string) string {
	switch mode {
	case "INSERT":
		// matrix style
		return fmt.Sprintf("\x1b[48;5;22m\x1b[38;5;255m%s\x1b[0m", status)
	case "WRITTEN TO FILE":
		return fmt.Sprintf("\x1b[48;5;46m\x1b[38;5;16m%s\x1b[0m", status)

	default:
		// blue
		return fmt.Sprintf("\x1b[48;5;23m\x1b[38;5;230m%s\x1b[0m", status)
	}
}
