package engine

import (
	"byte-space/computer"
	"byte-space/utils"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/afero"
)

func (e *Engine) runAdminCommand(command string) *EngineIPCMessage {
	fmt.Printf("Running admin command: %s\n", command)

	// parse the command	

	commandP := parseCommand(command)

	if len(commandP) == 0 {
		message := "No command provided" // this should be filtered out by the client but if the API is used directly.
		fmt.Println(message)
		data := newIPCMessage(message, utils.Error)
		return data
	}

	switch commandP[0] {
	case "spawn":
		return e.spawnNode(commandP)
	case "list-nodes":
		return e.listNodes(commandP)
	case "delete":
		return e.deleteNode(commandP)
	case "reset-network":
		return e.resetNetwork()
	case "adduser":
		return e.addUser(commandP)
	case "connect":
		return e.connectUserToNode(commandP)
	case "username":
		return e.username(commandP)
	case "login":
		return e.login(commandP)

	default:
		return newIPCMessage("not implemented", utils.Warning)

	}


}

func (e *Engine) spawnNode(commandParsed []string) *EngineIPCMessage {

	if len(commandParsed) != 4 {
		message := "Usage: spawn <type> <name> <ip>"
		fmt.Println(message)
		return newIPCMessage(message, utils.Error)
	}

	name := commandParsed[2]
	ip := commandParsed[3]
	nodeType := commandParsed[1]

	if nodeType != "computer" {
		message := fmt.Sprintf("Node type %s is not supported", nodeType)
		fmt.Println(message)
		return newIPCMessage(message, utils.Error)
	}

	// check uniqueness of name and ip
	for _, node := range e.nodes {
		if node.Name == name {
			message := fmt.Sprintf("A node with the name %s already exists", name)
			fmt.Println(message)
			return newIPCMessage(message, utils.Error)
		}
		if node.IP == ip {
			message := fmt.Sprintf("A node with the IP %s already exists", ip)
			fmt.Println(message)
			return newIPCMessage(message, utils.Error)
		}
	}

	newNode := computer.New(name, ip, nodeType)
	e.nodes[ip] = newNode

	e.SaveNetwork()

	message := fmt.Sprintf("A %s node named: %s with IP: %s spawned successfully\nTip: adduser %s root <password>", nodeType, name, ip, name)
	fmt.Println(message)

	return newIPCMessage(message, utils.Success)
}


func (e *Engine) listNodes(commandParsed []string) *EngineIPCMessage {
	if len(commandParsed) > 2 {
		message := "Usage: list-nodes"
		fmt.Println(message)
		return newIPCMessage(message, utils.Error)
	}
	message := "" 
	listFormat := "%s: %s: %s\n"
	if len(commandParsed) == 1 {
		for _, node := range(e.nodes) {
			message += fmt.Sprintf(listFormat, node.Type, node.Name, node.IP)
		}

	}
	if len(commandParsed) == 2 {
		nodeType := commandParsed[1]
		for _, node := range(e.nodes) {
			if node.Type == nodeType {
				message += fmt.Sprintf(listFormat, node.Type, node.Name, node.IP)
			}
		}
	}

	if len(e.nodes) == 0 {
		message = "No machines on network"	
	}

	if len(e.nodes) != 0 {
		message = message[:len(message)-1]
	}

	fmt.Println(message)
	return newIPCMessage(message, utils.Success)

}

func (e *Engine) deleteNode(commandParsed []string) *EngineIPCMessage {
	if len(commandParsed) != 2 {
		message := "Usage: delete <name>"
		fmt.Println(message)
		return newIPCMessage(message, utils.Error)
	}

	nodeName := commandParsed[1]
	message := ""
	status := utils.Success

	node, found := getNodeByName(e, nodeName)
	if !found {
		message = fmt.Sprintf("No node with the name %s found", nodeName)
		fmt.Println(message)
		return newIPCMessage(message, utils.Error)
	}

	delete(e.nodes, node.IP)
	// delete on disk
	path := networkPath + "/nodes/" + node.Name
	err := os.RemoveAll(path)
	if err != nil {
		return newIPCMessage(fmt.Sprintf("Error deleting filesystem: %s", err), utils.Error)
	}
	message = fmt.Sprintf("Node %s deleted successfully", nodeName)
	status =  utils.Success

	e.SaveNetwork()

	fmt.Println(message)

	return newIPCMessage(message, status)

}

func getNodeByName(e *Engine, name string) (*computer.Computer, bool) {
	for _, node := range e.nodes {
		if node.Name == name {
			return node, true
		}
	}

	return nil, false

}

func (e *Engine) addUser(commandParsed []string)  *EngineIPCMessage {

	if len(commandParsed) != 4 {
		message := "Usage: adduser <name> <username> <password>"
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
	username := commandParsed[2]
	password := commandParsed[3]
	msg, uid := findUID(node)

	if msg != "" {
		message := fmt.Sprintf("Error finding UID: %s", msg)
		fmt.Println(message)
		return newIPCMessage(message, utils.Error)
	}

	// Check uniqueness of username
	msg, unique := isUsernameUnique(node, username)

	if msg != "" {
		message := fmt.Sprintf("Error checking username uniqueness: %s", msg)
		fmt.Println(message)
		return newIPCMessage(message, utils.Error)
	}

	if !unique {
		message := fmt.Sprintf("Username %s already exists on node %s", username, name)
		fmt.Println(message)
		return newIPCMessage(message, utils.Error)
	}

	// Add user

	response, stat := addUserToNode(node, username, password, uid)

	fmt.Println(response)
	return newIPCMessage(response, stat)
}


func findUID(node *computer.Computer) (string, int) {

	data, err := afero.ReadFile(node.Filesystem, "/etc/passwd")

	if err != nil {
		return fmt.Sprintf("Error reading passwd: %s", err), utils.Error
	}

	lines := strings.Split(string(data), "\n")
	userCount := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			userCount++
		}
	}

	nextUID := 1000 + userCount
	return "", nextUID
}

func isUsernameUnique(node *computer.Computer, username string) (string, bool) {

	data, err := afero.ReadFile(node.Filesystem, "/etc/passwd")

	if err != nil {
		return fmt.Sprintf("Error reading passwd: %s", err), false
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			fields := strings.Split(line, ":")
			if len(fields) >= 1 && fields[0] == username {
				return "", false
			}
		}
	}

	return "", true

}

func addUserToNode(node *computer.Computer, username string, password string, uid int) (string, int) {
	existingData, err := afero.ReadFile(node.Filesystem, "/etc/passwd")
	if err != nil {
		existingData = []byte("")  // File doesn't exist, start fresh
	}

	line := ""
	if username == "root" {
		line = fmt.Sprintf("%s:%s:%d:/root", username, password, uid)
	} else{
		line = fmt.Sprintf("%s:%s:%d:/home/%s", username, password, uid, username)
	}

	newContent := string(existingData) + line + "\n"

	// Write back
	err = afero.WriteFile(node.Filesystem, "/etc/passwd", []byte(newContent), 0644)
	if err != nil {
		return fmt.Sprintf("Error writing to passwd: %s", err), utils.Error
	}

	return fmt.Sprintf("Successfully added %s", username), utils.Success
}

