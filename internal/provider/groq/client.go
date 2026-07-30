package groq

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/sergioazevedo/model-fleet/internal/provider"
)

type GroqClient struct {
	apiKey     string
	httpClient *http.Client
}

type groqRequest struct {
	Model           string              `json:"model,omitempty"`
	Messages        []groqMessage       `json:"messages,omitempty"`
	ResponseFormat  *groqResponseFormat `json:"response_format,omitempty"`
	ReasoningEffort *string             `json:"reasoning_effort,omitempty"`
	Temperature     *float64            `json:"temperature,omitempty"`
	Tools           []groqTool          `json:"tools,omitempty"`
	ToolChoice      string              `json:"tool_choice,omitempty"`
}

type groqResponse struct {
	Choices []choice `json:"choices"`
	Usage   usage    `json:"usage"`
}

type choice struct {
	Message      groqMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

type groqMessage struct {
	Role       string         `json:"role"`
	Content    string         `json:"content"`
	ToolCalls  []groqToolCall `json:"tool_calls,omitempty"`
	ToolCallID string         `json:"tool_call_id,omitempty"`
}

type groqTool struct {
	Type     string           `json:"type"`
	Function groqToolFunction `json:"function"`
}

type groqToolFunction struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"`
}

type groqToolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function groqFunctionCall `json:"function"`
}

type groqFunctionCall struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}
type usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

type groqResponseFormat struct {
	Type string `json:"type,omitempty"`
}

func New(apiKey string, httpClient *http.Client) *GroqClient {
	return &GroqClient{
		apiKey:     apiKey,
		httpClient: httpClient,
	}
}

func (c *GroqClient) Complete(
	ctx context.Context,
	deployment provider.ModelDeployment,
	request provider.CompletionRequest,
) (provider.CompletionResult, error) {
	req, err := buildRequest(
		ctx,
		deployment,
		request,
		map[string]string{
			"Content-Type":  "application/json",
			"Authorization": fmt.Sprintf("Bearer %s", c.apiKey),
		},
	)
	if err != nil {
		return provider.CompletionResult{}, fmt.Errorf("failed to build request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return provider.CompletionResult{}, fmt.Errorf("failed to complete request: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return provider.CompletionResult{}, fmt.Errorf("groq api error: status=%d body=%s", resp.StatusCode, string(bodyBytes))
	}

	result, err := decodeResponse(resp)
	if err != nil {
		return provider.CompletionResult{}, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil

}

func buildRequest(
	ctx context.Context,
	deployment provider.ModelDeployment,
	request provider.CompletionRequest,
	headers map[string]string,
) (*http.Request, error) {
	targetURL := strings.TrimRight(deployment.Endpoint, "/") + "/chat/completions"
	body, err := encodeRequestBody(deployment, request)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request body: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		targetURL,
		strings.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	return req, nil
}

func encodeRequestBody(
	deployment provider.ModelDeployment,
	request provider.CompletionRequest,
) (string, error) {
	messages := []groqMessage{}
	for _, v := range request.Messages {
		mappedToolCalls, err := mapToGroqToolCalls(v.ToolCalls)
		if err != nil {
			return "", fmt.Errorf("failed to map tool calls: %w", err)
		}

		messages = append(messages, groqMessage{
			Role:       v.Role,
			Content:    v.Content,
			ToolCalls:  mappedToolCalls,
			ToolCallID: v.ToolCallID,
		})
	}

	reqBody := groqRequest{
		Model:           deployment.ModelID,
		Messages:        messages,
		Temperature:     request.Temperature,
		ReasoningEffort: request.ReasoningEffort,
		ResponseFormat:  mapToGroqResponseFormat(request.ResponseFormat),
	}
	if len(request.Tools) > 0 {
		reqBody.Tools = mapToGroqTools(request.Tools)
		reqBody.ToolChoice = "auto"
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request body: %w", err)
	}

	return string(body), nil
}

func mapToGroqResponseFormat(format *provider.ResponseFormat) *groqResponseFormat {
	if format == nil {
		return nil
	}

	return &groqResponseFormat{Type: format.String()}
}

func mapToGroqToolCalls(toolCalls []provider.ToolCall) ([]groqToolCall, error) {
	groqToolCalls := []groqToolCall{}
	for _, call := range toolCalls {
		if !json.Valid(call.Arguments) {
			return nil, fmt.Errorf("invalid JSON in tool call arguments for ID %s", call.ID)
		}

		groqToolCalls = append(groqToolCalls, groqToolCall{
			ID:   call.ID,
			Type: "function",
			Function: groqFunctionCall{
				Name:      call.Name,
				Arguments: string(call.Arguments),
			},
		})
	}

	return groqToolCalls, nil
}

func mapToGroqTools(tools []provider.Tool) []groqTool {
	groqTools := []groqTool{}
	for _, tool := range tools {
		groqTools = append(groqTools, groqTool{
			Type: "function",
			Function: groqToolFunction{
				Name:        tool.Name,
				Description: tool.Description,
				Parameters:  tool.Parameters,
			},
		})
	}

	return groqTools
}

func decodeResponse(resp *http.Response) (provider.CompletionResult, error) {
	var groqResp groqResponse
	if err := json.NewDecoder(resp.Body).Decode(&groqResp); err != nil {
		return provider.CompletionResult{}, fmt.Errorf("failed to decode response: %w", err)
	}

	// Drain remaining body
	_, _ = io.Copy(io.Discard, resp.Body)

	usage := provider.Usage{
		PromptTokens:     groqResp.Usage.PromptTokens,
		CompletionTokens: groqResp.Usage.CompletionTokens,
		TotalTokens:      groqResp.Usage.TotalTokens,
	}

	if len(groqResp.Choices) == 0 {
		return provider.CompletionResult{Usage: usage}, fmt.Errorf("no content generated")
	}

	var toolCalls []provider.ToolCall
	for _, call := range groqResp.Choices[0].Message.ToolCalls {
		if !json.Valid([]byte(call.Function.Arguments)) {
			return provider.CompletionResult{}, fmt.Errorf("invalid JSON in tool call arguments for ID %s", call.ID)
		}

		toolCalls = append(toolCalls, provider.ToolCall{
			ID:        call.ID,
			Name:      call.Function.Name,
			Arguments: json.RawMessage(call.Function.Arguments),
		})
	}

	data := groqResp.Choices[0]

	return provider.CompletionResult{
		Response: provider.CompletionResponse{
			Message: provider.Message{
				Role:      data.Message.Role,
				Content:   data.Message.Content,
				ToolCalls: toolCalls,
			},
			FinishReason: data.FinishReason,
		},
		Usage: usage,
	}, nil
}
