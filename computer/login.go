package computer

import (
	"byte-space/utils"
	"context"
)

type LoginProgram struct {
	id          string
	graphicsAPI *GraphicsAPI
	ttyAPI      *TTYAPI
	Kernel      *Kernel
	proc        *Process
}

func (p *LoginProgram) SetProcess(proc *Process) {
	p.proc = proc
}

func (p *LoginProgram) TTYAPI() *TTYAPI {
	return p.ttyAPI
}

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

func (p *LoginProgram) Run(ctx context.Context,returnStatus chan int, params []string) {
	if p.graphicsAPI == nil {
		returnStatus <- utils.Error
		return
	}

	thisComputer := p.Kernel.computer

	p.graphicsAPI.Write(thisComputer.OS.GetIssue())
	p.graphicsAPI.Write("\r\nUSERNAME: ")

	username := ""
	password := ""

	for {
		value, status := p.ttyAPI.Read(ctx)
		switch status {
		case utils.Success:
			if value == "" {
				returnStatus <- utils.Error
				return
			}

			if username == "" {
				username = value
				p.ttyAPI.SetPasswdMode(true)
				p.graphicsAPI.Write("\r\nPASSWORD: ")

			} else if password == "" {
				password = value
				p.ttyAPI.SetPasswdMode(false)

				if thisComputer.OS.Login(username, password) == utils.Success {
					p.graphicsAPI.Write(thisComputer.OS.GetMotd() + username)

					sessionStatus, sessionID := thisComputer.NewSession(username, p.ttyAPI.tty)
					if sessionStatus != utils.Success {
						returnStatus <- utils.Error
						return
					}

					p.ttyAPI.SetSession(thisComputer.sessions[sessionID])
					returnStatus <- utils.Success
					return
				}

				returnStatus <- utils.Error
				return
			}

		case utils.Exit:
			returnStatus <- utils.Error
			return
		}
	}
}

func (p *LoginProgram) HandleSignal(sig Signal) {
	if sig == SIGINT {
			// close this program, this will close any child processes which is neat, cuz they will be children.
			// propagation
			// i think u can call ctxCancel even if its already canceled so it should be okay.
			p.proc.ctxCancel()
	}
}

func (p *LoginProgram) ID() string {
	return p.id
}
