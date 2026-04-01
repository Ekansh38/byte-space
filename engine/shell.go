package engine

import (
	"fmt"
	//"fmt"
	//"os"
	"path"
	"strings"

	"byte-space/utils"
	"github.com/spf13/afero"
)

type Shell struct {
	done        chan struct{}
	tty         *TTY
	id          string
	graphicsAPI *GraphicsAPI
}

func getUniqueID(runningPrograms []string) string {
	existing := make(map[string]bool)
	for _, p := range runningPrograms {
		existing[p] = true
	}

	id := "0"
	for existing[id] {
		id += id
	}

	return id
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

func (s *Shell) Run(returnStatus chan int, params []string) {
	s.done = make(chan struct{})
	if s.graphicsAPI == nil {
		returnStatus <- utils.Error
		return
	}
	prompt := fmt.Sprintf("\n\r%s$ ", s.tty.Session.WorkingDir)
	s.graphicsAPI.Write(prompt)
	prompt = fmt.Sprintf("%s$ ", s.tty.Session.WorkingDir)

	var runningPrograms []string
	for {
		data, status := s.tty.Read(s, s.done)
		switch status {
		case utils.Success:
			value, flags := parse(data)

			switch value[0] {
			case "exit":
				returnStatus <- utils.Success
				return
			case "pwd":
				if len(value) != 1 {
					s.graphicsAPI.Write("Usage: pwd\n")
					break
				}
				dataToDisplay := fmt.Sprintf("%s\n", s.tty.Session.WorkingDir)
				s.graphicsAPI.Write(dataToDisplay)
			case "cd":
				if len(flags) > 0 {
					s.graphicsAPI.Write("No flags implemented\n")
					break
				}
				if len(value) != 2 {
					s.graphicsAPI.Write("Usage: cd <path>\n")
					break
				}

				dir := value[1]

				if !strings.HasPrefix(dir, "/") {
					dir = path.Join(s.tty.Session.WorkingDir, dir)
				}

				dir = path.Clean(dir)

				_, err := afero.ReadDir(s.tty.Session.Computer.Filesystem, dir)
				if err != nil {
					message := "Invalid directory\n"
					s.graphicsAPI.Write(message)
					break
				}
				s.tty.Session.WorkingDir = dir
				s.graphicsAPI.Write("\n")

			case "ls":
				ls := &Ls{tty: s.tty, id: "ls-" + getUniqueID(runningPrograms)}
				s.tty.SetForegroundProcess(ls)

				params := append(value[1:], flags...)
				status := make(chan int)

				s.tty.engine.EventBus.Publish(EventProgramStarted, map[string]interface{}{
					"program_id": ls.ID(),
					"tty_id":     s.tty.id,
				})

				go ls.Run(status, params)

				<-status

				// set shell back to foreground
				s.tty.SetForegroundProcess(s)
			case "clear":
				clear := &Clear{tty: s.tty, id: "clear-" + getUniqueID(runningPrograms)}
				s.tty.SetForegroundProcess(clear)

				params := append(value[1:], flags...)
				status := make(chan int)
				
				s.tty.engine.EventBus.Publish(EventProgramStarted, map[string]interface{}{
					"program_id": clear.ID(),
					"tty_id":     s.tty.id,
				})

				go clear.Run(status, params)

				<-status

				// set shell back to foreground
				s.tty.SetForegroundProcess(s)

			case "cat":
				cat := &Cat{tty: s.tty, id: "cat-" + getUniqueID(runningPrograms)}
				s.tty.SetForegroundProcess(cat)

				params := append(value[1:], flags...)
				status := make(chan int)

				s.tty.engine.EventBus.Publish(EventProgramStarted, map[string]interface{}{
					"program_id": cat.ID(),
					"tty_id":     s.tty.id,
				})

				go cat.Run(status, params)

				<-status

				// set shell back to foreground
				s.tty.SetForegroundProcess(s)

			case "":
				break

			default:
				s.graphicsAPI.Write("no such command!\n")
			}

			prompt = fmt.Sprintf("%s$ ", s.tty.Session.WorkingDir)
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
