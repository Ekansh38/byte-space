package main

import "byte-space/client"

func main() {

	mode := client.GetModeFlag()
	client.ConnectToEngine(mode)

}
