package computer

import (
	"os"
	"strings"

	"github.com/spf13/afero"
)

type NetworkAPI interface {
	// Whatever programs need, like send packet and stuf
}

type OS struct {
	Computer *Computer
	Network  NetworkAPI
}

func (o *OS) Mkdir(path string) error {
	return o.Computer.filesystem.MkdirAll(path, 0o755)
}

func (o *OS) WriteFile(path string, data []byte) error {
	return afero.WriteFile(o.Computer.filesystem, path, data, 0o644)
}

func (o *OS) GetIssue() string {
	path := "/etc/issue"
	data, err := afero.ReadFile(o.Computer.filesystem, path)
	if err != nil {
		return "Error reading issue file"
	}
	return string(data)
}

func (o *OS) GetMotd() string {
	path := "/etc/motd"
	data, err := afero.ReadFile(o.Computer.filesystem, path)
	if err != nil {
		return "Error reading motd file"
	}
	return string(data)
}

func (o *OS) Login(username string, password string) int {
	path := "/etc/passwd"
	data, err := afero.ReadFile(o.Computer.filesystem, path)
	if err != nil {
		return 1
	}

	lines := strings.Split(string(data), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		fields := strings.Split(line, ":")

		if len(fields) >= 2 && fields[0] == username && fields[1] == password {
			// Login successful
			return 0
		}
	}

	return 1 // fail
}

func (o *OS) HasDirectory(path string) bool {
	info, err := o.Computer.filesystem.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func (o *OS) ReadFile(path string) ([]byte, error) {
	return afero.ReadFile(o.Computer.filesystem, path)
}

func (o *OS) ReadDir(path string) ([]os.FileInfo, error) {
	return afero.ReadDir(o.Computer.filesystem, path)
}
