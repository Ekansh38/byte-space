package computer

import (
	"byte-space/utils"
	"context"
	"fmt"
	"log"
	"strings"
)

type VEdit struct { // Not nvim, not vim, not even vi, just v
	id     string
	Kernel *Kernel
	proc   *Process
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
	for i := range bufferTemp {
		buffer[i] = []byte(bufferTemp[i])
	}
	// now buffer is of type [][]byte and it stores each line as a new element in the array.

	cursorPos := 0
	cursorCol := 0
	cursorRow := 0
	mode := 0 // 0 = normal mode, 1 = insert mode

	for {
		keystroke, status := p.Kernel.Read(p.proc, 0, ctx)
		log.Printf("v: read keystroke=%q status=%d\n", keystroke, status)
		switch status {
		case utils.Success:
			if keystroke == "\x13" {
				err := p.Kernel.WriteFile(p.proc, target, buffer)
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
					if buffer[cursorPos+1] != '\n' {
						cursorPos += 1
						p.Kernel.Write(p.proc, 1, []byte("\033[1C"))
					}
				} else if keystroke == "h" {
					if cursorPos-1 >= 0 && buffer[cursorPos-1] != '\n' {
						cursorPos -= 1
						p.Kernel.Write(p.proc, 1, []byte("\033[1D"))
					}
				} else if keystroke == "i" {
					mode = 1
					continue
				}

				if keystroke == "j" {
					prefix := findPrefix(cursorPos, buffer)
					log.Println(prefix)
					newCursorPos := cursorPos
					i := cursorPos
					for {
						if buffer[i] == '\n' {
							newCursorPos = i + prefix
							break
						}
						if i+1 < len(buffer) {
							i++
						} else {
							break
						}
					}
					if newCursorPos >= len(buffer) {
						newCursorPos = cursorPos
					} else {
						cursorPos = newCursorPos
						if prefix-1 == 0 {
							p.Kernel.Write(p.proc, 1, []byte(fmt.Sprintf("\033[1E")))
						} else {
							p.Kernel.Write(p.proc, 1, []byte(fmt.Sprintf("\033[1E\033[%dC", prefix-1)))
						}
					}
					log.Printf("%q", buffer)
					log.Println(cursorPos)
				}

				if keystroke == "k" {
					prefix := findPrefix(cursorPos, buffer)
					log.Println(prefix)
					newCursorPos := cursorPos
					i := cursorPos - prefix
					if i <= 0 {
						i = 0
					}
					log.Printf("FIRST BACKWORD: %d", i)
					if i-1 >= 0 {
						i--
					}
					for {
						if buffer[i] == '\n' {
							newCursorPos = i + prefix
							log.Printf("YAHHHHOOOO %d", newCursorPos)
							break
						}
						if i-1 >= 0 {
							i--
						} else {
							break
						}
					}
					if newCursorPos >= 0 {
						cursorPos = newCursorPos
						if prefix-1 == 0 {
							p.Kernel.Write(p.proc, 1, []byte(fmt.Sprintf("\033[1F")))
						} else {
							p.Kernel.Write(p.proc, 1, []byte(fmt.Sprintf("\033[1F\033[%dC", prefix-1)))
						}
					}
					log.Printf("%q", buffer)
					log.Println(cursorPos)
				}
			} else if mode == 1 { // Insert mode
				if keystroke == "\x1b\x1b" || keystroke == "\x1b" {
					mode = 0
					continue
				}
				log.Println("INSERT MODE")

				index := cursorPos
				data := keystroke
				if data == "\t" {
					data = "    "
				}

				r := []rune(data)
				runes := []rune(string(buffer))
				log.Printf("RUNES: %q\n", runes)
				runes = append(runes[:index], append(r, runes[index:]...)...)
				buffer = []byte(string(runes))
				cursorPos += len(r)
				right, _, _ := strings.Cut(string(runes[cursorPos:]), string('\n'))
				if len(right) > 0 {
					output := right + " "
					output += fmt.Sprintf("\x1b[%dD", len(right)+1)
					p.Kernel.Write(p.proc, 1, []byte(output))
				}
			}

		case utils.Exit:
			returnStatus <- utils.Error
			return
		}
	}

	_ = cursorCol
	_ = cursorRow
}

func findPrefix(cursorPos int, buffer []byte) int {
	foundNewline := false
	skip := false
	prefix := 1
	i := cursorPos - 1

	if cursorPos <= 0 {
		skip = true
	}

	for {
		if skip {
			break
		}
		if buffer[i] == '\n' {
			prefix = cursorPos - i
			foundNewline = true
			break
		}
		if i-1 <= 0 {
			break
		}
		i--
	}
	if !foundNewline {
		prefix = cursorPos + 1
	}

	return prefix
}
