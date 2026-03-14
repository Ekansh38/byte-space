package engine

type ClientIPCMessage struct {
	Program string `json:"program"`
	RequestID int `json:"request_id"`
	SessionID int `json:"session_id"`
	Command string `json:"command"`
}


type EngineIPCMessage struct {
	ResponseID int `json:"response_id"`
	Status int `json:"status"`
	Result string `json:"result"`
}

func newIPCMessage(s string, status int) *EngineIPCMessage {
	return &EngineIPCMessage{Result: s, Status: status}
}
