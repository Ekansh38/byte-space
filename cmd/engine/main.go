package main

import (
	"log"
	"os"

	"byte-space/engine"
)

func main() {
	f, _ := os.OpenFile("debug.log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0600)
	log.SetOutput(f)

	eng := engine.NewEngine()

	// catch ctrl-c and make every filesystem shutdown nicely TODO
	// use signal.Notify os.Interrupt

	eng.Run()
}
