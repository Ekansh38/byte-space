package computer

import (
	"fmt"
	"strings"

	//"os/user"
	"byte-space/utils"
)

type LoginProgram struct {
	done        chan struct{}
	id          string
	graphicsAPI *GraphicsAPI
	ttyAPI      *TTYAPI
	Kernel      *Kernel
	NetworkAPI  NetworkAPI
}

func (p *LoginProgram) SetProcess(proc *Process) {}

func (p *LoginProgram) SetTTyAPI(api *TTYAPI) {
	p.ttyAPI = api
}

func (p *LoginProgram) SetKernel(api *Kernel) {
	p.Kernel = api
}

func (p *LoginProgram) AddGraphicsAPI(api *GraphicsAPI) {
	p.graphicsAPI = api
}

func (p *LoginProgram) RemoveGraphicsAPI() {
	p.graphicsAPI = nil
}

func (p *LoginProgram) Run(returnStatus chan int, params []string) {
	// data
	ipAdress := ""
	username := ""
	password := ""

	p.done = make(chan struct{})
	if p.graphicsAPI == nil {
		returnStatus <- utils.Error
		return
	}

	computers := p.NetworkAPI.ListMachinesOnNetwork() // used a string builder because += is not efficient
	var choices strings.Builder

	for i := range computers {
		fmt.Fprintf(&choices, "%s: %s\n", computers[i].IP, computers[i].Name)
	}
	fmt.Fprintf(&choices, "\n\n\rIP ADDRESS: ")

	p.graphicsAPI.Write("\033[H\033[2J")
	p.graphicsAPI.Write(choices.String())

	var mainComputer *Computer

	for {
		value, status := p.ttyAPI.Read(p.done)
		switch status {
		case utils.Success:
			if value == "" {
				returnStatus <- utils.Error
				return
			}

			if ipAdress == "" {
				ipAdress = value
				ok := false
				if mainComputer, ok = p.NetworkAPI.GetNode(ipAdress); ok {
					p.graphicsAPI.Write(mainComputer.OS.GetIssue())
					p.graphicsAPI.Write("\nUSERNAME: ") // change to mainComputer.OS.GetUsernamePrompt or sm
				} else {
					// p.graphics.Write("Invalid IP address, no computer on network!")
					returnStatus <- utils.Error
					return
				}
			} else if username == "" {
				username = value
				p.ttyAPI.SetPasswdMode(true)
				p.graphicsAPI.Write("\nPASSWORD: ") // change to mainComputer.OS.GetPasswordPrompt or sm

			} else if password == "" {
				password = value
				// try login
				p.ttyAPI.SetPasswdMode(false)
				if mainComputer.OS.Login(username, password) == utils.Success {
					p.graphicsAPI.Write(mainComputer.OS.GetMotd())

					sessionStatus, sessionID := mainComputer.NewSession(username, p.ttyAPI.tty)
					if sessionStatus != utils.Success {
						returnStatus <- utils.Error
						return
					}

					p.ttyAPI.SetSession(mainComputer.sessions[sessionID])
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
