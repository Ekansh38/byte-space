package client

import (
	"byte-space/utils"
	"log"
	"net"
	"os"

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
				if int(b[0]) == 27 {
					canWrite = false
				}
			} else if len(keystroke) == 1 {
				if int(b[0]) == 91 && int(keystroke[0]) == 27{
					keystroke = append(keystroke, b[0])
					canWrite = false
				}
			} else if len(keystroke) == 2 {
				if int(keystroke[1]) == 91 && int(keystroke[0]) == 27{
					keystroke = append(keystroke, b[0])
					canWrite = true
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

