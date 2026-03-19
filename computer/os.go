package computer

import (
	"github.com/spf13/afero"
	"strings"
)

type OS struct {
	Computer    *Computer
}

func (o *OS) GetIssue() string {
	path := "/etc/issue"
	data, err := afero.ReadFile(o.Computer.Filesystem, path)
	if err != nil {
		return "Error reading issue file"
	}
	return string(data)
}

func (o *OS) GetMotd() string {
	path := "/etc/motd"
	data, err := afero.ReadFile(o.Computer.Filesystem, path)
	if err != nil {
		return "Error reading issue file"
	}
	return string(data)
}
func (o *OS) Login(username string, password string) int {
    path := "/etc/passwd"
    data, err := afero.ReadFile(o.Computer.Filesystem, path)
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
    
    return 1  // fail
}

func (o *OS) HasDirectory(path string) bool {
	info, err := o.Computer.Filesystem.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}	
