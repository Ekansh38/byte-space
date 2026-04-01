
package engine

import (
	"fmt"
	"path"
	"strings"

	"byte-space/utils"

	"github.com/spf13/afero"
)

type Ls struct {
	tty         *TTY
	id          string
	graphicsAPI *GraphicsAPI
}

func (p *Ls) Run(returnStatus chan int, params []string) {
	if p.graphicsAPI != nil {
		if len(params) > 1{
			p.graphicsAPI.Write("Usage: ls <path>, no path for working dir\n")
			returnStatus <- utils.Error
			return
		}

		dir := p.tty.Session.WorkingDir
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

		p.graphicsAPI.Write(output)
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

type Clear struct {
	tty         *TTY
	id          string
	graphicsAPI *GraphicsAPI
}

func (p *Clear) Run(returnStatus chan int, params []string) {
	if p.graphicsAPI != nil {
		if len(params) > 0{
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
