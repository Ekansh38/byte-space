package computer

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"

	"byte-space/utils"
)

func (c *Computer) HandleClient(conn net.Conn) {
	// create a new tty

	ttyID := fmt.Sprintf("tty-%d", len(c.ttys))

	c.EventBus.Publish(EventTTYCreated, map[string]interface{}{
		"tty_id": ttyID,
	})

	tty := NewTTY(conn, c.EventBus, ttyID)

	// TEMP session
	tty.Session = &Session{
		Computer:    c,
		CurrentUser: "",
		TTY:         tty,
	}

	go func() {
		daddyProc := &Process{
			PID:  0, // TODO, dont hardcode this, actually it doesnt really maatrwe
			UID:  "root",
			EUID: "root",
			CWD:  "/",
			PGID: 0,
		}
		daddyProgram := &Shell{
			ttyAPI: &TTYAPI{tty: tty, proc: daddyProc},
		}
		daddyProc.Program = daddyProgram // BOOTSTRAPPP!!!! I learned that word yesterday from the crafting interpetters book, pull urself up from ur own bootstraps!!!

		if err := c.Kernel.Exec(daddyProc, "/bin/login", []string{}, &ExecOpts{}); err != nil {
			tty.writeToClient("\nInvalid login conditionals or exit login program.\r\n", utils.Exit)
			c.EventBus.Publish(EventTTYClosed, map[string]interface{}{"tty_id": tty.id})
			conn.Close()
			return
		}

		tty.Session.Computer.ttys = append(tty.Session.Computer.ttys, tty)

		// change the uid to the actual user
		loggedInUser := tty.Session.CurrentUser
		daddyProc.UID = loggedInUser
		daddyProc.EUID = loggedInUser

		workingDir := "/"

		if loggedInUser == "root" {
			workingDir = "/root"
		} else {
			workingDir = "/home/" + loggedInUser
		}

		daddyProc.CWD = workingDir

		// change to actual session
		targetKernel := tty.Session.Computer.Kernel

		if err := targetKernel.Exec(daddyProc, "/bin/sh", []string{}, &ExecOpts{}); err != nil {
			tty.writeToClient("\nExiting with an error", utils.Exit)
		} else {
			tty.writeToClient("\nExiting...", utils.Exit)
		}

		c.EventBus.Publish(EventTTYClosed, map[string]interface{}{"tty_id": tty.id})
		conn.Close()
	}()

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
