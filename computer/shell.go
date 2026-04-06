package computer

import (
	"fmt"
	"path"
	"strings"

	"byte-space/utils"
)

type Shell struct {
	done        chan struct{}
	tty         *TTY
	id          string
	graphicsAPI *GraphicsAPI
	ttyAPI      *TTYAPI
	Kernel      *Kernel
	proc        *Process
}

func (s *Shell) SetProcess(proc *Process) {
	s.proc = proc
	s.tty = proc.TTY
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

func (s *Shell) SetTTyAPI(api *TTYAPI) {
	s.ttyAPI = api
}

func (s *Shell) SetKernel(api *Kernel) {
	s.Kernel = api
}

func (s *Shell) Run(returnStatus chan int, params []string) {
	s.done = make(chan struct{})
	s.SetTTyAPI(&TTYAPI{tty: s.tty, proc: s.proc}) // shell has special permission to make its own API for itself.
	if s.graphicsAPI == nil {
		returnStatus <- utils.Error
		return
	}
	prompt := fmt.Sprintf("\n\r%s$ ", s.tty.Session.WorkingDir)
	s.graphicsAPI.Write(prompt)

	for {
		prefix := ""
		data, status := s.ttyAPI.Read(s.done)
		switch status {
		case utils.Success:
			value, flags := parse(data)

			switch value[0] {
			case "exit":
				returnStatus <- utils.Success
				s.tty.networkAPI.PublishEvent(EventProgramExited, map[string]interface{}{
					"program_id": s.ID(),
					"status":     0,
					"tty_id":     s.tty.id,
				})

				return
			case "pwd":
				if len(value) != 1 {
					s.graphicsAPI.Write("Usage: pwd\n")
					break
				}
				dataToDisplay := fmt.Sprintf("\n%s\n", s.tty.Session.WorkingDir)
				s.graphicsAPI.Write(dataToDisplay)
			case "cd":
				if len(flags) > 0 {
					s.graphicsAPI.Write("\nNo flags implemented\n")
					break
				}
				if len(value) != 2 {
					s.graphicsAPI.Write("\nUsage: cd <path>\n")
					break
				}

				dir := value[1]

				if !strings.HasPrefix(dir, "/") {
					dir = path.Join(s.tty.Session.WorkingDir, dir)
				}

				dir = path.Clean(dir)

				_, err := s.tty.Session.Computer.OS.ReadDir(dir)
				if err != nil {
					message := "\nInvalid directory\n"
					s.graphicsAPI.Write(message)
					break
				}

				s.tty.Session.WorkingDir = dir
				s.tty.networkAPI.PublishEvent(EventWorkingDirChanged, map[string]interface{}{
					"dir":    dir,
					"tty_id": s.tty.id,
				})
				s.graphicsAPI.Write("\n")

			case "ls":
				if err := s.Kernel.Exec(s.tty.Session, "/bin/ls", append(value[1:], flags...), &ExecOpts{PGID: 0, Background: false}); err != nil {
					s.graphicsAPI.Write("\n" + err.Error() + "\n")
				}
				s.tty.SetForegroundPGID(s.proc.PGID)
			case "clear":
				if err := s.Kernel.Exec(s.tty.Session, "/bin/clear", append(value[1:], flags...), &ExecOpts{PGID: 0, Background: false}); err != nil {
					s.graphicsAPI.Write("\n" + err.Error() + "\n")
				}
				s.tty.SetForegroundPGID(s.proc.PGID)
			case "cat":
				if err := s.Kernel.Exec(s.tty.Session, "/bin/cat", append(value[1:], flags...), &ExecOpts{PGID: 0, Background: false}); err != nil {
					s.graphicsAPI.Write("\n" + err.Error() + "\n")
				}
				s.tty.SetForegroundPGID(s.proc.PGID)
			case "adduser":
				if err := s.Kernel.Exec(s.tty.Session, "/bin/adduser", append(value[1:], flags...), &ExecOpts{PGID: 0, Background: false}); err != nil {
					s.graphicsAPI.Write("\n" + err.Error() + "\n")
				}
				s.tty.SetForegroundPGID(s.proc.PGID)

			case "":
				prefix = "\n"
				break

			default:
				s.graphicsAPI.Write("\nno such command!\n")

			}

			prompt = fmt.Sprintf("%s\r%s$ ", prefix, s.tty.Session.WorkingDir)
			s.graphicsAPI.Write(prompt)
		case utils.Exit:
			returnStatus <- utils.Error
			return
		}
	}
}

func (s *Shell) ID() string {
	return s.id
}

func (s *Shell) HandleSignal(sig Signal) {
}

func (s *Shell) AddGraphicsAPI(api *GraphicsAPI) {
	s.graphicsAPI = api
}

func (s *Shell) RemoveGraphicsAPI() {
	s.graphicsAPI = nil
}
