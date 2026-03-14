package engine


import (
	"encoding/json"
	"fmt"
	"log"
	"net"
)

func sendIPCMessage(c net.Conn, message string, status int) {

		sendData := newIPCMessage(message, status)
		jsonData, err := json.Marshal(sendData)
		if err != nil {
			log.Fatalf("Error occurred during marshalling: %s", err.Error())
		}
		fmt.Printf("Sending data: %s\n", string(jsonData))
		c.Write([]byte(jsonData))

}

func (e *Engine) handleClient(c net.Conn) {
	for {
		var data []byte = make([]byte, 1024)
		n, err := c.Read(data)
		if err != nil {
			log.Printf("Error reading data: %s", err.Error())
			return
		}
		var message ClientIPCMessage
		err = json.Unmarshal(data[:n], &message)
		if err != nil {
			log.Fatalf("Error unmarshalling JSON: %v", err)
		}

		returnValue := ""

		if message.Command == "exit" {
			sendIPCMessage(c, "Exiting...", 10)
			c.Close()
			fmt.Println("Connection closed")
			return
		}

		if message.Program == "admin" {
			returnValue = e.runAdminCommand(message.Command)
		}

		sendIPCMessage(c, returnValue, 0)


	}
}
