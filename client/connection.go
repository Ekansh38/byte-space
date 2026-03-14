package client // package client is the main client application

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"

	"byte-space/engine"
)

func writeToEngine(c net.Conn, s string, mode string) {
	data := engine.ClientIPCMessage{Program: mode, RequestID: 1, IP: "nil", Command: s}

	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Fatalf("Error occurred during marshalling: %s", err.Error())
	}

	_, err = c.Write([]byte(jsonData))
	if err != nil {
		log.Println("Could not write to server!")
	}
}

func engineReader(c net.Conn) int {
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

	fmt.Println(message.Result)

	if message.Status == 10 { // exit status
		return 10
	}
	return 0
}

func ConnectToEngine(mode string) {
	c, err := net.Dial("unix", "/tmp/engine.sock")
	if err != nil {
		fmt.Println("Could not connect to engine!")
		os.Exit(couldNotConnectToEngine)
	}

	commandLoop(c, mode)
}
