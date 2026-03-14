package engine

import (
	"fmt"
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
	fmt.Println("Engine is running");
	for {
		// Wait for a connection.
		conn, err := l.Accept()
		if err != nil {
			log.Fatal(err)
		}
		fmt.Println("Received a connection")
		go handleClient(conn)
	}


}
