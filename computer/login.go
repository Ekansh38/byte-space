package computer

import (
	"context"
	"strings"

	"byte-space/utils"
)

type LoginProgram struct {
	id     string
	Kernel *Kernel
	proc   *Process
}

func (p *LoginProgram) SetProcess(proc *Process) { p.proc = proc }
func (p *LoginProgram) SetKernel(k *Kernel)      { p.Kernel = k }
func (p *LoginProgram) ID() string               { return p.id }

func (p *LoginProgram) HandleSignal(sig Signal) {
	if sig == SIGINT {
		p.proc.CtxCancel()
	}
}

func (p *LoginProgram) Run(ctx context.Context, returnStatus chan int, params []string) {
	thisComputer := p.Kernel.computer

	p.Kernel.Write(p.proc, 1, []byte(thisComputer.OS.GetIssue()))
	p.Kernel.Write(p.proc, 1, []byte("\r\nUSERNAME: "))

	username := ""
	password := ""

	for {
		value, status := p.Kernel.Read(p.proc, 0, ctx)
		switch status {
		case utils.Success:
			if value == "" {
				returnStatus <- utils.Error
				return
			}

			if username == "" {
				username = value
				p.Kernel.Ioctl(p.proc, 0, TIOCPASSWD, true)
				p.Kernel.Write(p.proc, 1, []byte("\r\nPASSWORD: "))

			} else if password == "" {
				password = value
				p.Kernel.Ioctl(p.proc, 0, TIOCPASSWD, false)

				if thisComputer.OS.Login(username, password) == utils.Success {
					p.Kernel.Write(p.proc, 1, []byte(strings.ReplaceAll(thisComputer.OS.GetMotd(), "[[USERNAME]]", username)))

					sessionStatus, _ := p.Kernel.NewSession(p.proc, username)
					if sessionStatus != utils.Success {
						returnStatus <- utils.Error
						return
					}

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
