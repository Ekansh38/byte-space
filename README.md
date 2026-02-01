<div align="center">

<img width="400" alt="Screenshot 2026-01-09 at 7 30 26 PM" src="https://github.com/user-attachments/assets/35ab24fc-0016-49a1-866d-3f9782d589c9" />

# byte-space

**Simulating the Early Internet**

[![Built with Go](https://img.shields.io/badge/Built%20with-Go-00ADD8?style=flat&logo=go)](https://go.dev)

**Terminal-Based Network Simulation** • **Real-Time Packet Visualization** • **Custom Markup Language**

*Experience the internet circa 1986 — Telnet, FTP, SMTP, and packet tracing*

---

</div>

Terminal-based internet simulator from the early internet era. Build networks, browse websites in terminals, send email, and watch packets flow in real-time.

byte-space creates a simulated internet environment where you can spawn virtual computers, connect them with networks, and watch packets travel in real-time as you browse websites and send email—all in your terminal.

## What It Does

- **Virtual Computers** - Spawn nodes with their own filesystems and shells
- **Network Simulation** - Connect machines and watch packets flow between them
- **Terminal Web Browser** - Browse websites rendered in your terminal using a custom markup language
- **Email System** - Send mail between virtual machines using SMTP
- **Custom Protocols** - Built-from-scratch implementations of DNS, Telnet, HTTP, and more
- **Packet Visualization** - Real-time graphical view of network traffic using Ebiten

The simulation uses a tick-based system where you can control network speed—from instant delivery to slow-motion packet travel for learning and visualization.

## Architecture

The system consists of four programs that communicate via Unix domain sockets:

- **Simulation Engine** - Core network simulator managing virtual computers and packet routing
- **Admin CLI** - Create and configure nodes, manage network topology
- **User CLI** - Connect to virtual machines, run commands, browse the network
- **Visualizer** - Real-time packet flow animation

See [ARCHITECTURE.md](ARCHITECTURE.md) for detailed design documentation.

## License

MIT
