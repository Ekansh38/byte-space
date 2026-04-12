package admin

import (
	"net"

	"byte-space/utils"
)

func commandLoop(c net.Conn) {
	prompt := "admin> "
	for {
		value := getInput(prompt)

		writeToEngine(c, value)

		stat := engineReader(c)
		if stat == utils.Exit {
			return
		}
	}
}
