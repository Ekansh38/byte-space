package engine

type ClientIPCMessage struct {
	Program string `json:"program"`
	RequestID int `json:"request_id"`
	SessionID string `json:"session_id"`
	Command string `json:"command"`
}


type EngineIPCMessage struct {
	SessionID string `json:"session_id"`
	Status int `json:"status"`
	Result string `json:"result"`
	Prompt string `json:"prompt"`
}


func newIPCMessage(s string, status int) *EngineIPCMessage {
	return &EngineIPCMessage{Result: s, Status: status}
}
