package models

// ChatRequest is the JSON payload received from the user
type ChatRequest struct {
	UserMessage string `json:"user_message"` // the message sent by the user
}

// ChatResponse is the JSON payload sent back to the client
type ChatResponse struct {
	AgentMessage string `json:"agent_message"` // the reply from the BPO agent
}
