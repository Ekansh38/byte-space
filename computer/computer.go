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

func New(name string, ip string, nodeType string) *Computer {
    basePath := fmt.Sprintf("./data/networks/current/nodes/%s", name) // uniqueness of name is checked in the handler.go of the engine package.
    os.MkdirAll(basePath, 0755)
    
    return &Computer{
        Name:       name,
        IP:         ip,
        Type:         nodeType,
        OS:         &OS{},
        Filesystem: afero.NewBasePathFs(afero.NewOsFs(), basePath),
    }
}
