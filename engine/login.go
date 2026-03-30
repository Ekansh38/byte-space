package engine

import (
	"fmt"
	"byte-space/utils"
)

type LoginProgram struct {
	done     chan struct{}
	id       string
	tty      *TTY // if not foreground then nil
	graphics *GraphicsAPI
	Engine *Engine
}

func (p *LoginProgram) AddGraphicsAPI(api *GraphicsAPI) {
	p.graphics = api
}

func (p *LoginProgram) RemoveGraphicsAPI() {
	p.graphics = nil
}

func (p *LoginProgram) Run(returnStatus chan int) {
	p.done = make(chan struct{})
	if p.graphics == nil {
		returnStatus <- utils.Error
		return
	}

	computers := p.Engine.ListMachinesOnNetwork()
	var choices string
	for i := 0; i < len(computers); i++ {
		choices += fmt.Sprintf("%s: %s\n", computers[i].IP, computers[i].Name)
	}
	choices += "\n\n\rIP ADDRESS: "

	p.graphics.Write(choices)

	for {
		value, status := p.tty.Read(p, p.done)
		if status == utils.Success {
			fmt.Printf("VALUE TO LOGIN PROGRAM: %q\n\n", value)
		} else if status == utils.Exit {
			returnStatus <- utils.Error
			return
		}
	}
	returnStatus <- utils.Success
}

func (p *LoginProgram) HandleSignal(sig Signal) {
	if sig == SIGINT {
		select {
		case <-p.done:
			// already closed, do nothing
		default:
			close(p.done)
		}
	}
}

func (p *LoginProgram) ID() string {
	return p.id
}

func (e *Engine) connectUserToNode(commandParsed []string) *EngineIPCMessage {
	if len(commandParsed) != 2 {
		message := "Usage: connect <username>"
		fmt.Println(message)
		return newIPCMessage(message, utils.Error)
	}

	name := commandParsed[1]
	node, status := getNodeByName(e, name)
	if !status {
		message := fmt.Sprintf("No node with the name %s found", name)
		fmt.Println(message)
		return newIPCMessage(message, utils.Error)
	}

	message := node.OS.GetIssue() + "\nusername: "
	fmt.Println(message)
	return newIPCMessage(message, utils.Success)
}

func (e *Engine) username(commandParsed []string) *EngineIPCMessage {
	if len(commandParsed) != 2 {
		message := "Usage: username <username>"
		fmt.Println(message)
		return newIPCMessage(message, utils.Error)
	}

	return newIPCMessage("password: ", utils.Success)
}

func (e *Engine) login(commandParsed []string) *EngineIPCMessage {
	if len(commandParsed) != 4 {
		message := "Usage: login <username> <password>"
		fmt.Println(message)
		return newIPCMessage(message, utils.Error)
	}

	name := commandParsed[1]
	username := commandParsed[2]
	password := commandParsed[3]
	node, status := getNodeByName(e, name)
	if !status {
		message := fmt.Sprintf("No node with the name %s found", name)
		fmt.Println(message)
		return newIPCMessage(message, utils.Error)
	}

	loginStatus := node.OS.Login(username, password)

	if loginStatus == 0 {
		message := node.OS.GetMotd()
		fmt.Println(message)
		// create new session
		sessionStatus, sessionID := e.NewSession(node, username)
		if sessionStatus != utils.Success {
			message := "Failed to create session"
			return newIPCMessage(message, utils.Error)
		}
		return &EngineIPCMessage{Result: message, Status: utils.Success, SessionID: sessionID}
	}

	message := "Invalid username or password"
	fmt.Println(message)
	return newIPCMessage(message, utils.Error)
}
