package engine

import (
	"byte-space/computer"
	"byte-space/utils"
	"fmt"
	"strings"
)

func (e *Engine) runAdminCommand(command string) (string, int){
	fmt.Printf("Running admin command: %s\n", command)

	// parse the command	

	commandParsed := strings.Fields(command)

	if len(commandParsed) == 0 {
		message := "No command provided" // this should be filtered out by the client but if the API is used directly.
		fmt.Println(message)
		return message, utils.Error
	}

	switch commandParsed[0] {
	case "spawn":
		return e.spawnNode(commandParsed)
	case "list-nodes":
		return e.listNodes(commandParsed)
	case "delete":
		return e.deleteNode(commandParsed)
	case "reset-network":
		return e.resetNetwork()
	default:
		return "not implemented", utils.Warning

	}


}

func (e *Engine) spawnNode(commandParsed []string) (string, int) {

	if len(commandParsed) != 4 {
		message := "Usage: spawn <type> <name> <ip>"
		fmt.Println(message)
		return message, utils.Error
	}

	name := commandParsed[2]
	ip := commandParsed[3]
	nodeType := commandParsed[1]

	if nodeType != "computer" {
		message := fmt.Sprintf("Node type %s is not supported", nodeType)
		fmt.Println(message)
		return message, utils.Error
	}

	// check uniqueness of name and ip
	for _, node := range e.nodes {
		if node.Name == name {
			message := fmt.Sprintf("A node with the name %s already exists", name)
			fmt.Println(message)
			return message, utils.Error
		}
		if node.IP == ip {
			message := fmt.Sprintf("A node with the IP %s already exists", ip)
			fmt.Println(message)
			return message, utils.Error
		}
	}

	newNode := computer.New(name, ip, nodeType)
	e.nodes[ip] = newNode

	e.SaveNetwork()

	message := fmt.Sprintf("A %s node named: %s with IP: %s spawned successfully", nodeType, name, ip)
	fmt.Println(message)
	return message, utils.Success
}


func (e *Engine) listNodes(commandParsed []string) (string, int) {
	if len(commandParsed) != 1 {
		message := "Usage: list-nodes"
		fmt.Println(message)
		return message, utils.Error
	}

	message := "" 

	for _, node := range(e.nodes) {
		message += fmt.Sprintf("%s: %s: %s\n", node.Type, node.Name, node.IP)
	}

	fmt.Println(message)
	return message, utils.Success

}

func (e *Engine) deleteNode(commandParsed []string) (string, int) {
	if len(commandParsed) != 2 {
		message := "Usage: delete <name>"
		fmt.Println(message)
		return message, utils.Error
	}

	nodeName := commandParsed[1]
	message := ""
	status := utils.Success

	node, found := getNodeByName(e, nodeName)
	if !found {
		message = fmt.Sprintf("No node with the name %s found", nodeName)
		status = utils.Error
		fmt.Println(message)
		return message, status
	}

	delete(e.nodes, node.IP)
	message = fmt.Sprintf("Node %s deleted successfully", nodeName)
	status =  utils.Success

	fmt.Println(message)

	return message, status

}

func getNodeByName(e *Engine, name string) (*computer.Computer, bool) {
	for _, node := range e.nodes {
		if node.Name == name {
			return node, true
		}
	}

	return nil, false

}
