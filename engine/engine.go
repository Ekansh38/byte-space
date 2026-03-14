package engine

import (
	"byte-space/computer"
	"log"
)

type Engine struct {
	nodes map[string]*computer.Computer // by IP address
	sessions map[string]*Session // by session IP
}

type Session struct {
    SessionID   string
    Computer    *computer.Computer
    CurrentUser string
    WorkingDir  string
    Environment map[string]string
}

func NewEngine() *Engine {
	e := &Engine{nodes: make(map[string]*computer.Computer)}

	// load network
	err := e.LoadNetwork()
    if err != nil {
        log.Printf("Warning: Could not load network: %s", err)
    }
    
    return e

}

