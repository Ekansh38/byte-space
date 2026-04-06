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
	ttyID := fmt.Sprintf("tty-%d", len(c.ttys))

	c.OS.Network.PublishEvent(EventTTYCreated, map[string]interface{}{
		"tty_id": ttyID,
	})

	tty := NewTTY(conn, c.OS.Network, ttyID)
	c.ttys = append(c.ttys, tty)

	// TEMP session
	tty.Session = &Session{
		Computer:    c,
		CurrentUser: "",
		WorkingDir:  "/",
		TTY:         tty,
	}

	go func() {
		network := c.OS.Network

		if err := c.Kernel.Exec(tty.Session, "/bin/login", []string{}, &ExecOpts{}); err != nil {
			network.WriteToClient(conn, "\nInvalid login conditionals or exit login program.\r\n", utils.Exit)
			network.PublishEvent(EventTTYClosed, map[string]interface{}{"tty_id": tty.id})
			conn.Close()
			return
		}

		// change to actual session
		targetKernel := tty.Session.Computer.Kernel
		if err := targetKernel.Exec(tty.Session, "/bin/sh", []string{}, &ExecOpts{}); err != nil {
			network.WriteToClient(conn, "Exiting with an error", utils.Exit)
		} else {
			network.WriteToClient(conn, "Exiting...", utils.Exit)
		}

		network.PublishEvent(EventTTYClosed, map[string]interface{}{"tty_id": tty.id})
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
