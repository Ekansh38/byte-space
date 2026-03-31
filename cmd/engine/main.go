package main
import (
	"byte-space/engine"
)

func main() {
	var engine *engine.Engine = engine.NewEngine()	
	//engine.RunAdminCommand("spawn computer ekansh 192.168.1.1")
	//engine.RunAdminCommand("adduser ekansh root 1234")
	engine.Run()
}
