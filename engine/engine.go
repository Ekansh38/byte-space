package engine

import (
	"fmt"
	"log"

	"byte-space/computer"
	"byte-space/utils"
)

type Engine struct {
	nodes    map[string]*computer.Computer // by IP address
	sessions map[string]*Session           // by session IP
	EventBus *EventBus                     // to transmit events to tui
	ttys     []*TTY
}

// API

func (e *Engine) ListMachinesOnNetwork() []computer.Computer {
	var computers []computer.Computer
	for _, computer := range e.nodes {
		computers = append(computers, *computer)
	}
	return computers
}

type Session struct {
	SessionID   string
	Computer    *computer.Computer
	CurrentUser string
	WorkingDir  string
}

func NewEngine() *Engine {
	e := &Engine{nodes: make(map[string]*computer.Computer), sessions: make(map[string]*Session), EventBus: NewEventBus()}

	// load network
	err := e.LoadNetwork()
	if err != nil {
		log.Printf("Warning: Could not load network: %s", err)
	}

	return e
}

func (e *Engine) NewSession(node *computer.Computer, username string) (int, string) {
	// generate unique session ID
	sessionID := e.generateSessionID()
	workingDir := "/"

	if username == "root" {
		workingDir = "/root"
	} else {
		workingDir = "/home/" + username
	}

	e.sessions[sessionID] = &Session{
		SessionID:   sessionID,
		Computer:    node,
		CurrentUser: username,
		WorkingDir:  workingDir,
	}

	if !(e.sessions[sessionID].Computer.OS.HasDirectory(workingDir)) {
		e.sessions[sessionID].Computer.OS.Mkdir(workingDir)
	}

	e.EventBus.Publish(EventSessionCreated, map[string]interface{}{
		"session_id":  sessionID,
		"user":        username,
		"computer":    node.Name,
		"working_dir": workingDir, 
	})

	return utils.Success, sessionID
}

func (e *Engine) generateSessionID() string {
	// count number of active sessions
	count := len(e.sessions)
	sessionID := fmt.Sprintf("session-%d", count+1)
	return sessionID
}
