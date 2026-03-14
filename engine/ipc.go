package engine

type ClientIPCMessage struct {
	Program string `json:"program"`
	RequestID int `json:"request_id"`
	IP string `json:"ip"`
	Command string `json:"command"`
}


type EngineIPCMessage struct {
	Response int `json:"response_id"`
	Status int `json:"status"`
	Result string `json:"result"`
}

func newIPCMessage(s string, status int) *EngineIPCMessage {
	return &EngineIPCMessage{Result: s, Status: status}
}
