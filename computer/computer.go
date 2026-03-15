// Package computer creates basic virtual computers for the network
package computer

import (
	"github.com/spf13/afero"
	"fmt"
	"os"
)

type Computer struct {
	Name string
	IP string
	Type string
	OS *OS
	Filesystem afero.Fs
}

func initFileSystem(fs afero.Fs, hostname string, ip string) {

	// root:password:uid:homedir

	fs.MkdirAll("/var/log/", 0755)
	fs.MkdirAll("/etc/", 0755)
	    
    // Helper to create file only if doesn't exist
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

func New(name string, ip string, nodeType string) *Computer {
    basePath := fmt.Sprintf("./data/networks/current/nodes/%s", name) // uniqueness of name is checked in the handler.go of the engine package.
    os.MkdirAll(basePath, 0755)

	fs := afero.NewBasePathFs(afero.NewOsFs(), basePath)
	initFileSystem(fs, name, ip)


    
	computer := &Computer{
        Name:       name,
        IP:         ip,
        Type:         nodeType,
        OS:         &OS{},
        Filesystem: fs,
    }

	computer.OS = &OS{Computer: computer}
	return computer
}
