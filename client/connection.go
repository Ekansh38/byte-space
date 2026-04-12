// Package client handles the connection to the engine and the communication between them. It also contains the main loop for the client.
package client

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"

	"byte-space/engine"
	"byte-space/utils"

	"golang.org/x/term"
)

func writeToEngine(c net.Conn, rawKeystroke string) int {
	w, h, _ := term.GetSize(int(os.Stdin.Fd()))
	data := engine.ClientIPCMessage{Program: "user", Keystroke: rawKeystroke, Width: w, Height: h}

	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Fatalf("Error occurred during marshalling: %s", err.Error())
	}

	jsonData = append(jsonData, '\n')

	_, err = c.Write([]byte(jsonData))
	if err != nil {
		log.Println("Could not write to server!")
		return utils.Error
	}
	return utils.Success
}

func engineReader(c net.Conn, done chan struct{}) {
	for {
		scanner := bufio.NewScanner(c)

		for scanner.Scan() {
			line := scanner.Text()
			var message engine.EngineIPCMessage
			if err := json.Unmarshal([]byte(line), &message); err != nil {
				log.Println("Error unmarshalling JSON:", err)
				continue
			}

			if message.Status == utils.Exit {
				runBanansi(&message)
				close(done)
				fmt.Printf("\n\r\n") // avoid that annoying % from zsh.
				return
			}
			runBanansi(&message)
		}
	}
}

func runBanansi(message *engine.EngineIPCMessage) {
	for _, char := range message.Result {
		switch char {
		case '\r':
			fmt.Print("\r")
		case '\n':
			fmt.Print("\n\r")
		default:
			fmt.Printf("%c", char)
		}
	}
}

func ConnectToEngine() {
	c, err := net.Dial("unix", "/tmp/engine.sock")
	if err != nil {
		fmt.Println("Could not connect to engine!")
		os.Exit(couldNotConnectToEngine)
	}

	sendInitMessage(c)

	done := make(chan struct{})

	go engineReader(c, done)
	commandLoop(c, done)
}

func sendInitMessage(c net.Conn) {
	w, h, _ := term.GetSize(int(os.Stdin.Fd()))
	data := engine.ClientIPCMessage{Program: "user", Keystroke: "", Width: w, Height: h}

	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Fatalf("Error occurred during marshalling: %s", err.Error())
	}

	jsonData = append(jsonData, '\n')

	_, err = c.Write([]byte(jsonData))
	if err != nil {
		log.Println("Could not write to server!")
	}
}

