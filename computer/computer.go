// Package computer creates basic virtual computers for the network
package computer

import (
	"fmt"
	"os"

	"byte-space/utils"
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

type NetworkAPI interface {
	// Whatever programs need, like send packet and stuf
	ListMachinesOnNetwork() []Computer
	GetNode(ip_address string) (node *Computer, ok bool)
}

type Computer struct {
	Name    string
	IP      string
	Type    string
	Network NetworkAPI
	Kernel  *Kernel
	fs      *FileSystem

	sessions map[string]*Session
	ttys     []*TTY
}

// initFSTree builds the standard first-boot tree straight into BS-EXTFS.
// only runs when the disk got freshly formatted.
func (k *Kernel) initFSTree(hostname string, ip string) {
	// root dir gets . and .. first, both point at itself (its the root!)
	rootOps := &DirectoryOps{inodeNum: 2}
	rootOps.addDentry(k, DirEntry{Inum: 2, Name: "."})
	rootOps.addDentry(k, DirEntry{Inum: 2, Name: ".."})

	root := &Process{UID: "root", EUID: "root", CWD: "/"}

	mkdir := func(path string) { k.mkDir(root, path) }
	create := func(path, content string) {
		k.createFile(root, path)
		if content != "" {
			k.writeFile(root, path, []byte(content))
		}
	}

	mkdir("/var")
	mkdir("/var/log")
	mkdir("/etc")
	mkdir("/bin")
	mkdir("/home")
	mkdir("/tmp")
	mkdir("/root")
	k.chmod(root, "/root", 7, 0) // /root is not accessible to others

	create("/var/log/lastlogin", "")
	create("/etc/passwd", "")
	create("/etc/hostname", hostname)
	create("/etc/issue", fmt.Sprintf(defaultEtcIssue, hostname, ip))
	create("/etc/motd", defaultEtcMotd)

	for _, bin := range []string{"ls", "cat", "clear", "adduser", "login", "sh", "mkdir", "touch", "chmod", "rm", "v"} {
		create("/bin/"+bin, "")
	}

	// adduser and login run as root (they need to read/write /etc/passwd),
	// setuid lives on the inode now
	for _, p := range []string{"/bin/adduser", "/bin/login"} {
		if inum, err := k.ResolvePath(root, p); err == nil {
			if in, err := k.getInode(inum); err == nil {
				in.setuid = true
				k.computer.fs.writeInode(in, inum)
			}
		}
	}
}

func NewComputer(name string, ip string, nodeType string, e NetworkAPI) *Computer {
	basePath := fmt.Sprintf("./data/networks/current/nodes/%s", name) // uniqueness of name is checked in the handler.go of the engine package.
	os.MkdirAll(basePath, 0o755)

	computer := &Computer{
		Name:     name,
		IP:       ip,
		Type:     nodeType,
		Network:  e,
		sessions: make(map[string]*Session),
		fs:       NewFileSystem(basePath),
	}

	computer.Kernel = &Kernel{
		computer:   computer,
		inodeCache: make(map[uint32]*inode),
		programs:   map[string]func(int) Program{},
		procs:      map[int]*Process{},
	}

	if computer.fs.freshFormat {
		computer.Kernel.initFSTree(name, ip)
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

	// make the home dir on first login, owned by the user
	k := node.Kernel
	root := &Process{UID: "root", EUID: "root", CWD: "/"}
	if _, err := k.ResolvePath(root, workingDir); err != nil {
		k.mkDir(root, workingDir)
		if inum, err := k.ResolvePath(root, workingDir); err == nil {
			if in, err := k.getInode(inum); err == nil {
				in.owner = ownerBytes(username)
				in.otherMode = 5
				if username == "root" {
					in.otherMode = 0 // /root is not accessible to others
				}
				k.computer.fs.writeInode(in, inum)
			}
		}
	}

	return utils.Success, sessionID
}
