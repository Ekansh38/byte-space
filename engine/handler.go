package engine

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"strings"

	"byte-space/computer"
	"byte-space/utils"
)

func (e *Engine) handleClient(conn net.Conn) {

	// PICK THE MACHINE on the engine side, but login (username, password)
	// on the computer itself, realistic!!
	// no need for all that previous bootstrapping stuff.

	write := func(msg string, status int) {
		data, _ := json.Marshal(computer.NewIPCMessage(msg, status))
		conn.Write(append(data, '\n'))
	}

	// show available machines
	nodes := e.ListMachinesOnNetwork()
	var sb strings.Builder
	sb.WriteString("\033[H\033[2J")
	for _, c := range nodes {
		fmt.Fprintf(&sb, "%s: %s\r\n", c.IP, c.Name)
	}
	sb.WriteString("\r\nSelect home workstation (you can telnet onto different machines later)\n\n")

	sb.WriteString("IP ADDRESS: ")
	write(sb.String(), utils.Success)

	// ik i am redoing some tty logic but worse, JUST LET ME BE!!
	var buf strings.Builder
	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		var msg ClientIPCMessage
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			continue
		}
		switch msg.Keystroke {
		case "\r":
			write("\r\n", utils.Success)
			ip := buf.String()
			target, ok := e.GetNode(ip)
			if !ok {
				write("\r\nNo machine at that address.\r\n", utils.Exit)
				conn.Close()
				return
			}
			target.HandleClient(conn) // bye bye!
			return

		case "\x1b[A", "\x1b[B", "\x1b[C", "\x1b[D":
			continue
		case "\x7f":
			if buf.Len() > 0 {
				runes := []rune(buf.String())
				buf.Reset()
				buf.WriteString(string(runes[:len(runes)-1]))
				write("\b \b", utils.Success)
			}
		default:
			buf.WriteString(msg.Keystroke)
			write(msg.Keystroke, utils.Success)
		}
	}

	conn.Close()
}
