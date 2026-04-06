package engine

import (
	"byte-space/computer"
	"encoding/json"
	"log"
	"net"
)

func (e *Engine) WriteToClient(c net.Conn, sendData string, status int) {
	jsonData, err := json.Marshal(computer.NewIPCMessage(sendData, status))
	if err != nil {
		log.Fatalf("Error occurred during marshalling: %s", err.Error())
	}

	jsonData = append(jsonData, '\n')
	c.Write([]byte(jsonData))
}

func (e *Engine) handleClient(conn net.Conn) {
	for _, computer := range e.nodes { // random first computer for now.
		computer.HandleClient(conn)
		return
	}
}
