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

type grokResponse struct {
	Choices []choice
	Usage   usage
}

type choice struct {
	Message      responseMessage `json:"message"`
	FinishReason string          `json:"finish_reason"`
}

type responseMessage struct {
	Role    string
	Content string
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

	req, err := http.NewRequestWithContext(ctx, "POST", targetURL, strings.NewReader(""))
	if err != nil {
		return provider.CompletionResult{},
			fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return provider.CompletionResult{},
			fmt.Errorf("failed to complete request: %w", err)
	}

	var grokResponse grokResponse
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(&grokResponse); err != nil {
		return provider.CompletionResult{},
			fmt.Errorf("failed to decode response: %w", err)
	}

	usage := provider.Usage{
		PromptTokens:     grokResponse.Usage.PromptTokens,
		CompletionTokens: grokResponse.Usage.CompletionTokens,
		TotalTokens:      grokResponse.Usage.TotalTokens,
	}

	if len(grokResponse.Choices) == 0 {
		return provider.CompletionResult{Usage: usage}, nil
	}

	data := grokResponse.Choices[0]
	return provider.CompletionResult{
		Response: provider.CompletionResponse{
			Message: provider.Message{
				Role:    data.Message.Role,
				Content: data.Message.Content,
			},
			FinishReason: data.FinishReason,
		},
		Usage: usage,
	}, err
}
