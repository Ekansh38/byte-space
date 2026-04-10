package computer

import (
	"context"
	"fmt"
	"log"
	"strings"

	"byte-space/utils"
)

type VEdit struct { // Not nvim, not vim, not even vi, just v
	id         string
	Kernel     *Kernel
	proc       *Process
	termHeight int
	termWdith  int
}

func (p *VEdit) SetProcess(proc *Process) { p.proc = proc }
func (p *VEdit) SetKernel(k *Kernel)      { p.Kernel = k }
func (p *VEdit) ID() string               { return p.id }

func (p *VEdit) HandleSignal(sig Signal) {
	if sig == SIGINT {
		p.Kernel.Ioctl(p.proc, 0, TIOCBUFFCLEAR, nil)
		p.Kernel.Ioctl(p.proc, 0, TIOCRAW, false)
		p.Kernel.Write(p.proc, 1, []byte("\033[H\033[2J"))
		p.Kernel.Write(p.proc, 1, []byte("\n(SIGINT), force quitting!\n"))
		p.proc.ctxCancel()

	} else if sig == SIGWINCH {
		var ws Winsize
		p.Kernel.Ioctl(p.proc, 0, TIOCGWINSZ, &ws)
		p.termHeight = ws.Height
		p.termWdith = ws.Width

		log.Printf("HIEHGT: %d", p.termHeight)
		log.Printf("WIDTH: %d", p.termWdith)

		// tell run, later tho TODO

	}
}

func (p *VEdit) Run(ctx context.Context, returnStatus chan int, params []string) {
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

	if err := p.Kernel.Ioctl(p.proc, 0, TIOCRAW, true); err != nil {
		p.Kernel.Write(p.proc, 1, []byte(fmt.Sprintf("\nError setting raw mode: %s\n", err)))
		returnStatus <- utils.Error
		return
	}

	p.Kernel.Write(p.proc, 1, []byte("\033[H\033[2J"))
	p.Kernel.Write(p.proc, 1, content)
	p.Kernel.Write(p.proc, 1, []byte("\033[H"))

	var buffer [][]byte
	bufferTemp := strings.Split(string(content), "\n")
	buffer = make([][]byte, len(bufferTemp))

	for i := range bufferTemp {
		buffer[i] = []byte(bufferTemp[i])
	}
	// now buffer is of type [][]byte and it stores each line as a new element in the array.

	cursorCol := 0
	cursorRow := 0

	var ws Winsize
	p.Kernel.Ioctl(p.proc, 0, TIOCGWINSZ, &ws)
	p.termHeight = ws.Height
	p.termWdith = ws.Width

	log.Printf("HIEHGT: %d", p.termHeight)
	log.Printf("WIDTH: %d", p.termWdith)

	yOffset := 0

	mode := 0 // 0 = normal mode, 1 = insert mode

	for {
		keystroke, status := p.Kernel.Read(p.proc, 0, ctx)
		log.Printf("v: read keystroke=%q status=%d\n", keystroke, status)
		switch status {
		case utils.Success:
			if keystroke == "\x13" {
				// convert via adding newlines to each split
				var actualBuffer []byte
				for i := range buffer {
					actualBuffer = append(actualBuffer, []byte(string(buffer[i])+"\n")...)
				}
				err := p.Kernel.WriteFile(p.proc, target, actualBuffer) // with newlines and all
				if err != nil {
					p.Kernel.Ioctl(p.proc, 0, TIOCBUFFCLEAR, nil)
					p.Kernel.Ioctl(p.proc, 0, TIOCRAW, false)
					p.Kernel.Write(p.proc, 1, []byte("ERROR WRITING TO FILE!"))
					p.proc.ctxCancel()
				}

				p.Kernel.Ioctl(p.proc, 0, TIOCBUFFCLEAR, nil)
				p.Kernel.Ioctl(p.proc, 0, TIOCRAW, false)
				p.Kernel.Write(p.proc, 1, []byte("File Saved"))
				p.proc.ctxCancel()
			}

			if mode == 0 { // Normal mode
				if keystroke == "l" {
					if buffer[cursorRow][cursorCol+1] != '\n' {
						cursorCol += 1
					}
				} else if keystroke == "h" {
					if cursorCol-1 >= 0 && buffer[cursorRow][cursorCol-1] != '\n' {
						cursorCol -= 1
					}
				} else if keystroke == "i" {
					mode = 1
					continue
				}

				if keystroke == "j" {
					if cursorRow+1 < len(buffer) {
						cursorRow++
					}

					if keystroke == "k" {
						if cursorRow-1 >= 0 {
							cursorRow--
						}
					}
				} else if mode == 1 { // Insert mode
					if keystroke == "\x1b\x1b" || keystroke == "\x1b" {
						mode = 0
						continue
					}
					log.Println("INSERT MODE")
				}

				// REDRAW

				p.Kernel.Write(p.proc, 1, []byte("\033[H\033[2J"))
				var drawBuf []byte
				for i := range p.termHeight {
					if yOffset+i < len(buffer) {
						drawBuf = append(drawBuf, buffer[yOffset+i]...)
					} else {
						break
					}
				}
				p.Kernel.Write(p.proc, 1, drawBuf)

				log.Printf("DRAWBUf: %q", drawBuf)

			}

		case utils.Exit:
			returnStatus <- utils.Error
			return
		}
	}
}
