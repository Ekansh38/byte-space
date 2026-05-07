package login

import (
	"context"
	"fmt"
	"strings"

	"byte-space/computer"
	"byte-space/utils"
)

type LoginProgram struct {
	id     string
	Kernel *computer.Kernel
	proc   *computer.Process
}

func New(pid int) computer.Program { return &LoginProgram{id: fmt.Sprintf("login-%d", pid)} }

func (p *LoginProgram) SetProcess(proc *computer.Process) { p.proc = proc }
func (p *LoginProgram) SetKernel(k *computer.Kernel)      { p.Kernel = k }
func (p *LoginProgram) ID() string                        { return p.id }

func (p *LoginProgram) HandleSignal(sig computer.Signal) {
	if sig == computer.SIGINT {
		p.proc.CtxCancel()
	}
}

func (p *LoginProgram) Run(ctx context.Context, returnStatus chan int, params []string) {
	if result, err := p.Kernel.Syscall(p.proc, computer.SYS_READ, "/etc/issue"); err == nil {
		issue, _ := result.([]byte)
		p.Kernel.Write(p.proc, 1, issue)
	}

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
				p.Kernel.Syscall(p.proc, computer.SYS_IOCTL, 0, computer.TIOCPASSWD, true)
				p.Kernel.Write(p.proc, 1, []byte("\r\nPASSWORD: "))
			} else if password == "" {
				password = value
				p.Kernel.Syscall(p.proc, computer.SYS_IOCTL, 0, computer.TIOCPASSWD, false)

				if p.checkCredentials(username, password) {
					if result, err := p.Kernel.Syscall(p.proc, computer.SYS_READ, "/etc/motd"); err == nil {
						motd, _ := result.([]byte)
						p.Kernel.Write(p.proc, 1, []byte(strings.ReplaceAll(string(motd), "[[USERNAME]]", username)))
					}

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

func (p *LoginProgram) checkCredentials(username, password string) bool {
	result, err := p.Kernel.Syscall(p.proc, computer.SYS_READ, "/etc/passwd")
	if err != nil {
		return false
	}
	data, _ := result.([]byte)
	for _, line := range strings.Split(string(data), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, ":")
		if len(fields) >= 2 && fields[0] == username && fields[1] == password {
			return true
		}
	}
	return false
}
