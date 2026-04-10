package computer

import (
	"context"
	"fmt"
	"path"
	"strings"

	"byte-space/utils"
)

type Shell struct {
	id     string
	Kernel *Kernel
	proc   *Process
}

func (s *Shell) SetProcess(proc *Process) { s.proc = proc }
func (s *Shell) SetKernel(k *Kernel)      { s.Kernel = k }
func (s *Shell) ID() string               { return s.id }
func (s *Shell) HandleSignal(sig Signal)  {}

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
	s.Kernel.Write(s.proc, 1, []byte(fmt.Sprintf("\n\r%s$ ", s.proc.CWD)))

	for {
		prefix := ""
		data, status := s.Kernel.Read(s.proc, 0, ctx)
		switch status {
		case utils.Success:
			value, flags := parse(data)

			switch value[0] {
			case "exit":
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

				s.Kernel.PublishEvent(s.proc, EventWorkingDirChanged, map[string]interface{}{
					"dir":    dir,
					"tty_id": s.Kernel.GetTtyID(s.proc),
				})
				s.Kernel.Write(s.proc, 1, []byte("\n"))

			case "ls":
				if err := s.Kernel.Exec(ctx, s.proc, "/bin/ls", append(value, flags...), &ExecOpts{PGID: 0, Background: false}); err != nil {
					s.Kernel.Write(s.proc, 1, []byte("\n"+err.Error()+"\n"))
				}
				s.Kernel.Ioctl(s.proc, 0, TIOCSPGRP, s.proc.PGID)
			case "clear":
				if err := s.Kernel.Exec(ctx, s.proc, "/bin/clear", append(value, flags...), &ExecOpts{PGID: 0, Background: false}); err != nil {
					s.Kernel.Write(s.proc, 1, []byte("\n"+err.Error()+"\n"))
				}
				s.Kernel.Ioctl(s.proc, 0, TIOCSPGRP, s.proc.PGID)
			case "cat":
				if err := s.Kernel.Exec(ctx, s.proc, "/bin/cat", append(value, flags...), &ExecOpts{PGID: 0, Background: false}); err != nil {
					s.Kernel.Write(s.proc, 1, []byte("\n"+err.Error()+"\n"))
				}
				s.Kernel.Ioctl(s.proc, 0, TIOCSPGRP, s.proc.PGID)
			case "adduser":
				if err := s.Kernel.Exec(ctx, s.proc, "/bin/adduser", append(value, flags...), &ExecOpts{PGID: 0, Background: false}); err != nil {
					s.Kernel.Write(s.proc, 1, []byte("\n"+err.Error()+"\n"))
				}
				s.Kernel.Ioctl(s.proc, 0, TIOCSPGRP, s.proc.PGID)
			case "mkdir":
				if err := s.Kernel.Exec(ctx, s.proc, "/bin/mkdir", append(value, flags...), &ExecOpts{PGID: 0, Background: false}); err != nil {
					s.Kernel.Write(s.proc, 1, []byte("\n"+err.Error()+"\n"))
				}
				s.Kernel.Ioctl(s.proc, 0, TIOCSPGRP, s.proc.PGID)
			case "touch":
				if err := s.Kernel.Exec(ctx, s.proc, "/bin/touch", append(value, flags...), &ExecOpts{PGID: 0, Background: false}); err != nil {
					s.Kernel.Write(s.proc, 1, []byte("\n"+err.Error()+"\n"))
				}
				s.Kernel.Ioctl(s.proc, 0, TIOCSPGRP, s.proc.PGID)
			case "chmod":
				if err := s.Kernel.Exec(ctx, s.proc, "/bin/chmod", append(value, flags...), &ExecOpts{PGID: 0, Background: false}); err != nil {
					s.Kernel.Write(s.proc, 1, []byte("\n"+err.Error()+"\n"))
				}
				s.Kernel.Ioctl(s.proc, 0, TIOCSPGRP, s.proc.PGID)
			case "rm":
				if err := s.Kernel.Exec(ctx, s.proc, "/bin/rm", append(value, flags...), &ExecOpts{PGID: 0, Background: false}); err != nil {
					s.Kernel.Write(s.proc, 1, []byte("\n"+err.Error()+"\n"))
				}
				s.Kernel.Ioctl(s.proc, 0, TIOCSPGRP, s.proc.PGID)
			case "v":
				if err := s.Kernel.Exec(ctx, s.proc, "/bin/v", append(value, flags...), &ExecOpts{PGID: 0, Background: false}); err != nil {
					s.Kernel.Write(s.proc, 1, []byte("\n"+err.Error()+"\n"))
				}
				s.Kernel.Ioctl(s.proc, 0, TIOCSPGRP, s.proc.PGID)
			case "":
				prefix = "\n"
			default:
				s.Kernel.Write(s.proc, 1, []byte("\nno such command!\n"))
			}

			s.Kernel.Write(s.proc, 1, []byte(fmt.Sprintf("%s\r%s$ ", prefix, s.proc.CWD)))
		case utils.Exit:
			returnStatus <- utils.Error
			return
		}
	}
}
