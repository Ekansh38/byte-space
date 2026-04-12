package admin

import (
	"byte-space/engine"
	"byte-space/utils"
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

var (
	successStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("2")).  // Green
	Bold(true)

	errorStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("1")).   // Red
	Bold(true)

	warningStyle = lipgloss.NewStyle().
	Foreground(lipgloss.Color("11")).  // Yellow
	Bold(true)
)


func displayResponse(msg *engine.EngineIPCMessage) {

	// FOR PROMPTING
	if strings.HasSuffix(msg.Result, ": ") {
		fmt.Printf("%s",msg.Result)
		return
	}
	if msg.Status == utils.SuccessDoNotDisplay {
		return
	}

	switch msg.Status {
	case utils.Success:
		fmt.Println(successStyle.Render("" + msg.Result))

	case utils.Error:
		fmt.Println(errorStyle.Render("" + msg.Result))

	case utils.Warning:
		fmt.Println(warningStyle.Render("" + msg.Result))

	case utils.Exit:
		fmt.Println(successStyle.Render("Goodbye!"))
	}
}

