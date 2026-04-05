package engine

import (
	"fmt"
	"os"
	"path"
	"strings"

	"byte-space/computer"
)

type Kernel struct {
	session *Session
	user    string
}

func (k *Kernel) Exec(session *Session, program Program, value []string, flags []string) {
	effectiveUser := s.tty.Session.CurrentUser
	if program.Setuid() {
		effectiveUser = program.Owner()
	}
	program.SetTTyAPI(&TTYAPI{tty: s.tty, program: program})
	program.SetKernel(&Kernel{session: s.tty.Session, user: effectiveUser})
	s.tty.SetForegroundProcess(program)

	params := append(value[1:], flags...)
	status := make(chan int)

	s.tty.engine.EventBus.Publish(EventProgramStarted, map[string]interface{}{
		"program_id": program.ID(),
		"tty_id":     s.tty.id,
	})

	go program.Run(status, params)

	<-status

	s.tty.engine.EventBus.Publish(EventProgramExited, map[string]interface{}{
		"program_id": program.ID(),
		"status":     0,
		"tty_id":     s.tty.id,
	})

	// set shell back to foreground
	s.tty.SetForegroundProcess(s)
}

func (o *Kernel) GetWorkingDir() string {
	return o.session.WorkingDir
}

func (o *Kernel) GetCurrentUser() string {
	return o.user
}

func (o *Kernel) resolvePath(target string) string {
	target = path.Clean(target)
	if strings.HasPrefix(target, "~") {
		if o.user == "root" {
			target = path.Join("/root", target[1:])
		} else {
			target = path.Join("/home", o.user, target[1:])
		}
	}
	if !strings.HasPrefix(target, "/") {
		target = path.Join(o.session.WorkingDir, target)
	}
	return path.Clean(target)
}

func (o *Kernel) canWrite(filePath string) bool {
	if o.user == "root" {
		return true
	}
	meta, ok := o.session.Computer.FsMetaData[filePath]
	if !ok {
		return true // no metadata means unrestricted
	}
	if meta.Owner == o.user {
		// &2 isolates the write bit
		return meta.OwnerMode&2 != 0
	}
	return meta.OtherMode&2 != 0
}

func (o *Kernel) canRead(filePath string) bool {
	if o.user == "root" {
		return true
	}
	meta, ok := o.session.Computer.FsMetaData[filePath]
	if !ok {
		return true // no metadata means unrestricted
	}
	if meta.Owner == o.user {
		// &4 isolates the read bit
		return meta.OwnerMode&4 != 0
	}
	return meta.OtherMode&4 != 0
}

func (o *Kernel) ReadFile(target string) ([]byte, error) {
	target = o.resolvePath(target)
	if !o.canRead(target) {
		return nil, fmt.Errorf("permission denied")
	}
	return o.session.Computer.OS.ReadFile(target)
}

func (o *Kernel) ReadDir(target string) ([]os.FileInfo, error) {
	target = o.resolvePath(target)
	if !o.canRead(target) {
		return nil, fmt.Errorf("permission denied")
	}
	return o.session.Computer.OS.ReadDir(target)
}

func (o *Kernel) MkDir(target string) error {
	target = o.resolvePath(target)
	parent := path.Dir(target)
	if !o.canWrite(parent) {
		return fmt.Errorf("permission denied")
	}
	if err := o.session.Computer.OS.Mkdir(target); err != nil {
		return err
	}
	o.session.Computer.FsMetaData[target] = computer.FileMetadata{
		Filepath:  target,
		Owner:     o.user,
		Setuid:    false,
		OwnerMode: 7,
		OtherMode: 5,
	}
	return nil
}

func (o *Kernel) WriteFile(target string, data []byte) error {
	target = o.resolvePath(target)
	parent := path.Dir(target)
	if !o.canWrite(parent) {
		return fmt.Errorf("permission denied")
	}
	if err := o.session.Computer.OS.WriteFile(target, data); err != nil {
		return err
	}
	o.session.Computer.FsMetaData[target] = computer.FileMetadata{
		Filepath:  target,
		Owner:     o.user,
		Setuid:    false,
		OwnerMode: 6,
		OtherMode: 4,
	}
	return nil
}
