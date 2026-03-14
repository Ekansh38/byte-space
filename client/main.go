package client

import (
	"bufio"
	"strings"
	"fmt"
	"net"
	"os"
)

func commandLoop(c net.Conn, mode string) {
	for {
		fmt.Print("> ")

		reader := bufio.NewReader(os.Stdin)

		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("An error occurred while reading input:", err)
			return
		}

		input = strings.TrimSuffix(input, "\n")

		// common checks
		if (input == ""){
			continue
		}
		if (input == "exit"){
			return
		}

		writeToEngine(c, input, mode)
		engineReader(c)
	}
}

