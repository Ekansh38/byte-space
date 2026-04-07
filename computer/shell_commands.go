package computer

import (
	"fmt"
	"path"
	"strings"

	"byte-space/utils"
)

type Ls struct {
	id          string
	graphicsAPI *GraphicsAPI
	ttyAPI      *TTYAPI
	Kernel      *Kernel
	proc        *Process
}

func (p *Ls) SetTTyAPI(api *TTYAPI) {
	p.ttyAPI = api
}

func (p *Ls) TTYAPI() *TTYAPI {
	return p.ttyAPI
}

func (p *Ls) SetKernel(api *Kernel) {
	p.Kernel = api
}

func (p *Ls) SetProcess(proc *Process) {
	p.proc = proc
}

func (p *Ls) Run(returnStatus chan int, params []string) {
	if p.graphicsAPI == nil {
		returnStatus <- utils.Error
		return
	}
	if len(params) > 2 {
		p.graphicsAPI.Write("Usage: ls <flag> <path>, no path for working dir\n")
		returnStatus <- utils.Error
		return
	}

	dir := p.proc.CWD
	flag := ""

	for i := range params {
		if strings.HasPrefix(params[i], "-") {
			flag = params[i]
		} else {
			dir = params[i]
		}
	}

	files, err := p.Kernel.ReadDir(p.proc, dir)
	if err != nil {
		p.graphicsAPI.Write("\n" + err.Error() + "\n")
		returnStatus <- utils.Error
		return
	}

	output := ""
	for _, file := range files {
		if flag == "" {
			if file.IsDir() {
				output += fmt.Sprintf("\033[34m%s\033[0m\n", file.Name())
			} else {
				output += fmt.Sprintf("%s\n", file.Name())
			}
		} else if flag == "-l" {
			filePath := path.Join(dir, file.Name())
			meta, _ := p.Kernel.Stat(p.proc, filePath)
			perms := formatMode(meta.OwnerMode, meta.OtherMode, meta.Setuid)
			owner := fmt.Sprintf("\033[96m%s\033[0m", meta.Owner)
			var name string
			if file.IsDir() {
				name = fmt.Sprintf("\033[94;1m%s\033[0m", file.Name())
			} else {
				name = fmt.Sprintf("\033[97m%s\033[0m", file.Name())
			}
			output += fmt.Sprintf("%s%s  %s\n", perms, owner, name)
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
	Kernel      *Kernel
	proc        *Process
}

func (p *Clear) SetTTyAPI(api *TTYAPI) {
	p.ttyAPI = api
}

func (p *Clear) TTYAPI() *TTYAPI {
	return p.ttyAPI
}

func (p *Clear) SetKernel(api *Kernel) {
	p.Kernel = api
}

func (p *Clear) SetProcess(proc *Process) {
	p.proc = proc
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
	Kernel      *Kernel
	proc        *Process
}

func (p *Cat) SetTTyAPI(api *TTYAPI) {
	p.ttyAPI = api
}

func (p *Cat) TTYAPI() *TTYAPI {
	return p.ttyAPI
}

func (p *Cat) SetKernel(api *Kernel) {
	p.Kernel = api
}

func (p *Cat) SetProcess(proc *Process) {
	p.proc = proc
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

	content, err := p.Kernel.ReadFile(p.proc, params[0])
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

type MkDir struct {
	id          string
	graphicsAPI *GraphicsAPI
	ttyAPI      *TTYAPI
	Kernel      *Kernel
	proc        *Process
}

func (p *MkDir) SetTTyAPI(api *TTYAPI) {
	p.ttyAPI = api
}

func (p *MkDir) TTYAPI() *TTYAPI {
	return p.ttyAPI
}

func (p *MkDir) SetKernel(api *Kernel) {
	p.Kernel = api
}

func (p *MkDir) SetProcess(proc *Process) {
	p.proc = proc
}

func (p *MkDir) Run(returnStatus chan int, params []string) {
	if len(params) != 1 {
		p.graphicsAPI.Write("\nUsage: mkdir <path>\n")
		returnStatus <- utils.Error
		return
	}

	target := params[0]
	err := p.Kernel.MkDir(p.proc, target)
	if err != nil {
		message := fmt.Sprintf("\nFailed to create directory %s\n", err)
		if p.graphicsAPI != nil {
			p.graphicsAPI.Write(message)
		}
		returnStatus <- utils.Error
		return
	}

	p.graphicsAPI.Write("\n")
	returnStatus <- utils.Success
}

func (p *MkDir) ID() string {
	return p.id
}

func (p *MkDir) HandleSignal(sig Signal) {
}

func (p *MkDir) AddGraphicsAPI(api *GraphicsAPI) {
	p.graphicsAPI = api
}

func (p *MkDir) RemoveGraphicsAPI() {
	p.graphicsAPI = nil
}

type Touch struct {
	id          string
	graphicsAPI *GraphicsAPI
	ttyAPI      *TTYAPI
	Kernel      *Kernel
	proc        *Process
}

func (p *Touch) SetTTyAPI(api *TTYAPI) {
	p.ttyAPI = api
}

func (p *Touch) TTYAPI() *TTYAPI {
	return p.ttyAPI
}

func (p *Touch) SetKernel(api *Kernel) {
	p.Kernel = api
}

func (p *Touch) SetProcess(proc *Process) {
	p.proc = proc
}

func (p *Touch) Run(returnStatus chan int, params []string) {
	if len(params) != 1 {
		p.graphicsAPI.Write("\nUsage: touch <path>\n")
		returnStatus <- utils.Error
		return
	}

	target := params[0]
	err := p.Kernel.CreateFile(p.proc, target)
	if err != nil {
		message := fmt.Sprintf("\nFailed to create directory %s\n", err)
		if p.graphicsAPI != nil {
			p.graphicsAPI.Write(message)
		}
		returnStatus <- utils.Error
		return
	}

	p.graphicsAPI.Write("\n")
	returnStatus <- utils.Success
}

func (p *Touch) ID() string {
	return p.id
}

func (p *Touch) HandleSignal(sig Signal) {
}

func (p *Touch) AddGraphicsAPI(api *GraphicsAPI) {
	p.graphicsAPI = api
}

func (p *Touch) RemoveGraphicsAPI() {
	p.graphicsAPI = nil
}

func formatMode(owner, other uint8, setuid bool) string {
	r := func(mode uint8) string {
		if mode&4 != 0 {
			return "\033[93mr\033[0m" // bright yellow
		}
		return "\033[90m-\033[0m" // dark gray
	}
	w := func(mode uint8) string {
		if mode&2 != 0 {
			return "\033[91mw\033[0m" // bright red
		}
		return "\033[90m-\033[0m"
	}
	x := func(mode uint8) string {
		if mode&1 != 0 {
			return "\033[92mx\033[0m" // bright green
		}
		return "\033[90m-\033[0m"
	}
	perms := r(owner) + w(owner) + x(owner) + r(other) + w(other) + x(other)
	if setuid {
		perms += "\033[95ms\033[0m " // bright magenta
	} else {
		perms += "  "
	}
	return perms
}
