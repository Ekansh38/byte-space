package main

import (
	"fmt"

	"byte-space/computer"
)

func main() {
	basePath := fmt.Sprintf("./data/networks/current/nodes/TESTNODE/")
	computer.NewFileSystem(basePath)
}
