package groq

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/sergioazevedo/model-fleet/internal/provider"
)

type GroqClient struct {
	apiKey     string
	httpClient *http.Client
}

type groqRequest struct {
	Model       string        `json:"model,omitempty"`
	Messages    []groqMessage `json:"messages,omitempty"`
	Temperature *float64      `json:"temperature,omitempty"`
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
	Role    string `json:"role"`
	Content string `json:"content"`
}

type usage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
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
	targetURL := strings.TrimRight(deployment.Endpoint, "/") + "/chat/completions"

	body, err := encodeRequest(deployment, request)
	if err != nil {
		return provider.CompletionResult{},
			fmt.Errorf("failed to encode request body: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		targetURL,
		strings.NewReader(body),
	)
	if err != nil {
		return provider.CompletionResult{},
			fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return provider.CompletionResult{},
			fmt.Errorf("failed to complete request: %w", err)
	}

	var grokResponse groqResponse
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(&grokResponse); err != nil {
		return provider.CompletionResult{},
			fmt.Errorf("failed to decode response: %w", err)
	}

	return decodeResponse(grokResponse), nil
}

func encodeRequest(
	deployment provider.ModelDeployment,
	request provider.CompletionRequest,
) (string, error) {
	messages := []groqMessage{}
	for _, v := range request.Messages {
		messages = append(messages, groqMessage{
			Role:    v.Role,
			Content: v.Content,
		})
	}

	reqBody := groqRequest{
		Model:       deployment.ModelID,
		Messages:    messages,
		Temperature: request.Temperature,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request body: %w", err)
	}

	return string(body), nil
}

func decodeResponse(resp groqResponse) provider.CompletionResult {
	usage := provider.Usage{
		PromptTokens:     resp.Usage.PromptTokens,
		CompletionTokens: resp.Usage.CompletionTokens,
		TotalTokens:      resp.Usage.TotalTokens,
	}

	if len(resp.Choices) == 0 {
		return provider.CompletionResult{Usage: usage}
	}

	data := resp.Choices[0]
	return provider.CompletionResult{
		Response: provider.CompletionResponse{
			Message: provider.Message{
				Role:    data.Message.Role,
				Content: data.Message.Content,
			},
			FinishReason: data.FinishReason,
		},
		Usage: usage,
	}
}
