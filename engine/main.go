// Package engine is the main backend simulation
package engine

import (
	"encoding/json"
	"log"
	"net"
	"fmt"
	"os"
)

type Engine struct {
//	nodes map[string]*computer.Computer
}

type EngineICPMessage struct {
	Result string `json:"result"`
}

func replyToClient(c net.Conn) {
	for {
		var data []byte = make([]byte, 1024)
		n, err := c.Read(data)
		if err != nil {
			log.Printf("Error reading data: %s", err.Error())
			return
		}

		fmt.Printf("Received data: %s\n", string(data[:n]))

		sendData := newICPMessage("hey...")
		jsonData, err := json.Marshal(sendData)
		if err != nil {
			log.Fatalf("Error occurred during marshalling: %s", err.Error())
		}
		fmt.Printf("Sending data: %s\n", string(jsonData))
		c.Write([]byte(jsonData))
	}
}

func (e *Engine) Run() {
	os.Remove("/tmp/engine.sock");
	l, err := net.Listen("unix", "/tmp/engine.sock");
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()
	fmt.Println("Engine is running");
	for {
		// Wait for a connection.
		conn, err := l.Accept()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("Received a connection")
		go replyToClient(conn)
	}


}

func newICPMessage(s string) *EngineICPMessage {
	return &EngineICPMessage{Result: s}
}


func NewEngine() *Engine {
	return &Engine{}
}


