package engine

import (
	"log"

	"sync"
	"byte-space/computer"
)

type Engine struct {
	nodes    map[string]*computer.Computer // by IP address // RACE CONDITIONS ON THIS!@!!!!!
	nodesMu sync.RWMutex
	
	EventBus *computer.EventBus            // to transmit events to tui
}

// API

func (e *Engine) ListMachinesOnNetwork() []computer.Computer {
	var computers []computer.Computer
	e.nodesMu.RLock()
	for _, computer := range e.nodes {
		computers = append(computers, *computer)
	}
	defer e.nodesMu.RUnlock()
	return computers
}

func (e *Engine) GetFsMetaData(computerName string) map[string]computer.FileMetadata {
	e.nodesMu.RLock()
	for _, c := range e.nodes {
		if c.Name == computerName {
			return c.FsMetaData
		}
	}
	defer e.nodesMu.RUnlock()
	return nil
}

func NewEngine() *Engine {
	e := &Engine{nodes: make(map[string]*computer.Computer), EventBus: computer.NewEventBus()}

	// load network
	err := e.LoadNetwork()
	if err != nil {
		log.Printf("Warning: Could not load network: %s", err)
	}

	return e
}

func (e *Engine) GetNode(ip_address string) (node *computer.Computer, ok bool) {
	e.nodesMu.RLock()
	node, ok = e.nodes[ip_address]
	defer e.nodesMu.RUnlock()
	return node, ok
}
