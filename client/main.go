package client

import (
	"fmt"
	"net"
	"golang.org/x/term"
	"os"
	"log"
)

func commandLoop(c net.Conn, mode string) {
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		log.Fatalf("Failed to set raw mode: %v", err)
	}
	defer term.Restore(fd, oldState)

	var history []string
	historyIdx := -1
	var buf []byte

	printPrompt := func() {
		fmt.Print("\r> " + string(buf) + "\033[K")
	}

	redrawLine := func(newBuf []byte) {
		buf = newBuf
		fmt.Print("\r> " + string(buf) + "\033[K")
	}

	printPrompt()

	b := make([]byte, 1)
	for {
		_, err := os.Stdin.Read(b)
		if err != nil {
			term.Restore(fd, oldState)
			fmt.Println("\r")
			return
		}

		switch b[0] {
		case 0x0d, 0x0a: // Enter
			fmt.Print("\r\n")
			input := string(buf)
			buf = buf[:0]
			historyIdx = -1

			if input == "" {
				printPrompt()
				continue
			}

			if input == clearCommand {
				fmt.Print("\033[H\033[2J")
				printPrompt()
				continue
			}

			history = append(history, input)

			term.Restore(fd, oldState)
			writeToEngine(c, input, mode)
			if engineReader(c) == 10 {
				fmt.Println("Exiting client...")
				return
			}
			oldState, _ = term.MakeRaw(fd)
			printPrompt()

		case 0x7f, 0x08: // Backspace
			if len(buf) > 0 {
				buf = buf[:len(buf)-1]
				fmt.Print("\r> " + string(buf) + "\033[K")
			}

		case 0x1b: // Escape sequence (arrow keys)
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
				redrawLine([]byte(history[historyIdx]))
			case 'B': // Down arrow
				if historyIdx == -1 {
					continue
				}
				historyIdx++
				if historyIdx >= len(history) {
					historyIdx = -1
					redrawLine([]byte{})
				} else {
					redrawLine([]byte(history[historyIdx]))
				}
			}

		case 0x03: // Ctrl+C
			term.Restore(fd, oldState)
			fmt.Print("\r\n")
			os.Exit(0)

		default:
			if b[0] >= 0x20 { // printable chars only
				buf = append(buf, b[0])
				fmt.Print("\r> " + string(buf) + "\033[K")
			}
		}
	}
}
