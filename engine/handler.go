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
	jsonData, err := json.Marshal(sendData)
	if err != nil {
		log.Fatalf("Error occurred during marshalling: %s", err.Error())
	}

	jsonData = append(jsonData, '\n')
	c.Write([]byte(jsonData))
}

func (e *Engine) monitroLoginAndShellStatusForExit(loginStatus chan int, c net.Conn, tty *TTY) {
	loginStatusValue := <-loginStatus
	e.EventBus.Publish(EventProgramExited, map[string]interface{}{
		"program_id": "login-0",
		"status":     0,
		"tty_id":     tty.id,
	})
	if loginStatusValue == utils.Error {
		data := newIPCMessage("\nInvalid login conditionals or exit login program.\r\n", utils.Exit)
		writeToClient(c, data)

		e.EventBus.Publish(EventTTYClosed, map[string]interface{}{
			"tty_id": tty.id,
		})

		c.Close()
		return
	} else {
		// create the shell, set the foreground, run the shell.
		shell := &Shell{tty: tty, id: "shell-0"}
		tty.SetForegroundProcess(shell)
		var returnStatus chan int = make(chan int)
		go shell.Run(returnStatus, []string{})

		e.EventBus.Publish(EventProgramStarted, map[string]interface{}{
			"program_id": "shell-0",
			"tty_id":     tty.id,
		})

		theValue := <-returnStatus

		if theValue == utils.Success {
			writeToClient(c, newIPCMessage("Exiting...", utils.Exit))
			e.EventBus.Publish(EventTTYClosed, map[string]interface{}{
				"tty_id": tty.id,
			})
		} else {
			writeToClient(c, newIPCMessage("Exiting with an error", utils.Exit))
			e.EventBus.Publish(EventTTYClosed, map[string]interface{}{
				"tty_id": tty.id,
			})
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
	ttyID := fmt.Sprintf("tty-%d", len(e.ttys))
	e.EventBus.Publish(EventTTYCreated, map[string]interface{}{
		"tty_id": ttyID,
	})

	tty := NewTTY(c, e, ttyID)
	e.ttys = append(e.ttys, tty)

	loginProgram := &LoginProgram{id: "login-0", Engine: e}
	loginProgram.ttyAPI = &TTYAPI{tty: tty, program: loginProgram}

	tty.SetForegroundProcess(loginProgram)

	loginStatus := make(chan int)
	go loginProgram.Run(loginStatus, []string{})
	e.EventBus.Publish(EventProgramStarted, map[string]interface{}{
		"program_id": "login-0",
		"tty_id":     tty.id,
	})
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
			e.EventBus.Publish(EventClientToEngine, map[string]interface{}{
				"key": message.Keystroke,
				"tty": ttyID,
			})

			tty.HandleKeystroke(message.Keystroke)

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
