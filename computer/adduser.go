package computer

import (
	"context"
	"fmt"
	"strings"

	"byte-space/utils"
)

type Adduser struct {
	id     string
	Kernel *Kernel
	proc   *Process
}

func (p *Adduser) SetProcess(proc *Process) { p.proc = proc }
func (p *Adduser) SetKernel(k *Kernel)      { p.Kernel = k }
func (p *Adduser) ID() string               { return p.id }

func (p *Adduser) HandleSignal(sig Signal) {
	if sig == SIGINT {
		p.Kernel.Ioctl(p.proc, 0, TIOCBUFFCLEAR, nil)
		p.Kernel.Write(p.proc, 1, []byte("\n(SIGINT), force quitting!\n"))
		p.proc.ctxCancel()
	}
}

func (p *Adduser) Run(ctx context.Context, returnStatus chan int, params []string) {
	if len(params) != 1 {
		p.Kernel.Write(p.proc, 1, []byte("\nUsage: adduser\n"))
		returnStatus <- utils.Error
		return
	}

	p.Kernel.Write(p.proc, 1, []byte("\nEnter username: "))

	username := ""
	usernameRecorded := false
	password := ""
	passwordRecorded := false

	for {
		value, status := p.Kernel.Read(p.proc, 0, ctx)
		switch status {
		case utils.Success:
			if !usernameRecorded {
				if value == "" {
					p.Kernel.Write(p.proc, 1, []byte("\that a horrible username, its empty child!\n"))
					returnStatus <- utils.Error
					return
				}
				if !p.isUsernameUnique(value) {
					p.Kernel.Write(p.proc, 1, []byte("\nUsername already exists, don't be so generic\n"))
					returnStatus <- utils.Error
					return
				}
				username = value
				usernameRecorded = true
				p.Kernel.Write(p.proc, 1, []byte("\nPassword: "))
				p.Kernel.Ioctl(p.proc, 0, TIOCPASSWD, true)
			} else if !passwordRecorded {
				password = value
				passwordRecorded = true
				p.Kernel.Ioctl(p.proc, 0, TIOCPASSWD, false)

				uid, ok := p.findUID()
				if !ok {
					p.Kernel.Write(p.proc, 1, []byte("\nErr: could not create valid UID\n"))
					returnStatus <- utils.Error
					return
				}

				msg := p.addUser(username, password, uid)
				p.Kernel.Write(p.proc, 1, []byte("\n"+msg+"\n"))
				returnStatus <- utils.Success
				return
			}

		case utils.Exit:
			returnStatus <- utils.Error
			return
		}
	}
}

func (p *Adduser) findUID() (int, bool) {
	data, err := p.Kernel.ReadFile(p.proc, "/etc/passwd")
	if err != nil {
		return 0, false
	}
	userCount := 0
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) != "" {
			userCount++
		}
	}
	return 1000 + userCount, true
}

func (p *Adduser) isUsernameUnique(username string) bool {
	data, err := p.Kernel.ReadFile(p.proc, "/etc/passwd")
	if err != nil {
		return false
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, ":")
		if len(fields) >= 1 && fields[0] == username {
			return false
		}
	}
	return true
}

func (p *Adduser) addUser(username, password string, uid int) string {
	existing, err := p.Kernel.ReadFile(p.proc, "/etc/passwd")
	if err != nil {
		existing = []byte("")
	}

	homedir := "/home/" + username
	if username == "root" {
		homedir = "/root"
	}

	line := fmt.Sprintf("%s:%s:%d:%s\n", username, password, uid, homedir)
	if err := p.Kernel.WriteFile(p.proc, "/etc/passwd", append(existing, []byte(line)...)); err != nil {
		return fmt.Sprintf("Error writing to passwd: %s", err)
	}
	return fmt.Sprintf("Successfully added %s", username)
}
