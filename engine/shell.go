package engine

import (
	"fmt"
	"os"
	"path"
	"strings"

	"byte-space/utils"

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
	case "mkdir":
		return s.mkdir(command)
	case "rm":
		return s.rm(command)
	case "touch":
		return s.touch(command)
	case "cat":
		return s.cat(command)
	default:
		return newIPCMessage("not implemented", utils.Warning)
	}
}

func (s *Shell) ls(commandParsed []string) *EngineIPCMessage {
	lsDir := s.Session.WorkingDir

	_, targets := s.flagsHelper(commandParsed)

	if len(targets) > 1 {
		return newIPCMessage("ls: too many arguments", utils.Error)
	}

	lsDir = s.expandPath(lsDir)

	files, err := afero.ReadDir(s.Session.Computer.Filesystem, lsDir)
	if err != nil {
		message := "Invalid directory"
		return newIPCMessage(message, utils.Error)
	}

	output := ""
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
	_, targets := s.flagsHelper(commandParsed)

	if len(targets) == 0 {
		// if no target is provided, cd to home directory
		if s.Session.CurrentUser == "root" {
			s.Session.WorkingDir = "/root"
		} else {
			s.Session.WorkingDir = fmt.Sprintf("/home/%s", s.Session.CurrentUser)
		}
		return &EngineIPCMessage{
			Result: "",
			Status: utils.SuccessDoNotDisplay,
			Prompt: s.Session.WorkingDir + "$ ",
		}
	} else if len(targets) > 1 {
		return newIPCMessage("cd: too many arguments", utils.Error)
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

func (s *Shell) mkdir(commandParsed []string) *EngineIPCMessage {
	_, targets := s.flagsHelper(commandParsed)

	if len(targets) == 0 {
		return newIPCMessage("mkdir: missing operand", utils.Error)
	} else if len(targets) > 1 {
		return newIPCMessage("mkdir: too many operands", utils.Error)
	}

	dir := s.expandPath(targets[0])

	if !strings.HasPrefix(dir, "/") {
		dir = path.Join(s.Session.WorkingDir, dir)
	}

	dir = path.Clean(dir)

	if s.directoryExistsCaseSensitive(dir) {
		return newIPCMessage("mkdir: cannot create directory: File exists", utils.Error)
	}

	err := s.Session.Computer.Filesystem.Mkdir(dir, 0755)
	if err != nil {
		message := "Failed to create directory"
		if strings.HasSuffix(err.Error(), "no such file or directory") {
			message = "mkdir: cannot create directory: No such file or directory"
		}

		return newIPCMessage(message, utils.Error)
	}

	return newIPCMessage("", utils.SuccessDoNotDisplay)
}

func (s *Shell) directoryExistsCaseSensitive(targetPath string) bool {
	parentDir := path.Dir(targetPath)
	targetName := path.Base(targetPath)

	if targetPath == "/" {
		return true
	}

	files, err := afero.ReadDir(s.Session.Computer.Filesystem, parentDir)
	if err != nil {
		return false
	}

	for _, file := range files {
		if file.Name() == targetName {
			return true
		}
	}

	return false
}

func (s *Shell) rm(commandParsed []string) *EngineIPCMessage {
	if len(commandParsed) > 3 {
		return newIPCMessage("rm: too many arguments", utils.Error)
	}

	// Flags

	flags, targets := s.flagsHelper(commandParsed)

	if len(targets) == 0 {
		return newIPCMessage("rm: missing operand", utils.Error)
	} else if len(targets) > 1 {
		return newIPCMessage("rm: too many operands", utils.Error)
	}

	target := s.expandPath(targets[0])

	if !strings.HasPrefix(target, "/") {
		target = path.Join(s.Session.WorkingDir, target)
	}

	if flags["r"] {
		err := s.Session.Computer.Filesystem.RemoveAll(target)
		if err != nil && !flags["f"] {
			message := "Failed to remove directory"
			if strings.HasSuffix(err.Error(), "no such file or directory") {
				message = "rm: cannot remove: No such file or directory"
			}
			return newIPCMessage(message, utils.Error)
		}
	}

	if !flags["r"] {
		fileInfo, err := s.Session.Computer.Filesystem.Stat(target)
		if err != nil && !flags["f"] {
			message := "Failed to remove file"
			if strings.HasSuffix(err.Error(), "no such file or directory") {
				message = "No such file or directory"
			}
			return newIPCMessage(message, utils.Error)
		}

		if fileInfo.IsDir() {
			return newIPCMessage("rm: cannot remove directory: Is a directory", utils.Error)
		}

		err = s.Session.Computer.Filesystem.Remove(target)
		if err != nil && !flags["f"] {
			message := "Failed to remove file"
			if strings.HasSuffix(err.Error(), "no such file or directory") {
				message = "No such file or directory"
			}
			if strings.HasSuffix(err.Error(), "directory not empty") {
				message = "Directory not empty"
			}
			return newIPCMessage(message, utils.Error)
		}
	}

	return newIPCMessage("", utils.SuccessDoNotDisplay)
}

func (s *Shell) expandPath(target string) string {
	// expand ~ to home directory
	target = path.Clean(target)
	if strings.HasPrefix(target, "~") {
		if s.Session.CurrentUser == "root" {
			target = path.Join("/root/", target[1:])
		} else {
			target = path.Join("/home/", fmt.Sprintf("%s/", s.Session.CurrentUser), target[1:])
		}
	}

	return target
}

func (s *Shell) flagsHelper(commandParsed []string) (map[string]bool, []string) {
	flags := make(map[string]bool)
	targets := []string{}
	if len(commandParsed) >= 2 {
		for _, arg := range commandParsed[1:] {
			if strings.HasPrefix(arg, "-") {
				for _, flag := range arg[1:] {
					flags[string(flag)] = true
				}
			} else {
				targets = append(targets, arg)
			}
		}
	}

	return flags, targets
}

func (s *Shell) touch(commandParsed []string) *EngineIPCMessage {
	_, targets := s.flagsHelper(commandParsed)

	if len(targets) == 0 {
		return newIPCMessage("touch: missing file operand", utils.Error)
	} else if len(targets) > 1 {
		return newIPCMessage("touch: too many file operands", utils.Error)
	}

	target := s.expandPath(targets[0])

	if !strings.HasPrefix(target, "/") {
		target = path.Join(s.Session.WorkingDir, target)
	}

	target = s.expandPath(target)

	target = path.Clean(target)

	file, err := s.Session.Computer.Filesystem.OpenFile(target, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		message := "Failed to create file"
		if strings.HasSuffix(err.Error(), "no such file or directory") {
			message = "touch: cannot touch: No such file or directory"
		}
		return newIPCMessage(message, utils.Error)
	}
	file.Close()

	return newIPCMessage("", utils.SuccessDoNotDisplay)
}

func (s *Shell) cat(commandParsed []string) *EngineIPCMessage {
	_, targets := s.flagsHelper(commandParsed)

	if len(targets) == 0 {
		return newIPCMessage("cat: missing file operand", utils.Error)
	} else if len(targets) > 1 {
		return newIPCMessage("cat: too many file operands", utils.Error)
	}

	target := s.expandPath(targets[0])

	if !strings.HasPrefix(target, "/") {
		target = path.Join(s.Session.WorkingDir, target)
	}

	target = s.expandPath(target) 

	file, err := s.Session.Computer.Filesystem.Open(target)
	if err != nil {
		message := "Failed to open file"
		if strings.HasSuffix(err.Error(), "no such file or directory") {
			message = "cat: cannot open: No such file or directory"
		}
		return newIPCMessage(message, utils.Error)
	}
	defer file.Close()

	content, err := afero.ReadAll(file)
	if err != nil {
		message := "Failed to read file"
		return newIPCMessage(message, utils.Error)
	}

	return newIPCMessage(string(content), utils.Success)
}

