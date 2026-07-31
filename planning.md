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

```
Kernel.Open(proc, path, flags):
  1. resolvePath(path) → inodeNum          (Linux: namei / i_op->lookup)
  2. getInode(inodeNum) → inode            (Linux: iget_locked)
     - disk inode: load 128 bytes, attach ops by fType
     - virtual: already in cache with ops set
  3. canAccess(proc, inode, flags)         (Linux: inode_permission)
  4. inode.ops.CreateFD(...) → fd          (Linux: alloc_file + copy i_fop)
  5. fd.Open() — validate; may fail        (Linux: f_op->open)
  6. allocate fdNum, install in proc.FDs   (Linux: fd_install)
```

### After Open: Read, Write, Close

The kernel just dispatches to the FD. No type checking, no switch
statements. The FD knows what to do because it's the right concrete type.

```
Kernel.Read/Write(proc, fdNum, ...):
  fd = proc.FDs[fdNum]  (nil → ErrBadFD)
  dispatch to fd.Read(buf) / fd.Write(data)

Kernel.Close(proc, fdNum):
  same lookup, call fd.Close(), then delete(proc.FDs, fdNum)
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

```
Kernel.getInode(num):
  1. cache hit → return
  2. num >= PROC_PID_BASE → build virtual per-pid inode
     ops = ProcPidOps{PROC_PID_STATUS}, cache, return
  3. else disk inode → read 128 bytes, attach ops by fType:
       S_IFREG → RegularFileOps
       S_IFDIR → DirectoryOps
     cache, return
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

Root dir already has "proc" → 10000 on disk from mkfs. `initProcFS`
just registers the virtual inodes in the cache:

```
Kernel.initProcFS():
  cache[PROC_ROOT_VNODE]      = Inode{ops: ProcDirOps{}}
  cache[PROC_PROCESSES_VNODE] = Inode{ops: ProcInfoOps{PROC_PROCESSES}}
  cache[PROC_MEMINFO_VNODE]   = Inode{ops: ProcInfoOps{PROC_MEMINFO}}
```

```go
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

type RegularFileFD struct {
    kernel   *Kernel
    inodeNum uint32
    offset   uint64
    flags    int
}
```

```
CreateFD: build RegularFileFD{inodeNum, offset:0, flags}
Open():   nothing to validate
Read(buf):
  - block = inode.direct[offset / BLOCKSIZE]
  - copy from block[offset % BLOCKSIZE:] into buf
  - advance offset, return n
Write(data): TODO — write to blocks, allocate as needed, update inode.size
Close(): nothing to clean up
```

### ProcInfoFD (system-wide proc files)

```go
type ProcInfoOps struct {
    infoType ProcInfoType  // PROC_PROCESSES, PROC_MEMINFO
}

type ProcInfoFD struct {
    kernel   *Kernel
    inodeNum uint32
    offset   uint64
    flags    int
    infoType ProcInfoType
    cached   []byte          // generated once per open, then served from cache
}
```

```
CreateFD: build ProcInfoFD{infoType: ops.infoType, cached: nil}
Open():   nothing to validate
Read(buf):
  - if cached == nil, generate from kernel state per infoType:
      PROC_PROCESSES: "PID\tNAME\tSTATE\n" header + one row per kernel.processes
      PROC_MEMINFO:   "MemTotal: X\nMemFree: Y\n"
  - copy from cached[offset:] into buf, advance offset (EOF when done)
Close(): clear cached
```

### ProcPidFD (per-process proc files)

```go
type ProcPidOps struct {
    pidType ProcPidType  // STATUS, CMDLINE
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
```

```
CreateFD: extractPidFromPath(path) → pid; build ProcPidFD{pid, pidType, cached: nil}
Open():   kernel.processes[pid] == nil → ErrNoSuchProcess
Read(buf):
  - if cached == nil, generate per pidType:
      PROC_PID_STATUS:  "Name: X\nPid: N\nState: Y\n"
      PROC_PID_CMDLINE: proc.Args joined with '\x00'
  - copy from cached[offset:], advance offset (EOF when done)
Close(): clear cached
```

### SocketFD

```go
type SocketFD struct {
    kernel   *Kernel
    inodeNum uint32
    flags    int
    socket   *Socket
}
```

```
Open():
  - socket.state == CLOSED → ErrSocketClosed
  - lazy-init recvBuf channel (cap 100)
Read(buf):  pull from socket.recvBuf, copy into buf
Write(data): push to socket.sendBuf, return len(data)
Offset()/SetOffset(): no-op (streaming, no seek)
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
```

```
ReadEntries:
  - walk inode.direct, stop at first zero blockNum
  - for each block: ReadBlock → decodeDirEntries → append
  - return entries
```

### Virtual Directory ReadEntries (ProcDirOps)

No disk involved. Entries are hardcoded + dynamically generated from
kernel state.

```go
type ProcDirOps struct{}
```

```
ReadEntries:
  - hardcoded: ".", "..", "processes" (10001), "meminfo" (10002)
  - dynamic: for each proc in kernel.processes,
      append DirEntry{Inum: 20000+pid, Name: itoa(pid)}
```

### Non-Directory Ops

`RegularFileOps`, `ProcInfoOps`, `ProcPidOps` all return `nil` from
`ReadEntries` — they aren't directories.

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
```

```
Read(buf):
  - if cached == nil: ops.ReadEntries → format as "name\tinum\n" lines → cached
  - copy from cached[offset:], advance offset (EOF when done)
Write(data): ErrIsDirectory
Close():     clear cached
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
6. Open the new file and return the fd
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
   - Evict from inode cache
   Else: persist the updated inode to disk
```

### Creating a Directory (mkdir)

Same flow as file creation, but with `fType=S_IFDIR` and one extra
step — allocate a data block and seed it with `.` (→ new inode) and
`..` (→ parent inode) before adding the entry to the parent.

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

Internal kernel helpers that read/write the parent directory's data
blocks directly — no FD, no offset tracking:

```
addDirEntry(parent, name, inodeNum):
  - read parent.direct[0]
  - append encodeDirEntry(name, inodeNum) to block data
  - write block back, update parent.size, persist parent inode

removeDirEntry(parent, name):
  - read parent.direct[0]
  - remove the named entry from block data
  - write block back, update parent.size, persist parent inode
```


---


## Future Ideas (not yet designed)

- Capability-based security (FDs become capabilities with embedded permissions)
- First-class DB support in the kernel (new FD type + syscall)
- Bitmap allocation using Plan 9 assembly for first-free scan


---


## Session: 2026-07-29 — BS-EXTFS Status Scope


### Where Things Stand Right Now

**Solid and tested (disk layer):**
- `MemDisk`, `Disk` interface, `ReadBlock`/`WriteBlock`
- `readDataBlock`/`writeDataBlock`
- SuperBlock struct, serialization, load on boot
- `readInode`/`writeInode` — full binary serialization, round-trip tested
- `AllocInode`/`FreeInodeFromBitmap`
- `Falloc` — grow and shrink path, all indirection levels (direct/find/sind/tind), tested
- `findFreeBit`/`setBit`/`clearBit`/`virtualToPlaceRelative`
- `encodeDirEntry`/`decodeDirEntries`
- Root inode (inum=2) created on fresh format
- `NewTestFileSystem()` test harness

**Stub or partially broken (interface layer):**
- `InodeOperations` and `FD` interfaces defined but nothing real implements them yet
- `DirectoryOps.ReadEntries` — loop body walks blocks but never accumulates entries and has no return statement (explicit TODO in code, won't compile cleanly)
- `DirectoryFD` — all methods are stubs returning 0/nil
- `ResolvePath` — multiple known bugs: `root.ops` is commented out (nil panic), cache miss on child inode dereferences nil pointer, no ops assigned on loaded inodes, infinite loop if name not found in a dir

**Not started:**
- `addDirEntry` / `removeDirEntry`
- Root dir `.` and `..` entries on fresh format (needs addDirEntry first)
- `getInode(inum)` helper (cache + load + ops assign in one place)
- `FileFD` — file read/write
- `SyncInode` / `SyncAll` (dirty flag exists on inode but nobody flushes it)
- First-boot FS tree via BS-EXTFS (currently still built with afero in `initFileSystem`)
- All syscalls still delegate to `k.computer.OS.*` (afero) with `// TODO(fs migration)` markers


### MVP Definition

ByteSpace boots and runs using **BS-EXTFS as its only filesystem**. afero is gone.

Concretely:
- `ls`, `cat`, `mkdir`, `touch`, `rm`, `login`, `adduser` all work through BS-EXTFS inodes
- `/etc/passwd`, `/etc/hostname`, `/etc/issue`, `/etc/motd` live in BS-EXTFS data blocks
- Permissions enforced via `inode.ownerMode`/`otherMode`, not the JSON FsMetaData sidecar
- `computer.go` no longer imports afero; `os.go`, `FsMetaData`, `saveMetaData`, `loadMetaData`, `populateFileMetadata` are deleted


### What Must Exist Before ByteSpace Can Use the Filesystem Normally

These are blocking dependencies in order — each one gates the next:

1. **`addDirEntry`** — everything that creates files, dirs, or populates root needs this
2. **Root dir `.`/`..` populated on format** — without this, path resolution panics at root
3. **`ReadEntries` completed** — path resolution calls this at every directory level
4. **`getInode(inum)` helper** — eliminates three separate cache+load+ops-assign code paths
5. **`ResolvePath` fixed** — all syscalls flow through this; currently nil-panics
6. **`FileFD`** — once path resolution works, reading and writing file content needs this
7. **`mkDir` / `createFile` ported to BS-EXTFS** — uses AllocInode + addDirEntry + writeInode
8. **`readFile` / `writeFile` ported to BS-EXTFS** — uses ResolvePath + FileFD
9. **First-boot FS tree built via BS-EXTFS** — replaces `initFileSystem(afero)`
10. **afero deleted** — the finish line; nothing in `computer/` imports it


### Next 5 Implementation Steps (high level)

**Step 1 — Finish directory data block primitives (`fs_dir.go`)**

finish the getInode kernel helper method.

The three raw operations everything above depends on:
- Write `addDirEntry(dirInode *inode, name string, inum uint32) error`: walk the inode's existing data blocks looking for a 64-byte slot where inum==0 (free). Encode and write it there. If no slot exists, call `Falloc(dirInode, dirInode.size+BLOCKSIZE)` to get a new block, then write to the first slot in that block. Write the data block back to disk. Persist the updated inode.
- Write `removeDirEntry(dirInode *inode, name string) error`: walk blocks, match by name, zero the 64 bytes, write block back. Leave the block allocated — no shrink for now.

**Step 2 — `getInode` helper + fix `ResolvePath` (`kernel.go`)**

These two go together because fixing ResolvePath requires getInode to be solid first.
- Add `func (k *Kernel) getInode(inum uint32) (*inode, error)` to Kernel: check `k.inodeCache[inum]`, if miss allocate `&inode{}`, call `fs.readInode`, set `.ops` based on `fType` (`S_IFDIR` → `&DirectoryOps{inodeNum: inum}`, `S_IFREG` → `&RegularFileOps{}`), store in cache, return.
- Rewrite the `ResolvePath` walk to use `getInode`. Fix the cache-miss nil dereference (currently reads into a nil pointer). Fix the infinite loop (currently never advances `dirs` when name isn't found). Add ENOTDIR guard when a non-dir appears mid-path. Wire in the root's ops on first load.

**Step 3 — Bootstrap root directory and initial FS tree (`fs.go`, `computer.go`)**

The first time the disk is formatted, it's completely empty — even the root inode has no data blocks.
- Immediately after `fs.writeInode(root, 2)` in `NewFileSystem`, call `addDirEntry` twice: `.` → inum 2, `..` → inum 2. This gives ResolvePath something to actually find.
- Port `initFileSystem` off afero. Replace the afero `Mkdir`/`Create`/`WriteString` calls with direct BS-EXTFS operations (AllocInode + writeInode + addDirEntry + Falloc + writeDataBlock) to create `/etc`, `/bin`, `/home`, `/var/log`, `/tmp` and the standard text files. Only run on first boot (`!isInitialized`).

**Step 4 — Implement `FileFD` (`fs_fd.go`)**

New file. This is the FD type for regular files — the thing that makes `read()`/`write()` work on file content.
- `FileFD{kernel *Kernel, inodeNum uint32, offset uint64, flags int}`
- `RegularFileOps.CreateFD` returns a `&FileFD{...}`
- `Read(buf)`: load the inode, compute which virtual block `offset` lands in using `virtualToPlaceRelative`, resolve to a physical data block number (walk direct/find/sind/tind), call `readDataBlock`, copy the right slice into `buf`, advance `offset`.
- `Write(data)`: mirror of Read. If `offset + len(data) > inode.size`, call `Falloc` first to extend, then write the block(s). Persist the updated inode.

**Step 5 — Migrate syscalls off afero, one at a time (`kernel.go`)**

Each syscall has a `// TODO(fs migration):` comment marking where to switch. Do them in order of difficulty:
1. `readFile`: ResolvePath → getInode → FileFD → fd.Open → fd.Read → return bytes
2. `writeFile`: ResolvePath → getInode → FileFD → fd.Write (with Falloc inside)
3. `mkDir`: ResolvePath parent → AllocInode → writeInode (S_IFDIR) → addDirEntry(parent) → addDirEntry(new, ".", "..") 
4. `createFile`: ResolvePath parent → AllocInode → writeInode (S_IFREG) → addDirEntry(parent)
5. `removeAll`: ResolvePath → removeDirEntry from parent → FreeInodeFromBitmap → free data blocks

Once all five are migrated and nothing references `k.computer.OS.*` or `k.computer.filesystem`, delete `os.go`, remove the `filesystem afero.Fs` field, delete `FsMetaData`/`saveMetaData`/`loadMetaData`/`populateFileMetadata`. That's the finish line.
