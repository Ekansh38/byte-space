package main

import (
	"main/utils"
	"main/engine"
  	"bufio"
	"flag"
	"fmt"
	"net"
	"os"
	"log"
	"encoding/json"
	"strings"
)

// error enum
const (
	invalidMode = 1
	couldNotConnectToEngine = 2
)

func writeToEngine(c net.Conn, s string, mode string) {
	data := utils.ClientICPMessage{Program: mode, RequestId: 1, IP: "nil", Command: s}


	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Fatalf("Error occurred during marshalling: %s", err.Error())
	}

	_, err = c.Write([]byte(jsonData));
	if err != nil {
		log.Println("Could not write to server!")
	}
}

func engineReader(c net.Conn) {
//	for {
		var data  = make([]byte, 1024)
		n, err := c.Read(data);
		if err != nil {
			log.Println("Cannot read data from engine!")	
		}
		var message engine.EngineICPMessage
		err = json.Unmarshal(data[:n], &message)
		if err != nil {
			log.Fatalf("Error unmarshalling JSON: %v", err)
		}

		fmt.Println(message.Result)
	

//	}
}

// private
func getModeFlag() string {
	var modeFlag string
	flag.StringVar(&modeFlag, "mode", "user", "Mode of operation: 'user' or 'admin'")
	flag.Parse()

	if modeFlag != "user" && modeFlag != "admin" {
		fmt.Println("Please provide a valid mode!")
		os.Exit(1)
	}

	return modeFlag
}

func connectToEngine(mode string) {
	c, err := net.Dial("unix", "/tmp/engine.sock")
	if err != nil {
		fmt.Println("Could not connect to engine!")
		os.Exit(couldNotConnectToEngine)
	}

	commandLoop(c, mode)
}

func commandLoop(c net.Conn, mode string) {
	for {
		fmt.Print("> ")

		reader := bufio.NewReader(os.Stdin)

		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("An error occurred while reading input:", err)
			return
		}

		input = strings.TrimSuffix(input, "\n")

		// common checks
		if (input == ""){
			continue
		}
		if (input == "exit"){
			return
		}

		writeToEngine(c, input, mode)
		engineReader(c)
	}
}

func main() {

	mode := getModeFlag()
	connectToEngine(mode)

	
}

