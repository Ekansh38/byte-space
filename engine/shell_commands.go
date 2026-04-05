package engine

import (
	"fmt"
	"strings"

	"byte-space/utils"
)

type Ls struct {
	id          string
	graphicsAPI *GraphicsAPI
	ttyAPI      *TTYAPI
	Kernel       *Kernel
}

func (p *Ls) SetTTyAPI(api *TTYAPI) {
	p.ttyAPI = api
}

func (p *Ls) SetKernel(api *Kernel) {
	p.Kernel = api
}

func (p *Ls) Owner() string {
	return "root"
}

func (p *Ls) Setuid() bool {
	return false
}

func (p *Ls) Run(returnStatus chan int, params []string) {
	if p.graphicsAPI == nil {
		returnStatus <- utils.Error
		return
	}
	if len(params) > 1 {
		p.graphicsAPI.Write("Usage: ls <path>, no path for working dir\n")
		returnStatus <- utils.Error
		return
	}

	dir := p.Kernel.GetWorkingDir()
	if len(params) == 1 {
		dir = params[0]
	}

	files, err := p.Kernel.ReadDir(dir)
	if err != nil {
		p.graphicsAPI.Write("\n" + err.Error() + "\n")
		returnStatus <- utils.Error
		return
	}

	output := ""
	for _, file := range files {
		if file.IsDir() {
			output += fmt.Sprintf("\033[34m%s\033[0m\n", file.Name())
		} else {
			output += fmt.Sprintf("%s\n", file.Name())
		}
	}

	if len(output) > 0 {
		output = output[:len(output)-1]
	}

	p.graphicsAPI.Write("\n" + output + "\n")
	returnStatus <- utils.Success
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
	id          string
	graphicsAPI *GraphicsAPI
	ttyAPI      *TTYAPI
	Kernel       *Kernel
}

func (p *Clear) SetTTyAPI(api *TTYAPI) {
	p.ttyAPI = api
}

func (p *Clear) SetKernel(api *Kernel) {
	p.Kernel = api
}

func (p *Clear) Owner() string {
	return "root"
}

func (p *Clear) Setuid() bool {
	return false
}

func (p *Clear) Run(returnStatus chan int, params []string) {
	if p.graphicsAPI == nil {
		returnStatus <- utils.Error
		return
	}
	if len(params) > 0 {
		p.graphicsAPI.Write("Usage: clear\n")
		returnStatus <- utils.Error
		return
	}
	p.graphicsAPI.Write("\033[H\033[2J")
	returnStatus <- utils.Success
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

type Cat struct {
	id          string
	graphicsAPI *GraphicsAPI
	ttyAPI      *TTYAPI
	Kernel       *Kernel
}

func (p *Cat) SetTTyAPI(api *TTYAPI) {
	p.ttyAPI = api
}

func (p *Cat) SetKernel(api *Kernel) {
	p.Kernel = api
}

func (p *Cat) Owner() string {
	return "root"
}

func (p *Cat) Setuid() bool {
	return false
}

func (p *Cat) Run(returnStatus chan int, params []string) {
	if p.graphicsAPI == nil {
		returnStatus <- utils.Error
		return
	}
	if len(params) != 1 {
		p.graphicsAPI.Write("\nUsage: cat <path>\n")
		returnStatus <- utils.Error
		return
	}

	content, err := p.Kernel.ReadFile(params[0])
	if err != nil {
		message := "\nFailed to open file\n"
		if strings.HasSuffix(err.Error(), "no such file or directory") {
			message = "\ncat: cannot open: No such file or directory\n"
		}
		p.graphicsAPI.Write(message)
		returnStatus <- utils.Error
		return
	}

	p.graphicsAPI.Write("\n" + string(content) + "\n")
	returnStatus <- utils.Success
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
