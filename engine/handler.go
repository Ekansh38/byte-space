package engine


import (
	"byte-space/utils"
	"encoding/json"
	"fmt"
	"log"
	"net"
)

func sendIPCMessage(c net.Conn, sendData *EngineIPCMessage) {

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

		msg := newIPCMessage("", utils.Success)

		if message.Command == "exit" {
			data := newIPCMessage("Exiting...", utils.Exit)
			sendIPCMessage(c, data)
			c.Close()
			fmt.Println("Connection closed")
			return
		}

		if message.Program == "admin" || message.Program == "connection" {
			msg = e.runAdminCommand(message.Command)
		}

		if message.Program == "user" {
			session := e.sessions[message.SessionID]
			msg = newIPCMessage(fmt.Sprintf("hey hey hey user. is this your data? %s %s %s %s", session.CurrentUser, session.Computer.IP, session.SessionID, session.Computer.Name), utils.Success)
		}

		sendIPCMessage(c, msg)


	}
}
