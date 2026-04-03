package engine

import (
	"fmt"
	//"os/user"
	"strings"

	"byte-space/computer"
	"byte-space/utils"
)

type LoginProgram struct {
	done        chan struct{}
	id          string
	tty         *TTY
	graphicsAPI *GraphicsAPI // if not foreground then nil
	Engine      *Engine
}

func (p *LoginProgram) AddGraphicsAPI(api *GraphicsAPI) {
	p.graphicsAPI = api
}

func (p *LoginProgram) RemoveGraphicsAPI() {
	p.graphicsAPI = nil
}

func (p *LoginProgram) Run(returnStatus chan int, params []string) {
	// data
	ip_address := ""
	username := ""
	password := ""

	p.done = make(chan struct{})
	if p.graphicsAPI == nil {
		returnStatus <- utils.Error
		return
	}

	computers := p.Engine.ListMachinesOnNetwork() // used a string builder because += is not efficient
	var choices strings.Builder

	for i := range computers {
		fmt.Fprintf(&choices, "%s: %s\n", computers[i].IP, computers[i].Name)
	}
	fmt.Fprintf(&choices, "\n\n\rIP ADDRESS: ")

	p.graphicsAPI.Write("\033[H\033[2J")
	p.graphicsAPI.Write(choices.String())

	var mainComputer *computer.Computer

	for {
		value, status := p.tty.Read(p, p.done)
		switch status {
		case utils.Success:
			if value == "" {
				returnStatus <- utils.Error
				return
			}

			if ip_address == "" {
				ip_address = value
				ok := false
				if mainComputer, ok = p.Engine.nodes[ip_address]; ok {
					p.graphicsAPI.Write(mainComputer.OS.GetIssue())
					p.graphicsAPI.Write("\nUSERNAME: ") // change to mainComputer.OS.GetUsernamePrompt or sm
				} else {
					// p.graphics.Write("Invalid IP address, no computer on network!")
					returnStatus <- utils.Error
					return
				}
			} else if username == "" {
				username = value
				p.tty.PasswdMode = true
				p.graphicsAPI.Write("\nPASSWORD: ") // change to mainComputer.OS.GetPasswordPrompt or sm

			} else if password == "" {
				password = value
				// try login
				p.tty.PasswdMode = false
				if mainComputer.OS.Login(username, password) == utils.Success {
					p.graphicsAPI.Write(mainComputer.OS.GetMotd())

					sessionStatus, sessionID := p.Engine.NewSession(mainComputer, username)
					if sessionStatus != utils.Success {
						returnStatus <- utils.Error
						return
					}

					p.tty.Session = p.Engine.sessions[sessionID]
					returnStatus <- utils.Success
					return

				} else {
					returnStatus <- utils.Error
					return
				}
			}

		case utils.Exit:
			returnStatus <- utils.Error
			return
		}
	}
}

func (p *LoginProgram) HandleSignal(sig Signal) {
	if sig == SIGINT {
		select {
		case <-p.done:
			// already closed, do nothing
		default:
			close(p.done)
		}
	}
}

func (p *LoginProgram) ID() string {
	return p.id
}
