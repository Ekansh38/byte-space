package main

import "main/client"

func main() {

	mode := client.GetModeFlag()
	client.ConnectToEngine(mode)

}
