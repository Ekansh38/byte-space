package engine

import (
	"fmt"
	//"fmt"
	//"os"
	"path"
	"strconv"
	"strings"

	"byte-space/utils"
)

type Shell struct {
	done        chan struct{}
	tty         *TTY
	id          string
	graphicsAPI *GraphicsAPI
	ttyAPI      *TTYAPI
	Kernel       *Kernel
	nextID      int
}

func (p *Shell) Owner() string {
	return "root"
}

func (p *Shell) Setuid() bool {
	return false
}


func (s *Shell) getUniqueID() string {
	id := s.nextID
	s.nextID++
	return strconv.Itoa(id)
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
	s.SetTTyAPI(&TTYAPI{tty: s.tty, program: s}) // shell has special permission to make its own API for itself.
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
				s.tty.engine.EventBus.Publish(EventProgramExited, map[string]interface{}{
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
				s.graphicsAPI.Write("\n")

			case "ls":
				program := &Ls{id: "ls-" + s.getUniqueID()}
				exec(program, s, value, flags)
			case "clear":
				program := &Clear{id: "clear-" + s.getUniqueID()}
				exec(program, s, value, flags)
			case "cat":
				program := &Cat{id: "cat-" + s.getUniqueID()}
				exec(program, s, value, flags)

			case "adduser":
				program := &Adduser{id: "adduser-" + s.getUniqueID()}
				exec(program, s, value, flags)

			case "":
				prefix = "\n"
				break

			default:
				s.graphicsAPI.Write("\nno such command!\n")

			}

			prompt = fmt.Sprintf("%s%s$ ", prefix, s.tty.Session.WorkingDir)
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
