// Package engine is the main sim
package engine

import (
	"bufio"
	"encoding/json"
	"log"
	"net"
	"os"
)


func (e *Engine) Run() {
	os.Remove("/tmp/engine.sock");
	l, err := net.Listen("unix", "/tmp/engine.sock");
	if err != nil {
		log.Fatal(err)
	}
	defer l.Close()
	for {
		// Wait for a connection.
		conn, err := l.Accept()
		if err != nil {
			log.Fatal(err)
		}
		go e.handleConn(conn)

	}
}

func (e *Engine) handleConn(c net.Conn) {
	// handle init message, based on that send off to correct place (for client, handleclient, for admin handleAdmin)

	scanner := bufio.NewScanner(c)
	for scanner.Scan() {
		var msg ClientIPCMessage
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			continue
		}

		if msg.Program == "user" {
			e.handleClient(c)
		} else if msg.Program == "admin" {
			e.handleAdmin(c)
		}
	}

}
