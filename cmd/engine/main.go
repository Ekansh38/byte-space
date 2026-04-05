package main

import (
	"fmt"
	"os"

	"byte-space/engine"
	"byte-space/tui"
	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	// Create engine
	eng := engine.NewEngine()

	events := eng.EventBus.Subscribe()

	go eng.Run()

	p := tea.NewProgram(
		tui.NewModel(events, eng),
		tea.WithAltScreen(),
	)

	eng.RunAdminCommand("spawn computer ekanshgPC 192.188.0.0")

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}
}
