// Package engine is the main sim
package engine

import (
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
		go e.handleClient(conn)

	}


}
