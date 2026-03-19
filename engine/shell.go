package engine

import (
	"byte-space/utils"
	"fmt"
	"path"
	"strings"

	"github.com/spf13/afero"
)

type Shell struct {
	Session *Session
}

func (s *Shell) Run(command string) *EngineIPCMessage {
	// later will be updated for pipes and redirection but for now just a simple command parser

	commandP := parseCommand(command)
	if len(commandP) == 0 {
		message := "No command provided" // this should be filtered out by the client but if the API is used directly.
		fmt.Println(message)
		return newIPCMessage(message, utils.Error)
	}

	return s.RunCommand(commandP)

}

func (s *Shell) RunCommand(command []string) *EngineIPCMessage {
	switch command[0] {
	case "ls":
		return s.ls(command)
	case "cd":
		return s.cd(command)
	case "pwd":
		return s.pwd(command)
	case "whoami":
		return s.whoami(command)
	default:
		return newIPCMessage("not implemented", utils.Warning)
	}

}

func (s *Shell) ls(commandParsed []string) *EngineIPCMessage {
	lsDir := s.Session.WorkingDir
	output := ""

	if len(commandParsed) > 1 {
		for _, arg := range commandParsed[1:] {
			if strings.HasPrefix(arg, "-") {
				return newIPCMessage("Flags not supported yet", utils.Error)
			}
			if strings.HasPrefix(arg, "/") {
				lsDir = arg
			} else {
				lsDir = s.Session.WorkingDir + "/" + arg
			}
		}
	}


	files, err := afero.ReadDir(s.Session.Computer.Filesystem, lsDir)
	if err != nil {
		message := "Invalid directory"
		return newIPCMessage(message, utils.Error)
	}

	for _, file := range files {
		if file.IsDir() {
			output += fmt.Sprintf("\033[34m%s\033[0m\n", file.Name())
		} else {
			output += fmt.Sprintf("%s\n", file.Name())
		}
	}

	// remove extra trailing newline
	if len(output) > 0 {
		output = output[:len(output)-1]
	}

	return newIPCMessage(output, utils.Success)

}

func (s *Shell) cd(commandParsed []string) *EngineIPCMessage {

	if len(commandParsed) == 1 {
		return newIPCMessage("No directory provided", utils.Error)
	}
	for _, arg := range commandParsed[1:] {
		if strings.HasPrefix(arg, "-") {
			return newIPCMessage("Flags not supported yet", utils.Error)
		}
	}

	dir := commandParsed[1]

	if !strings.HasPrefix(dir, "/") {
		dir = path.Join(s.Session.WorkingDir, dir)
	}

	dir = path.Clean(dir)


	_, err := afero.ReadDir(s.Session.Computer.Filesystem, dir)
	if err != nil {
		message := "Invalid directory"
		return newIPCMessage(message, utils.Error)
	}

	s.Session.WorkingDir = dir

	return &EngineIPCMessage{
		Result: "",
		Status: utils.SuccessDoNotDisplay,
		Prompt: s.Session.WorkingDir + "$ ",
	}
}

func parseCommand(command string) []string {
	return strings.Fields(command)
}


func (s *Shell) pwd(commandParsed []string) *EngineIPCMessage {
	if len(commandParsed) > 1 {
		return newIPCMessage("pwd: too many arguments", utils.Error)
	}
	return &EngineIPCMessage{
		Result: s.Session.WorkingDir,
		Status: utils.Success,
	}
}

func (s *Shell) whoami(commandParsed []string) *EngineIPCMessage {
	if len(commandParsed) > 1 {
		return newIPCMessage("usage: whoami", utils.Error)
	}
	return &EngineIPCMessage{
		Result: s.Session.CurrentUser,
		Status: utils.Success,
	}
}
