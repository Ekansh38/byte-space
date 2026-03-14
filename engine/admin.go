package engine

import (
	"fmt"
	"strings"
	"byte-space/computer"
)

func (e *Engine) runAdminCommand(command string) string {
	fmt.Printf("Running admin command: %s\n", command)

	// parse the command	

	commandParsed := strings.Fields(command)

	if len(commandParsed) == 0 {
		message := "No command provided"
		fmt.Println(message)
		return message
	}

	switch commandParsed[0] {
	case "spawn":
		return e.spawnNode(commandParsed)
	case "reset-network":
		return e.resetNetwork()
	default:
		return "not implemented"

	}


}

func (e *Engine) spawnNode(commandParsed []string) string {

	if len(commandParsed) != 4 {
		message := "Usage: spawn <name> <ip> <type>"
		fmt.Println(message)
		return message
	}

	name := commandParsed[1]
	ip := commandParsed[2]
	nodeType := commandParsed[3]

	if nodeType != "computer" {
		message := fmt.Sprintf("Node type %s is not supported", nodeType)
		fmt.Println(message)
		return message
	}

	// check uniqueness of name and ip
	for _, node := range e.nodes {
		if node.Name == name {
			message := fmt.Sprintf("A node with the name %s already exists", name)
			fmt.Println(message)
			return message
		}
		if node.IP == ip {
			message := fmt.Sprintf("A node with the IP %s already exists", ip)
			fmt.Println(message)
			return message
		}
	}

	newNode := computer.New(name, ip, nodeType)
	e.nodes[ip] = newNode

	e.SaveNetwork()

	message := fmt.Sprintf("A %s node named: %s with IP: %s spawned successfully", nodeType, name, ip)
	fmt.Println(message)
	return message
}



