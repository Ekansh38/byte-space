# byte-space Architecture

## Overview

Terminal-based internet simulator from the early internet era. Build networks, browse websites in terminals, send email, watch packets flow in real-time. Learn networking fundamentals hands-on.

Language: Go

## System Architecture

### Multi-Process Design

Four separate programs:

- Simulation Engine - Manages all virtual computers, routes packets, handles state
- Admin CLI - Spawn/manage nodes, configure network
- User CLI - Connect to nodes, run commands
- Visualizer - Real-time packet animation (Ebiten GUI)
- Launcher - Start/stop all programs, opens new terminal windows

### Communication

IPC: Unix domain socket (/tmp/byte-space.sock)

- All programs connect to engine via socket
- JSON message protocol
- Fast local communication

### Two-Layer Communication

#### Layer 1: External IPC (CLI ↔ Engine)

Purpose: Real communication between programs
Format: JSON over Unix socket
Speed: Instant (no simulation)

Request:
```json
{
  "program": "client|admin|viz",
  "request_id": 1,
  "ip": "192.fake.2.5",
  "command": "ls"
}
```

Response:
```json
{
  "request_id": 1,
  "output": "home var etc",
  "error": null
}
```

Request ID: Incrementing number per client (1, 2, 3...)

#### Layer 2: Virtual Packets (Node ↔ Node)

Purpose: Simulated network inside engine
Format: Custom packet structure
Speed: Configurable delay (0 to 100+ ticks)
Packet travels through network with visual animation

## Virtual Computers

### Structure

- Each virtual computer = Go struct + goroutine
- Runs INSIDE engine process (not separate programs)
- Stored in map: `map[string]*VirtualComputer`
- Key: IP address or node name

### Components Per Node

- Virtual filesystem (afero)
- Shell instance
- Packet queue (Go channel)
- Services list (HTTP, SMTP, etc.)
- Own goroutine for packet processing

### Authentication & Config Files

Each node has files in virtual filesystem:

- /etc/passwd - User accounts (username:password:uid:gid:fullname:homedir:shell)
- /etc/issue - Pre-login banner (customizable per machine)
- /etc/motd - Post-login message (customizable per machine)
- /etc/prompt - Shell prompt template (customizable per machine)
- /etc/hostname - Machine name
- /etc/services - Running services

Engine reads these files to handle login/display. Users can edit files to customize behavior.

## What's Built Custom

- Custom Shell Language - Interactive bash-like interpreter
- ByteShell Scripting - Automation language (built in phases)
- Custom Markup Language - HTML-like for terminal rendering
- HTTP Protocol - Web server for terminal websites
- SMTP Protocol - Email system
- DNS Protocol - Domain resolution
- Telnet Protocol - Remote shell access
- Packet Router - Routes packets through virtual network
- Terminal Renderer - Renders markup in terminal
- Visualizer - Real-time packet animation

## What Uses Libraries

- afero - Virtual filesystems (don't reinvent POSIX)
- Ebiten - 2D visualization
- Go stdlib - Everything else (networking, concurrency, terminal)

## Simulation System

### Tick-Based

- 100ms per tick (10 ticks/second)
- Engine increments tick counter
- Packets have arrival tick
- Animation based on progress (0.0 to 1.0)

### Configurable Speed

Presets in config.json:

- instant - 0 ticks (no delay, packets teleport)
- fast - 1-2 ticks (~0.1-0.2s)
- normal - 5-10 ticks (~0.5-1s) [DEFAULT]
- slow - 20-30 ticks (~2-3s)
- very-slow - 50-100 ticks (~5-10s)

Purpose:

- Instant mode = practical use
- Slow mode = teaching/learning (see packets clearly)
- User chooses based on goal

## ByteShell Scripting (Phased Development)

Phase 1: Sequential Commands (1 day)
Run commands from file, one after another. No variables, no loops.

Phase 2: Variables (3 days)
$var = "value" support

Phase 3: Conditionals (1 week)
if/then/else support

Phase 4: Loops (1 week)
for and while support

Each phase ships independently. Stop at any phase.

## Code Organization

### Engine Struct

- Network-level operations
- Routing, tick system
- Managing simulation
- IPC message handling

Files:

- engine.go - Core struct + methods
- ipc.go - IPC handling
- nodes.go - Node management

### VirtualComputer Struct

- Node-specific operations
- Execute commands
- Handle packets
- Authenticate users

### Shell Struct

- Parse commands
- Execute built-ins
- Handle shell operations

Methods can be split across multiple files in same package.

## Data Storage

In-Memory (RAM):

- Fast access
- No persistence needed (simulation is ephemeral)
- Can add save/load later if desired

Map structure:
```go
nodes map[string]*VirtualComputer
```

Access via Engine methods:

- GetNode(ip) - Find node
- SpawnNode(name, ip, type) - Create node
- ListNodes() - Get all nodes

## Terminal Handling

Separate file: terminal.go (config-based, AI-written)
Config: config.json or ~/.byte-space/config.json

Features:

- Auto-detects terminal (Ghostty support)
- Opens new windows for CLIs
- Configurable preferred terminal
- Fallback to common terminals

User configures once, system handles rest.

## Development Philosophy

### Iterative & Feature-Driven

- Ship features incrementally
- Each feature has deadline
- Always have working version
- Stop at any milestone

### Good Enough > Perfect

- First version doesn't need to be perfect
- Ship and iterate
- Keep momentum

### One Feature at a Time

- Focus on current feature only
- Ship, celebrate, move to next
- No overwhelm

## Build Milestones

MVP (~4 months):

- Simulation engine + virtual nodes
- IPC + CLI tools
- Networking + packet routing
- Visualization
- DNS + HTTP + markup language
- ByteShell v1 (sequential)

v1.0 (~6 months):

- SMTP email
- Telnet remote access
- ByteShell v2 (variables)
- Polish + documentation

v1.5+ (~8+ months):

- ByteShell v3/v4 (conditionals/loops)
- Security scenarios
- Advanced features

Each milestone is fully functional and portfolio-ready.

## Current Focus

Building local command execution first:

- Engine receives IPC message
- Parses JSON (program, request_id, ip, command)
- Finds VirtualComputer by IP
- Calls node.ExecuteCommand(command)
- Returns output via IPC

Virtual network packets come after local commands work.
