// package utils has some extra structs and functions that are used by both engine and computer packages. It is a common practice to have a utils package for shared code. The main.go file in the engine package contains the implementation of the Engine struct and its Run method, which listens for incoming connections on a Unix socket and handles them concurrently.
package utils

type ClientICPMessage struct {
	Program string `json:"program"`
	RequestId int `json:"request_id"`
	IP string `json:"ip"`
	Command string `json:"command"`
}


