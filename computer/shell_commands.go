package computer

import (
	"context"
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

func (p *Ls) Run(ctx context.Context, returnStatus chan int, params []string) {
	if p.graphicsAPI == nil {
		returnStatus <- utils.Error
		return
	}
	if len(params) > 3 {
		p.graphicsAPI.Write("Usage: ls <flag> <path>, no path for working dir\n")
		returnStatus <- utils.Error
		return
	}

	dir := p.proc.CWD
	flag := ""
	params = params[1:]

	for i := range params {
		if strings.HasPrefix(params[i], "-") {
			flag = params[i]
		} else {
			dir = params[i] // sets dir
		}
	}

	files, err := p.Kernel.ReadDir(p.proc, dir)
	if err != nil {
		p.graphicsAPI.Write("\n" + err.Error() + "\n")
		returnStatus <- utils.Error
		return
	}

	output := ""
	longestOwnerName := -1
	for _, file := range files {
		filePath := path.Join(dir, file.Name())
		meta, _ := p.Kernel.Stat(p.proc, filePath)
		ownerLen := len(meta.Owner)
		if ownerLen > longestOwnerName {
			longestOwnerName = ownerLen
		}
	}

	for _, file := range files {
		if flag == "" {
			if file.IsDir() {
				output += fmt.Sprintf("\033[94;1m%s\033[0m\n", file.Name())
			} else {
				output += fmt.Sprintf("\033[97m%s\033[0m\n", file.Name())
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
			spacesToAdd := longestOwnerName - len(meta.Owner) // meta.Owner doesnt include the ANSI color values
			for range spacesToAdd {
				owner += " "
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

func (p *Clear) Run(ctx context.Context, returnStatus chan int, params []string) {
	if p.graphicsAPI == nil {
		returnStatus <- utils.Error
		return
	}
	if len(params) > 1 {
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

func (p *Cat) Run(ctx context.Context, returnStatus chan int, params []string) {
	if p.graphicsAPI == nil {
		returnStatus <- utils.Error
		return
	}
	if len(params) != 2 {
		p.graphicsAPI.Write("\nUsage: cat <path>\n")
		returnStatus <- utils.Error
		return
	}

	content, err := p.Kernel.ReadFile(p.proc, params[1])
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

func (p *MkDir) Run(ctx context.Context, returnStatus chan int, params []string) {
	if len(params) != 2 {
		p.graphicsAPI.Write("\nUsage: mkdir <path>\n")
		returnStatus <- utils.Error
		return
	}

	target := params[1]
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

func (p *Touch) Run(ctx context.Context, returnStatus chan int, params []string) {
	if len(params) != 2 {
		p.graphicsAPI.Write("\nUsage: touch <path>\n")
		returnStatus <- utils.Error
		return
	}

	target := params[1]
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

type Chmod struct {
	id          string
	graphicsAPI *GraphicsAPI
	ttyAPI      *TTYAPI
	Kernel      *Kernel
	proc        *Process
}

func (p *Chmod) SetTTyAPI(api *TTYAPI) {
	p.ttyAPI = api
}

func (p *Chmod) TTYAPI() *TTYAPI {
	return p.ttyAPI
}

func (p *Chmod) SetKernel(api *Kernel) {
	p.Kernel = api
}

func (p *Chmod) SetProcess(proc *Process) {
	p.proc = proc
}

func (p *Chmod) Run(ctx context.Context, returnStatus chan int, params []string) {
	if len(params) != 3 {
		p.graphicsAPI.Write("\nUsage: chmod <mode> <path>\n")
		returnStatus <- utils.Error
		return
	}

	modeStr := params[1]
	target := params[2]

	newOwner, newOther, err := parseChmodMode(modeStr, p.Kernel, p.proc, target)
	if err != nil {
		p.graphicsAPI.Write(fmt.Sprintf("\nchmod: invalid mode: %s\n", modeStr))
		returnStatus <- utils.Error
		return
	}

	if err := p.Kernel.Chmod(p.proc, target, newOwner, newOther); err != nil {
		p.graphicsAPI.Write(fmt.Sprintf("\nchmod: %s\n", err.Error()))
		returnStatus <- utils.Error
		return
	}

	p.graphicsAPI.Write("\n")
	returnStatus <- utils.Success
}

func (p *Chmod) ID() string {
	return p.id
}

func (p *Chmod) HandleSignal(sig Signal) {
}

func (p *Chmod) AddGraphicsAPI(api *GraphicsAPI) {
	p.graphicsAPI = api
}

func (p *Chmod) RemoveGraphicsAPI() {
	p.graphicsAPI = nil
}

func parseChmodMode(mode string, k *Kernel, proc *Process, target string) (uint8, uint8, error) {
	opIdx := strings.IndexAny(mode, "+-=")
	if opIdx < 0 {
		return 0, 0, fmt.Errorf("missing operator")
	}

	who := mode[:opIdx]
	op := mode[opIdx]
	permsStr := mode[opIdx+1:]

	if who == "" || who == "a" {
		who = "uo"
	}

	for _, w := range who {
		if w != 'u' && w != 'o' {
			return 0, 0, fmt.Errorf("invalid who: %c", w)
		}
	}

	var bits uint8
	for _, c := range permsStr {
		switch c {
		case 'r':
			bits |= 4
		case 'w':
			bits |= 2
		case 'x':
			bits |= 1
		default:
			return 0, 0, fmt.Errorf("invalid permission: %c", c)
		}
	}

	meta, ok := k.Stat(proc, target)
	if !ok {
		return 0, 0, fmt.Errorf("no such file or directory")
	}

	apply := func(current uint8) uint8 {
		switch op {
		case '+':
			return current | bits
		case '-':
			return current &^ bits
		case '=':
			return bits
		}
		return current
	}

	newOwner := meta.OwnerMode
	newOther := meta.OtherMode

	for _, w := range who {
		switch w {
		case 'u':
			newOwner = apply(meta.OwnerMode)
		case 'o':
			newOther = apply(meta.OtherMode)
		}
	}

	return newOwner, newOther, nil
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

type Rm struct {
	id          string
	graphicsAPI *GraphicsAPI
	ttyAPI      *TTYAPI
	Kernel      *Kernel
	proc        *Process
}

func (p *Rm) SetTTyAPI(api *TTYAPI) {
	p.ttyAPI = api
}

func (p *Rm) TTYAPI() *TTYAPI {
	return p.ttyAPI
}

func (p *Rm) SetKernel(api *Kernel) {
	p.Kernel = api
}

func (p *Rm) SetProcess(proc *Process) {
	p.proc = proc
}

func (p *Rm) Run(ctx context.Context, returnStatus chan int, params []string) {
	if p.graphicsAPI == nil {
		returnStatus <- utils.Error
		return
	}
	if len(params) != 2 {
		p.graphicsAPI.Write("\nUsage: rm <path>, equivalent to rm -rf\n")
		returnStatus <- utils.Error
		return
	}

	err := p.Kernel.RemoveAll(p.proc, params[1])
	if err != nil {
		message := fmt.Sprintf("\nFailed to delete file: %s\n", err)
		p.graphicsAPI.Write(message)
		returnStatus <- utils.Error
		return
	}

	p.graphicsAPI.Write("\n")
	returnStatus <- utils.Success
}

func (p *Rm) ID() string {
	return p.id
}

func (p *Rm) HandleSignal(sig Signal) {
}

func (p *Rm) AddGraphicsAPI(api *GraphicsAPI) {
	p.graphicsAPI = api
}

func (p *Rm) RemoveGraphicsAPI() {
	p.graphicsAPI = nil
}
