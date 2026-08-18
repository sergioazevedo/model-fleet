package openaicompatible_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/sergioazevedo/model-fleet/internal/provider"
	"github.com/sergioazevedo/model-fleet/internal/provider/openaicompatible"
	"github.com/sergioazevedo/model-fleet/internal/provider/providertest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testDeployment = provider.ModelDeployment{
	ModelID:  "openai/gpt-oss-120b",
	Endpoint: "https://provider.test",
}

func TestClient_Complete(t *testing.T) {
	t.Run("encodes the request", func(t *testing.T) {
		expectedJSON := `{
			"model": "openai/gpt-oss-120b",
			"messages": [
				{"role": "user", "content": "Suggest a simple dinner"},
				{
					"role": "assistant",
					"content": "",
					"tool_calls": [{
						"id": "call-1",
						"type": "function",
						"function": {
							"name": "find_recipe",
							"arguments": "{\"query\":\"pasta\"}"
						}
					}]
				},
				{"role": "tool", "content": "Pasta primavera", "tool_call_id": "call-1"}
			],
			"response_format": {"type": "json_object"},
			"reasoning_effort": "medium",
			"temperature": 0.2,
			"tools": [{
				"type": "function",
				"function": {
					"name": "find_recipe",
					"description": "Find a recipe",
					"parameters": {"type": "object"}
				}
			}],
			"tool_choice": "auto"
		}`

		resultBody := `{"choices":[{"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],"usage":{}}`
		httpClient, spy := providertest.NewHTTPClient(resultBody, http.StatusOK)
		client := openaicompatible.New("test-api-key", httpClient)
		responseFormat := provider.ResponseFormatJSON

		_, err := client.Complete(
			context.Background(),
			testDeployment,
			provider.CompletionRequest{
				Messages: []provider.Message{
					{Role: "user", Content: "Suggest a simple dinner"},
					{
						Role: "assistant",
						ToolCalls: []provider.ToolCall{
							{
								ID:        "call-1",
								Name:      "find_recipe",
								Arguments: json.RawMessage(`{"query":"pasta"}`),
							},
						},
					},
					{Role: "tool", Content: "Pasta primavera", ToolCallID: "call-1"},
				},
				Tools: []provider.Tool{
					{
						Name:        "find_recipe",
						Description: "Find a recipe",
						Parameters:  json.RawMessage(`{"type":"object"}`),
					},
				},
				Temperature:     ptr(0.2),
				ReasoningEffort: ptr("medium"),
				ResponseFormat:  &responseFormat,
			},
		)

		require.NoError(t, err)
		require.NotNil(t, spy.Request)
		assert.Equal(t, http.MethodPost, spy.Request.Method)
		assert.Equal(t, "https://provider.test/chat/completions", spy.Request.URL.String())
		assert.Equal(t, "application/json", spy.Request.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-api-key", spy.Request.Header.Get("Authorization"))
		assert.JSONEq(t, expectedJSON, spy.RequestBody)
	})

	t.Run("decodes response", func(t *testing.T) {
		const response = `{
			"choices": [{
				"message": {
					"role": "assistant",
					"content": "Try pasta",
					"tool_calls": [{
						"id": "call-1",
						"type": "function",
						"function": {
							"name": "find_recipe",
							"arguments": "{\"query\":\"pasta\"}"
						}
					}]
				},
				"finish_reason": "tool_calls"
			}],
			"usage": {
				"prompt_tokens": 10,
				"completion_tokens": 2,
				"total_tokens": 12
			}
		}`

		httpClient, _ := providertest.NewHTTPClient(response, http.StatusOK)
		client := openaicompatible.New("test-api-key", httpClient)

		result, err := client.Complete(
			context.Background(),
			testDeployment,
			provider.CompletionRequest{
				Messages: []provider.Message{
					{Role: "user", Content: "Suggest a simple dinner"},
				},
			},
		)
		require.NoError(t, err)

		want := provider.CompletionResult{
			Response: provider.CompletionResponse{
				Message: provider.Message{
					Role:    "assistant",
					Content: "Try pasta",
					ToolCalls: []provider.ToolCall{
						{
							ID:        "call-1",
							Name:      "find_recipe",
							Arguments: json.RawMessage(`{"query":"pasta"}`),
						},
					},
				},
				FinishReason: "tool_calls",
			},
			Usage: provider.Usage{
				PromptTokens:     10,
				CompletionTokens: 2,
				TotalTokens:      12,
			},
		}

		assert.Equal(t, want, result)
	})

	t.Run("normalizes unsuccessful responses", func(t *testing.T) {
		const response = `{
			"error": {
				"message": "provider message"
			}
		}`

		tests := []struct {
			name         string
			status       int
			wantCategory provider.ErrorCategory
		}{
			{name: "bad request", status: http.StatusBadRequest, wantCategory: provider.ErrorCategoryInvalidRequest},
			{name: "unprocessable entity", status: http.StatusUnprocessableEntity, wantCategory: provider.ErrorCategoryInvalidRequest},
			{name: "unauthorized", status: http.StatusUnauthorized, wantCategory: provider.ErrorCategoryAuthenticationFailed},
			{name: "forbidden", status: http.StatusForbidden, wantCategory: provider.ErrorCategoryAuthenticationFailed},
			{name: "model not found", status: http.StatusNotFound, wantCategory: provider.ErrorCategoryModelUnavailable},
			{name: "rate limited", status: http.StatusTooManyRequests, wantCategory: provider.ErrorCategoryRateLimited},
			{name: "internal server error", status: http.StatusInternalServerError, wantCategory: provider.ErrorCategoryProviderUnavailable},
			{name: "bad gateway", status: http.StatusBadGateway, wantCategory: provider.ErrorCategoryProviderUnavailable},
			{name: "service unavailable", status: http.StatusServiceUnavailable, wantCategory: provider.ErrorCategoryProviderUnavailable},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				httpClient, _ := providertest.NewHTTPClient(response, tt.status)
				client := openaicompatible.New("test-api-key", httpClient)

				_, err := client.Complete(
					context.Background(),
					testDeployment,
					provider.CompletionRequest{},
				)

				var providerErr *provider.ProviderError
				require.ErrorAs(t, err, &providerErr)
				assert.Equal(t, tt.wantCategory, providerErr.Category)
				assert.Equal(t, tt.status, providerErr.StatusCode)
				assert.EqualError(t, providerErr.Cause, "provider message")
			})
		}
	})
}

func ptr[T any](value T) *T {
	return &value
}
