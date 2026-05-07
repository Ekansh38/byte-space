package cmds

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"

	"byte-space/computer"
	"byte-space/utils"
)

type Ls struct {
	id     string
	Kernel *computer.Kernel
	proc   *computer.Process
}

// TEMP, JUST FOR NOW!!, later we will have an actual interpreted programming language. no need for these unique factories, we will just have 1 generic.
func NewLs(pid int) computer.Program    { return &Ls{id: fmt.Sprintf("ls-%d", pid)} }
func NewClear(pid int) computer.Program { return &Clear{id: fmt.Sprintf("clear-%d", pid)} }
func NewCat(pid int) computer.Program   { return &Cat{id: fmt.Sprintf("cat-%d", pid)} }
func NewMkDir(pid int) computer.Program { return &MkDir{id: fmt.Sprintf("mkdir-%d", pid)} }
func NewTouch(pid int) computer.Program { return &Touch{id: fmt.Sprintf("touch-%d", pid)} }
func NewChmod(pid int) computer.Program { return &Chmod{id: fmt.Sprintf("chmod-%d", pid)} }
func NewRm(pid int) computer.Program    { return &Rm{id: fmt.Sprintf("rm-%d", pid)} }

func (p *Ls) SetProcess(proc *computer.Process) { p.proc = proc }
func (p *Ls) SetKernel(k *computer.Kernel)      { p.Kernel = k }
func (p *Ls) ID() string               { return p.id }
func (p *Ls) HandleSignal(sig computer.Signal)  {}

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

	result, err := p.Kernel.Syscall(p.proc, computer.SYS_READDIR, dir)
	if err != nil {
		p.Kernel.Write(p.proc, 1, []byte("\n"+err.Error()+"\n"))
		returnStatus <- utils.Error
		return
	}
	files, _ := result.([]os.FileInfo)

	output := ""
	longestOwnerName := -1

	for _, file := range files {
		filePath := path.Join(dir, file.Name())
		statResult, _ := p.Kernel.Syscall(p.proc, computer.SYS_STAT, filePath)
		meta, _ := statResult.(computer.FileMetadata)
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
			statResult2, _ := p.Kernel.Syscall(p.proc, computer.SYS_STAT, filePath)
			meta, _ := statResult2.(computer.FileMetadata)
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
	Kernel *computer.Kernel
	proc   *computer.Process
}

func (p *Clear) SetProcess(proc *computer.Process) { p.proc = proc }
func (p *Clear) SetKernel(k *computer.Kernel)      { p.Kernel = k }
func (p *Clear) ID() string               { return p.id }
func (p *Clear) HandleSignal(sig computer.Signal)  {}

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
	Kernel *computer.Kernel
	proc   *computer.Process
}

func (p *Cat) SetProcess(proc *computer.Process) { p.proc = proc }
func (p *Cat) SetKernel(k *computer.Kernel)      { p.Kernel = k }
func (p *Cat) ID() string               { return p.id }
func (p *Cat) HandleSignal(sig computer.Signal)  {}

func (p *Cat) Run(ctx context.Context, returnStatus chan int, params []string) {
	if len(params) != 2 {
		p.Kernel.Write(p.proc, 1, []byte("\nUsage: cat <path>\n"))
		returnStatus <- utils.Error
		return
	}

	catResult, err := p.Kernel.Syscall(p.proc, computer.SYS_READ, params[1])
	if err != nil {
		message := "\nFailed to open file\n"
		if strings.HasSuffix(err.Error(), "no such file or directory") {
			message = "\ncat: cannot open: No such file or directory\n"
		}
		p.Kernel.Write(p.proc, 1, []byte(message))
		returnStatus <- utils.Error
		return
	}
	content, _ := catResult.([]byte)

	p.Kernel.Write(p.proc, 1, []byte("\n"+string(content)+"\n"))
	returnStatus <- utils.Success
}


type MkDir struct {
	id     string
	Kernel *computer.Kernel
	proc   *computer.Process
}

func (p *MkDir) SetProcess(proc *computer.Process) { p.proc = proc }
func (p *MkDir) SetKernel(k *computer.Kernel)      { p.Kernel = k }
func (p *MkDir) ID() string               { return p.id }
func (p *MkDir) HandleSignal(sig computer.Signal)  {}

func (p *MkDir) Run(ctx context.Context, returnStatus chan int, params []string) {
	if len(params) != 2 {
		p.Kernel.Write(p.proc, 1, []byte("\nUsage: mkdir <path>\n"))
		returnStatus <- utils.Error
		return
	}

	if _, err := p.Kernel.Syscall(p.proc, computer.SYS_MKDIR, params[1]); err != nil {
		p.Kernel.Write(p.proc, 1, []byte(fmt.Sprintf("\nFailed to create directory %s\n", err)))
		returnStatus <- utils.Error
		return
	}

	p.Kernel.Write(p.proc, 1, []byte("\n"))
	returnStatus <- utils.Success
}


type Touch struct {
	id     string
	Kernel *computer.Kernel
	proc   *computer.Process
}

func (p *Touch) SetProcess(proc *computer.Process) { p.proc = proc }
func (p *Touch) SetKernel(k *computer.Kernel)      { p.Kernel = k }
func (p *Touch) ID() string               { return p.id }
func (p *Touch) HandleSignal(sig computer.Signal)  {}

func (p *Touch) Run(ctx context.Context, returnStatus chan int, params []string) {
	if len(params) != 2 {
		p.Kernel.Write(p.proc, 1, []byte("\nUsage: touch <path>\n"))
		returnStatus <- utils.Error
		return
	}

	if _, err := p.Kernel.Syscall(p.proc, computer.SYS_CREATE, params[1]); err != nil {
		p.Kernel.Write(p.proc, 1, []byte(fmt.Sprintf("\nFailed to create file %s\n", err)))
		returnStatus <- utils.Error
		return
	}

	p.Kernel.Write(p.proc, 1, []byte("\n"))
	returnStatus <- utils.Success
}


type Chmod struct {
	id     string
	Kernel *computer.Kernel
	proc   *computer.Process
}

func (p *Chmod) SetProcess(proc *computer.Process) { p.proc = proc }
func (p *Chmod) SetKernel(k *computer.Kernel)      { p.Kernel = k }
func (p *Chmod) ID() string               { return p.id }
func (p *Chmod) HandleSignal(sig computer.Signal)  {}

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

	if _, err := p.Kernel.Syscall(p.proc, computer.SYS_CHMOD, params[2], newOwner, newOther); err != nil {
		p.Kernel.Write(p.proc, 1, []byte(fmt.Sprintf("\nchmod: %s\n", err.Error())))
		returnStatus <- utils.Error
		return
	}

	p.Kernel.Write(p.proc, 1, []byte("\n"))
	returnStatus <- utils.Success
}

func parseChmodMode(mode string, k *computer.Kernel, proc *computer.Process, target string) (uint8, uint8, error) {
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

	statResult, err := k.Syscall(proc, computer.SYS_STAT, target)
	if err != nil {
		return 0, 0, fmt.Errorf("no such file or directory")
	}
	meta, _ := statResult.(computer.FileMetadata)

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
	Kernel *computer.Kernel
	proc   *computer.Process
}

func (p *Rm) SetProcess(proc *computer.Process) { p.proc = proc }
func (p *Rm) SetKernel(k *computer.Kernel)      { p.Kernel = k }
func (p *Rm) ID() string               { return p.id }
func (p *Rm) HandleSignal(sig computer.Signal)  {}

func (p *Rm) Run(ctx context.Context, returnStatus chan int, params []string) {
	if len(params) != 2 {
		p.Kernel.Write(p.proc, 1, []byte("\nUsage: rm <path>, equivalent to rm -rf\n"))
		returnStatus <- utils.Error
		return
	}

	if _, err := p.Kernel.Syscall(p.proc, computer.SYS_REMOVE, params[1]); err != nil {
		p.Kernel.Write(p.proc, 1, []byte(fmt.Sprintf("\nFailed to delete file: %s\n", err)))
		returnStatus <- utils.Error
		return
	}

	p.Kernel.Write(p.proc, 1, []byte("\n"))
	returnStatus <- utils.Success
}







