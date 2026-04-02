package main

import (
	//"fmt"
	//"os"

	//tea "github.com/charmbracelet/bubbletea"
	"byte-space/engine"
	//"byte-space/tui"
)

func main() {
	// Create engine
	eng := engine.NewEngine()

	// Subscribe to events
	//events := eng.EventBus.Subscribe()

	// Start engine in background
	eng.Run()

	// Start TUI
//	p := tea.NewProgram(
//		tui.NewModel(events),
//		tea.WithAltScreen(),
//	)

//	if _, err := p.Run(); err != nil {
//		fmt.Printf("Error: %v\n", err)
//		os.Exit(1)
//	}
}
