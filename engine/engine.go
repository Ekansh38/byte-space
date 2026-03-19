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
}

type Session struct {
	SessionID   string
	Computer    *computer.Computer
	CurrentUser string
	WorkingDir  string
	Environment map[string]string
	Shell       *Shell
}

func NewEngine() *Engine {
	e := &Engine{nodes: make(map[string]*computer.Computer), sessions: make(map[string]*Session)}

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
		Environment: make(map[string]string),
	}

	shell := &Shell{Session: e.sessions[sessionID]}
	if !(shell.Session.Computer.OS.HasDirectory(workingDir)) {
		shell.mkdir([]string{"mkdir", workingDir})
	}

	return utils.Success, sessionID
}

func (e *Engine) generateSessionID() string {
	// count number of active sessions
	count := len(e.sessions)
	sessionID := fmt.Sprintf("session-%d", count+1)
	return sessionID
}
