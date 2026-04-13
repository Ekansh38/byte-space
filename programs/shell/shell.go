package shell

import (
	"context"
	"fmt"
	"log"
	"path"
	"strings"

	"byte-space/computer"
	"byte-space/utils"
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
func (s *Shell) HandleSignal(sig computer.Signal)  {}

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
	s.Kernel.Ioctl(s.proc, 0, computer.TIOCRAW, true) // set TTY to raw mode.

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

			switch value[0] {
			// BUILT-IN commands, part of the shell not separate programs.
			case "exit":
				s.Kernel.Ioctl(s.proc, 0, computer.TIOCRAW, false) // set TTY to back to canonical mode.
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

				if err := s.Kernel.ChangeDirectory(s.proc, dir); err != nil {
					s.Kernel.Write(s.proc, 1, []byte("\n"+err.Error()+"\n"))
					break
				}

				s.Kernel.PublishEvent(s.proc, computer.EventWorkingDirChanged, map[string]interface{}{
					"dir":    dir,
					"tty_id": s.Kernel.GetTtyID(s.proc),
				})
				s.Kernel.Write(s.proc, 1, []byte("\n"))

			// PROGRAM LAUNCH PROCESS
			// Fork+Exec in foreground. (one syscall, simplified)
			// error handling
			// set foreground process

			case "ls":
				if err := s.Kernel.Exec(ctx, s.proc, "/bin/ls", append(value, flags...), &computer.ExecOpts{PGID: 0, Background: false}); err != nil {
					s.Kernel.Write(s.proc, 1, []byte("\n"+err.Error()+"\n"))
				}
				s.Kernel.Ioctl(s.proc, 0, computer.TIOCSPGRP, s.proc.PGID)
			case "clear":
				if err := s.Kernel.Exec(ctx, s.proc, "/bin/clear", append(value, flags...), &computer.ExecOpts{PGID: 0, Background: false}); err != nil {
					s.Kernel.Write(s.proc, 1, []byte("\n"+err.Error()+"\n"))
				}
				s.Kernel.Ioctl(s.proc, 0, computer.TIOCSPGRP, s.proc.PGID)
			case "cat":
				if err := s.Kernel.Exec(ctx, s.proc, "/bin/cat", append(value, flags...), &computer.ExecOpts{PGID: 0, Background: false}); err != nil {
					s.Kernel.Write(s.proc, 1, []byte("\n"+err.Error()+"\n"))
				}
				s.Kernel.Ioctl(s.proc, 0, computer.TIOCSPGRP, s.proc.PGID)
			case "adduser":
				if err := s.Kernel.Exec(ctx, s.proc, "/bin/adduser", append(value, flags...), &computer.ExecOpts{PGID: 0, Background: false}); err != nil {
					s.Kernel.Write(s.proc, 1, []byte("\n"+err.Error()+"\n"))
				}
				s.Kernel.Ioctl(s.proc, 0, computer.TIOCSPGRP, s.proc.PGID)
			case "mkdir":
				if err := s.Kernel.Exec(ctx, s.proc, "/bin/mkdir", append(value, flags...), &computer.ExecOpts{PGID: 0, Background: false}); err != nil {
					s.Kernel.Write(s.proc, 1, []byte("\n"+err.Error()+"\n"))
				}
				s.Kernel.Ioctl(s.proc, 0, computer.TIOCSPGRP, s.proc.PGID)
			case "touch":
				if err := s.Kernel.Exec(ctx, s.proc, "/bin/touch", append(value, flags...), &computer.ExecOpts{PGID: 0, Background: false}); err != nil {
					s.Kernel.Write(s.proc, 1, []byte("\n"+err.Error()+"\n"))
				}
				s.Kernel.Ioctl(s.proc, 0, computer.TIOCSPGRP, s.proc.PGID)
			case "chmod":
				if err := s.Kernel.Exec(ctx, s.proc, "/bin/chmod", append(value, flags...), &computer.ExecOpts{PGID: 0, Background: false}); err != nil {
					s.Kernel.Write(s.proc, 1, []byte("\n"+err.Error()+"\n"))
				}
				s.Kernel.Ioctl(s.proc, 0, computer.TIOCSPGRP, s.proc.PGID)
			case "rm":
				if err := s.Kernel.Exec(ctx, s.proc, "/bin/rm", append(value, flags...), &computer.ExecOpts{PGID: 0, Background: false}); err != nil {
					s.Kernel.Write(s.proc, 1, []byte("\n"+err.Error()+"\n"))
				}
				s.Kernel.Ioctl(s.proc, 0, computer.TIOCSPGRP, s.proc.PGID)
			case "v":
				if err := s.Kernel.Exec(ctx, s.proc, "/bin/v", append(value, flags...), &computer.ExecOpts{PGID: 0, Background: false}); err != nil {
					s.Kernel.Write(s.proc, 1, []byte("\n"+err.Error()+"\n"))
				}
				s.Kernel.Ioctl(s.proc, 0, computer.TIOCRAW, true)          // set to raw mode just in case!
				s.Kernel.Ioctl(s.proc, 0, computer.TIOCSPGRP, s.proc.PGID) // set shell back to foreground
			case "":
				prefix = "\n"
			default:
				s.Kernel.Write(s.proc, 1, []byte("\nno such command!\n"))
			}

			s.Kernel.Write(s.proc, 1, []byte(fmt.Sprintf("%s\r%s$ ", prefix, s.proc.CWD)))
		case utils.Exit:

			s.Kernel.Ioctl(s.proc, 0, computer.TIOCRAW, false) // set TTY to back to canonical mode.
			returnStatus <- utils.Error
			return
		}
	}
}
