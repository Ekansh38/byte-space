package computer

import (
	"fmt"
	"strings"

	"byte-space/utils"
)

type Adduser struct {
	id          string
	done        chan struct{}
	graphicsAPI *GraphicsAPI
	ttyAPI      *TTYAPI
	Kernel      *Kernel
	proc        *Process
}

func (p *Adduser) SetProcess(proc *Process) {
	p.proc = proc
}

func (p *Adduser) SetTTyAPI(api *TTYAPI) {
	p.ttyAPI = api
}

func (p *Adduser) TTYAPI() *TTYAPI {
	return p.ttyAPI
}

func (p *Adduser) SetKernel(api *Kernel) {
	p.Kernel = api
}

func (p *Adduser) AddGraphicsAPI(api *GraphicsAPI) {
	p.graphicsAPI = api
}

func (p *Adduser) RemoveGraphicsAPI() {
	p.graphicsAPI = nil
}

func (p *Adduser) ID() string {
	return p.id
}

func (p *Adduser) HandleSignal(sig Signal) {
	if sig == SIGINT {
		select {
		case <-p.done:
		default:
			p.ttyAPI.BuffClear()
			p.graphicsAPI.Write("\n(SIGINT), force quitting!\n")
			close(p.done)
		}
	}
}

func (p *Adduser) Run(returnStatus chan int, params []string) {
	if p.graphicsAPI == nil {
		returnStatus <- utils.Error
		return
	}
	if len(params) != 0 {
		p.graphicsAPI.Write("\nUsage: adduser\n")
		returnStatus <- utils.Error
		return
	}

	p.done = make(chan struct{})
	p.graphicsAPI.Write("\nEnter username: ")

	username := ""
	usernameRecorded := false
	password := ""
	passwordRecorded := false

	for {
		value, status := p.ttyAPI.Read(p.done)
		switch status {
		case utils.Success:
			if !usernameRecorded {
				if value == "" {
					p.graphicsAPI.Write("\that a horrible username, its empty child!\n")
					returnStatus <- utils.Error
					return
				}
				if !p.isUsernameUnique(value) {
					p.graphicsAPI.Write("\nUsername already exists, don't be so generic\n")
					returnStatus <- utils.Error
					return
				}
				username = value
				usernameRecorded = true
				p.graphicsAPI.Write("\nPassword: ")
				p.ttyAPI.SetPasswdMode(true)
			} else if !passwordRecorded {
				password = value
				passwordRecorded = true
				p.ttyAPI.SetPasswdMode(false)

				uid, ok := p.findUID()
				if !ok {
					p.graphicsAPI.Write("\nErr: could not create valid UID\n")
					returnStatus <- utils.Error
					return
				}

				msg := p.addUser(username, password, uid)
				p.graphicsAPI.Write("\n" + msg + "\n")
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
