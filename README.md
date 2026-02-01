<div align="center">

<img width="400" alt="byte-space logo" src="https://github.com/user-attachments/assets/35ab24fc-0016-49a1-866d-3f9782d589c9" />

<br>

# byte-space

**Simulating the Early Internet**

[![Built with Go](https://img.shields.io/badge/Built%20with-Go-00ADD8?style=flat&logo=go)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

**Terminal-Based Network Simulation** • **Real-Time Packet Visualization** • **Custom Markup Language**

_Experience the early internet — Telnet, FTP, SMTP, and packet tracing_

[Installation](#installation) • [Quick Start](#quick-start) • [Features](#features) • [Architecture](#architecture) • [Contributing](#contributing)

---

</div>

Terminal-based internet simulator from the early internet era. Build networks, browse websites in terminals, send email, and watch packets flow in real-time.

byte-space creates a simulated internet environment where you can spawn virtual computers, connect them with networks, and watch packets travel in real-time as you browse websites and send email—all in your terminal.

## Features

- **Virtual Computers** - Spawn nodes with their own filesystems and shells
- **Network Simulation** - Connect machines and watch packets flow between them
- **Terminal Web Browser** - Browse websites rendered in your terminal using a custom markup language
- **Email System** - Send mail between virtual machines using SMTP
- **Custom Protocols** - Built-from-scratch implementations of DNS, Telnet, HTTP, and more
- **Packet Visualization** - Real-time graphical view of network traffic using Ebiten
- **Time Control** - Tick-based system with adjustable network speed (instant to slow-motion)

## Installation

### From Source

```bash
git clone https://github.com/Ekansh38/byte-space.git
cd byte-space
go build ./...
```

### Using Go Install

```bash
go install github.com/Ekansh38/byte-space@latest
```

> **Note:** Requires Go 1.21 or higher

## Quick Start

Coming soon.

## Architecture

The system consists of four programs that communicate via Unix domain sockets:

- **Simulation Engine** - Core network simulator managing virtual computers and packet routing
- **Admin CLI** - Create and configure nodes, manage network topology
- **User CLI** - Connect to virtual machines, run commands, browse the network
- **Visualizer** - Real-time packet flow animation

See [ARCHITECTURE.md](ARCHITECTURE.md) for detailed design documentation.

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

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

## License

MIT - see [LICENSE](LICENSE) file for details
