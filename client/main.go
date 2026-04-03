package client

import (
	"log"
	"net"
	"os"

	"byte-space/utils"

	"golang.org/x/term"
)

func commandLoop(c net.Conn, mode string, done <-chan struct{}) {
	fd := int(os.Stdin.Fd())
	oldState, err := term.MakeRaw(fd)
	if err != nil {
		log.Fatalf("Failed to set raw mode: %v", err)
	}
	defer term.Restore(fd, oldState)

	input := make(chan []byte)
	keystroke := make([]byte, 0)

	go readLoop(input)

	for {
		select {
		case b, ok := <-input:
			if !ok {
				return
			}
			canWrite := true

			if len(keystroke) == 0 {
				keystroke = append(keystroke, b[0])
				if int(b[0]) == 27 { // ESC
					canWrite = false
				}
			} else if len(keystroke) == 1 && int(keystroke[0]) == 27 {
				keystroke = append(keystroke, b[0])
				if int(b[0]) == 91 { // [
					canWrite = false
				}
			} else if len(keystroke) == 2 && int(keystroke[1]) == 91 {
				keystroke = append(keystroke, b[0])

				// Simple sequences: A, B, C, D (arrows)
				if b[0] >= 'A' && b[0] <= 'D' {
					canWrite = true
				} else {
					canWrite = false
				}
			} else if len(keystroke) >= 3 {
				keystroke = append(keystroke, b[0])

				// Check for sequence terminator
				if (b[0] >= 'A' && b[0] <= 'Z') || b[0] == '~' {
					canWrite = true
				} else {
					canWrite = false
				}
			}

			if canWrite {
				if writeToEngine(c, string(keystroke), mode) == utils.Error {
					return
				}
				keystroke = make([]byte, 0)
			}

		case <-done:
			return
		}
	}
}

func readLoop(input chan []byte) {
	for {
		b := make([]byte, 1)
		n, err := os.Stdin.Read(b)
		if err != nil {
			close(input)
			return
		}
		input <- b[:n]
	}
}
