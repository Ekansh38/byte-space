package computer

import (
	"context"
	"fmt"
	"path"
	"strings"

	"byte-space/utils"
)

type Ls struct {
	id     string
	Kernel *Kernel
	proc   *Process
}

func (p *Ls) SetProcess(proc *Process) { p.proc = proc }
func (p *Ls) SetKernel(k *Kernel)      { p.Kernel = k }
func (p *Ls) ID() string               { return p.id }
func (p *Ls) HandleSignal(sig Signal)  {}

func (p *Ls) Run(ctx context.Context, returnStatus chan int, params []string) {
	if len(params) > 3 {
		p.Kernel.Write(p.proc, 1, []byte("Usage: ls <flag> <path>, no path for working dir\n"))
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
			dir = params[i]
		}
	}

	files, err := p.Kernel.ReadDir(p.proc, dir)
	if err != nil {
		p.Kernel.Write(p.proc, 1, []byte("\n"+err.Error()+"\n"))
		returnStatus <- utils.Error
		return
	}

	output := ""
	longestOwnerName := -1

	for _, file := range files {
		filePath := path.Join(dir, file.Name())
		meta, _ := p.Kernel.Stat(p.proc, filePath)
		if len(meta.Owner) > longestOwnerName {
			longestOwnerName = len(meta.Owner)
		}
	}

	for _, file := range files {
		if flag == "" {
			// REGULAR FORMAT
			if file.IsDir() {
				output += fmt.Sprintf("\033[94;1m%s\033[0m\n", file.Name())
			} else {
				output += fmt.Sprintf("\033[97m%s\033[0m\n", file.Name())
			}
		} else if flag == "-l" {
			// LONG FORMAT
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
			spacesToAdd := longestOwnerName - len(meta.Owner)
			for range spacesToAdd {
				owner += " "
			}
			output += fmt.Sprintf("%s%s  %s\n", perms, owner, name)
		}
	}

	if len(output) > 0 {
		output = output[:len(output)-1]
	}

	p.Kernel.Write(p.proc, 1, []byte("\n"+output+"\n"))
	returnStatus <- utils.Success
}


type Clear struct {
	id     string
	Kernel *Kernel
	proc   *Process
}

func (p *Clear) SetProcess(proc *Process) { p.proc = proc }
func (p *Clear) SetKernel(k *Kernel)      { p.Kernel = k }
func (p *Clear) ID() string               { return p.id }
func (p *Clear) HandleSignal(sig Signal)  {}

func (p *Clear) Run(ctx context.Context, returnStatus chan int, params []string) {
	if len(params) > 1 {
		p.Kernel.Write(p.proc, 1, []byte("Usage: clear\n"))
		returnStatus <- utils.Error
		return
	}
	p.Kernel.Write(p.proc, 1, []byte("\033[H\033[2J"))
	returnStatus <- utils.Success
}


type Cat struct {
	id     string
	Kernel *Kernel
	proc   *Process
}

func (p *Cat) SetProcess(proc *Process) { p.proc = proc }
func (p *Cat) SetKernel(k *Kernel)      { p.Kernel = k }
func (p *Cat) ID() string               { return p.id }
func (p *Cat) HandleSignal(sig Signal)  {}

func (p *Cat) Run(ctx context.Context, returnStatus chan int, params []string) {
	if len(params) != 2 {
		p.Kernel.Write(p.proc, 1, []byte("\nUsage: cat <path>\n"))
		returnStatus <- utils.Error
		return
	}

	content, err := p.Kernel.ReadFile(p.proc, params[1])
	if err != nil {
		message := "\nFailed to open file\n"
		if strings.HasSuffix(err.Error(), "no such file or directory") {
			message = "\ncat: cannot open: No such file or directory\n"
		}
		p.Kernel.Write(p.proc, 1, []byte(message))
		returnStatus <- utils.Error
		return
	}

	p.Kernel.Write(p.proc, 1, []byte("\n"+string(content)+"\n"))
	returnStatus <- utils.Success
}


type MkDir struct {
	id     string
	Kernel *Kernel
	proc   *Process
}

func (p *MkDir) SetProcess(proc *Process) { p.proc = proc }
func (p *MkDir) SetKernel(k *Kernel)      { p.Kernel = k }
func (p *MkDir) ID() string               { return p.id }
func (p *MkDir) HandleSignal(sig Signal)  {}

func (p *MkDir) Run(ctx context.Context, returnStatus chan int, params []string) {
	if len(params) != 2 {
		p.Kernel.Write(p.proc, 1, []byte("\nUsage: mkdir <path>\n"))
		returnStatus <- utils.Error
		return
	}

	if err := p.Kernel.MkDir(p.proc, params[1]); err != nil {
		p.Kernel.Write(p.proc, 1, []byte(fmt.Sprintf("\nFailed to create directory %s\n", err)))
		returnStatus <- utils.Error
		return
	}

	p.Kernel.Write(p.proc, 1, []byte("\n"))
	returnStatus <- utils.Success
}


type Touch struct {
	id     string
	Kernel *Kernel
	proc   *Process
}

func (p *Touch) SetProcess(proc *Process) { p.proc = proc }
func (p *Touch) SetKernel(k *Kernel)      { p.Kernel = k }
func (p *Touch) ID() string               { return p.id }
func (p *Touch) HandleSignal(sig Signal)  {}

func (p *Touch) Run(ctx context.Context, returnStatus chan int, params []string) {
	if len(params) != 2 {
		p.Kernel.Write(p.proc, 1, []byte("\nUsage: touch <path>\n"))
		returnStatus <- utils.Error
		return
	}

	if err := p.Kernel.CreateFile(p.proc, params[1]); err != nil {
		p.Kernel.Write(p.proc, 1, []byte(fmt.Sprintf("\nFailed to create file %s\n", err)))
		returnStatus <- utils.Error
		return
	}

	p.Kernel.Write(p.proc, 1, []byte("\n"))
	returnStatus <- utils.Success
}


type Chmod struct {
	id     string
	Kernel *Kernel
	proc   *Process
}

func (p *Chmod) SetProcess(proc *Process) { p.proc = proc }
func (p *Chmod) SetKernel(k *Kernel)      { p.Kernel = k }
func (p *Chmod) ID() string               { return p.id }
func (p *Chmod) HandleSignal(sig Signal)  {}

func (p *Chmod) Run(ctx context.Context, returnStatus chan int, params []string) {
	if len(params) != 3 {
		p.Kernel.Write(p.proc, 1, []byte("\nUsage: chmod <mode> <path>\n"))
		returnStatus <- utils.Error
		return
	}

	newOwner, newOther, err := parseChmodMode(params[1], p.Kernel, p.proc, params[2])
	if err != nil {
		p.Kernel.Write(p.proc, 1, []byte(fmt.Sprintf("\nchmod: invalid mode: %s\n", params[1])))
		returnStatus <- utils.Error
		return
	}

	if err := p.Kernel.Chmod(p.proc, params[2], newOwner, newOther); err != nil {
		p.Kernel.Write(p.proc, 1, []byte(fmt.Sprintf("\nchmod: %s\n", err.Error())))
		returnStatus <- utils.Error
		return
	}

	p.Kernel.Write(p.proc, 1, []byte("\n"))
	returnStatus <- utils.Success
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
			return "\033[93mr\033[0m"
		}
		return "\033[90m-\033[0m"
	}
	w := func(mode uint8) string {
		if mode&2 != 0 {
			return "\033[91mw\033[0m"
		}
		return "\033[90m-\033[0m"
	}
	x := func(mode uint8) string {
		if mode&1 != 0 {
			return "\033[92mx\033[0m"
		}
		return "\033[90m-\033[0m"
	}
	perms := r(owner) + w(owner) + x(owner) + r(other) + w(other) + x(other)
	if setuid {
		perms += "\033[95ms\033[0m "
	} else {
		perms += "  "
	}
	return perms
}


type Rm struct {
	id     string
	Kernel *Kernel
	proc   *Process
}

func (p *Rm) SetProcess(proc *Process) { p.proc = proc }
func (p *Rm) SetKernel(k *Kernel)      { p.Kernel = k }
func (p *Rm) ID() string               { return p.id }
func (p *Rm) HandleSignal(sig Signal)  {}

func (p *Rm) Run(ctx context.Context, returnStatus chan int, params []string) {
	if len(params) != 2 {
		p.Kernel.Write(p.proc, 1, []byte("\nUsage: rm <path>, equivalent to rm -rf\n"))
		returnStatus <- utils.Error
		return
	}

	if err := p.Kernel.RemoveAll(p.proc, params[1]); err != nil {
		p.Kernel.Write(p.proc, 1, []byte(fmt.Sprintf("\nFailed to delete file: %s\n", err)))
		returnStatus <- utils.Error
		return
	}

	p.Kernel.Write(p.proc, 1, []byte("\n"))
	returnStatus <- utils.Success
}
