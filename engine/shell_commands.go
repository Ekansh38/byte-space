package engine

import (
	"fmt"
	"path"
	"strings"

	"byte-space/utils"

	"github.com/spf13/afero"
)

func expandPath(target string, tty *TTY) string {
	// expand ~ to home directory
	target = path.Clean(target)
	if strings.HasPrefix(target, "~") {
		if tty.Session.CurrentUser == "root" {
			target = path.Join("/root/", target[1:])
		} else {
			target = path.Join("/home/", fmt.Sprintf("%s/", tty.Session.CurrentUser), target[1:])
		}
	}

	return target
}

type Ls struct {
	tty         *TTY
	id          string
	graphicsAPI *GraphicsAPI
}

func (p *Ls) Run(returnStatus chan int, params []string) {
	if p.graphicsAPI != nil {
		if len(params) > 1 {
			p.graphicsAPI.Write("Usage: ls <path>, no path for working dir\n")
			returnStatus <- utils.Error
			return
		}

		dir := expandPath(p.tty.Session.WorkingDir, p.tty)
		if len(params) == 1 {
			dir = params[0]

			if !strings.HasPrefix(dir, "/") {
				dir = path.Join(p.tty.Session.WorkingDir, dir)
			}

			dir = path.Clean(dir)
		}

		files, err := afero.ReadDir(p.tty.Session.Computer.Filesystem, dir)
		if err != nil {
			message := "Invalid directory\n"
			p.graphicsAPI.Write(message)
		}

		output := ""
		for _, file := range files {
			if file.IsDir() {
				output += fmt.Sprintf("\033[34m%s\033[0m\n", file.Name())
			} else {
				output += fmt.Sprintf("%s\n", file.Name())
			}
		}

		// remove extra trailing newline
		if len(output) > 0 {
			output = output[:len(output)-1]
		}

		output += "\n"

		p.graphicsAPI.Write("\n"+output)
		returnStatus <- utils.Success
	}
}

func (p *Ls) ID() string {
	return p.id
}

func (p *Ls) HandleSignal(sig Signal) {
}

func (p *Ls) AddGraphicsAPI(api *GraphicsAPI) {
	p.graphicsAPI = api
}

func (p *Ls) RemoveGraphicsAPI() {
	p.graphicsAPI = nil
}

// CLEAR COMMANDD

type Clear struct {
	tty         *TTY
	id          string
	graphicsAPI *GraphicsAPI
}

func (p *Clear) Run(returnStatus chan int, params []string) {
	if p.graphicsAPI != nil {
		if len(params) > 0 {
			p.graphicsAPI.Write("Usage: clear\n")
			returnStatus <- utils.Error
			return
		}
		returnStatus <- utils.Success
		p.graphicsAPI.Write("\033[H\033[2J")
		return
	}
}

func (p *Clear) ID() string {
	return p.id
}

func (p *Clear) HandleSignal(sig Signal) {
}

func (p *Clear) AddGraphicsAPI(api *GraphicsAPI) {
	p.graphicsAPI = api
}

func (p *Clear) RemoveGraphicsAPI() {
	p.graphicsAPI = nil
}

// CAY COMMAND

type Cat struct {
	tty         *TTY
	id          string
	graphicsAPI *GraphicsAPI
}

func (p *Cat) Run(returnStatus chan int, params []string) {
	if p.graphicsAPI != nil {
		if len(params) != 1 {
			p.graphicsAPI.Write("\nUsage: cat <path>\n")
			returnStatus <- utils.Error
			return
		}
		target := expandPath(params[0], p.tty)
		if !strings.HasPrefix(target, "/") {
			target = path.Join(p.tty.Session.WorkingDir, target)
		}

		target = expandPath(target, p.tty)

		file, err := p.tty.Session.Computer.Filesystem.Open(target)
		if err != nil {
			message := "\nFailed to open file\n"
			if strings.HasSuffix(err.Error(), "no such file or directory") {
				message = "cat: cannot open: No such file or directory\n"
			}
			p.graphicsAPI.Write(message)
			returnStatus <- utils.Error
			return
		}
		defer file.Close()

		content, err := afero.ReadAll(file)
		if err != nil {
			message := "\nFailed to read file\n"
			p.graphicsAPI.Write(message)
			returnStatus <- utils.Error
			return
		}

		returnStatus <- utils.Success
		p.graphicsAPI.Write("\n"+string(content)+"\n")
		return
	}
}

func (p *Cat) ID() string {
	return p.id
}

func (p *Cat) HandleSignal(sig Signal) {
}

func (p *Cat) AddGraphicsAPI(api *GraphicsAPI) {
	p.graphicsAPI = api
}

func (p *Cat) RemoveGraphicsAPI() {
	p.graphicsAPI = nil
}
