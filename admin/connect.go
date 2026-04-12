// Package admin handles the connection to the engine for running admin commands
// it runs in permanent canonical mode from client side for simplicity and ease of use
// the client is a "smart" client to limit bloating the engine.
package admin

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	"byte-space/engine"
	"byte-space/utils"
)

func writeToEngine(c net.Conn, value string) int {
	data := engine.AdminIPCMessage{Program: "admin", Value: value}

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

func engineReader(c net.Conn) int {
	scanner := bufio.NewScanner(c)

	for scanner.Scan() {
		line := scanner.Text()
		var message engine.EngineIPCMessage
		if err := json.Unmarshal([]byte(line), &message); err != nil {
			log.Println("Error unmarshalling JSON:", err)
			continue
		}

		if message.Status == utils.Exit {
			fmt.Printf("\n\r\n") // avoid that annoying % from zsh.
			return utils.Exit
		} else {
			fmt.Printf("%s", message.Result)
			return utils.Success
		}
	}
	return utils.Success
}

func ConnectToEngine() {
	c, err := net.Dial("unix", "/tmp/engine.sock")
	if err != nil {
		fmt.Println("Could not connect to engine!")
		os.Exit(couldNotConnectToEngine)
	}

	sendInitMessage(c)

	commandLoop(c)
}

func sendInitMessage(c net.Conn) {
	data := engine.AdminIPCMessage{Program: "admin", Value: ""}

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

var stdinReader = bufio.NewReader(os.Stdin)

func getInput(prompt string) string {
	fmt.Print(prompt)
	input, err := stdinReader.ReadString('\n')
	if err != nil {
		log.Fatalf("Error reading input: %v", err)
	}
	return strings.TrimRight(input, "\r\n")
}
