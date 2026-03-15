package engine

import (
	"byte-space/utils"
	"fmt"
)


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

func (e *Engine) username(commandParsed []string)  *EngineIPCMessage {

	if len(commandParsed) != 2 {
		message := "Usage: username <username>"
		fmt.Println(message)
		return newIPCMessage(message, utils.Error)
	}

	return newIPCMessage("password: ", utils.Success)
}

func (e *Engine) login(commandParsed []string)  *EngineIPCMessage {

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
		return &EngineIPCMessage{Result: message, Status: utils.Success, SessionID: sessionID, Prompt: e.sessions[sessionID].WorkingDir + "$ "}
	}

	message := "Invalid username or password"
	fmt.Println(message)
	return newIPCMessage(message, utils.Error)

}

