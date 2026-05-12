package shell

import (
	"byte-space/computer"
	"byte-space/utils"
	"context"
	"fmt"
	"log"
	"path"
	"strings"
)

type Shell struct {
	id             string
	Kernel         *computer.Kernel
	proc           *computer.Process
	buffer         string // switch this to []byte, one day!!
	cursorPosition int
	history        []string
	posInHistory   int // -1 is most recent, end of list. then while navigating history with arrow keys, it changes.
}

func New(pid int) computer.Program {
	return &Shell{id: fmt.Sprintf("sh-%d", pid)}
}

func (s *Shell) SetProcess(proc *computer.Process) { s.proc = proc }
func (s *Shell) SetKernel(k *computer.Kernel)      { s.Kernel = k }
func (s *Shell) ID() string                        { return s.id }
func (s *Shell) HandleSignal(sig computer.Signal) {
	if sig == computer.SIGINT {
		s.buffer = ""
		s.cursorPosition = 0
		s.posInHistory = -1
		s.Kernel.Write(s.proc, 1, []byte(fmt.Sprintf("\n\r%s$ ", s.proc.CWD)))
	}
}

func parse(value string) ([]string, []string) {
	parts := strings.Fields(value)

	var commands []string
	var flags []string

	for _, part := range parts {
		if strings.HasPrefix(part, "-") || strings.HasPrefix(part, "--") {
			flags = append(flags, part)
		} else {
			commands = append(commands, part)
		}
	}

	if len(commands) == 0 {
		commands = []string{""}
	}

	return commands, flags
}

func (s *Shell) Run(ctx context.Context, returnStatus chan int, params []string) {
	s.posInHistory = -1 // default, starting val
	s.Kernel.Write(s.proc, 1, []byte(fmt.Sprintf("\n\r%s$ ", s.proc.CWD)))
	s.Kernel.Syscall(s.proc, computer.SYS_IOCTL, 0, computer.TIOCRAW, true) // set TTY to raw mode.

	for {
		prefix := ""
		data, status := s.Kernel.Read(s.proc, 0, ctx)
		switch status {
		case utils.Success:

			// canonical logic, plus history logic.

			thingyMaBOB := s.canonicalLogic(data)
			log.Printf("HISTORY %#v, POSINHISTORY: %d, BUFFER %s, CURSORPOS %d", s.history, s.posInHistory, s.buffer, s.cursorPosition)
			if thingyMaBOB == "" {
				continue
			}

			value, flags := parse(thingyMaBOB)

			// nice closure. switches to canonical before exec and restores raw after.
			execFg := func(bin string) {
				s.Kernel.Syscall(s.proc, computer.SYS_IOCTL, 0, computer.TIOCRAW, false)
				if _, err := s.Kernel.Syscall(s.proc, computer.SYS_EXEC, ctx, bin, append(value, flags...), &computer.ExecOpts{}); err != nil {
					s.Kernel.Write(s.proc, 1, []byte("\n"+err.Error()+"\n"))
				}
				s.Kernel.Syscall(s.proc, computer.SYS_IOCTL, 0, computer.TIOCSPGRP, s.proc.PGID)
				s.Kernel.Syscall(s.proc, computer.SYS_IOCTL, 0, computer.TIOCRAW, true)
			}

			switch value[0] {
			// BUILT-IN commands, part of the shell not separate programs.
			case "exit":
				s.Kernel.Syscall(s.proc, computer.SYS_IOCTL, 0, computer.TIOCRAW, false) // set TTY to back to canonical mode.
				returnStatus <- utils.Success
				return
			case "pwd":
				if len(value) != 1 {
					s.Kernel.Write(s.proc, 1, []byte("Usage: pwd\n"))
					break
				}
				s.Kernel.Write(s.proc, 1, []byte(fmt.Sprintf("\n%s\n", s.proc.CWD)))
			case "cd":
				if len(flags) > 0 {
					s.Kernel.Write(s.proc, 1, []byte("\nNo flags implemented\n"))
					break
				}
				if len(value) != 2 {
					s.Kernel.Write(s.proc, 1, []byte("\nUsage: cd <path>\n"))
					break
				}

				dir := value[1]
				if !strings.HasPrefix(dir, "/") {
					dir = path.Join(s.proc.CWD, dir)
				}
				dir = path.Clean(dir)

				if _, err := s.Kernel.Syscall(s.proc, computer.SYS_CHDIR, dir); err != nil {
					s.Kernel.Write(s.proc, 1, []byte("\n"+err.Error()+"\n"))
					break
				}

				s.Kernel.Write(s.proc, 1, []byte("\n"))

			// PROGRAM LAUNCH PROCESS
			// Fork+Exec in foreground. (one syscall, simplified)
			// error handling
			// set foreground process

			// TODO: fix this into just a regular search through PATH, instead of hardcoding bin names here.
			// maybe even support aliases and stuff. would be fun.
			// Later when I do the BS-LANG

			case "ls":
				execFg("/bin/ls")
			case "clear":
				execFg("/bin/clear")
			case "cat":
				execFg("/bin/cat")
			case "adduser":
				execFg("/bin/adduser")
			case "mkdir":
				execFg("/bin/mkdir")
			case "touch":
				execFg("/bin/touch")
			case "chmod":
				execFg("/bin/chmod")
			case "rm":
				execFg("/bin/rm")
			case "v":
				execFg("/bin/v")
				s.Kernel.Syscall(s.proc, computer.SYS_IOCTL, 0, computer.TIOCRAW, true) // set to raw mode just in case!
			case "":
				prefix = "\n"
			default:
				s.Kernel.Write(s.proc, 1, []byte("\nno such command!\n"))
			}

			s.Kernel.Write(s.proc, 1, []byte(fmt.Sprintf("%s\r%s$ ", prefix, s.proc.CWD)))
		case utils.Exit:

			s.Kernel.Syscall(s.proc, computer.SYS_IOCTL, 0, computer.TIOCRAW, false) // set TTY to back to canonical mode.
			returnStatus <- utils.Error
			return
		}
	}
}
