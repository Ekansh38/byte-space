package main

import (
	"fmt"

	"byte-space/computer"
)

func main() {
	basePath := fmt.Sprintf("./data/networks/current/nodes/TESTNODE/")
	//fs := computer.NewFileSystem(basePath)
	computer.NewFileSystem(basePath)

	//fs.Falloc()
}
