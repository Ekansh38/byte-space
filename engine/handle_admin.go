package engine

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"

	"byte-space/computer"
	"byte-space/utils"
)

func (e *Engine) handleAdmin(conn net.Conn) {
	write := func(msg string, status int) {
		data, _ := json.Marshal(computer.NewIPCMessage(msg, status))
		conn.Write(append(data, '\n'))
	}

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		var msg AdminIPCMessage
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			continue
		}

		commandP := strings.Fields(msg.Value)

		if len(commandP) == 0 {
			message := "No command provided\n" // should be filtered out...
			write(message, utils.Error)
		}

		switch commandP[0] {
		case "spawn":
			write(e.spawnNode(commandP))
		case "list-nodes", "ls":
			write(e.listNodes(commandP))
		case "delete":
			write(e.deleteNode(commandP))
		case "reset-network":
			write(e.resetNetwork())
		case "exit":
			write("Exiting...", utils.Exit)
		default:
			write("not implemented\n", utils.Error)

		}
	}

	conn.Close()
}

func (e *Engine) listNodes(commandParsed []string) (string, int) {
	if len(commandParsed) > 2 {
		message := "Usage: list-nodes\n"
		return message, utils.Error
	}
	message := ""
	listFormat := "%s: %s: %s\n"
	if len(commandParsed) == 1 {
		for _, node := range e.nodes {
			message += fmt.Sprintf(listFormat, node.Type, node.Name, node.IP)
		}
	}
	if len(commandParsed) == 2 {
		nodeType := commandParsed[1]
		for _, node := range e.nodes {
			if node.Type == nodeType {
				message += fmt.Sprintf(listFormat, node.Type, node.Name, node.IP)
			}
		}
	}

	if len(e.nodes) == 0 {
		message = "No machines on network\n"
	}

	if len(e.nodes) != 0 {
		message = message[:len(message)-1]
	}

	return message+"\n", utils.Success
}

func (e *Engine) spawnNode(commandParsed []string) (string, int) {
	e.nodesMu.Lock()
	defer e.nodesMu.Unlock()

	if len(commandParsed) != 4 {
		message := "Usage: spawn <type> <name> <ip>\n"
		return message, utils.Error
	}

	name := commandParsed[2]
	ip := commandParsed[3]
	nodeType := commandParsed[1]

	if nodeType != "computer" {
		message := fmt.Sprintf("Node type %s is not supported\n", nodeType)
		return message, utils.Error
	}

	// check uniqueness of name and ip
	for _, node := range e.nodes {
		if node.Name == name {
			message := fmt.Sprintf("A node with the name %s already exists\n", name)
			return message, utils.Error
		}
		if node.IP == ip {
			message := fmt.Sprintf("A node with the IP %s already exists\n", ip)
			return message, utils.Error
		}
	}

	newNode := computer.NewComputer(name, ip, nodeType, e, e.EventBus)
	registerPrograms(newNode)
	e.nodes[ip] = newNode

	e.SaveNetwork()

	message := fmt.Sprintf("A %s node named: %s with IP: %s spawned successfully\nTip: adduser %s root <password>\n", nodeType, name, ip, name)

	return message, utils.Success
}

func (e *Engine) deleteNode(commandParsed []string) (string, int) {
	if len(commandParsed) != 2 {
		message := "Usage: delete <name>\n"
		return message, utils.Error
	}

	nodeName := commandParsed[1]
	message := ""
	status := utils.Success

	node, found := getNodeByName(e, nodeName)
	if !found {
		message = fmt.Sprintf("No node with the name %s found\n", nodeName)
		fmt.Println(message)
		return message, utils.Error
	}

	delete(e.nodes, node.IP)
	// delete on disk
	path := networkPath + "/nodes/" + node.Name
	err := os.RemoveAll(path)
	if err != nil {
		return fmt.Sprintf("Error deleting filesystem: %s\n", err), utils.Error
	}
	message = fmt.Sprintf("Node %s deleted successfully\n", nodeName)
	status = utils.Success

	e.SaveNetwork()

	fmt.Println(message)

	return message, status
}
