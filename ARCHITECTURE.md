# byte-space Architecture

## System Design

Four separate programs communicate via Unix domain socket:

- **Simulation Engine** - Manages virtual computers, routes packets, handles state (backend work)
- **Admin CLI** - Spawn/manage nodes, configure network
- **User CLI** - Connect to nodes, run commands
- **Visualizer** - Packet animation (Ebiten)
- **Launcher** - Launches these other four programs

## Communication Layers

### IPC (CLI ↔ Engine)


#### Client -> Engine

Program string ("admin", "user")
SessionID string ("session-x")
Command string ("ls")


#### Engine -> Client

SessionID string ("session-x")
Commands string[] 

Commands I need:

MY OWN ANSI. BANASI.




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
- Run()

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

### Package: client (admin or user)

uses mode param to distinguish between admin or user.


Connects to simulation engine.
Takes input and sends ICP messages to simulation engine.

### Package: launcher

launches admin panel, simulation engine and user cli in different terminals, but simulation engine in background.
