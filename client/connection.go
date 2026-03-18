// Package client handles the connection to the engine and the communication between them. It also contains the main loop for the client.
package client 

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"byte-space/utils"
	"os"

	"byte-space/engine"
)

var (
	sessionID = ""
	prompt = adminPrompt
)

func writeToEngine(c net.Conn, s string, mode string) {
	data := engine.ClientIPCMessage{Program: mode, RequestID: 1, Command: s, SessionID: sessionID}

	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Fatalf("Error occurred during marshalling: %s", err.Error())
	}

	_, err = c.Write([]byte(jsonData))
	if err != nil {
		log.Println("Could not write to server!")
	}
}

func engineReader(c net.Conn, output bool) engine.EngineIPCMessage {
	data := make([]byte, 1024)
	n, err := c.Read(data)
	if err != nil {
		log.Println("Cannot read data from engine!")
	}
	var message engine.EngineIPCMessage
	err = json.Unmarshal(data[:n], &message)
	if err != nil {
		log.Fatalf("Error unmarshalling JSON: %v", err)
	}

	if message.Status == utils.Exit {
		displayResponse(&message)
		os.Exit(0)
	}

	// check if prompt change:

	if message.Prompt != "" {
		prompt = message.Prompt
	}

	if output {
		displayResponse(&message)
	} else {
		return message
	}


	return engine.EngineIPCMessage{}
}


func ConnectToEngine(mode string) {
	c, err := net.Dial("unix", "/tmp/engine.sock")
	if err != nil {
		fmt.Println("Could not connect to engine!")
		os.Exit(couldNotConnectToEngine)
	}

	
	if mode == "user" {
		connectToWorkstation(c)
	}


	commandLoop(c, mode)
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

func connectToWorkstation(c net.Conn) {
	// list workstations and ask user to select one

	writeToEngine(c, "list-nodes computer", "admin")
	fmt.Println("Select a workstation to connect to:")
	msg := engineReader(c, false) 
	if msg.Result == "No machines on network" {
		fmt.Println("No workstations found on the network. Please add a workstation before connecting.")
		os.Exit(noWorkstationsFound)
	}

	fmt.Println(msg.Result)

	workstation := getInput("Enter workstation name (case-sensitive): ")
	
	writeToEngine(c, fmt.Sprintf("connect %s", workstation), "connection")

	msg = engineReader(c, false)
	if msg.Status != utils.Success {
		fmt.Printf("Could not connect to workstation %s. Please check the name and try again.\n", workstation)
		os.Exit(noWorkstationsFound)
	}
	fmt.Printf("%s", msg.Result)
	username := getInput("")
	writeToEngine(c, fmt.Sprintf("username %s", username), "connection")
	engineReader(c, true)
	password := getInput("")
	writeToEngine(c, fmt.Sprintf("login %s %s %s", workstation, username, password), "connection")
	loginMsg := engineReader(c, false)
	if loginMsg.Status != utils.Success {
		displayResponse(&engine.EngineIPCMessage{Status: loginMsg.Status, Result: "Login failed. Please check your credentials and try again."})
		os.Exit(invalidCredentials)
	}
	prompt = loginMsg.Prompt
	sessionID = loginMsg.SessionID
	fmt.Printf("Successfully connected to workstation %s with username %s. Session ID: %s\n", workstation, username, sessionID)
}



