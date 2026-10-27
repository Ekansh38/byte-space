<div align="center">

<img width="600" alt="byte-space logo" src="./assets/logo.png" />

<br>

# byte-space

**Simulating the Early Internet**

[![Built with Go](https://img.shields.io/badge/Built%20with-Go-00ADD8?style=flat&logo=go)](https://go.dev)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)

[Installation](#installation) • [Quick Start](#quick-start) • [Features](#features) • [Architecture](#architecture) • [Contributing](#contributing)

---

</div>

Terminal-based internet simulator from the early internet era. Build networks, browse websites in terminals, send email, and watch packets travel in real-time with the visualizer.


## Features

**Kernel + Process Model**
- Syscall API: `read`, `write`, `stat`, `exec`, `ioctl`, and more
- Process groups and foreground job control
- FD table inherited on exec, context-based cancellation
- Programs like `login` and `adduser` run with elevated permissions when needed using setuid

**TTY**
- Canonical and raw modes switchable at runtime
- Echo, password masking, cursor movement, tab expansion
- Ctrl-C routes SIGINT to the foreground process group, echoes `^C`, re-prompts
- SIGWINCH on terminal resize

**Shell**
- Line editing, command history (up/down arrows)
- Built-ins: `cd`, `pwd`, `exit`
- Programs: `ls`, `cat`, `clear`, `mkdir`, `touch`, `chmod`, `rm`, `adduser`, `v`

**Programs**
- `login` -- reads `/etc/issue`, authenticates against `/etc/passwd`, shows motd
- `adduser` -- creates users with masked password entry
- `v` -- terminal text editor, normal/insert modes, `Ctrl-S` to save
- standard unix utils: `ls -l`, `cat`, `chmod`, `rm`, `mkdir`, `touch`, `clear`

**Filesystem + Permissions**
- BS-EXTFS (Bull-Shit Extended FileSystem): a hand-rolled inode-based filesystem on a 64MB disk image — superblock, bitmaps, 128-byte inodes, direct + indirect block pointers, 64-byte directory entries. See [FILESYSTEM.md](/FILESYSTEM.md)
- Full owner/other `rwx` permission enforcement, stored on the inode
- `chmod` with `u`/`o`/`a` and `+`/`-`/`=` operators
- setuid programs (`login`, `adduser`) via the inode setuid bit
- Everything persists in each node's `disk.img`

## Installation

```bash
git clone https://github.com/Ekansh38/byte-space.git
cd byte-space
```

> **Note:** Requires Go 1.21 or higher

## Quick Start

Start the engine:
```bash
cd cmd/engine && go run .
```

Connect as a user (separate terminal):
```bash
cd cmd/client && go run .
```

No nodes exist on first run. Connect as admin to spawn one:
```bash
cd cmd/admin && go run .
```
```
admin> spawn computer mypc 192.168.1.1
admin> adduser mypc root hunter2
```

Then connect as a user, pick your machine, and log in. Once you're in, run `adduser` to create more accounts.

## Architecture

blog post coming soon, hold on tight!!

## Contributing

See [CONTRIBUTING](/CONTRIBUTING.md)

## License

MIT - see [LICENSE](LICENSE) file for details





