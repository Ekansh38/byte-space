package engine

import (
	"byte-space/computer"
	"byte-space/programs/registry"
)

func registerPrograms(c *computer.Computer) {
	registry.Register(c)
}
