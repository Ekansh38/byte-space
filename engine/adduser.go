package engine

import (
	"fmt"
	"strings"

	"byte-space/computer"
	"byte-space/utils"

	"github.com/spf13/afero"
)

type Adduser struct {
	tty         *TTY
	id          string
	done        chan struct{}
	graphicsAPI *GraphicsAPI
}

func (p *Adduser) Run(returnStatus chan int, params []string) {
	if p.graphicsAPI != nil {
		if len(params) != 0 {
			p.graphicsAPI.Write("\nUsage: adduser\n")
			returnStatus <- utils.Error
			return
		}
		p.graphicsAPI.Write("\nEnter username: ")
	}

	p.done = make(chan struct{})

	username := ""
	usernameRecorded := false
	password := ""
	passwordRecorded := false

	p.tty.Canonical = true
	p.tty.PasswdMode = false

	if p.tty.Session.CurrentUser != "root" {
		p.graphicsAPI.Write("\nYou are not root!!!! I AM GROOT!!!\n")
		returnStatus <- utils.Error
		return
	}

	for {
		value, status := p.tty.Read(p, p.done)
		switch status {
		case utils.Success:

			if !usernameRecorded && value != "" {
				uniqueness := isUsernameUnique(p.tty.Session.Computer, value)
				if !uniqueness {
					p.graphicsAPI.Write("\nUsername already exists, dont be so generic\n")
					returnStatus <- utils.Error
					return
				}

				username = value
				usernameRecorded = true
				p.graphicsAPI.Write("\nPassword: ")
			} else if !passwordRecorded { // passwd can be blank if user is dumb asf
				password = value
				passwordRecorded = true

				status, UID := findUID(p.tty.Session.Computer)
				if status == utils.Error {
					p.graphicsAPI.Write("\nErr: could not create valid UID\n")
				}

				msg, _ := addUserToNode(p.tty.Session.Computer, username, password, UID)
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

func (p *Adduser) ID() string {
	return p.id
}

func (p *Adduser) HandleSignal(sig Signal) {
	if sig == SIGINT {
		p.graphicsAPI.Write("\nCLOSING PROGRAM, SIGINT\n")
		close(p.done)
	}
}

func (p *Adduser) AddGraphicsAPI(api *GraphicsAPI) {
	p.graphicsAPI = api
}

func (p *Adduser) RemoveGraphicsAPI() {
	p.graphicsAPI = nil
}

func findUID(node *computer.Computer) (int, int) {
	data, err := afero.ReadFile(node.Filesystem, "/etc/passwd")
	if err != nil {
		return utils.Error, 0
	}

	lines := strings.Split(string(data), "\n")
	userCount := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			userCount++
		}
	}

	nextUID := 1000 + userCount
	return utils.Success, nextUID
}

func isUsernameUnique(node *computer.Computer, username string) bool {
	data, err := afero.ReadFile(node.Filesystem, "/etc/passwd")
	if err != nil {
		return false
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			fields := strings.Split(line, ":")
			if len(fields) >= 1 && fields[0] == username {
				return false
			}
		}
	}

	return true
}

func addUserToNode(node *computer.Computer, username string, password string, uid int) (string, int) {
	existingData, err := afero.ReadFile(node.Filesystem, "/etc/passwd")
	if err != nil {
		existingData = []byte("") // File doesn't exist, start fresh
	}

	line := ""
	if username == "root" {
		line = fmt.Sprintf("%s:%s:%d:/root", username, password, uid)
	} else {
		line = fmt.Sprintf("%s:%s:%d:/home/%s", username, password, uid, username)
	}

	newContent := string(existingData) + line + "\n"

	// Write back
	err = afero.WriteFile(node.Filesystem, "/etc/passwd", []byte(newContent), 0o644)
	if err != nil {
		return fmt.Sprintf("Error writing to passwd: %s", err), utils.Error
	}

	return fmt.Sprintf("Successfully added %s", username), utils.Success
}
