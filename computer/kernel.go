package computer

import (
	"fmt"
	"os"
	"path"
	"strings"

	"byte-space/utils"
)

type Kernel struct {
	computer *Computer
	programs map[string]func(int) Program // path to the factory, which can later change if I implement a language
	// later the factory can be just 1 function, and instead of a map, it can just read that file path and do the language stuff, check for shebang and all that.
	// rn we still need a map.

	pids  int
	procs map[int]*Process // pid to the running process instance (all processes on this computer, GLOBALY)
}

type ExecOpts struct {
	Background bool
	PGID       int
}

// Exec looks up the binary from the path then it creates a Process with correct EUID
func (k *Kernel) Exec(session *Session, binPath string, args []string, opts *ExecOpts) error {
	factory, ok := k.programs[binPath]
	if !ok {
		return fmt.Errorf("%s: command not found", binPath)
	}

	k.pids++
	pid := k.pids
	program := factory(pid)

	uid := session.CurrentUser
	euid := uid
	if meta, ok := k.computer.FsMetaData[binPath]; ok && meta.Setuid {
		euid = meta.Owner
	}

	pgid := opts.PGID
	if opts.PGID == 0 {
		pgid = pid
	}

	proc := &Process{
		PID:     pid,
		PGID:    pgid,
		UID:     uid,
		EUID:    euid,
		CWD:     session.WorkingDir,
		TTY:     session.TTY,
		Program: program,
	}

	k.procs[pid] = proc

	program.SetProcess(proc)
	program.SetTTyAPI(&TTYAPI{tty: session.TTY, proc: proc})
	program.SetKernel(k)
	session.TTY.SetForegroundPGID(pgid)

	status := make(chan int)

	k.computer.OS.Network.PublishEvent(EventProgramStarted, map[string]interface{}{
		"program_id": program.ID(),
		"tty_id":     session.TTY.id,
	})

	go program.Run(status, args)

	if opts.Background {
		go func() {
			exitCode := <-status
			k.computer.OS.Network.PublishEvent(EventProgramExited, map[string]interface{}{
				"program_id": program.ID(),
				"status":     exitCode,
				"tty_id":     session.TTY.id,
			})
			k.cleanupProcess(pid)
		}()
		return nil
	}

	exitCode := <-status

	k.computer.OS.Network.PublishEvent(EventProgramExited, map[string]interface{}{
		"program_id": program.ID(),
		"status":     exitCode,
		"tty_id":     session.TTY.id,
	})
	k.cleanupProcess(pid)

	if exitCode != utils.Success {
		return fmt.Errorf("%s: exited with status %d", binPath, exitCode)
	}
	return nil
}

func (k *Kernel) cleanupProcess(pid int) {
	delete(k.procs, pid)
}

func (k *Kernel) resolvePath(proc *Process, target string) string {
	target = path.Clean(target)

	if strings.HasPrefix(target, "~") {
		if proc.UID == "root" {
			target = path.Join("/root", target[1:])
		} else {
			target = path.Join("/home", proc.UID, target[1:])
		}
	}
	if !strings.HasPrefix(target, "/") {
		target = path.Join(proc.CWD, target)
	}
	return path.Clean(target)
}

func (k *Kernel) canWrite(effectiveUser string, filePath string) bool { // used internally by kernel for checking
	if effectiveUser == "root" {
		return true
	}
	meta, ok := k.computer.FsMetaData[filePath]
	if !ok {
		return true
	}
	if meta.Owner == effectiveUser {
		return meta.OwnerMode&2 != 0 // gets the first bit, like the 2's bit
	}
	return meta.OtherMode&2 != 0
}

func (k *Kernel) canRead(effectiveUser string, filePath string) bool { // used internally by kernel for checking
	if effectiveUser == "root" {
		return true
	}
	meta, ok := k.computer.FsMetaData[filePath]
	if !ok {
		return true
	}
	if meta.Owner == effectiveUser {
		return meta.OwnerMode&4 != 0 // gets the middle bit, divide by 4
	}
	return meta.OtherMode&4 != 0
}

func (k *Kernel) ReadFile(proc *Process, target string) ([]byte, error) { // syscal
	target = k.resolvePath(proc, target)
	if !k.canRead(proc.EUID, target) {
		return nil, fmt.Errorf("permission denied")
	}
	return k.computer.OS.ReadFile(target)
}

func (k *Kernel) ReadDir(proc *Process, target string) ([]os.FileInfo, error) { // syscall
	target = k.resolvePath(proc, target)
	if !k.canRead(proc.EUID, target) {
		return nil, fmt.Errorf("permission denied")
	}
	return k.computer.OS.ReadDir(target)
}

func (k *Kernel) MkDir(proc *Process, target string) error { // syscall
	target = k.resolvePath(proc, target)
	parent := path.Dir(target)
	if !k.canWrite(proc.EUID, parent) {
		return fmt.Errorf("permission denied")
	}
	if err := k.computer.OS.Mkdir(target); err != nil {
		return err
	}
	k.computer.FsMetaData[target] = FileMetadata{
		Filepath:  target,
		Owner:     proc.EUID,
		Setuid:    false,
		OwnerMode: 7,
		OtherMode: 5,
	}
	return nil
}

func (k *Kernel) WriteFile(proc *Process, target string, data []byte) error { // syscall
	target = k.resolvePath(proc, target)
	parent := path.Dir(target)
	if !k.canWrite(proc.EUID, parent) {
		return fmt.Errorf("permission denied")
	}
	return k.computer.OS.WriteFile(target, data) // little more abstraction didnt hurt anybody! the kernel never touch afero directly, YUCK
}
