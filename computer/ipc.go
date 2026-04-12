package computer

type ClientIPCMessage struct {
	Program   string `json:"program"`
	Keystroke string `json:"keystroke"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
}

type AdminIPCMessage struct {
	Program string `json:"program"`
	Value   string `json:"value"`
}

type EngineIPCMessage struct {
	SessionID string `json:"session_id"`
	Status    int    `json:"status"`
	Result    string `json:"result"`
}

func NewIPCMessage(s string, status int) *EngineIPCMessage {
	return &EngineIPCMessage{Result: s, Status: status}
}
