package client

import (
	"flag"
	"os"
	"fmt"
)

func GetModeFlag() string {
	var modeFlag string
	flag.StringVar(&modeFlag, "mode", "user", "Mode of operation: 'user' or 'admin'")
	flag.Parse()

	if modeFlag != "user" && modeFlag != "admin" {
		fmt.Println("Please provide a valid mode!")
		os.Exit(1)
	}

	return modeFlag
}
