package computer

// okay ill be honest this is not really a real kernel. all the cool juicy stuff like virtual memory, timers all that
// is mostly managed by the go runtime and the real operating system kernel, this is more like a syscall api.
// keep that in mind!

// one day ill build an ACTUAL kernel that boots on startup and does all that jazz but, imma focus on byte-space for now...

import (
	"context"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"

	"byte-space/utils"
)

// FDType represents the type of a file description, rn its only TTY but latr it can be pipes and stuff
type FDType int

const (
	FDTTY    FDType = iota
	FDFile          // for later additions, rn no need!
	FDSocket        // for networking later
	FDPipe
)

// Multiple processes share the same FileDescription pointer
type FileDescription struct {
	Type FDType
	TTY  *TTY // valid when Type == FDTTY else its NIL
	refs int
}

type IoctlReq int

const (
	TIOCRAW       IoctlReq = iota // raw mode (bool: true = raw, false = canonical)
	TIOCPASSWD                    // password masking (bool: true = mask input)
	TIOCSPGRP                     // set foreground process group (int: pgid)
	TIOCSWINSZ                    // set terminal dimensions (Winsize)
	TIOCBUFFCLEAR                 // clear the line buffer (bool: true = clear)
	TIOCGWINSZ                    // get terminal dimensions (*Winsize, written in place)
	TIOCSESSION                   // attach session to TTY (*Session)
)

// Winsize holds terminal dimensions, mirrors the Unix winsize struct.
type Winsize struct {
	Width  int
	Height int
}

// any nerds curious what TIOC means?
// it means TTY I/O Control (pretty cool if I do say so myself!)

type Program interface {
	SetProcess(proc *Process)
	SetKernel(k *Kernel)
	ID() string
	Run(ctx context.Context, returnStatus chan int, params []string)
	HandleSignal(sig Signal)
}

type Kernel struct {
	computer *Computer
	EventBus *EventBus
	programs map[string]func(int) Program // path to the factory, which can later change if I implement a language
	// later the factory can be just 1 function, and instead of a map, it can just read that file path and do the language stuff, check for shebang and all that.
	// rn we still need a map.

	pids     int        // keeps track of the highest ever PID
	freePIDs []int      // stores PIDs of exited processes to reuse
	pidsMu   sync.Mutex // fix racing

	procs   map[int]*Process // pid to the running process instance (all processes on this computer, GLOBALY)
	procsMu sync.Mutex

	fsMu sync.RWMutex

	openFiles   []*FileDescription // kernel-owned open file description table
	openFilesMu sync.Mutex
}

func (k *Kernel) RegisterProgram(path string, factory func(int) Program) {
	k.programs[path] = factory
}

func (k *Kernel) PublishEvent(proc *Process, eventType EventType, data map[string]interface{}) {
	k.EventBus.Publish(eventType, data)
}

func (k *Kernel) GetTtyID(proc *Process) string {
	if len(proc.FDs) > 0 && proc.FDs[0] != nil && proc.FDs[0].Type == FDTTY {
		return proc.FDs[0].TTY.id
	}
	return ""
}

// returns the file descriptor to a new tty
func (k *Kernel) OpenTTY(tty *TTY) *FileDescription {
	k.openFilesMu.Lock()
	defer k.openFilesMu.Unlock()
	desc := &FileDescription{Type: FDTTY, TTY: tty, refs: 1}
	k.openFiles = append(k.openFiles, desc)
	return desc
}

func (k *Kernel) Read(proc *Process, fd int, ctx context.Context) (string, int) {
	if fd < 0 || fd >= len(proc.FDs) || proc.FDs[fd] == nil {
		return "bad file descriptor", utils.Error
	}
	switch proc.FDs[fd].Type {
	case FDTTY:
		return proc.FDs[fd].TTY.Read(proc, ctx)
	}
	return "unsupported fd type", utils.Error
}

func (k *Kernel) Write(proc *Process, fd int, data []byte) (int, error) {
	if fd < 0 || fd >= len(proc.FDs) || proc.FDs[fd] == nil {
		return 0, fmt.Errorf("bad file descriptor")
	}
	switch proc.FDs[fd].Type {
	case FDTTY:
		return proc.FDs[fd].TTY.Write(data)
	}
	return 0, fmt.Errorf("unsupported fd type")
}

// Ik that there are better ways to approach this in go, type safe ways, but I wanna stick with the old school unix/C style cuz its a classic!
func (k *Kernel) Ioctl(proc *Process, fd int, req IoctlReq, arg any) error {
	if fd < 0 || fd >= len(proc.FDs) || proc.FDs[fd] == nil {
		return fmt.Errorf("bad file descriptor")
	}
	if proc.FDs[fd].Type != FDTTY {
		return fmt.Errorf("not a tty")
	}
	tty := proc.FDs[fd].TTY
	switch req {
	case TIOCRAW:
		raw, ok := arg.(bool)
		if !ok {
			return fmt.Errorf("Invalid type for TIOCRAW, needed bool!")
		}
		tty.Canonical = !raw
		tty.Echo = !raw
	case TIOCPASSWD:
		value, ok := arg.(bool)
		if !ok {
			return fmt.Errorf("Invalid type for TIOCPASSWD, needed bool!")
		}
		tty.PasswdMode = value
	case TIOCSPGRP:
		value, ok := arg.(int)
		if !ok {
			return fmt.Errorf("Invalid type for TIOCSPGRP, needed int!")
		}
		tty.SetForegroundPGID(value)
	case TIOCSWINSZ:
		value, ok := arg.(Winsize)
		if !ok {
			return fmt.Errorf("Invalid type for TIOCSWINSZ, needed Winsize!")
		}
		ws := value
		tty.SetWinsize(ws.Width, ws.Height)
	case TIOCBUFFCLEAR:
		value, ok := arg.(bool)
		if !ok {
			return fmt.Errorf("Invalid type for TIOCBUFFCLEAR, needed bool!")
		}
		if value {
			tty.BuffClear()
		}
	case TIOCGWINSZ:
		value, ok := arg.(*Winsize)

		if !ok {
			return fmt.Errorf("Invalid type for TIOCGWINSZ, needed *Winsize!")
		}

		ws := value
		ws.Width = tty.Width
		ws.Height = tty.Height

	case TIOCSESSION:
		value, ok := arg.(*Session)

		if !ok {
			return fmt.Errorf("Invalid type for TIOCSESSION, needed *Session!")
		}
		tty.Session = value
	}
	return nil
}

func (k *Kernel) NewSession(proc *Process, username string) (int, string) {
	var tty *TTY
	if len(proc.FDs) > 0 && proc.FDs[0] != nil && proc.FDs[0].Type == FDTTY {
		tty = proc.FDs[0].TTY
	}
	status, sessionID := k.computer.NewSession(username, tty)
	if status == utils.Success && tty != nil {
		tty.Session = k.computer.sessions[sessionID]
	}
	return status, sessionID
}

func (k *Kernel) GetNodeOnNetwork(ipAdress string) (*Computer, bool) {
	return k.computer.OS.Network.GetNode(ipAdress)
}

func (k *Kernel) nextPID() int {
	k.pidsMu.Lock()
	defer k.pidsMu.Unlock() // catch the exit

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
// fork actually duplicates the process, then checks if the pid = 0, if it does that line executes on the child which calls exec.
// I have simplifed this a lot to where exec makes a child, copys the data and runs the program

func (k *Kernel) Exec(parentCtx context.Context, parentProc *Process, binPath string, args []string, opts *ExecOpts) error {
	factory, ok := k.programs[binPath]
	if !ok {
		return fmt.Errorf("%s: command not found", binPath)
	}

	pid := k.nextPID()
	program := factory(pid)

	// make child context for exit propagation

	ctx, ctxCancel := context.WithCancel(parentCtx)

	uid := parentProc.UID
	euid := uid
	if meta, ok := k.getMetaData(binPath); ok && meta.Setuid {
		euid = meta.Owner
	}

	pgid := opts.PGID
	if opts.PGID == 0 {
		pgid = pid
	}

	proc := &Process{
		PID:       pid,
		PGID:      pgid,
		UID:       uid,
		EUID:      euid,
		CWD:       parentProc.CWD,
		Program:   program,
		CtxCancel: ctxCancel,
	}

	k.procsMu.Lock()
	// critical code
	k.procs[pid] = proc
	k.procsMu.Unlock()

	// Inherit parent's FD table — shallow copy so child shares the same FileDescriptions
	childFDs := make([]*FileDescription, len(parentProc.FDs))
	copy(childFDs, parentProc.FDs)
	proc.FDs = childFDs

	program.SetProcess(proc)
	program.SetKernel(k)

	if len(childFDs) > 0 && childFDs[0] != nil && childFDs[0].Type == FDTTY {
		childFDs[0].TTY.SetForegroundPGID(pgid)
	}

	status := make(chan int)

	k.EventBus.Publish(EventProgramStarted, map[string]interface{}{
		"program_id": program.ID(),
		"tty_id":     k.GetTtyID(parentProc),
	})

	go program.Run(ctx, status, args)

	if opts.Background {
		go func() {
			exitCode := <-status
			k.EventBus.Publish(EventProgramExited, map[string]interface{}{
				"program_id": program.ID(),
				"status":     exitCode,
				"tty_id":     k.GetTtyID(parentProc),
			})
			k.cleanupProcess(pid)
		}()
		return nil
	}

	exitCode := <-status

	k.EventBus.Publish(EventProgramExited, map[string]interface{}{
		"program_id": program.ID(),
		"status":     exitCode,
		"tty_id":     k.GetTtyID(parentProc),
	})
	k.cleanupProcess(pid)

	if exitCode != utils.Success {
		return fmt.Errorf("%s: exited with status %d", binPath, exitCode)
	}
	return nil
}

func (k *Kernel) cleanupProcess(pid int) {
	k.procsMu.Lock()
	delete(k.procs, pid)
	k.procsMu.Unlock()
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

func (k *Kernel) getMetaData(filePath string) (FileMetadata, bool) {
	k.fsMu.RLock()
	meta, ok := k.computer.FsMetaData[filePath]
	k.fsMu.RUnlock()

	return meta, ok
}

func (k *Kernel) getMetaDataLocked(filePath string) (FileMetadata, bool) {
	meta, ok := k.computer.FsMetaData[filePath]
	return meta, ok
}

func (k *Kernel) canWrite(effectiveUser string, filePath string) bool { // used internally by kernel for checking
	if effectiveUser == "root" {
		return true
	}

	meta, ok := k.getMetaDataLocked(filePath)

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

	meta, ok := k.getMetaDataLocked(filePath)
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
	meta, ok := k.getMetaDataLocked(filePath)
	if !ok {
		return true
	}
	if meta.Owner == effectiveUser {
		return meta.OwnerMode&1 != 0 // bit 0 = execute
	}
	return meta.OtherMode&1 != 0
}

func (k *Kernel) ReadFile(proc *Process, target string) ([]byte, error) { // syscal
	k.fsMu.Lock()
	defer k.fsMu.Unlock()

	target = k.resolvePath(proc, target)
	if !k.canRead(proc.EUID, target) {
		return nil, fmt.Errorf("permission denied")
	}
	return k.computer.OS.ReadFile(target)
}

func (k *Kernel) ReadDir(proc *Process, target string) ([]os.FileInfo, error) { // syscall
	k.fsMu.Lock()
	defer k.fsMu.Unlock()

	target = k.resolvePath(proc, target)
	if !k.canRead(proc.EUID, target) {
		return nil, fmt.Errorf("permission denied")
	}
	return k.computer.OS.ReadDir(target)
}

func (k *Kernel) RemoveAll(proc *Process, target string) error { // syscall
	k.fsMu.Lock()
	defer k.fsMu.Unlock()

	target = k.resolvePath(proc, target)
	parentDir := target
	targetStat, err := k.computer.filesystem.Stat(target)
	if err != nil {
		return fmt.Errorf("error")
	}

	if !targetStat.IsDir() {
		before, _, ok := strings.Cut(target, "/")
		if !ok {
			return fmt.Errorf("error")
		}
		parentDir = before
	}

	if !k.canWrite(proc.EUID, parentDir) || !k.canExecute(proc.EUID, parentDir) {
		return fmt.Errorf("permission denied")
	}
	k.computer.OS.RemoveAll(target)
	return nil
}

func (k *Kernel) Stat(proc *Process, target string) (FileMetadata, bool) { // syscall
	k.fsMu.Lock()
	defer k.fsMu.Unlock()

	target = k.resolvePath(proc, target)
	meta, ok := k.computer.FsMetaData[target]
	return meta, ok
}

func (k *Kernel) MkDir(proc *Process, target string) error { // syscall
	k.fsMu.Lock()
	defer k.fsMu.Unlock()

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
	k.fsMu.Lock()
	defer k.fsMu.Unlock()

	target = k.resolvePath(proc, target)
	parent := path.Dir(target)
	// TOCTOU racing // no more toctou racing
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

func (k *Kernel) WriteFile(proc *Process, target string, content []byte) error { // syscall
	k.fsMu.Lock()
	defer k.fsMu.Unlock()

	target = k.resolvePath(proc, target)
	parent := path.Dir(target)
	// later: check all parent dirs for execute, ALL, rn only checking one TODO

	if !k.canExecute(proc.EUID, parent) || !k.canWrite(proc.EUID, target) {
		return fmt.Errorf("permission denied")
	}

	if err := k.computer.OS.WriteFile(target, content); err != nil {
		return err
	}

	return nil
}

func (k *Kernel) ChangeDirectory(proc *Process, target string) error { // syscall
	target = k.resolvePath(proc, target)
	if !k.computer.OS.HasDirectory(target) {
		return fmt.Errorf("%s: no such file or directory", target)
	}
	k.fsMu.RLock()
	canExec := k.canExecute(proc.EUID, target)
	k.fsMu.RUnlock()
	if !canExec {
		return fmt.Errorf("%s: permission denied", target)
	}
	proc.CWD = target
	return nil
}

func (k *Kernel) Chmod(proc *Process, target string, newOwnerMode uint8, newOtherMode uint8) error { // syscall
	k.fsMu.Lock()
	target = k.resolvePath(proc, target)

	meta, ok := k.getMetaDataLocked(target)

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
	k.fsMu.Unlock()
	return nil
}

func (k *Kernel) GetProcs() map[int]*Process {
	k.procsMu.Lock()
	defer k.procsMu.Unlock()
	copy := make(map[int]*Process, len(k.procs))
	for pid, proc := range k.procs {
		copy[pid] = proc
	}

	return copy
	// copy so no more racing!!
}
