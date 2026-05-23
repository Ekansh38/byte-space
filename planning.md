# BS-EXTFS: Filesystem Design

This document describes the filesystem design for byte-space. It covers
how inodes, file descriptors, and special files work — and how they
compare to real Linux.


---


## Part 1: How Linux Does It (The Real Thing)

Before diving into byte-space, here's how Linux actually works. This
is what we're modeling, simplified.

### The Three Layers

Linux has three layers of abstraction between "user calls read()" and
"bytes come off the disk":

```
User program
    │
    │  read(fd, buf, 4096)
    ▼
┌─────────────────────────────┐
│  VFS (Virtual File System)  │   Generic layer. Doesn't know about
│                             │   ext4, procfs, or any specific FS.
│  - Manages inodes in RAM    │   Just dispatches to the right ops.
│  - Manages open files       │
│  - Resolves paths           │
└──────────┬──────────────────┘
           │  file->f_op->read()
           ▼
┌─────────────────────────────┐
│  Filesystem Driver          │   ext4, procfs, tmpfs, etc.
│  (one per filesystem type)  │   Each provides its own ops.
│                             │
│  ext4: reads from disk      │
│  procfs: generates from RAM │
└──────────┬──────────────────┘
           │  (ext4 only)
           ▼
┌─────────────────────────────┐
│  Block Layer / Disk         │   Physical storage.
└─────────────────────────────┘
```

### Linux Inode (in memory)

When Linux loads an inode from disk, it creates a `struct inode` in RAM:

```c
struct inode {
    // Metadata (from disk)
    umode_t         i_mode;       // type + permissions
    uid_t           i_uid;        // owner
    gid_t           i_gid;        // group
    loff_t          i_size;       // file size
    struct timespec i_atime;      // access time
    struct timespec i_mtime;      // modify time
    
    // Ops — function pointer tables (set at load time, NOT on disk)
    struct inode_operations     *i_op;   // directory-level ops
    struct file_operations      *i_fop;  // per-open-file ops
    struct super_operations     *s_op;   // filesystem-level ops
    
    // ... lots more
};
```

**Key point:** The ops pointers are NOT stored on disk. On disk, ext4
stores a type field (regular, directory, symlink, etc). When the kernel
loads the inode from disk into RAM, it looks at the type and attaches
the right ops:

```c
// Inside ext4 driver, when loading an inode from disk:
if (S_ISREG(inode->i_mode)) {
    inode->i_op  = &ext4_file_inode_operations;
    inode->i_fop = &ext4_file_operations;
} else if (S_ISDIR(inode->i_mode)) {
    inode->i_op  = &ext4_dir_inode_operations;
    inode->i_fop = &ext4_dir_operations;
}
```

For procfs, the inode never came from disk in the first place — it was
created in RAM with procfs ops already attached:

```c
// Inside procfs, when creating /proc/meminfo:
inode->i_fop = &proc_meminfo_operations;
```

### Linux Open Flow

```c
// User calls: open("/etc/passwd", O_RDONLY)

// 1. VFS resolves path by walking directories.
//    At each component, it calls the parent dir's i_op->lookup().
//    This returns the child inode.

// 2. VFS allocates a struct file (≈ our FD).
file = alloc_file();
file->f_op = inode->i_fop;   // copy ops pointer from inode

// 3. VFS calls the file's open method.
file->f_op->open(inode, file);

// 4. File is stored in the process's file descriptor table.
fd_install(fd_number, file);

// 5. Later, when user calls read():
file->f_op->read(file, buf, count, &pos);
//                ^^^^
// The file carries its own ops. VFS just dispatches.
// ext4_file_operations.read reads from disk.
// proc_meminfo_operations.read generates data from kernel state.
```

### Linux Has Two Ops Tables on Inodes

```
inode_operations (i_op)                   file_operations (i_fop)
(yes it's called "inode_operations"
even though these are mostly dir ops.
Linux puts all inode-level ops in one
struct. For regular files, most are NULL.)
─────────────────────            ───────────────────────
lookup()  — find child in dir    read()    — read bytes
create()  — make new file        write()   — write bytes
mkdir()   — make new dir         open()    — initialize per-open state
unlink()  — delete file          release() — close/cleanup
rename()  — move/rename          mmap()    — memory map
link()    — hard link            fsync()   — flush to disk
```

`i_op` is about **managing the directory tree** (creating, deleting,
finding files). `i_fop` is about **what happens when you open and
read/write a file**. The inode carries both, but they serve different
purposes.


---


## Part 2: How byte-space Does It

We simplify Linux's design. Instead of two separate ops tables, we
have one: `InodeOperations` with a single method `CreateFD()`. Instead
of a VFS layer with mount points, we have a single inode namespace
with a range split between disk and virtual inodes.

The result behaves the same — just with less indirection.

### The Inode

One struct for everything. The first 128 bytes get serialized to/from
disk. The `num` and `ops` fields are runtime-only — set when the inode
is loaded into memory or created as a virtual inode.

```go
type Inode struct {
    // --- serialized to disk (128 bytes, hand-rolled) ---
    size       uint32
    fType      InodeType  // S_IFREG (0) or S_IFDIR (1) on disk
    refs       uint16
    owner      [14]byte
    setuid     uint8
    ownerMode  uint8
    otherMode  uint8
    direct     [12]uint32
    find       uint32      // first-indirect
    sind       uint32      // second-indirect
    tind       uint32      // third-indirect
    createdAt  uint64
    modifiedAt uint64
    _reserved  [28]byte
    
    // --- runtime only (never touches disk) ---
    num   uint32           // position in inode table, or synthetic
    ops   InodeOperations  // factory that creates the right FD type
}
```

**Comparison to Linux:**

| Linux | byte-space | Purpose |
|-------|-----------|---------|
| `inode->i_mode` | `inode.fType` | File type (on disk) |
| `inode->i_fop` | `inode.ops` | How to create/handle open files (runtime only) |
| `inode->i_op` | (built into kernel) | Directory management (we use kernel methods instead) |
| `inode->i_ino` | `inode.num` | Inode number |

Just like Linux, the ops field is **never on disk**. It's attached at
load time based on the type.

### InodeOperations: The Interface

```go
type InodeOperations interface {
    CreateFD(kernel *Kernel, inodeNum uint32, path string, flags int) FD
    ReadEntries(kernel *Kernel) []DirEntry
}

type DirEntry struct {
    Inum uint32
    Name string
}
```

Two methods:

- `CreateFD()` — factory that builds the right FD type when a process
  opens the file. After that, the FD is self-contained.
- `ReadEntries()` — returns directory children for path resolution.
  Only meaningful for directory ops. Non-directory ops return nil.

This is byte-space's version of Linux's `i_fop` + `i_op`. In Linux,
the inode carries separate tables for file operations and directory
operations. In byte-space, we put both on one interface. `CreateFD`
corresponds to `i_fop` (per-open-file behavior). `ReadEntries`
corresponds to `i_op->lookup` (finding children during path walks).

**CreateFD is a factory. ReadEntries is a directory query. Non-directory
ops return nil for ReadEntries. After CreateFD, the FD is self-contained.
You never call back into ops (except ReadEntries for DirectoryFD).**

```
Linux:                              byte-space:
  file = alloc_file()                 fd = inode.ops.CreateFD(...)
  file->f_op = inode->i_fop          // (ops already baked into the FD)
  file->f_op->open(inode, file)      fd.Open()
  file->f_op->read(file, ...)        fd.Read(buf)
```

**Why this differs from Linux (and why that's fine):**

In Linux, the FD (`struct file`) is a dumb bag of state — offset,
flags, a back-pointer to the inode. The ops live in a separate
function table (`f_op`), and the kernel passes the FD to the ops
as an argument: `file->f_op->read(file, buf, n, &pos)`. The FD
doesn't "know" how to read — it gets read BY the ops.

In byte-space, the FD IS the ops. `RegularFileFD` has both the
state (offset, inodeNum) AND the methods (Read, Write, Close) on
the same struct. There's no separate function table. This is more
idiomatic Go / OOP — an interface with methods on a concrete struct,
rather than C-style function pointer tables receiving a state bag.

Functionally identical. The FD still holds per-open state, the
behavior still varies by file type. We just let Go's interface
dispatch do what Linux does with function pointers.

Different inode types → different ops → different FD types:

| Inode ops | Creates | ReadEntries | FD does |
|-----------|---------|-------------|---------|
| `RegularFileOps` | `RegularFileFD` | nil | Reads/writes disk blocks |
| `DiskDirOps` | `DirectoryFD` | Parses data blocks | Lists directory entries from disk |
| `ProcDirOps` | `DirectoryFD` | Generates from code | Lists /proc entries |
| `ProcInfoOps` | `ProcInfoFD` | nil | Generates /proc data from kernel state |
| `ProcPidOps` | `ProcPidFD` | nil | Generates per-process data |
| `SocketOps` | `SocketFD` | nil | Reads/writes from channels |

### The FD Interface

```go
type FD interface {
    Open() error                    // validate & initialize (can fail)
    Read(buf []byte) (int, error)   // read bytes
    Write(data []byte) (int, error) // write bytes
    Close() error                   // cleanup
    
    InodeNum() uint32
    Offset() uint64
    SetOffset(uint64)
    Flags() int
}
```

Each FD type is a concrete struct implementing this interface.
The process holds `map[int]FD` — different concrete types in the
same table, dispatched polymorphically.

**Comparison to Linux:**

| Linux `struct file` | byte-space FD | |
|-------|-----------|--|
| `f_op` (function pointer table) | (methods on the concrete struct) | How read/write work |
| `f_pos` | `Offset()` | Current position |
| `f_flags` | `Flags()` | Open flags (read/write) |
| `f_inode` | `InodeNum()` | Which inode this file refers to |


---


## Part 3: The Open Flow

### How It All Fits Together

```go
func (k *Kernel) Open(proc *Process, path string, flags int) (int, error) {
    // 1. Walk directory entries to find the inode number.
    //    This is uniform — same code for /etc/passwd and /proc/processes.
    //    (Linux: VFS namei/path_walk, calling i_op->lookup at each step)
    inodeNum := k.resolvePath(path)
    
    // 2. Get the inode. If it's a disk inode not yet in cache, load it
    //    from disk and attach ops based on fType. If it's virtual, it's
    //    already in the cache with ops set.
    //    (Linux: iget/iget_locked, calls s_op->read_inode for disk inodes)
    inode := k.getInode(inodeNum)
    
    // 3. Check permissions.
    //    (Linux: inode_permission, checks i_mode against credentials)
    if !k.canAccess(proc, inode, flags) {
        return -1, ErrPermissionDenied
    }
    
    // 4. Ask the inode's ops to build the right FD type.
    //    This is just a constructor — sets fields, no validation.
    //    (Linux: alloc_file + copy f_op from inode->i_fop)
    fd := inode.ops.CreateFD(k, inodeNum, path, flags)
    
    // 5. Let the FD validate and initialize itself.
    //    Can fail (e.g. process doesn't exist for /proc/[pid]).
    //    (Linux: file->f_op->open(inode, file))
    if err := fd.Open(); err != nil {
        return -1, err
    }
    
    // 6. Store in process FD table.
    //    (Linux: fd_install(fd_number, file))
    fdNum := k.allocateFD(proc)
    proc.FDs[fdNum] = fd
    
    return fdNum, nil
}
```

### After Open: Read, Write, Close

The kernel just dispatches to the FD. No type checking, no switch
statements. The FD knows what to do because it's the right concrete type.

```go
func (k *Kernel) Read(proc *Process, fdNum int, buf []byte) (int, error) {
    fd := proc.FDs[fdNum]
    if fd == nil {
        return 0, ErrBadFD
    }
    return fd.Read(buf)  // RegularFileFD reads disk. ProcInfoFD generates data.
}

func (k *Kernel) Write(proc *Process, fdNum int, data []byte) (int, error) {
    fd := proc.FDs[fdNum]
    if fd == nil {
        return 0, ErrBadFD
    }
    return fd.Write(data)
}

func (k *Kernel) Close(proc *Process, fdNum int) error {
    fd := proc.FDs[fdNum]
    if fd == nil {
        return ErrBadFD
    }
    fd.Close()
    delete(proc.FDs, fdNum)
    return nil
}
```

**In Linux this is identical:**
```c
// Linux kernel's sys_read:
file = fget(fd);           // look up struct file from fd number
file->f_op->read(file, buf, count, &pos);  // dispatch
```


---


## Part 4: Disk Inodes vs Virtual Inodes

### The Problem

Some files live on disk (`/etc/passwd`, `/home/user/notes.txt`).
Some files are fabricated from kernel state (`/proc/processes`,
`/proc/123/status`). Both need inodes. Both need to work with the
same path resolution and open flow.

**How Linux solves this:** Separate filesystems with separate inode
number spaces, connected by mount points. VFS tracks which filesystem
owns each inode via `(device, inode_number)` pairs.

**How byte-space solves this:** Single inode number space, split by
range. Simpler, works for our scale.

### The Inode Number Split

```
Inode 0          : invalid (zero means "no inode")
Inode 1          : reserved
Inode 2          : root directory (/)
Inodes 3–8063    : disk inodes (allocated from bitmap)
Inodes 10000+    : virtual inodes (constants or computed)
Inodes 20000+    : per-pid virtual inodes (20000 + pid)
```

### The Inode Cache

The kernel has one map. Both disk and virtual inodes end up here.

```go
type Kernel struct {
    fs         *FileSystem
    inodeCache map[uint32]*Inode
}
```

**How inodes enter the cache:**

| Source | When | How ops are set |
|--------|------|-----------------|
| Disk inode | On demand, first access | Load 128 bytes from disk, fType → ops |
| Virtual inode (fixed) | At boot (initProcFS) | Created in Go, ops set directly |
| Virtual inode (per-pid) | On demand, first access | Created in Go, ops set directly |

### getInode: The Single Entry Point

```go
func (k *Kernel) getInode(num uint32) *Inode {
    // 1. Check cache (covers everything already loaded)
    if inode, ok := k.inodeCache[num]; ok {
        return inode
    }
    
    // 2. Virtual per-pid inode? Create on the fly.
    if num >= PROC_PID_BASE {
        pid := num - PROC_PID_BASE
        inode := &Inode{
            num: num,
            ops: &ProcPidOps{pidType: PROC_PID_STATUS},
        }
        k.inodeCache[num] = inode
        return inode
    }
    
    // 3. Disk inode. Load from disk, attach ops by fType.
    inode := k.fs.readInodeFromDisk(num)
    inode.num = num
    switch inode.fType {
    case S_IFREG:
        inode.ops = &RegularFileOps{}
    case S_IFDIR:
        inode.ops = &DirectoryOps{}
    }
    k.inodeCache[num] = inode
    return inode
}
```

**Comparison to Linux:** Linux's `iget_locked()` does the same thing —
checks the inode cache first, and if not present, calls the filesystem's
`read_inode` to load it from disk (or generate it for procfs). Same
pattern, different implementation.

### Virtual Mounts Are Hardcoded On Disk

At mkfs time, the root directory's data blocks contain a "proc" entry
pointing to inode 10000. This is baked into the disk image. The kernel
doesn't inject or merge anything at runtime — path resolution just
follows the directory entry to inode 10000 like any other entry.

When `getInode(10000)` is called, it finds the inode already in the
cache (placed there at boot by `initProcFS`). No disk read happens.

Tradeoff: adding a new virtual mount (e.g. `/dev`) means reformatting.
For an educational OS with a known set of virtual filesystems, this
is fine.


---


## Part 5: Boot Sequence

```go
func (k *Kernel) initProcFS() {
    // Root dir already has "proc" → 10000 on disk from mkfs.
    // Just register the virtual inodes.
    
    k.inodeCache[PROC_ROOT_VNODE] = &Inode{
        num: PROC_ROOT_VNODE,
        ops: &ProcDirOps{},
    }
    
    k.inodeCache[PROC_PROCESSES_VNODE] = &Inode{
        num: PROC_PROCESSES_VNODE,
        ops: &ProcInfoOps{infoType: PROC_PROCESSES},
    }
    
    k.inodeCache[PROC_MEMINFO_VNODE] = &Inode{
        num: PROC_MEMINFO_VNODE,
        ops: &ProcInfoOps{infoType: PROC_MEMINFO},
    }
}

const (
    MAX_DISK_INODES      = 8064
    PROC_ROOT_VNODE      = 10000
    PROC_PROCESSES_VNODE = 10001
    PROC_MEMINFO_VNODE   = 10002
    PROC_PID_BASE        = 20000
)
```


---


## Part 6: Concrete Examples

### Example A: `cat /etc/passwd` (disk file)

```
RESOLVE PATH
  Start at inode 2 (root dir, always cached at boot)
  Read root's directory entries FROM DISK
  Find "etc" → inode 5
  
  getInode(5) → not in cache → load from disk
    fType = S_IFDIR → ops = DirectoryOps
    Cache it
  
  Read inode 5's directory entries from disk
  Find "passwd" → inode 17
  
  getInode(17) → not in cache → load from disk
    fType = S_IFREG → ops = RegularFileOps
    Cache it

OPEN
  inode 17, ops = RegularFileOps
  ops.CreateFD() → RegularFileFD{inodeNum: 17, offset: 0}
  fd.Open() → nil (nothing to validate for regular files)
  Store as FD 3 in process table

READ
  fd.Read(buf)
  → Look up inode 17 in cache → get block pointers
  → Read data blocks from disk
  → Copy into buf, advance offset
```

### Example B: `cat /proc/processes` (virtual file)

```
RESOLVE PATH
  Start at inode 2 (root dir, always cached at boot)
  Read root's directory entries FROM DISK
  Find "proc" → inode 10000        ← this entry was written at mkfs

  getInode(10000) → IS in cache (virtual, from initProcFS)
    ops = ProcDirOps

  ProcDirOps generates directory entries IN MEMORY:
    "."         → 10000
    ".."        → 2
    "processes" → 10001
    "meminfo"   → 10002
    "42"        → 20042   (PID 42 exists)
    "89"        → 20089   (PID 89 exists)
  Find "processes" → inode 10001

  getInode(10001) → IS in cache (virtual, from initProcFS)
    ops = ProcInfoOps{infoType: PROC_PROCESSES}

OPEN
  inode 10001, ops = ProcInfoOps
  ops.CreateFD() → ProcInfoFD{inodeNum: 10001, infoType: PROCESSES}
  fd.Open() → nil (nothing to validate for system-wide proc files)
  Store as FD 3 in process table

READ
  fd.Read(buf)
  → No disk involved
  → Generate process table from kernel.processes
  → Cache the result in the FD (so re-reads are consistent)
  → Copy into buf, advance offset
```

### Example C: `cat /proc/42/status` (per-pid virtual file)

```
RESOLVE PATH
  Start at inode 2 → root dir entries from disk → "proc" → inode 10000
  
  getInode(10000) → cached, ProcDirOps
  ProcDirOps generates entries, including "42" → inode 20042
  
  getInode(20042) → NOT in cache
    num >= PROC_PID_BASE → create virtual inode
    ops = ProcPidOps{pidType: PROC_PID_STATUS}
    Cache it

  (If 20042 had sub-entries like "status", "cmdline", those would be
   generated by ProcPidOps acting as a directory. For simplicity, you
   might just make /proc/42 directly be the status file.)

OPEN
  inode 20042, ops = ProcPidOps
  ops.CreateFD() → ProcPidFD{pid: 42, pidType: STATUS}
  fd.Open() → checks kernel.processes[42] exists → nil (success)
  Store as FD 3

READ
  fd.Read(buf)
  → Generate "Name: sh\nPid: 42\nState: running\n" from kernel state
  → Cache in FD, copy to buf
```


---


## Part 7: The FD Types

### RegularFileFD

```go
type RegularFileOps struct{}  // stateless — same for every regular file

func (r *RegularFileOps) CreateFD(kernel *Kernel, inodeNum uint32, path string, flags int) FD {
    return &RegularFileFD{
        kernel:   kernel,
        inodeNum: inodeNum,
        offset:   0,
        flags:    flags,
    }
}

type RegularFileFD struct {
    kernel   *Kernel
    inodeNum uint32
    offset   uint64
    flags    int
}

func (r *RegularFileFD) Open() error  { return nil }

func (r *RegularFileFD) Read(buf []byte) (int, error) {
    inode := r.kernel.getInode(r.inodeNum)
    blockIndex := r.offset / BLOCKSIZE
    blockOffset := r.offset % BLOCKSIZE
    blockNum := inode.direct[blockIndex]
    blockData := r.kernel.fs.ReadBlock(blockNum)
    n := copy(buf, blockData[blockOffset:])
    r.offset += uint64(n)
    return n, nil
}

func (r *RegularFileFD) Write(data []byte) (int, error) {
    // Write to disk blocks, allocate new blocks if needed
    // Update inode.size, update offset
    return 0, nil // TODO
}

func (r *RegularFileFD) Close() error { return nil }
```

### ProcInfoFD (system-wide proc files)

```go
type ProcInfoOps struct {
    infoType ProcInfoType  // PROC_PROCESSES, PROC_MEMINFO
}

func (ops *ProcInfoOps) CreateFD(kernel *Kernel, inodeNum uint32, path string, flags int) FD {
    return &ProcInfoFD{
        kernel:   kernel,
        inodeNum: inodeNum,
        offset:   0,
        flags:    flags,
        infoType: ops.infoType,
        cached:   nil,
    }
}

type ProcInfoFD struct {
    kernel   *Kernel
    inodeNum uint32
    offset   uint64
    flags    int
    infoType ProcInfoType
    cached   []byte          // generated once per open, then served from cache
}

func (p *ProcInfoFD) Open() error { return nil }

func (p *ProcInfoFD) Read(buf []byte) (int, error) {
    if p.cached == nil {
        switch p.infoType {
        case PROC_PROCESSES:
            var b strings.Builder
            b.WriteString("PID\tNAME\tSTATE\n")
            for _, proc := range p.kernel.processes {
                fmt.Fprintf(&b, "%d\t%s\t%s\n", proc.PID, proc.Name, proc.State)
            }
            p.cached = []byte(b.String())
        case PROC_MEMINFO:
            p.cached = []byte(fmt.Sprintf("MemTotal: %d\nMemFree: %d\n",
                p.kernel.totalMemory, p.kernel.freeMemory))
        }
    }
    if p.offset >= uint64(len(p.cached)) {
        return 0, io.EOF
    }
    n := copy(buf, p.cached[p.offset:])
    p.offset += uint64(n)
    return n, nil
}

func (p *ProcInfoFD) Close() error {
    p.cached = nil
    return nil
}
```

### ProcPidFD (per-process proc files)

```go
type ProcPidOps struct {
    pidType ProcPidType  // STATUS, CMDLINE
}

func (ops *ProcPidOps) CreateFD(kernel *Kernel, inodeNum uint32, path string, flags int) FD {
    pid := extractPidFromPath(path)
    return &ProcPidFD{
        kernel:   kernel,
        inodeNum: inodeNum,
        offset:   0,
        flags:    flags,
        pid:      pid,
        pidType:  ops.pidType,
        cached:   nil,
    }
}

type ProcPidFD struct {
    kernel   *Kernel
    inodeNum uint32
    offset   uint64
    flags    int
    pid      int
    pidType  ProcPidType
    cached   []byte
}

func (p *ProcPidFD) Open() error {
    // This is where validation happens — can fail
    if p.kernel.processes[p.pid] == nil {
        return ErrNoSuchProcess
    }
    return nil
}

func (p *ProcPidFD) Read(buf []byte) (int, error) {
    if p.cached == nil {
        proc := p.kernel.processes[p.pid]
        switch p.pidType {
        case PROC_PID_STATUS:
            p.cached = []byte(fmt.Sprintf("Name: %s\nPid: %d\nState: %s\n",
                proc.Name, proc.PID, proc.State))
        case PROC_PID_CMDLINE:
            p.cached = []byte(strings.Join(proc.Args, "\x00"))
        }
    }
    if p.offset >= uint64(len(p.cached)) {
        return 0, io.EOF
    }
    n := copy(buf, p.cached[p.offset:])
    p.offset += uint64(n)
    return n, nil
}

func (p *ProcPidFD) Close() error {
    p.cached = nil
    return nil
}
```

### SocketFD

```go
type SocketFD struct {
    kernel   *Kernel
    inodeNum uint32
    flags    int
    socket   *Socket
}

func (s *SocketFD) Open() error {
    if s.socket.state == CLOSED {
        return ErrSocketClosed
    }
    if s.socket.recvBuf == nil {
        s.socket.recvBuf = make(chan []byte, 100)
    }
    return nil
}

func (s *SocketFD) Read(buf []byte) (int, error) {
    data := <-s.socket.recvBuf
    n := copy(buf, data)
    return n, nil
}

func (s *SocketFD) Write(data []byte) (int, error) {
    s.socket.sendBuf <- data
    return len(data), nil
}

func (s *SocketFD) Offset() uint64      { return 0 }
func (s *SocketFD) SetOffset(o uint64)  {}
```


---


## Part 8: Process FD Table

```go
type Process struct {
    PID  int
    FDs  map[int]FD  // interface — holds any FD type
}
```

Example state for process 123:

```
FD 0 (stdin):  TTY_FD{tty: ...}
FD 1 (stdout): TTY_FD{tty: ...}
FD 3:          RegularFileFD{inodeNum: 42, offset: 512}
FD 4:          ProcPidFD{pid: 99, cached: nil}
FD 5:          SocketFD{socket: ...}
```

Two processes open the same file → same inode, same ops, but two
completely separate FDs with independent offsets and state.


---


## Part 9: Why This Design Works

**Single uniform flow for everything:**
The kernel doesn't care what kind of file it's opening. It always does:
resolve path → get inode → inode.ops.CreateFD() → fd.Open() → done.
The polymorphism handles the rest. This is exactly how Linux works —
the VFS doesn't know about ext4 or procfs, it just calls the ops.

**Ops are a factory, nothing more:**
`inode.ops.CreateFD()` returns an FD. After that, the kernel never
touches ops again. The FD is self-contained. In Linux, this is
`inode->i_fop` being copied into `file->f_op` at open time — same
idea, the file (FD) carries its own behavior from that point on.

**Per-open state lives in the FD:**
Two opens of the same file = two FDs with separate offsets, separate
caches, separate everything. The inode is shared (it describes the
file), but the FD is per-open (it describes one particular session
with that file).

**Disk and virtual files are interchangeable:**
The FD holds an inode number. It doesn't know or care whether that
number points to a disk inode or a virtual one. The kernel's
`getInode()` handles the distinction. Path resolution is uniform.


---


## Part 10: Directory Operations — ReadEntries

Path resolution needs to look up child entries **before any FD exists**
— no process has opened anything yet, the kernel is just walking the
path tree. That's why `ReadEntries()` lives on `InodeOperations`
alongside `CreateFD()`.

For non-directory ops (RegularFileOps, ProcInfoOps, etc.), `ReadEntries`
returns nil. For directory ops, it returns `[]DirEntry` — just inode
number and name, nothing else. The type of the child gets figured out
later when `getInode` loads the inode and attaches ops.

```go
type DirEntry struct {
    Inum uint32
    Name string
}
```

### Who Calls ReadEntries

Path resolution. When the kernel walks `/proc/processes`, it hits each
directory and calls `ReadEntries()`:

```
resolvePath("/proc/processes"):
  "/" → inode 2 (root, disk dir)
       DiskDirOps.ReadEntries() → reads/parses data blocks from disk
       Find "proc" → inum 10000

  "proc" → inode 10000 (virtual dir)
       ProcDirOps.ReadEntries() → generates entries from code
       Find "processes" → inum 10001

  done → inode 10001
```

The path walker doesn't know or care whether entries come from disk
blocks or from Go code. It just calls `ReadEntries()` on whatever
ops the directory inode has.

### Disk Directory ReadEntries (DiskDirOps)

Reads actual data blocks and parses the fixed-size 64-byte entries
(format defined in FILESYSTEM.md).

```go
type DiskDirOps struct {
    inodeNum uint32
}

func (d *DiskDirOps) ReadEntries(kernel *Kernel) []DirEntry {
    inode := kernel.getInode(d.inodeNum)
    var entries []DirEntry
    for _, blockNum := range inode.direct {
        if blockNum == 0 { break }
        blockData := kernel.fs.ReadBlock(blockNum)
        entries = append(entries, decodeDirEntries(blockData)...)
    }
    return entries
}
```

### Virtual Directory ReadEntries (ProcDirOps)

No disk involved. Entries are hardcoded + dynamically generated from
kernel state.

```go
type ProcDirOps struct{}

func (d *ProcDirOps) ReadEntries(kernel *Kernel) []DirEntry {
    entries := []DirEntry{
        {PROC_ROOT_VNODE, "."},
        {2, ".."},
        {PROC_PROCESSES_VNODE, "processes"},   // hardcoded: 10001
        {PROC_MEMINFO_VNODE, "meminfo"},       // hardcoded: 10002
    }
    // Dynamic: one entry per running process
    for _, proc := range kernel.processes {
        entries = append(entries, DirEntry{
            Inum: PROC_PID_BASE + uint32(proc.PID),  // 20000 + pid
            Name: strconv.Itoa(proc.PID),
        })
    }
    return entries
}
```

### Non-Directory Ops

Simply return nil:

```go
func (r *RegularFileOps) ReadEntries(kernel *Kernel) []DirEntry { return nil }
func (p *ProcInfoOps) ReadEntries(kernel *Kernel) []DirEntry    { return nil }
func (p *ProcPidOps) ReadEntries(kernel *Kernel) []DirEntry     { return nil }
```

### ReadEntries vs CreateFD — Why Both?

`ReadEntries` and `CreateFD` serve different callers at different times:

```
ReadEntries  →  called by the kernel's path resolver
                no FD exists, no process involved
                "what children does this directory have?"

CreateFD     →  called when a process opens the directory (e.g. ls /proc)
                creates an FD to serve read() calls
```

The DirectoryFD reuses `ReadEntries` internally when serving `read()`
calls — same data, just wrapped with offset tracking and caching.

**Comparison to Linux:**

| Linux | byte-space | Called by |
|-------|-----------|-----------|
| `i_op->lookup()` | `ops.ReadEntries()` | Path resolution (kernel) |
| `i_fop->readdir()` | `fd.Read()` on DirectoryFD | User process reading the dir |


---


## Part 11: The Directory FD

When a process opens a directory (e.g., `ls /etc`), it gets a
`DirectoryFD`. This is just a read-only FD whose "contents" are a
formatted listing of the directory's entries.

```go
type DirectoryFD struct {
    kernel   *Kernel
    inodeNum uint32
    offset   uint64
    flags    int
    ops      InodeOperations  // to call ReadEntries
    cached   []byte           // formatted entry list
}

func (d *DirectoryFD) Read(buf []byte) (int, error) {
    if d.cached == nil {
        entries := d.ops.ReadEntries(d.kernel)
        var b strings.Builder
        for _, e := range entries {
            fmt.Fprintf(&b, "%s\t%d\n", e.Name, e.Inum)
        }
        d.cached = []byte(b.String())
    }
    if d.offset >= uint64(len(d.cached)) {
        return 0, io.EOF
    }
    n := copy(buf, d.cached[d.offset:])
    d.offset += uint64(n)
    return n, nil
}

func (d *DirectoryFD) Write(data []byte) (int, error) {
    return 0, ErrIsDirectory  // can't write to a directory
}

func (d *DirectoryFD) Close() error {
    d.cached = nil
    return nil
}
```

The DirectoryFD reuses `ReadEntries` from the ops — same function
that path resolution uses. The only difference is the FD wraps it
with offset tracking, caching, and formatting.

A regular file's contents come from disk blocks. A proc file's
contents come from kernel state. A directory's contents come from
its entries. Same FD pattern, different data source.


---


## Part 12: Creating and Deleting Files

These are **kernel methods**, not FD operations. No FD is involved.

When you create or delete a file, no one has "opened" the parent
directory. There's no session to manage, no offset to track. The
kernel directly modifies the parent directory's data blocks and the
inode bitmap.

### Creating a File

```
touch /etc/newfile

1. Resolve "/etc" → inode 5 (the parent directory)

2. Allocate a new inode from the bitmap
   → inode 47 (first free slot)

3. Initialize the new inode on disk:
   inode 47: fType=S_IFREG, size=0, refs=1, no blocks

4. Add a directory entry to inode 5's data blocks:
   existing: ["passwd" → 17, "hosts" → 18]
   now:      ["passwd" → 17, "hosts" → 18, "newfile" → 47]

5. Write both the new inode and the updated directory blocks to disk
```

```go
func (k *Kernel) Create(proc *Process, path string, fType InodeType) (int, error) {
    parentPath, name := splitPath(path)       // "/etc", "newfile"
    parentInode := k.resolveAndGetInode(parentPath)

    newInodeNum := k.fs.AllocInode()           // grab from bitmap
    newInode := &Inode{fType: fType, refs: 1}
    k.fs.WriteInodeToDisk(newInodeNum, newInode)

    k.addDirEntry(parentInode, name, newInodeNum)  // update parent's blocks

    return k.Open(proc, path, O_RDWR)          // open it and return fd
}
```

### Deleting a File

```
rm /etc/newfile

1. Resolve "/etc" → inode 5 (parent directory)

2. Find "newfile" → inode 47 in the directory entries

3. Remove the entry from inode 5's data blocks:
   now: ["passwd" → 17, "hosts" → 18]

4. Decrement inode 47's ref count (refs--)

5. If refs == 0:
   - Free all data blocks (return to block bitmap)
   - Free inode 47 (return to inode bitmap)
```

```go
func (k *Kernel) Delete(proc *Process, path string) error {
    parentPath, name := splitPath(path)
    parentInode := k.resolveAndGetInode(parentPath)

    inodeNum := k.lookupInDir(parentInode, name)
    k.removeDirEntry(parentInode, name)

    inode := k.getInode(inodeNum)
    inode.refs--
    if inode.refs == 0 {
        k.fs.FreeBlocks(inode)
        k.fs.FreeInode(inodeNum)
        delete(k.inodeCache, inodeNum)
    } else {
        k.fs.WriteInodeToDisk(inodeNum, inode)
    }
    return nil
}
```

### Creating a Directory (mkdir)

Same flow as file creation, but with `fType=S_IFDIR` and the new
directory's data blocks are initialized with `.` and `..` entries:

```go
func (k *Kernel) Mkdir(proc *Process, path string) error {
    parentPath, name := splitPath(path)
    parentInode := k.resolveAndGetInode(parentPath)

    newInodeNum := k.fs.AllocInode()
    newInode := &Inode{fType: S_IFDIR, refs: 1}

    // Initialize with . and .. entries
    block := k.fs.AllocBlock()
    newInode.direct[0] = block
    entries := []DirEntry{
        {".", newInodeNum},
        {"..", parentInode.num},
    }
    k.fs.WriteBlock(block, encodeDirEntries(entries))

    k.fs.WriteInodeToDisk(newInodeNum, newInode)
    k.addDirEntry(parentInode, name, newInodeNum)
    return nil
}
```

### Why the Kernel, Not the FD?

```
read(), write(), close()    →  FD methods  (per-open session state)
create(), delete(), mkdir()  →  Kernel methods  (one-shot structural mutations)
```

An FD exists to manage a process's ongoing session with a file —
offset tracking, caching, per-open state. Creating/deleting files
isn't a session. There's nothing to track, no offset to advance.
The kernel manipulates directory blocks directly.

**In Linux this is the same split:** `create()`, `unlink()`, `mkdir()`
are on `inode_operations` (the parent directory's inode), not on
`file_operations` (the FD). The kernel manipulates directory blocks
directly through the filesystem driver without ever allocating a
`struct file`.

### addDirEntry / removeDirEntry

These are internal kernel helpers that read/write the parent
directory's data blocks directly — no FD, no offset tracking:

```go
func (k *Kernel) addDirEntry(parentInode *Inode, name string, inodeNum uint32) {
    blockNum := parentInode.direct[0]
    blockData := k.fs.ReadBlock(blockNum)

    entry := encodeDirEntry(name, inodeNum)
    blockData = append(blockData, entry...)

    k.fs.WriteBlock(blockNum, blockData)
    parentInode.size += uint32(len(entry))
    k.fs.WriteInodeToDisk(parentInode.num, parentInode)
}

func (k *Kernel) removeDirEntry(parentInode *Inode, name string) {
    blockNum := parentInode.direct[0]
    blockData := k.fs.ReadBlock(blockNum)

    blockData = removeDirEntryFromBlock(blockData, name)

    k.fs.WriteBlock(blockNum, blockData)
    parentInode.size = uint32(len(blockData))
    k.fs.WriteInodeToDisk(parentInode.num, parentInode)
}
```


---


## Future Ideas (not yet designed)

- Capability-based security (FDs become capabilities with embedded permissions)
- First-class DB support in the kernel (new FD type + syscall)
- Bitmap allocation using Plan 9 assembly for first-free scan
