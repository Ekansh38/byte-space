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
	EventBus *EventBus
	programs map[string]func(int) Program // path to the factory, which can later change if I implement a language
	// later the factory can be just 1 function, and instead of a map, it can just read that file path and do the language stuff, check for shebang and all that.
	// rn we still need a map.

	pids     int   // keeps track of the highest ever PID
	freePIDs []int // stores PIDs of exited processes to reuse

	procs map[int]*Process // pid to the running process instance (all processes on this computer, GLOBALY)
}

func (k *Kernel) PublishEvent(proc *Process, eventType EventType, data map[string]interface{}) {
	k.EventBus.Publish(eventType, data)
}

func (k *Kernel) GetTtyID(proc *Process) string {
	return proc.Program.TTYAPI().GetTTYID()
}

func (k *Kernel) GetNodeOnNetwork(ipAdress string) (*Computer, bool) {
	return k.computer.OS.Network.GetNode(ipAdress)
}

func (k *Kernel) nextPID() int {
	if len(k.freePIDs) > 0 {
		// reuse
		pid := k.freePIDs[len(k.freePIDs)-1]
		k.freePIDs = k.freePIDs[:len(k.freePIDs)-1]
		return pid
	}

	// just increment if no free ones
	k.pids++
	return k.pids
}

type ExecOpts struct {
	Background bool
	PGID       int
}

func (k *Kernel) ListMachinesOnNetwork(proc *Process) []Computer {
	return k.computer.OS.Network.ListMachinesOnNetwork()
}

// Exec looks up the binary from the path then it creates a Process with correct EUID
// Practically fork & exec all in one
func (k *Kernel) Exec(parentProc *Process, binPath string, args []string, opts *ExecOpts) error {
	factory, ok := k.programs[binPath]
	if !ok {
		return fmt.Errorf("%s: command not found", binPath)
	}

	pid := k.nextPID()
	program := factory(pid)

	uid := parentProc.UID
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
		CWD:     parentProc.CWD,
		Program: program,
	}

	k.procs[pid] = proc

	program.SetProcess(proc)
	program.SetTTyAPI(&TTYAPI{tty: parentProc.Program.TTYAPI().tty, proc: proc})
	program.SetKernel(k)
	parentProc.Program.TTYAPI().tty.SetForegroundPGID(pgid)

	status := make(chan int)

	k.EventBus.Publish(EventProgramStarted, map[string]interface{}{
		"program_id": program.ID(),
		"tty_id":     parentProc.Program.TTYAPI().tty.id,
	})

	go program.Run(status, args)

	if opts.Background {
		go func() {
			exitCode := <-status
			k.EventBus.Publish(EventProgramExited, map[string]interface{}{
				"program_id": program.ID(),
				"status":     exitCode,
				"tty_id":     parentProc.Program.TTYAPI().tty.id,
			})
			k.cleanupProcess(pid)
		}()
		return nil
	}

	exitCode := <-status

	k.EventBus.Publish(EventProgramExited, map[string]interface{}{
		"program_id": program.ID(),
		"status":     exitCode,
		"tty_id":     parentProc.Program.TTYAPI().tty.id,
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
	if target == "" {
		target = proc.CWD
	}

	home := "/home/" + proc.UID
	if proc.UID == "root" {
		home = "/root"
	}

	if target == "~" {
		target = home
	} else if strings.HasPrefix(target, "~/") {
		target = path.Join(home, target[2:])
	}

	if !strings.HasPrefix(target, "/") {
		target = path.Join(proc.CWD, target)
	}
	target = path.Clean(target)

	if !strings.HasPrefix(target, "/") {
		return "/"
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

func (k *Kernel) canExecute(effectiveUser string, filePath string) bool { // used internally by kernel for checking
	if effectiveUser == "root" {
		return true
	}
	meta, ok := k.computer.FsMetaData[filePath]
	if !ok {
		return true
	}
	if meta.Owner == effectiveUser {
		return meta.OwnerMode&1 != 0 // bit 0 = execute
	}
	return meta.OtherMode&1 != 0
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

func (k *Kernel) Stat(proc *Process, target string) (FileMetadata, bool) { // syscall
	target = k.resolvePath(proc, target)
	meta, ok := k.computer.FsMetaData[target]
	return meta, ok
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
	k.computer.saveMetaData()
	return nil
}

func (k *Kernel) CreateFile(proc *Process, target string) error { // syscall
	target = k.resolvePath(proc, target)
	parent := path.Dir(target)
	if !k.canWrite(proc.EUID, parent) {
		return fmt.Errorf("permission denied")
	}
	if err := k.computer.OS.CreateFile(target); err != nil {
		return err
	}
	k.computer.FsMetaData[target] = FileMetadata{
		Filepath:  target,
		Owner:     proc.EUID,
		Setuid:    false,
		OwnerMode: 7,
		OtherMode: 5,
	}
	k.computer.saveMetaData()
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

func (k *Kernel) ChangeDirectory(proc *Process, target string) error { // syscall
	target = k.resolvePath(proc, target)
	if !k.computer.OS.HasDirectory(target) {
		return fmt.Errorf("%s: no such file or directory", target)
	}
	if !k.canExecute(proc.EUID, target) {
		return fmt.Errorf("%s: permission denied", target)
	}
	proc.CWD = target
	return nil
}

func (k *Kernel) Chmod(proc *Process, target string, newOwnerMode uint8, newOtherMode uint8) error { // syscall
	target = k.resolvePath(proc, target)
	meta, ok := k.computer.FsMetaData[target]
	if !ok {
		return fmt.Errorf("no such file or directory")
	}
	if proc.EUID != "root" && meta.Owner != proc.EUID {
		return fmt.Errorf("permission denied")
	}
	meta.OwnerMode = newOwnerMode
	meta.OtherMode = newOtherMode
	k.computer.FsMetaData[target] = meta
	k.computer.saveMetaData()
	return nil
}
