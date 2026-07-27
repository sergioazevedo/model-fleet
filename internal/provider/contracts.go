package provider

import (
	"context"
	"encoding/json"
)

type ModelDeployment struct {
	ID            string
	Provider      string
	ModelID       string
	Endpoint      string
	CredentialRef string
	QuotaPool     string
}

type CompletionRequest struct {
	Messages    []Message
	Tools       []Tool
	Temperature *float64
}

type Message struct {
	Role       string
	Content    string
	ToolCalls  []ToolCall
	ToolCallID string
}

type Tool struct {
	Name        string
	Description string
	Parameters  json.RawMessage
}

type ToolCall struct {
	ID        string
	Name      string
	Arguments json.RawMessage
}

type CompletionResponse struct {
	Message      Message
	FinishReason string
}

type Usage struct {
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

type CompletionResult struct {
	Response CompletionResponse
	Usage    Usage
}

type Client interface {
	Complete(
		context.Context,
		ModelDeployment,
		CompletionRequest,
	) (CompletionResult, error)
}
