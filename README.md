# byte-space

Terminal-based internet simulator from the early internet era. Build networks, browse websites in terminals, send email, and watch packets flow in real-time.

Written in Go.

## What It Does

byte-space creates a simulated internet environment where you can:

- Spawn virtual computers with their own filesystems and shells
- Connect nodes using simulated network protocols
- Watch packets travel through the network in real-time
- Browse websites rendered in your terminal
- Send email between virtual machines
- Learn how networking fundamentals work hands-on

Think of it as a complete network in a box—virtual computers, routers, protocols, and all—running entirely in your terminal.

## Architecture

The system consists of four separate programs:

- **Simulation Engine** - Manages all virtual computers, routes packets, handles simulation state
- **Admin CLI** - Spawn and configure nodes, manage the network
- **User CLI** - Connect to nodes, run commands, interact with the simulated internet
- **Visualizer** - Real-time packet animation using Ebiten

All programs communicate via Unix domain sockets using a JSON protocol. Virtual computers run inside the engine as goroutines, each with their own filesystem (using afero), shell instance, and packet queue.

## What's Custom Built

Everything networking-related is built from scratch for educational purposes:

- Custom shell interpreter (ByteShell)
- Markup language for terminal rendering (think HTML for terminals)
- HTTP server for terminal-based websites
- SMTP for email
- DNS for domain resolution
- Telnet for remote access
- Packet routing system
- Terminal renderer

The only libraries used are afero (virtual filesystems), Ebiten (visualization), and Go's standard library.

## How Simulation Works

The engine uses a tick-based system (100ms per tick). Packets travel through the network over multiple ticks, allowing you to watch them move in real-time. Speed is configurable:

- `instant` - No delay, packets arrive immediately
- `fast` - 1-2 ticks (~0.1-0.2s)
- `normal` - 5-10 ticks (~0.5-1s) [default]
- `slow` - 20-30 ticks (~2-3s)
- `very-slow` - 50-100 ticks (~5-10s)

Use instant mode for practical work, slower modes for learning and visualization.

## Shell Commands

The custom shell includes standard Unix commands:

```
pwd, ls, mkdir, cd, rm, cp, mv, cat, echo
```

Plus networking commands:

```
curl, telnet, mail, dig, traceroute, ping
```

Each virtual computer has its own authentication system with `/etc/passwd`, customizable banners (`/etc/issue`, `/etc/motd`), and shell prompts (`/etc/prompt`).

## Current Status

Active development. Building core features incrementally with working milestones at each stage. See [ARCHITECTURE.md](ARCHITECTURE.md) for detailed design documentation.

## License

MIT
