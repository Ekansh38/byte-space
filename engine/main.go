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
		go func(c net.Conn) {
			var data []byte = make([]byte, 1024)
			c.Read(data)

			fmt.Printf("Received data: %s\n", string(data))

			sendData := newICPMessage("hey...")
			jsonData, err := json.Marshal(sendData)
			if err != nil {
				log.Fatalf("Error occurred during marshalling: %s", err.Error())
			}
			fmt.Printf("Sending data: %s\n", string(jsonData))
			c.Write([]byte(jsonData))
		}(conn)
	}


}

func newICPMessage(s string) *EngineICPMessage {
	return &EngineICPMessage{Result: s}
}


func NewEngine() *Engine {
	return &Engine{}
}


