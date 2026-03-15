package client

import (
	"fmt"
	"net"
	"golang.org/x/term"
	"os"
	"log"
)

func commandLoop(c net.Conn, mode string, prompt string) {
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		log.Fatalf("Failed to set raw mode: %v", err)
	}
	defer term.Restore(fd, oldState)

	var history []string
	historyIdx := -1
	var buf []byte
	cursorPos := 0

	redraw := func() {
		fmt.Printf("\r%s%s\033[K", prompt, string(buf))
		if moveBack := len(buf) - cursorPos; moveBack > 0 {
			fmt.Printf("\033[%dD", moveBack)
		}
	}

	redraw()

	b := make([]byte, 1)
	for {
		_, err := os.Stdin.Read(b)
		if err != nil {
			term.Restore(fd, oldState)
			fmt.Print("\r\n")
			return
		}

		switch b[0] {
		case 0x0d, 0x0a: // Enter
			fmt.Print("\r\n")
			input := string(buf)
			buf = buf[:0]
			cursorPos = 0
			historyIdx = -1

			if input == "" {
				redraw()
				continue
			}

			if input == clearCommand {
				fmt.Print("\033[H\033[2J")
				redraw()
				continue
			}

			history = append(history, input)

			term.Restore(fd, oldState)
			writeToEngine(c, input, mode)
			i := engineReader(c, true)
			if i.Status == 10 {
				return
			}
			oldState, _ = term.MakeRaw(fd)
			redraw()

		case 0x7f, 0x08: // Backspace
			if cursorPos > 0 {
				buf = append(buf[:cursorPos-1], buf[cursorPos:]...)
				cursorPos--
				redraw()
			}

		case 0x1b: // arrow keys
			seq := make([]byte, 2)
			os.Stdin.Read(seq)
			if seq[0] != '[' {
				continue
			}
			switch seq[1] {
			case 'A': // Up arrow
				if len(history) == 0 {
					continue
				}
				if historyIdx == -1 {
					historyIdx = len(history) - 1
				} else if historyIdx > 0 {
					historyIdx--
				}
				buf = []byte(history[historyIdx])
				cursorPos = len(buf)
				redraw()
			case 'B': // Down arrow
				if historyIdx == -1 {
					continue
				}
				historyIdx++
				if historyIdx >= len(history) {
					historyIdx = -1
					buf = []byte{}
					cursorPos = 0
				} else {
					buf = []byte(history[historyIdx])
					cursorPos = len(buf)
				}
				redraw()
			case 'C': // Right arrow
				if cursorPos < len(buf) {
					cursorPos++
					redraw()
				}
			case 'D': // Left arrow
				if cursorPos > 0 {
					cursorPos--
					redraw()
				}
			}

		case 0x03: // Ctrl+C
			fmt.Print("\r\nPlease use the 'exit' command to quit.\r\n")
			redraw()

		default:
			if b[0] >= 0x20 { // printable chars only
				buf = append(buf, 0)
				copy(buf[cursorPos+1:], buf[cursorPos:])
				buf[cursorPos] = b[0]
				cursorPos++
				redraw()
			}
		}
	}
}
