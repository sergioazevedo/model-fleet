package openaicompatible

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sergioazevedo/model-fleet/internal/provider"
)

type Client struct {
	apiKey     string
	httpClient *http.Client
}

func New(apiKey string, httpClient *http.Client) *Client {
	return &Client{
		apiKey:     apiKey,
		httpClient: httpClient,
	}
}

func (c *Client) Complete(
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
		return provider.CompletionResult{}, normalizeErrorResponse(resp)
	}

	result, err := decodeResponse(resp)
	if err != nil {
		return provider.CompletionResult{}, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}

func normalizeErrorResponse(resp *http.Response) *provider.ProviderError {
	var category provider.ErrorCategory

	statusCode := resp.StatusCode
	retryAfter := time.Duration(0)
	switch {
	// 429 is a special case for rate limiting, so we handle it explicitly.
	case statusCode == http.StatusTooManyRequests:
		category = provider.ErrorCategoryRateLimited
		retryAfterHeader := resp.Header.Get("Retry-After")
		if retryAfterHeader != "" {
			if d, err := time.ParseDuration(retryAfterHeader + "s"); err == nil {
				retryAfter = d
			}
		}
	// 401 and 403 as authentication errors
	case statusCode == http.StatusUnauthorized ||
		statusCode == http.StatusForbidden:
		category = provider.ErrorCategoryAuthenticationFailed
	// 404 is a special case for model unavailability, so we handle it explicitly.
	case statusCode == http.StatusNotFound:
		category = provider.ErrorCategoryModelUnavailable
	// 500 and above are considered provider errors
	case statusCode >= 500:
		category = provider.ErrorCategoryProviderUnavailable
	default:
		category = provider.ErrorCategoryInvalidRequest
	}

	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))

	var respBody response
	var cause error

	if err := json.Unmarshal(bodyBytes, &respBody); err != nil {
		cause = fmt.Errorf("failed to decode error response: %w", err)
	} else if respBody.Error.Message != "" {
		cause = errors.New(respBody.Error.Message)
	}

	return &provider.ProviderError{
		Category:   category,
		StatusCode: resp.StatusCode,
		RetryAfter: retryAfter,
		Cause:      cause,
	}
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
	req provider.CompletionRequest,
) (string, error) {
	messages := []message{}
	for _, v := range req.Messages {
		mappedToolCalls, err := mapToolCalls(v.ToolCalls)
		if err != nil {
			return "", fmt.Errorf("failed to map tool calls: %w", err)
		}

		messages = append(messages, message{
			Role:       v.Role,
			Content:    v.Content,
			ToolCalls:  mappedToolCalls,
			ToolCallID: v.ToolCallID,
		})
	}

	reqBody := request{
		Model:           deployment.ModelID,
		Messages:        messages,
		Temperature:     req.Temperature,
		ReasoningEffort: req.ReasoningEffort,
		ResponseFormat:  mapToResponseFormat(req.ResponseFormat),
	}
	if len(req.Tools) > 0 {
		reqBody.Tools = mapToTools(req.Tools)
		reqBody.ToolChoice = "auto"
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request body: %w", err)
	}

	return string(body), nil
}

func mapToResponseFormat(format *provider.ResponseFormat) *responseFormat {
	if format == nil {
		return nil
	}

	return &responseFormat{Type: format.String()}
}

func mapToolCalls(toolCalls []provider.ToolCall) ([]toolCall, error) {
	result := []toolCall{}
	for _, call := range toolCalls {
		if !json.Valid(call.Arguments) {
			return nil, fmt.Errorf("invalid JSON in tool call arguments for ID %s", call.ID)
		}

		result = append(result, toolCall{
			ID:   call.ID,
			Type: "function",
			Function: toolFunctionCall{
				Name:      call.Name,
				Arguments: string(call.Arguments),
			},
		})
	}

	return result, nil
}

func mapToTools(tools []provider.Tool) []tool {
	result := []tool{}
	for _, t := range tools {
		result = append(result, tool{
			Type: "function",
			Function: toolFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.Parameters,
			},
		})
	}

	return result
}

func decodeResponse(resp *http.Response) (provider.CompletionResult, error) {
	var respBody response
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return provider.CompletionResult{}, fmt.Errorf("failed to decode response: %w", err)
	}

	// Drain remaining body
	_, _ = io.Copy(io.Discard, resp.Body)

	usage := provider.Usage{
		PromptTokens:     respBody.Usage.PromptTokens,
		CompletionTokens: respBody.Usage.CompletionTokens,
		TotalTokens:      respBody.Usage.TotalTokens,
	}

	if len(respBody.Choices) == 0 {
		return provider.CompletionResult{Usage: usage}, fmt.Errorf("no content generated")
	}

	var toolCalls []provider.ToolCall
	for _, call := range respBody.Choices[0].Message.ToolCalls {
		if !json.Valid([]byte(call.Function.Arguments)) {
			return provider.CompletionResult{}, fmt.Errorf("invalid JSON in tool call arguments for ID %s", call.ID)
		}

		toolCalls = append(toolCalls, provider.ToolCall{
			ID:        call.ID,
			Name:      call.Function.Name,
			Arguments: json.RawMessage(call.Function.Arguments),
		})
	}

	data := respBody.Choices[0]

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
