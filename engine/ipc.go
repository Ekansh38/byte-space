package engine

type ClientIPCMessage struct {
	Program string `json:"program"`
	RequestID int `json:"request_id"`
	IP string `json:"ip"`
	Command string `json:"command"`
}


type EngineIPCMessage struct {
	RequestID int `json:"request_id"`
	Status int `json:"status"`
	Result string `json:"result"`
}

func newIPCMessage(s string) *EngineIPCMessage {
	return &EngineIPCMessage{Result: s}
}
