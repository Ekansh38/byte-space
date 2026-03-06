package main
import (
	"main/engine"
)

func main() {
	var engine *engine.Engine = engine.NewEngine()	
	engine.Run()
}
