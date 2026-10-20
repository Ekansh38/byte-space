package computer

// okay ill be honest this is not really a real kernel. all the cool juicy stuff like virtual memory, timers all that
// is mostly managed by the go runtime and the real operating system kernel, this is more like a syscall api.
// keep that in mind!

// one day ill build an ACTUAL kernel that boots on startup and does all that jazz but, imma focus on byte-space for now...

import (
	"context"
	"fmt"
	"path"
	"slices"
	"strings"
	"sync"
	"time"

	"byte-space/utils"
)

type Errno int

const (
	EPERM   Errno = 1
	ENOENT  Errno = 2
	EBADF   Errno = 9
	EACCES  Errno = 13
	EEXIST  Errno = 17
	ENOTDIR Errno = 20
	EISDIR  Errno = 21
	EINVAL  Errno = 22
	ENOSPC  Errno = 28
)

// See following function to see the meaning of each error.

func (e Errno) Error() string {
	switch e {
	case EPERM:
		return "operation not permitted"
	case ENOENT:
		return "no such file or directory"
	case EBADF:
		return "bad file descriptor"
	case EACCES:
		return "permission denied"
	case EEXIST:
		return "file already exists"
	case ENOTDIR:
		return "not a directory"
	case EISDIR:
		return "is a directory"
	case EINVAL:
		return "invalid argument"
	case ENOSPC:
		return "no space left on device"
	default:
		return fmt.Sprintf("errno %d", int(e))
	}
}

type SyscallNum int

const (
	SYS_READ    SyscallNum = 0
	SYS_WRITE   SyscallNum = 1
	SYS_READDIR SyscallNum = 2
	SYS_STAT    SyscallNum = 3
	SYS_MKDIR   SyscallNum = 4
	SYS_CREATE  SyscallNum = 5
	SYS_REMOVE  SyscallNum = 6
	SYS_CHDIR   SyscallNum = 7
	SYS_CHMOD   SyscallNum = 8
	SYS_IOCTL   SyscallNum = 9
	SYS_EXEC    SyscallNum = 10
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

	InodeIndex uint32
	Offset     int64

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
// it means TTY I/O Control (pretty cool if I do say so meeself!)

type Program interface {
	SetProcess(proc *Process)
	SetKernel(k *Kernel)
	ID() string
	Run(ctx context.Context, returnStatus chan int, params []string)
	HandleSignal(sig Signal)
}

type Kernel struct {
	computer   *Computer
	inodeCache map[uint32]*inode            // indexed by inode number
	programs   map[string]func(int) Program // path to the factory, which can later change if I implement a language
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

// getInode is the ONE place where inodes get loaded: cache lookup, disk read
// and ops-assignment all live here. everything else just calls this.
func (k *Kernel) getInode(inum uint32) (*inode, error) {
	if in, ok := k.inodeCache[inum]; ok {
		return in, nil
	}

	in := &inode{}
	if err := k.computer.fs.readInode(in, inum); err != nil {
		return nil, err
	}

	// the fType byte IS the ops discriminator (see FILESYSTEM.md)
	switch in.fType {
	case S_IFDIR:
		in.ops = &DirectoryOps{inodeNum: inum}
	case S_IFREG:
		in.ops = &RegularFileOps{}
	}

	k.inodeCache[inum] = in
	return in, nil
}

// normalizePath does all the stringy cleanup: ~ expansion, relative -> absolute
// against the proc CWD, and path.Clean. the result is always a clean abs path.
func (k *Kernel) normalizePath(proc *Process, target string) string {
	// fix da stringy
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
		target = "/"
	}
	target = path.Clean(target)

	return target
}

func (k *Kernel) ResolvePath(proc *Process, target string) (uint32, error) {
	target = k.normalizePath(proc, target)

	// walk the directory tree now
	dirs := strings.Split(target, "/")
	dirs = slices.DeleteFunc(dirs, func(dir string) bool {
		return dir == ""
	}) // so idiomatic!

	mostRecentInode, err := k.getInode(2) // start at root
	if err != nil {
		return 0, err
	}

	for len(dirs) > 0 {
		if mostRecentInode.fType != S_IFDIR {
			return 0, ENOTDIR // thats a file, its like somebody giving a path /game.py/test
		}
		// unix style: you need x on every dir you walk through
		if !k.canExecute(proc.EUID, mostRecentInode) {
			return 0, EACCES
		}

		entries, err := mostRecentInode.ops.ReadEntries(k)
		if err != nil {
			return 0, err
		}

		var nextInum uint32
		for i := range entries {
			if entries[i].Name == dirs[0] {
				nextInum = entries[i].Inum
				break
			}
		}
		if nextInum == 0 {
			return 0, ENOENT
		}

		mostRecentInode, err = k.getInode(nextInum)
		if err != nil {
			return 0, err
		}

		dirs = dirs[1:] // remove that entry since we set it to the most recent inode.
	}

	return mostRecentInode.num, nil
}

// makeInode allocates + writes a fresh inode and links it into the parent dir.
// for dirs it also seeds . and .. so the new dir is instantly walkable.
func (k *Kernel) makeInode(parentInum uint32, name string, fType InodeType, owner string, setuid bool, ownerMode uint8, otherMode uint8) (uint32, error) {
	inum, err := k.computer.fs.AllocInode()
	if err != nil {
		return 0, err
	}

	err = k.computer.fs.writeInode(&inode{
		fType:      fType,
		refs:       1,
		owner:      ownerBytes(owner),
		setuid:     setuid,
		ownerMode:  ownerMode,
		otherMode:  otherMode,
		createdAt:  uint64(time.Now().Unix()),
		modifiedAt: uint64(time.Now().Unix()),
	}, inum)
	if err != nil {
		return 0, err
	}
	delete(k.inodeCache, inum) // the inum might be a reused one, dont serve a stale inode

	parentOps := &DirectoryOps{inodeNum: parentInum}
	if err := parentOps.addDentry(k, DirEntry{Inum: inum, Name: name}); err != nil {
		return 0, err
	}

	if fType == S_IFDIR {
		ops := &DirectoryOps{inodeNum: inum}
		if err := ops.addDentry(k, DirEntry{Inum: inum, Name: "."}); err != nil {
			return 0, err
		}
		if err := ops.addDentry(k, DirEntry{Inum: parentInum, Name: ".."}); err != nil {
			return 0, err
		}
	}

	return inum, nil
}

// freeInodeTree frees an inode, its data blocks, and (for dirs) everything
// under it. no hard links yet so freeing is unconditional, refs be damned.
func (k *Kernel) freeInodeTree(inum uint32) error {
	in, err := k.getInode(inum)
	if err != nil {
		return err
	}

	if in.fType == S_IFDIR {
		entries, err := in.ops.ReadEntries(k)
		if err != nil {
			return err
		}
		for _, entry := range entries {
			if entry.Name == "." || entry.Name == ".." {
				continue
			}
			if err := k.freeInodeTree(entry.Inum); err != nil {
				return err
			}
		}
	}

	if err := k.computer.fs.Falloc(in, 0); err != nil { // hands all the data blocks back
		return err
	}
	if err := k.computer.fs.FreeInodeFromBitmap(inum); err != nil {
		return err
	}
	delete(k.inodeCache, inum)
	return nil
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
func (k *Kernel) ioctl(proc *Process, fd int, req IoctlReq, arg any) error {
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
	return k.computer.Network.GetNode(ipAdress)
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
	return k.computer.Network.ListMachinesOnNetwork()
}

// Exec looks up the binary from the path then it creates a Process with correct EUID
// Practically fork & exec all in one
// fork actually duplicates the process, then checks if the pid = 0, if it does that line executes on the child which calls exec.
// I have simplifed this a lot to where exec makes a child, copys the data and runs the program

func (k *Kernel) exec(parentProc *Process, parentCtx context.Context, binPath string, args []string, opts *ExecOpts) error {
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
	k.fsMu.Lock()
	if inum, err := k.ResolvePath(parentProc, binPath); err == nil {
		if in, err := k.getInode(inum); err == nil && in.setuid {
			euid = ownerString(in.owner)
		}
	}
	k.fsMu.Unlock()

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

	go program.Run(ctx, status, args)

	if opts.Background {
		go func() {
			<-status
			k.cleanupProcess(pid)
		}()
		return nil
	}

	exitCode := <-status
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

func (k *Kernel) canWrite(effectiveUser string, in *inode) bool { // used internally by kernel for checking
	if effectiveUser == "root" {
		return true
	}
	if ownerString(in.owner) == effectiveUser {
		return in.ownerMode&2 != 0 // gets the first bit, like the 2's bit
	}
	return in.otherMode&2 != 0
}

func (k *Kernel) canRead(effectiveUser string, in *inode) bool { // used internally by kernel for checking
	if effectiveUser == "root" {
		return true
	}
	if ownerString(in.owner) == effectiveUser {
		return in.ownerMode&4 != 0 // gets the middle bit, divide by 4
	}
	return in.otherMode&4 != 0
}

func (k *Kernel) canExecute(effectiveUser string, in *inode) bool { // used internally by kernel for checking
	if effectiveUser == "root" {
		return true
	}
	if ownerString(in.owner) == effectiveUser {
		return in.ownerMode&1 != 0 // bit 0 = execute
	}
	return in.otherMode&1 != 0
}

func (k *Kernel) readFile(proc *Process, target string) ([]byte, error) { // syscall
	k.fsMu.Lock()
	defer k.fsMu.Unlock()

	inum, err := k.ResolvePath(proc, target)
	if err != nil {
		return nil, err
	}
	in, err := k.getInode(inum)
	if err != nil {
		return nil, err
	}

	if in.fType == S_IFDIR {
		return nil, EISDIR
	}
	if !k.canRead(proc.EUID, in) {
		return nil, EACCES
	}

	if in.size == 0 {
		return []byte{}, nil
	}

	fd := in.ops.CreateFD(k, inum, target, 0)
	if err := fd.Open(); err != nil {
		return nil, err
	}
	defer fd.Close()

	buf := make([]byte, in.size)
	if _, err := fd.Read(buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// FileInfo is what readDir hands back per entry. used to be os.FileInfo from
// afero but we only ever cared about the name and dir-ness.
// Inum is in here so ls -i can show off that these are real inodes on disk.
type FileInfo struct {
	Name  string
	IsDir bool
	Inum  uint32
}

func (k *Kernel) readDir(proc *Process, target string) ([]FileInfo, error) { // syscall
	k.fsMu.Lock()
	defer k.fsMu.Unlock()

	inum, err := k.ResolvePath(proc, target)
	if err != nil {
		return nil, err
	}
	in, err := k.getInode(inum)
	if err != nil {
		return nil, err
	}

	if in.fType != S_IFDIR {
		return nil, ENOTDIR
	}
	if !k.canRead(proc.EUID, in) {
		return nil, EACCES
	}

	entries, err := in.ops.ReadEntries(k)
	if err != nil {
		return nil, err
	}

	infos := make([]FileInfo, 0, len(entries))
	for _, entry := range entries {
		if entry.Name == "." || entry.Name == ".." {
			continue // ls doesnt show these, same as the afero days
		}
		child, err := k.getInode(entry.Inum)
		if err != nil {
			return nil, err
		}
		infos = append(infos, FileInfo{Name: entry.Name, IsDir: child.fType == S_IFDIR, Inum: entry.Inum})
	}
	return infos, nil
}

func (k *Kernel) removeAll(proc *Process, target string) error {
	k.fsMu.Lock()
	defer k.fsMu.Unlock()

	target = k.normalizePath(proc, target)
	if target == "/" {
		return EPERM // nice try
	}

	inum, err := k.ResolvePath(proc, target)
	if err != nil {
		return err
	}

	parentInum, err := k.ResolvePath(proc, path.Dir(target))
	if err != nil {
		return err
	}
	parentIn, err := k.getInode(parentInum)
	if err != nil {
		return err
	}

	if !k.canWrite(proc.EUID, parentIn) || !k.canExecute(proc.EUID, parentIn) {
		return EACCES
	}

	parentOps := &DirectoryOps{inodeNum: parentInum}
	if err := parentOps.removeDentry(k, path.Base(target)); err != nil {
		return err
	}
	return k.freeInodeTree(inum)
}

func (k *Kernel) stat(proc *Process, target string) (FileMetadata, bool) { // syscall
	k.fsMu.Lock()
	defer k.fsMu.Unlock()

	inum, err := k.ResolvePath(proc, target)
	if err != nil {
		return FileMetadata{}, false
	}
	in, err := k.getInode(inum)
	if err != nil {
		return FileMetadata{}, false
	}

	return FileMetadata{
		Filepath:  k.normalizePath(proc, target),
		Owner:     ownerString(in.owner),
		Setuid:    in.setuid,
		OwnerMode: in.ownerMode,
		OtherMode: in.otherMode,
	}, true
}

func (k *Kernel) mkDir(proc *Process, target string) error { // syscall
	k.fsMu.Lock()
	defer k.fsMu.Unlock()

	target = k.normalizePath(proc, target)
	if _, err := k.ResolvePath(proc, target); err == nil {
		return EEXIST
	}

	parentInum, err := k.ResolvePath(proc, path.Dir(target))
	if err != nil {
		return err
	}
	parentIn, err := k.getInode(parentInum)
	if err != nil {
		return err
	}
	if parentIn.fType != S_IFDIR {
		return ENOTDIR
	}
	if !k.canWrite(proc.EUID, parentIn) {
		return EACCES
	}

	_, err = k.makeInode(parentInum, path.Base(target), S_IFDIR, proc.EUID, false, 7, 5)
	return err
}

func (k *Kernel) createFile(proc *Process, target string) error { // syscall
	k.fsMu.Lock()
	defer k.fsMu.Unlock()

	target = k.normalizePath(proc, target)
	if _, err := k.ResolvePath(proc, target); err == nil {
		return nil // already exists, touch is a no-op
	}

	parentInum, err := k.ResolvePath(proc, path.Dir(target))
	if err != nil {
		return err
	}
	parentIn, err := k.getInode(parentInum)
	if err != nil {
		return err
	}
	if parentIn.fType != S_IFDIR {
		return ENOTDIR
	}
	if !k.canWrite(proc.EUID, parentIn) {
		return EACCES
	}

	_, err = k.makeInode(parentInum, path.Base(target), S_IFREG, proc.EUID, false, 7, 5)
	return err
}

func (k *Kernel) writeFile(proc *Process, target string, content []byte) error { // syscall
	k.fsMu.Lock()
	defer k.fsMu.Unlock()

	target = k.normalizePath(proc, target)
	// parent exec perms get checked by the ResolvePath walk itself now

	inum, err := k.ResolvePath(proc, target)
	if err == ENOENT {
		// afero WriteFile created missing files, keep that behaviour
		parentInum, perr := k.ResolvePath(proc, path.Dir(target))
		if perr != nil {
			return perr
		}
		parentIn, perr := k.getInode(parentInum)
		if perr != nil {
			return perr
		}
		if !k.canWrite(proc.EUID, parentIn) {
			return EACCES
		}
		inum, err = k.makeInode(parentInum, path.Base(target), S_IFREG, proc.EUID, false, 7, 5)
	}
	if err != nil {
		return err
	}

	in, err := k.getInode(inum)
	if err != nil {
		return err
	}
	if in.fType == S_IFDIR {
		return EISDIR
	}
	if !k.canWrite(proc.EUID, in) {
		return EACCES
	}

	// truncate to the new size (grow or shrink), then write from offset 0
	if err := k.computer.fs.Falloc(in, uint32(len(content))); err != nil {
		return err
	}
	if len(content) == 0 {
		return k.computer.fs.writeInode(in, inum)
	}

	fd := in.ops.CreateFD(k, inum, target, 0)
	if err := fd.Open(); err != nil {
		return err
	}
	defer fd.Close()

	_, err = fd.Write(content)
	return err
}

func (k *Kernel) changeDirectory(proc *Process, target string) error { // syscall
	target = k.normalizePath(proc, target)

	k.fsMu.Lock()
	inum, err := k.ResolvePath(proc, target)
	if err != nil {
		k.fsMu.Unlock()
		return fmt.Errorf("%s: no such file or directory", target)
	}
	in, err := k.getInode(inum)
	if err != nil || in.fType != S_IFDIR {
		k.fsMu.Unlock()
		return fmt.Errorf("%s: no such file or directory", target)
	}
	canExec := k.canExecute(proc.EUID, in)
	k.fsMu.Unlock()

	if !canExec {
		return fmt.Errorf("%s: permission denied", target)
	}
	proc.CWD = target
	return nil
}

func (k *Kernel) chmod(proc *Process, target string, newOwnerMode uint8, newOtherMode uint8) error { // syscall
	k.fsMu.Lock()
	defer k.fsMu.Unlock()

	inum, err := k.ResolvePath(proc, target)
	if err != nil {
		return fmt.Errorf("no such file or directory")
	}
	in, err := k.getInode(inum)
	if err != nil {
		return err
	}

	if proc.EUID != "root" && ownerString(in.owner) != proc.EUID {
		return fmt.Errorf("permission denied")
	}
	in.ownerMode = newOwnerMode
	in.otherMode = newOtherMode
	return k.computer.fs.writeInode(in, inum)
}

// Syscall is the single kernel entry point. Pass args in order, type-assert the result.
// Example: result, err := k.Syscall(proc, SYS_READ, "/path"); data := result.([]byte)
// Returns a Errorno if anything goes wrong.
func (k *Kernel) Syscall(proc *Process, num SyscallNum, args ...any) (any, error) {
	switch num {
	case SYS_READ:
		path, ok := args[0].(string)
		if !ok {
			return nil, EINVAL
		}
		return k.readFile(proc, path)
	case SYS_WRITE:
		path, ok := args[0].(string)
		if !ok {
			return nil, EINVAL
		}
		data, ok := args[1].([]byte)
		if !ok {
			return nil, EINVAL
		}
		return nil, k.writeFile(proc, path, data)
	case SYS_READDIR:
		path, ok := args[0].(string)
		if !ok {
			return nil, EINVAL
		}
		return k.readDir(proc, path)
	case SYS_STAT:
		path, ok := args[0].(string)
		if !ok {
			return nil, EINVAL
		}
		meta, ok := k.stat(proc, path)
		if !ok {
			return nil, ENOENT
		}
		return meta, nil
	case SYS_MKDIR:
		path, ok := args[0].(string)
		if !ok {
			return nil, EINVAL
		}
		return nil, k.mkDir(proc, path)
	case SYS_CREATE:
		path, ok := args[0].(string)
		if !ok {
			return nil, EINVAL
		}
		return nil, k.createFile(proc, path)
	case SYS_REMOVE:
		path, ok := args[0].(string)
		if !ok {
			return nil, EINVAL
		}
		return nil, k.removeAll(proc, path)
	case SYS_CHDIR:
		path, ok := args[0].(string)
		if !ok {
			return nil, EINVAL
		}
		return nil, k.changeDirectory(proc, path)
	case SYS_CHMOD:
		path, ok := args[0].(string)
		if !ok {
			return nil, EINVAL
		}
		ownerMode, ok := args[1].(uint8)
		if !ok {
			return nil, EINVAL
		}
		otherMode, ok := args[2].(uint8)
		if !ok {
			return nil, EINVAL
		}
		return nil, k.chmod(proc, path, ownerMode, otherMode)
	case SYS_IOCTL:
		fd, ok := args[0].(int)
		if !ok {
			return nil, EINVAL
		}
		req, ok := args[1].(IoctlReq)
		if !ok {
			return nil, EINVAL
		}
		return nil, k.ioctl(proc, fd, req, args[2])
	case SYS_EXEC:
		ctx, ok := args[0].(context.Context)
		if !ok {
			return nil, EINVAL
		}
		binPath, ok := args[1].(string)
		if !ok {
			return nil, EINVAL
		}
		execArgs, ok := args[2].([]string)
		if !ok {
			return nil, EINVAL
		}
		opts, ok := args[3].(*ExecOpts)
		if !ok {
			return nil, EINVAL
		}
		return nil, k.exec(proc, ctx, binPath, execArgs, opts)
	}
	return nil, EINVAL
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
