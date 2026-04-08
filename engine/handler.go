package engine

import (
	"net"
)

func (e *Engine) handleClient(conn net.Conn) {
	for _, computer := range e.nodes { // random first computer cuz if there is no computers u cant do anything anyway.
		computer.HandleClient(conn)
		return
	}
}
