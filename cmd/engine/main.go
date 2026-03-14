package main
import (
	"byte-space/engine"
)

func main() {
	var engine *engine.Engine = engine.NewEngine()	
	engine.Run()
}
