package engine

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
//	"net/url"

	"byte-space/utils"
)

func writeToClient(c net.Conn, sendData *EngineIPCMessage) {
	fmt.Printf("Keystroke: %q\n", sendData.Result)
	jsonData, err := json.Marshal(sendData)
	if err != nil {
		log.Fatalf("Error occurred during marshalling: %s", err.Error())
	}

	jsonData = append(jsonData, '\n')
	fmt.Printf("Sending data: %s\n", string(jsonData))
	c.Write([]byte(jsonData))
}

func (e *Engine) monitroLoginAndShellStatusForExit(loginStatus chan int, c net.Conn, tty *TTY) {
	loginStatusValue := <-loginStatus
	if loginStatusValue == utils.Error {
		data := newIPCMessage("Invalid login conditionals or exit login program.\r\n", utils.Exit)
		writeToClient(c, data)
		c.Close()
		fmt.Println("Connection closed")
		return
	} else {
		// create the shell, set the foreground, run the shell.
		shell := &Shell{tty: tty, id: "1"}
		tty.SetForegroundProcess(shell)
		var returnStatus chan int = make(chan int)
		go shell.Run(returnStatus, []string{})
		theValue := <-returnStatus
		if theValue == utils.Success {
			writeToClient(c, newIPCMessage("Exiting...", utils.Exit))
		} else {
			writeToClient(c, newIPCMessage("Exiting with an error", utils.Exit))
		}
		return
	}
}

func (e *Engine) handleClient(c net.Conn) {
	//mode := ""
	//scanner := bufio.NewScanner(c)
	//for scanner.Scan() {
	//line := scanner.Text()
	//var message ClientIPCMessage
	//if err := json.Unmarshal([]byte(line), &message); err != nil {
	//log.Println("Error unmarshalling JSON:", err)
	//continue
	//}
	//mode = message.Program
	//break
	//}

	// create the TTY, and run the login program in a goroutine
	tty := NewTTY(c)
	loginProgram := &LoginProgram{id: "0", tty: tty, Engine: e}
	tty.SetForegroundProcess(loginProgram)

	loginStatus := make(chan int)
	go loginProgram.Run(loginStatus, []string{})
	go e.monitroLoginAndShellStatusForExit(loginStatus, c, tty)

	for {
		scanner := bufio.NewScanner(c)
		for scanner.Scan() {
			line := scanner.Text()
			var message ClientIPCMessage
			if err := json.Unmarshal([]byte(line), &message); err != nil {
				log.Println("Error unmarshalling JSON:", err)
				continue
			}

			tty.HandleKeystroke(message.Keystroke)

			fmt.Printf("%q\n", message.Keystroke)

			// msg := newIPCMessage("", utils.Success)

			//if message.Program == "admin" || message.Program == "connection" {
			//msg = e.runAdminCommand(message.Keystroke)
			//}

			//if message.Program == "user" {
			//session := e.sessions[message.SessionID]
			//shell := &Shell{Session: session}
			//msg = shell.Run(message.Keystroke)
			//}

			// writeToClient(c, msg)
		}
	}
}
