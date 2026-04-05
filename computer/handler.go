package computer

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	//	"net/url"

	"byte-space/utils"
)

func (c *Computer) monitroLoginAndShellStatusForExit(loginStatus chan int, conn net.Conn, tty *TTY, graphicsAPI *GraphicsAPI) {
	e := c.OS.Network
	loginStatusValue := <-loginStatus
	e.PublishEvent(EventProgramExited, map[string]interface{}{
		"program_id": "login-0",
		"status":     0,
		"tty_id":     tty.id,
	})
	if loginStatusValue == utils.Error {
		data := "\nInvalid login conditionals or exit login program.\r\n"
		e.WriteToClient(conn, data, utils.Exit)

		e.PublishEvent(EventTTYClosed, map[string]interface{}{
			"tty_id": tty.id,
		})

		conn.Close()
		return
	} else {
		// create the shell, set the foreground, run the shell.
		shell := &Shell{tty: tty, id: "shell-0", Kernel: tty.Session.Computer.Kernel}
		shell.SetProcess(&Process{
			PID:  0,
			UID:  tty.Session.CurrentUser,
			EUID: tty.Session.CurrentUser,
			CWD:  tty.Session.WorkingDir,
			TTY:  tty,
		})
		tty.SetForegroundProcess(shell)
		var returnStatus chan int = make(chan int)
		go shell.Run(returnStatus, []string{})

		e.PublishEvent(EventProgramStarted, map[string]interface{}{
			"program_id": "shell-0",
			"tty_id":     tty.id,
		})

		theValue := <-returnStatus

		if theValue == utils.Success {
			e.WriteToClient(conn, "Exiting...", utils.Exit)
			e.PublishEvent(EventTTYClosed, map[string]interface{}{
				"tty_id": tty.id,
			})
		} else {
			e.WriteToClient(conn, "Exiting with an error", utils.Exit)
			e.PublishEvent(EventTTYClosed, map[string]interface{}{
				"tty_id": tty.id,
			})
		}
		return
	}
}

func (c *Computer) HandleClient(conn net.Conn) {
	// create the TTY, and run the login program in a goroutine
	ttyID := fmt.Sprintf("tty-%d", len(c.ttys))

	c.OS.Network.PublishEvent(EventTTYCreated, map[string]interface{}{
		"tty_id": ttyID,
	})

	tty := NewTTY(conn, c.OS.Network, ttyID)
	c.ttys = append(c.ttys, tty)

	loginProgram := &LoginProgram{id: "login-0", NetworkAPI: c.OS.Network}
	loginProgram.ttyAPI = &TTYAPI{tty: tty, program: loginProgram}

	tty.SetForegroundProcess(loginProgram)

	loginStatus := make(chan int)
	go loginProgram.Run(loginStatus, []string{})
	c.OS.Network.PublishEvent(EventProgramStarted, map[string]interface{}{
		"program_id": "login-0",
		"tty_id":     tty.id,
	})
	go c.monitroLoginAndShellStatusForExit(loginStatus, conn, tty, NewGraphicsAPI(tty)) // ik  both of them have a graphics API, but DEAL WITH IT U HATER!

	for {
		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			line := scanner.Text()
			var message ClientIPCMessage
			if err := json.Unmarshal([]byte(line), &message); err != nil {
				log.Println("Error unmarshalling JSON:", err)
				continue
			}
			tty.HandleKeystroke(message.Keystroke)
		}
	}
}
