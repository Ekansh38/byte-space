package engine

import (
	"byte-space/computer"
	"byte-space/utils"
	"fmt"
	"os"
	"strings"

)

func (e *Engine) RunAdminCommand(command string) *EngineIPCMessage {

	// parse the command	

	commandP :=strings.Fields(command)

	if len(commandP) == 0 {
		message := "No command provided" // this should be filtered out by the client but if the API is used directly.
		fmt.Println(message)
		data := newIPCMessage(message, utils.Error)
		return data
	}

	switch commandP[0] {
	case "spawn":
		return e.spawnNode(commandP)
	case "list-nodes", "ls":
		return e.listNodes(commandP)
	case "delete":
		return e.deleteNode(commandP)
	case "reset-network":
		return e.resetNetwork()
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

	newNode := computer.NewComputer(name, ip, nodeType)
	newNode.OS.Network = e
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





