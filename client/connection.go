// Package client handles the connection to the engine and the communication between them. It also contains the main loop for the client.
package client 

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"

	"byte-space/engine"
)

func writeToEngine(c net.Conn, s string, mode string) {
	data := engine.ClientIPCMessage{Program: mode, RequestID: 1, Command: s}

	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Fatalf("Error occurred during marshalling: %s", err.Error())
	}

	_, err = c.Write([]byte(jsonData))
	if err != nil {
		log.Println("Could not write to server!")
	}
}

func engineReader(c net.Conn, output bool) (int, string) {
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

	if output {
		displayResponse(&message)
	}

	if message.Status == 10 { // exit status
		return 10, ""
	}
	
	if !output {
		return message.Status, message.Result
	}

	return 0, ""
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


	commandLoop(c, mode, adminPrompt)
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
	_, msg := engineReader(c, false) 
	if msg == "No machines on network" {
		fmt.Println("No workstations found on the network. Please add a workstation before connecting.")
		os.Exit(noWorkstationsFound)
	}

	fmt.Println(msg)

	workstation := getInput("Enter workstation name (case-sensitive): ")
	
	writeToEngine(c, fmt.Sprintf("connect %s", workstation), "connection")

	status, _ := engineReader(c, true)
	if status != 0 {
		fmt.Printf("Could not connect to workstation %s. Please check the name and try again.\n", workstation)
		os.Exit(noWorkstationsFound)
	}
	username := getInput("")
	writeToEngine(c, fmt.Sprintf("username %s", username), "connection")
	engineReader(c, true)
	password := getInput("")
	writeToEngine(c, fmt.Sprintf("login %s %s %s", workstation, username, password), "connection")
	engineReader(c, true)

}



