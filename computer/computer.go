// Package computer creates basic virtual computers for the network
package computer

import (
	"fmt"
	"io/fs"
	"os"

	"byte-space/utils"
	"github.com/spf13/afero"
)

type Session struct {
	SessionID   string
	Computer    *Computer
	CurrentUser string
	TTY         *TTY
}

type FileMetadata struct {
	Filepath string // the file/folder this metadata applies to

	Owner string // the owner of the file/folder,
	// string of the username of the creator,
	// for system files it is root, for other stuff /home/user it is that user.

	Setuid bool // true means the user who runs that program can run it in the permissions of the owner

	OwnerMode uint8
	OtherMode uint8

	//    rwx     // read  write  execute permissions
	// 0: 000
	// 1: 001
	// 2: 010
	// 3: 011
	// 4: 100
	// 5: 101
	// 6: 110
	// 7: 111
}

type Computer struct {
	Name       string
	IP         string
	Type       string
	OS         *OS
	Kernel     *Kernel
	EventBus   *EventBus
	filesystem afero.Fs
	FsMetaData map[string]FileMetadata

	sessions map[string]*Session
	ttys     []*TTY
}

func initFileSystem(fs afero.Fs, hostname string, ip string) {
	// root:password:uid:homedir

	fs.MkdirAll("/var/log/", 0o755)
	fs.MkdirAll("/etc/", 0o755)
	fs.MkdirAll("/bin/", 0o755)

	// helper func, good go feature. rating 10/11, cool feature, very good boij
	createIfNotExists := func(path, content string) {
		if _, err := fs.Stat(path); os.IsNotExist(err) {
			f, _ := fs.Create(path)
			f.WriteString(content)
			f.Close()
		}
	}

	createIfNotExists("/var/log/lastlogin", "")
	createIfNotExists("/etc/passwd", "")
	createIfNotExists("/etc/hostname", hostname)
	createIfNotExists("/etc/issue", fmt.Sprintf(defaultEtcIssue, hostname, ip))
	createIfNotExists("/etc/motd", fmt.Sprintf(defaultEtcMotd))
	createIfNotExists("/bin/ls", "")
	createIfNotExists("/bin/cat", "")
	createIfNotExists("/bin/clear", "")
	createIfNotExists("/bin/adduser", "")
	createIfNotExists("/bin/login", "")
	createIfNotExists("/bin/sh", "")
	createIfNotExists("/bin/mkdir", "")
	createIfNotExists("/bin/touch", "")
}

func populateFileMetadata(filesystm afero.Fs, computer *Computer) {
	walkFunc := func(path string, info fs.FileInfo, err error) error {
		fileMetadata := &FileMetadata{
			Filepath:  path,
			Owner:     "root",
			Setuid:    false,
			OwnerMode: 7,
			OtherMode: 5,
		}
		computer.FsMetaData[path] = *fileMetadata
		return nil
	}

	afero.Walk(filesystm, "/", walkFunc)
}

func NewComputer(name string, ip string, nodeType string, e NetworkAPI, eb *EventBus) *Computer {
	basePath := fmt.Sprintf("./data/networks/current/nodes/%s", name) // uniqueness of name is checked in the handler.go of the engine package.
	os.MkdirAll(basePath, 0o755)

	filesystm := afero.NewBasePathFs(afero.NewOsFs(), basePath)
	initFileSystem(filesystm, name, ip)

	computer := &Computer{
		Name:       name,
		IP:         ip,
		Type:       nodeType,
		OS:         &OS{},
		EventBus:   eb,
		filesystem: filesystm,
		FsMetaData: map[string]FileMetadata{},
		sessions:   make(map[string]*Session),
	}

	populateFileMetadata(filesystm, computer)

	computer.OS = &OS{Computer: computer}
	computer.OS.Network = e
	computer.Kernel = &Kernel{
		computer: computer,
		EventBus: eb,
		programs: map[string]func(int) Program{
			"/bin/ls":      func(pid int) Program { return &Ls{id: fmt.Sprintf("ls-%d", pid)} },
			"/bin/cat":     func(pid int) Program { return &Cat{id: fmt.Sprintf("cat-%d", pid)} },
			"/bin/clear":   func(pid int) Program { return &Clear{id: fmt.Sprintf("clear-%d", pid)} },
			"/bin/adduser": func(pid int) Program { return &Adduser{id: fmt.Sprintf("adduser-%d", pid)} },
			"/bin/login":   func(pid int) Program { return &LoginProgram{id: fmt.Sprintf("login-%d", pid)} },
			"/bin/sh":      func(pid int) Program { return &Shell{id: fmt.Sprintf("sh-%d", pid)} },
			"/bin/mkdir":      func(pid int) Program { return &MkDir{id: fmt.Sprintf("mkdir-%d", pid)} },
			"/bin/touch":      func(pid int) Program { return &Touch{id: fmt.Sprintf("touch-%d", pid)} },
		},
		procs: map[int]*Process{},
	}

	// adduser runs as root so we gotta make setuid TRUE!
	computer.FsMetaData["/bin/adduser"] = FileMetadata{
		Filepath:  "/bin/adduser",
		Owner:     "root",
		Setuid:    true,
		OwnerMode: 7,
		OtherMode: 5,
	}
	return computer
}

func (c *Computer) GenerateSessionID() string {
	// count number of active sessions
	count := len(c.sessions)
	sessionID := fmt.Sprintf("session-%d", count+1)
	return sessionID
}

func (node *Computer) NewSession(username string, tty *TTY) (int, string) {
	sessionID := node.GenerateSessionID()

	var workingDir string
	if username == "root" {
		workingDir = "/root"
	} else {
		workingDir = "/home/" + username
	}

	session := &Session{
		SessionID:   sessionID,
		Computer:    node,
		CurrentUser: username,
		TTY:         tty,
	}
	node.sessions[sessionID] = session

	if !(node.OS.HasDirectory(workingDir)) {
		node.OS.Mkdir(workingDir)
	}

	node.EventBus.Publish(EventSessionCreated, map[string]interface{}{
		"session_id":  sessionID,
		"user":        username,
		"computer":    node.Name,
		"working_dir": workingDir,
		"tty_id":      tty.id,
	})

	return utils.Success, sessionID
}
