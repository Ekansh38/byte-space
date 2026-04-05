// Package computer creates basic virtual computers for the network
package computer

import (
	"fmt"
	"io/fs"
	"os"

	"github.com/spf13/afero"
)

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
	filesystem afero.Fs
	FsMetaData map[string]FileMetadata
}

func initFileSystem(fs afero.Fs, hostname string, ip string) {
	// root:password:uid:homedir

	fs.MkdirAll("/var/log/", 0o755)
	fs.MkdirAll("/etc/", 0o755)

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

func NewComputer(name string, ip string, nodeType string) *Computer {
	basePath := fmt.Sprintf("./data/networks/current/nodes/%s", name) // uniqueness of name is checked in the handler.go of the engine package.
	os.MkdirAll(basePath, 0o755)

	filesystm := afero.NewBasePathFs(afero.NewOsFs(), basePath)
	initFileSystem(filesystm, name, ip)

	computer := &Computer{
		Name:       name,
		IP:         ip,
		Type:       nodeType,
		OS:         &OS{},
		filesystem: filesystm,
		FsMetaData: map[string]FileMetadata{},
	}

	populateFileMetadata(filesystm, computer)

	computer.OS = &OS{Computer: computer}
	return computer
}
