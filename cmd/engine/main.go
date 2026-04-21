package main

import (
	"fmt"
	"log"
	"os"

	"byte-space/engine"
	"byte-space/tui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	f, _ := os.OpenFile("debug.log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	log.SetOutput(f)

	// Create engine
	eng := engine.NewEngine()

	events := eng.EventBus.Subscribe()

	go eng.Run()

	p := tea.NewProgram(
		tui.NewModel(events, eng),
		tea.WithAltScreen(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	// add like a shutdown thing for all the filesystems to close nicely TODO
	// use signal.Notify os.Interrupt

}
