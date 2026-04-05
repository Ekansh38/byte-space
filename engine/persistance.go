package engine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"byte-space/computer"
	"byte-space/utils"
)

const networkPath = "./data/networks/current/"

type NetworkConfig struct {
	Nodes []NodeConfig `json:"nodes"`
}

type NodeConfig struct {
	Name string `json:"name"`
	IP   string `json:"ip"`
	Type string `json:"type"`
}

func (e *Engine) SaveNetwork() error {
	err := os.MkdirAll(networkPath, 0o755)
	if err != nil {
		return err
	}

	config := NetworkConfig{
		Nodes: make([]NodeConfig, 0),
	}

	for _, node := range e.nodes {
		config.Nodes = append(config.Nodes, NodeConfig{
			Name: node.Name,
			IP:   node.IP,
			Type: node.Type,
		})
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	configPath := filepath.Join(networkPath, "network.json")
	return os.WriteFile(configPath, data, 0o644)
}

func (e *Engine) LoadNetwork() error {
	configPath := filepath.Join(networkPath, "network.json")

	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// No saved network
		return nil
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	var config NetworkConfig
	err = json.Unmarshal(data, &config)
	if err != nil {
		return err
	}

	for _, nodeConfig := range config.Nodes {
		node := computer.NewComputer(nodeConfig.Name, nodeConfig.IP, nodeConfig.Type, e)
		e.nodes[nodeConfig.IP] = node
	}

	fmt.Printf("Loaded %d nodes from disk\n", len(config.Nodes))
	return nil
}

func (e *Engine) resetNetwork() *computer.EngineIPCMessage {
	os.RemoveAll(networkPath)

	// Clear from memory
	e.nodes = make(map[string]*computer.Computer)

	return computer.NewIPCMessage("Network reset (memory + disk cleared)", utils.Success)
}
