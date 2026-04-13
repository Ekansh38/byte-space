// Package registry initializes all the program paths and stuff to kernel.
package registry

import (
	"byte-space/computer"
	"byte-space/programs/cmds"
	"byte-space/programs/shell"
	v "byte-space/programs/v"
)

// TEMP, JUST FOR NOW!!, later we will have an actual interpreted programming language. no need for these unique factories, we will just have 1 generic.

func Register(c *computer.Computer) {
	k := c.Kernel
	k.RegisterProgram("/bin/sh",    shell.New)
	k.RegisterProgram("/bin/ls",    cmds.NewLs)
	k.RegisterProgram("/bin/clear", cmds.NewClear)
	k.RegisterProgram("/bin/cat",   cmds.NewCat)
	k.RegisterProgram("/bin/mkdir", cmds.NewMkDir)
	k.RegisterProgram("/bin/touch", cmds.NewTouch)
	k.RegisterProgram("/bin/chmod", cmds.NewChmod)
	k.RegisterProgram("/bin/rm",    cmds.NewRm)
	k.RegisterProgram("/bin/v",     v.New)
}
