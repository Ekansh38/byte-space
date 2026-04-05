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
)

var (
	sessionID = ""
	prompt    = adminPrompt
)

func writeToEngine(c net.Conn, rawKeystroke string, mode string) int {
	data := engine.ClientIPCMessage{Program: mode, RequestID: 1, Keystroke: rawKeystroke, SessionID: sessionID}

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

func ConnectToEngine(mode string) {
	c, err := net.Dial("unix", "/tmp/engine.sock")
	if err != nil {
		fmt.Println("Could not connect to engine!")
		os.Exit(couldNotConnectToEngine)
	}

	//if mode == "user" {
	//	connectToWorkstation(c)
	//}

	done := make(chan struct{})

	go engineReader(c, done)
	commandLoop(c, mode, done)
}

func getInput(prompt string) string {
	fmt.Print(prompt)
	var input string
	_, err := fmt.Scanln(&input)
	if err != nil {
		log.Fatalf("Error reading input: %v", err)
	}
	return input
}

//func connectToWorkstation(c net.Conn) {
//	// list workstations and ask user to select one
//
//	writeToEngine(c, "list-nodes computer", "admin")
//	fmt.Println("Select a workstation to connect to:")
//	msg := engineReader(c)
//	if msg.Result == "No machines on network" {
//		fmt.Println("No workstations found on the network. Please add a workstation before connecting.")
//		os.Exit(noWorkstationsFound)
//	}
//
//	fmt.Println(msg.Result)
//
//	workstation := getInput("Enter workstation name (case-sensitive): ")
//
//	writeToEngine(c, fmt.Sprintf("connect %s", workstation), "connection")
//
//	msg = engineReader(c)
//	if msg.Status != utils.Success {
//		fmt.Printf("Could not connect to workstation %s. Please check the name and try again.\n", workstation)
//		os.Exit(noWorkstationsFound)
//	}
//	fmt.Printf("%s", msg.Result)
//	username := getInput("")
//	writeToEngine(c, fmt.Sprintf("username %s", username), "connection")
//	engineReader(c)
//	password := getInput("")
//	writeToEngine(c, fmt.Sprintf("login %s %s %s", workstation, username, password), "connection")
//	engineReader(c)
//	if loginMsg.Status != utils.Success {
//		displayResponse(&engine.EngineIPCMessage{Status: loginMsg.Status, Result: "Login failed. Please check your credentials and try again."})
//		os.Exit(invalidCredentials)
//	}
//	sessionID = loginMsg.SessionID
//	fmt.Printf("Successfully connected to workstation %s with username %s. Session ID: %s\n", workstation, username, sessionID)
//}
