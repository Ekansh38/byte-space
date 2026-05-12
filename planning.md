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
