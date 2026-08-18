package openaicompatible

import "encoding/json"

type toolFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type toolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function toolFunctionCall `json:"function"`
}
type message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type tool struct {
	Type     string       `json:"type"`
	Function toolFunction `json:"function"`
}

type toolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type choice struct {
	Message      message `json:"message"`
	FinishReason string  `json:"finish_reason"`
}

type usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type errorResponse struct {
	Message string `json:"message"`
}

type request struct {
	Model           string          `json:"model,omitempty"`
	Messages        []message       `json:"messages,omitempty"`
	ResponseFormat  *responseFormat `json:"response_format,omitempty"`
	ReasoningEffort *string         `json:"reasoning_effort,omitempty"`
	Temperature     *float64        `json:"temperature,omitempty"`
	Tools           []tool          `json:"tools,omitempty"`
	ToolChoice      string          `json:"tool_choice,omitempty"`
}

type response struct {
	Choices []choice      `json:"choices,omitempty"`
	Usage   usage         `json:"usage,omitempty"`
	Error   errorResponse `json:"error,omitempty"`
}
