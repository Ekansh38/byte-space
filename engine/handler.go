package engine

import (
	"net"
)

func (e *Engine) handleClient(conn net.Conn) {
	for _, computer := range e.nodes { // random first computer for now.
		computer.HandleClient(conn)
		return
	}
}
