package router

import "encoding/json"

type completionRequest struct {
	Application     string    `json:"application"`
	Role            string    `json:"role"`
	Messages        []message `json:"messages"`
	Tools           []tool    `json:"tools,omitempty"`
	Temperature     *float64  `json:"temperature,omitempty"`
	ResponseFormat  *string   `json:"response_format,omitempty"`
	ReasoningEffort *string   `json:"reasoning_effort,omitempty"`
}

type message struct {
	Role       string
	Content    string
	ToolCalls  []toolCall
	ToolCallID string
}

type tool struct {
	Name        string
	Description string
	Parameters  json.RawMessage
}

type toolCall struct {
	ID        string
	Name      string
	Arguments json.RawMessage
}
