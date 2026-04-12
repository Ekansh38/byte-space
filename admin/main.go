package admin

import (
	"fmt"
	"net"

	"byte-space/utils"
)

func commandLoop(c net.Conn) {
	prompt := "admin> "
	for {
		value := getInput(prompt)
		if value == "clear" {
			fmt.Printf("\033[2J\033[H")
			continue
		} else if value == "" {
			continue
		}

		writeToEngine(c, value)

		stat := engineReader(c)
		if stat == utils.Exit {
			return
		}
	}
}
