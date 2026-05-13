# FD + Socket Planning

## Two Type Systems

**Inode type** — filesystem identity, lives on disk, permanent:
- `S_IFREG`, `S_IFDIR`, `S_IFSOCK`
- Used by: stat, ls, chmod, rm

**FDBackend** — runtime behavior, lives in RAM, temporary:
- TTYBackend, InodeFDBackend, SocketFDBackend
- Used by: read, write

**Connection between them:** `open()` reads the inode type once, creates the right Backend, returns an fd int. After that the inode is never touched for read/write.

## FileDescription

```go
type FileDescription struct {
    Backend FDBackend  // all type-specific stuff lives here
    refs    int        // how many proc FD tables point here
}

type FDBackend interface {
    Read(proc *Process, ctx context.Context) (string, int)
    Write(data []byte) (int, error)
}
```

Backends:
- `TTYBackend` — wraps `*TTY` (no inode, created directly)
- `InodeFDBackend` — inodeIndex + offset, reads data blocks
- `SocketFDBackend` — reads/writes via channels

## Tables

- Kernel open file table — actual FileDescription objects
- Process FD table — pointers into kernel table (shallow copy on exec)
- refs counts how many proc tables point at a FileDescription, cleanup on 0

## Syscalls

| syscall | what it does |
|---------|-------------|
| open    | lookup inode type → create Backend → add to kernel table → return fd int |
| read    | proc.FDs[fd].Backend.Read() |
| write   | proc.FDs[fd].Backend.Write() |
| close   | decrement refs, cleanup if 0, remove from proc table |
| stat    | lookup inode metadata, no fd needed |
| socket  | create Socket object → wrap in SocketFDBackend → return fd, no inode yet |
| bind    | create S_IFSOCK inode + register in socketsByPath |
| listen  | alloc incoming chan, set state=LISTENING |
| connect | lookup socketsByPath, create connected pair, push to incoming |
| accept  | block on incoming chan, return new fd |

## Sockets

```go
type Socket struct {
    state    SocketState
    path     string
    incoming chan *Socket  // server accept queue
    recvBuf  chan []byte
    peer     *Socket
}
```

Kernel holds: `socketsByPath map[string]*Socket`

- bind()    → S_IFSOCK inode + socketsByPath entry (created together)
- rm()      → delete inode + socketsByPath entry (deleted together)
- read/write → channels only, inode never touched again





# File Descriptors vs Inodes - Complete Explanation

## The Core Distinction

**Inode:** On-disk structure describing a file
**File Descriptor (FD):** Per-process handle to an open file

---

## What is an Inode?

**Location:** Stored on disk in the inode table

**Contains:**
- File size
- File type (regular file, directory)
- Permissions (owner mode, other mode)
- Owner (username)
- Timestamps (created, modified)
- **Block pointers** (direct[12], indirect blocks)
- Hard link count (refs)

**Key Point:** Inodes exist whether file is open or not

**Example:**
Inode #42:
size: 1024 bytes
type: regular file
owner: "alice"
ownerMode: 0x7 (rwx)
otherMode: 0x4 (r--)
direct[0]: block 500
direct[1]: block 501
refs: 1

---

## What is a File Descriptor?

**Location:** Stored in process memory (per-process)

**Contains:**
- Reference to inode number
- Current offset (where you are in the file)
- Flags (read/write/append mode)

**Key Point:** FDs only exist while file is open by a process

**Example:**
Process 123's FD table:
FD 0 (stdin):  inode=1, offset=0, flags=READ
FD 1 (stdout): inode=2, offset=0, flags=WRITE
FD 2 (stderr): inode=2, offset=0, flags=WRITE
FD 3:          inode=42, offset=512, flags=READ
FD 4:          inode=99, offset=0, flags=WRITE

---

## The Relationship
Process
├─ FD 3 ────────┐
│               │
│               ├──> Inode #42 ──> direct[0] ──> Block 500 (actual data)
│               │                   direct[1] ──> Block 501 (actual data)
├─ FD 4 ────────┤
│
└──> Inode #99 ──> direct[0] ──> Block 600 (actual data)

**Flow:**
1. Process uses FD (FD 3)
2. FD points to inode number (42)
3. Kernel looks up inode 42
4. Inode has block pointers (500, 501)
5. Kernel reads blocks 500, 501
6. Returns data to process

---

## Complete Example: Reading a File

### Step 1: Open the file

**User code:**
```go
fd, err := proc.Open("/home/alice/notes.txt", READ)
// Returns: fd = 3
```

**What happens:**
1. Kernel resolves path → finds inode number (42)
2. Kernel checks permissions (can alice read inode 42?)
3. Kernel creates FD entry in process:
FD 3: inode=42, offset=0, flags=READ
4. Returns FD 3 to process

---

### Step 2: Read from the file

**User code:**
```go
data := make([]byte, 100)
n, err := proc.Read(fd, data)
```

**What happens:**
1. Process says "read from FD 3"
2. Kernel looks up FD 3 in process's FD table
   - Found: inode=42, offset=0
3. Kernel looks up inode 42 in inode table
   - Found: direct[0]=500, direct[1]=501
4. Kernel reads block 500 from disk
5. Kernel copies 100 bytes starting at offset 0
6. Kernel updates FD 3's offset: offset=100
7. Returns data to process

---

### Step 3: Read again

**User code:**
```go
n, err := proc.Read(fd, data)
```

**What happens:**
1. Process says "read from FD 3"
2. Kernel looks up FD 3: inode=42, **offset=100**
3. Kernel looks up inode 42: direct[0]=500
4. Kernel reads block 500, but starts at byte 100
5. Kernel updates FD 3's offset: offset=200
6. Returns data

**Key:** Offset is stored in FD, not inode

---

### Step 4: Close the file

**User code:**
```go
proc.Close(fd)
```

**What happens:**
1. Kernel removes FD 3 from process's FD table
2. Inode 42 still exists on disk
3. File can be opened again later (gets new FD)

---

## Why This Separation Matters

### Multiple processes can open same file

**Process A:**
FD 3: inode=42, offset=0

**Process B:**
FD 5: inode=42, offset=500

**Both point to inode 42, but have different:**
- FD numbers (3 vs 5)
- Offsets (0 vs 500)
- Can read independently

---

### Same process can open same file twice

**Process A:**
FD 3: inode=42, offset=0
FD 4: inode=42, offset=1000

**Both point to inode 42, but:**
- Different offsets
- Reading FD 3 doesn't affect FD 4
- Useful for reading different parts of file

---

## Data Structures in Code

### Inode (on disk)

```go
type Inode struct {
    size       uint32      // File size in bytes
    fType      InodeType   // Regular file or directory
    refs       uint16      // Hard link count
    owner      [14]byte    // Owner username
    setuid     bool        // Setuid bit
    ownerMode  uint8       // rwx for owner
    otherMode  uint8       // rwx for others
    direct     [12]uint32  // Direct block pointers
    find       uint32      // First indirect
    sind       uint32      // Second indirect
    tind       uint32      // Third indirect
    createdAt  uint64      // Unix timestamp
    modifiedAt uint64      // Unix timestamp
}
```

---

### File Descriptor (in process memory)

```go
type FileDescriptor struct {
    inodeNum uint32    // Which inode this FD points to
    offset   uint64    // Current position in file
    flags    int       // READ, WRITE, APPEND
}

type Process struct {
    PID  int
    FDs  map[int]*FileDescriptor  // FD number → descriptor
}
```

---

## Common Operations

### Open

```go
func (k *Kernel) Open(proc *Process, path string, flags int) (int, error) {
    // 1. Resolve path to inode number
    inodeNum := k.resolvePath(path)
    
    // 2. Check permissions
    inode := k.readInode(inodeNum)
    if !k.canAccess(proc, inode, flags) {
        return -1, ErrPermissionDenied
    }
    
    // 3. Allocate FD
    fd := k.allocateFD(proc)
    
    // 4. Create FD entry
    proc.FDs[fd] = &FileDescriptor{
        inodeNum: inodeNum,
        offset:   0,
        flags:    flags,
    }
    
    return fd, nil
}
```

---

### Read

```go
func (k *Kernel) Read(proc *Process, fd int, buf []byte) (int, error) {
    // 1. Look up FD
    fdEntry := proc.FDs[fd]
    if fdEntry == nil {
        return 0, ErrBadFD
    }
    
    // 2. Look up inode
    inode := k.readInode(fdEntry.inodeNum)
    
    // 3. Read data starting at offset
    n := k.readDataFromInode(inode, fdEntry.offset, buf)
    
    // 4. Update offset
    fdEntry.offset += uint64(n)
    
    return n, nil
}
```

---

### Write

```go
func (k *Kernel) Write(proc *Process, fd int, data []byte) (int, error) {
    // 1. Look up FD
    fdEntry := proc.FDs[fd]
    if fdEntry == nil {
        return 0, ErrBadFD
    }
    
    // 2. Look up inode
    inode := k.readInode(fdEntry.inodeNum)
    
    // 3. Write data at offset
    n := k.writeDataToInode(inode, fdEntry.offset, data)
    
    // 4. Update inode size if grew
    if fdEntry.offset + uint64(n) > uint64(inode.size) {
        inode.size = uint32(fdEntry.offset + uint64(n))
        k.writeInode(fdEntry.inodeNum, inode)
    }
    
    // 5. Update offset
    fdEntry.offset += uint64(n)
    
    return n, nil
}
```

---

### Close

```go
func (k *Kernel) Close(proc *Process, fd int) error {
    // 1. Check FD exists
    if proc.FDs[fd] == nil {
        return ErrBadFD
    }
    
    // 2. Remove FD from process
    delete(proc.FDs, fd)
    
    // Note: Inode still exists on disk
    
    return nil
}
```

---

## Key Insights

1. **Inode = file metadata + data location**
   - Lives on disk
   - Permanent (until file deleted)
   - Shared by all opens

2. **FD = handle to access inode**
   - Lives in process memory
   - Temporary (while file open)
   - Per-process, per-open

3. **Offset is in FD, not inode**
   - Each open has own offset
   - Reading FD 3 doesn't affect FD 4
   - Multiple processes can read independently

4. **FD → inode → blocks → data**
   - FD is the handle
   - Inode describes file
   - Blocks hold actual data

---

## Why This Design?

**Flexibility:**
- Multiple processes can open same file
- Same process can open file multiple times
- Each has independent offset

**Efficiency:**
- Inode stored once (on disk)
- FD is lightweight (just number + offset)
- Kernel mediates all access

**Security:**
- Kernel checks permissions on open
- After open, FD is trusted
- Process can't bypass permissions

---

## Analogy

**Inode = House**
- Has address, size, owner
- Exists whether you're visiting or not

**FD = Key to house**
- You get key when you "open" (visit)
- Key lets you access house
- You can be at different room (offset)
- Return key when "close" (leave)
- House still exists after you leave

---

## Common Mistakes

### Mistake 1: Thinking FD stores data
❌ FD stores data
✅ FD points to inode, inode points to blocks with data

### Mistake 2: Thinking offset is in inode
❌ Inode has offset field
✅ Offset is in FD (per-open, not per-file)

### Mistake 3: Thinking FD is global
❌ FD 3 means same thing to all processes
✅ FD 3 in process A ≠ FD 3 in process B

### Mistake 4: Thinking closing FD deletes file
❌ Close() deletes the file
✅ Close() removes FD, file (inode) still exists

---

## Summary

**When you open a file:**
1. Kernel finds inode number from path
2. Kernel creates FD pointing to that inode
3. Returns FD to process

**When you read/write:**
1. Process gives FD to kernel
2. Kernel looks up inode from FD
3. Kernel reads/writes blocks from inode
4. Kernel updates offset in FD

**When you close:**
1. Kernel removes FD from process
2. Inode remains on disk
3. Can be opened again (new FD)

**Key relationship:**
Process → FD → Inode Number → Inode (on disk) → Block Pointers → Blocks (data)

