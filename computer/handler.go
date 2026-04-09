package computer

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"context"

	"byte-space/utils"
)

func (c *Computer) HandleClient(conn net.Conn) {
	ttyID := fmt.Sprintf("tty-%d", len(c.ttys))

	c.EventBus.Publish(EventTTYCreated, map[string]interface{}{
		"tty_id": ttyID,
	})

	tty := NewTTY(conn, c.EventBus, ttyID)

	// tmep session for tty.read
	tty.Session = &Session{
		Computer: c,
		TTY:      tty,
	}

	connCtx, connCancel := context.WithCancel(context.Background())

	go func() {
		daddyProc := &Process{
			PID:  0,
			UID:  "root",
			EUID: "root",
			CWD:  "/",
			PGID: 0,
		}
		daddyProgram := &Shell{
			ttyAPI: &TTYAPI{tty: tty, proc: daddyProc},
		}
		daddyProc.Program = daddyProgram // BOOTSTRAPPP!!!! I learned that word yesterday from the crafting interpetters book, pull urself up from ur own bootstraps!!!

		if err := c.Kernel.Exec(connCtx, daddyProc, "/bin/login", []string{}, &ExecOpts{}); err != nil {
			tty.writeToClient("\nInvalid login or exit.\r\n", utils.Exit)
			c.EventBus.Publish(EventTTYClosed, map[string]interface{}{"tty_id": tty.id})
			conn.Close()
			return
		}

		c.ttys = append(c.ttys, tty)

		loggedInUser := tty.Session.CurrentUser
		daddyProc.UID = loggedInUser
		daddyProc.EUID = loggedInUser

		if loggedInUser == "root" {
			daddyProc.CWD = "/root"
		} else {
			daddyProc.CWD = "/home/" + loggedInUser
		}

		if err := c.Kernel.Exec(connCtx, daddyProc, "/bin/sh", []string{}, &ExecOpts{}); err != nil {
			tty.writeToClient("\nExiting with an error", utils.Exit)
		} else {
			tty.writeToClient("\nExiting...", utils.Exit)
		}

		c.EventBus.Publish(EventTTYClosed, map[string]interface{}{"tty_id": tty.id})
		conn.Close()
	}()

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		var message ClientIPCMessage
		if err := json.Unmarshal(scanner.Bytes(), &message); err != nil {
			log.Println("Error unmarshalling JSON:", err)
			continue
		}
		tty.HandleKeystroke(message.Keystroke)
	}
	defer connCancel()
}
