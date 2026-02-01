# byte-space Architecture

## System Design

Four separate programs communicate via Unix domain socket:

- **Simulation Engine** - Manages virtual computers, routes packets, handles state
- **Admin CLI** - Spawn/manage nodes, configure network
- **User CLI** - Connect to nodes, run commands
- **Visualizer** - Real-time packet animation (Ebiten)

## Communication Layers

### IPC (CLI ↔ Engine)
- Protocol: JSON over Unix socket (`/tmp/byte-space.sock`)
- Purpose: Real communication between programs
- Speed: Instant

Request format:
```json
{
  "program": "client|admin|viz",
  "request_id": 1,
  "ip": "192.fake.2.5",
  "command": "ls"
}
```

### Virtual Packets (Node ↔ Node)
- Protocol: Custom packet structure
- Purpose: Simulated network inside engine
- Speed: Configurable (0-100+ ticks)

## Virtual Computers

Each node is a Go struct with:
- Virtual filesystem (afero)
- Shell instance
- Packet queue (channel)
- Services (HTTP, SMTP, etc.)
- Own goroutine for packet processing

Stored as: `map[string]*VirtualComputer` keyed by IP

### Config Files
- `/etc/passwd` - User accounts
- `/etc/issue` - Login banner
- `/etc/motd` - Message of the day
- `/etc/prompt` - Shell prompt template
- `/etc/hostname` - Machine name
- `/etc/services` - Running services

## Simulation System

Tick-based timing (100ms/tick):
- **instant** - 0 ticks (no delay)
- **fast** - 1-2 ticks (~100-200ms)
- **normal** - 5-10 ticks (~500ms-1s)
- **slow** - 20-30 ticks (~2-3s)

## Custom Implementations

Built from scratch:
- Shell language & scripting
- Markup language for terminal rendering
- HTTP, SMTP, DNS, Telnet protocols
- Packet router
- Terminal renderer
- Packet visualizer

Using libraries:
- afero - Virtual filesystems
- Ebiten - 2D visualization
- Go stdlib - Everything else

## Code Structure

### engine package
**Engine struct** - Network operations, routing, tick system, IPC
- `engine.go` - Core struct + methods
- `ipc.go` - IPC handling
- `nodes.go` - Node management

**VirtualComputer struct** - Node operations, command execution, packet handling

### computer package
**Computer struct** - Node data and filesystem

### shell package
**Shell struct** - Command parsing and execution

## Project Structure

### Package: engine

#### Struct: Engine

Fields:
- Nodes map[string]\*computer.Computer // Keyed by IP address

Methods:
- HandleIPCMessage(data []byte, clientID string)
- GetNode(ip string) \*computer.Computer
- SendResponse(clientID string, requestID int, output string, err error)
- SpawnNode(name, ip, nodeType string)

Package Methods:
- NewEngine() \*Engine

#### Struct: Message

Fields:
- Program string
- RequestID int
- IP string
- Command string

Package Methods:
- NewMessage(data []byte) (\*Message, error)

#### Struct: Response

Fields:
- RequestID int
- Output string
- Error string

Package Methods:
- NewResponse(requestID int, output string, err error) \*Response

### Package: computer

#### Struct: Computer

Fields:
- Name string
- IP string
- Type string
- Filesystem afero.Fs

Package Methods:
- New(name, ip, nodeType string) \*Computer

### Package: shell

#### Struct: Shell

Fields:
- Comp \*computer.Computer

Methods:
- RunCommand(line string) (string, error)
- Parse
- Blabla shell stuf

Package Methods:
- New(comp \*computer.Computer) \*Shell
