package main

import (
	"main/utils"
	"main/engine"
//	"bufio"
	"flag"
	"fmt"
	"net"
	"os"
	"log"
	"encoding/json"
)

// error enum
const (
	invalidMode = 1
	couldNotConnectToEngine = 2
)

func writeToEngine(c net.Conn, s string) {
	data := utils.ClientICPMessage{Program: getModeFlag(), RequestId: 1, IP: "nil", Command: s}


	jsonData, err := json.Marshal(data)
	if err != nil {
		log.Fatalf("Error occurred during marshalling: %s", err.Error())
	}

	_, err = c.Write([]byte(jsonData));
	if err != nil {
		log.Println("Could not write to server!")
	}
	fmt.Printf("wrote %s\n", jsonData)
}

func engineReader(c net.Conn) {
	writeToEngine(c, "hey... from client")
	for {
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


	}
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

func connectToEngine() {
	c, err := net.Dial("unix", "/tmp/engine.sock")
	if err != nil {
		fmt.Println("Could not connect to engine!")
		os.Exit(couldNotConnectToEngine)
	}

	engineReader(c)
}

func main() {

	connectToEngine()

	
}

