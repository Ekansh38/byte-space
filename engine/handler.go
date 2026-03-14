package engine


import (
	"encoding/json"
	"fmt"
	"log"
	"net"
)


func handleClient(c net.Conn) {
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

		fmt.Printf("Received data: %s\n", message.Command)

		sendData := newIPCMessage("hey...")
		jsonData, err := json.Marshal(sendData)
		if err != nil {
			log.Fatalf("Error occurred during marshalling: %s", err.Error())
		}
		fmt.Printf("Sending data: %s\n", string(jsonData))
		c.Write([]byte(jsonData))
	}
}

