package router_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/sergioazevedo/model-fleet/internal/config"
	"github.com/sergioazevedo/model-fleet/internal/openaiwire"
	"github.com/sergioazevedo/model-fleet/internal/provider"
	"github.com/sergioazevedo/model-fleet/internal/provider/providertest"
	"github.com/sergioazevedo/model-fleet/internal/router"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandler_Completion(t *testing.T) {
	t.Run("routes an OpenAI-compatible request and returns the provider response", func(t *testing.T) {
		responseFormat := &openaiwire.ResponseFormat{Type: "json_object"}
		providerResponse := openaiwire.ChatCompletionResponse{
			ID:      "completion-1",
			Object:  "chat.completion",
			Created: 123,
			Model:   "provider-reported-alias",
			Choices: []openaiwire.Choice{
				{
					Message: openaiwire.Message{
						Role:    "assistant",
						Content: "Try pasta",
					},
					FinishReason: "stop",
				},
			},
			Usage: openaiwire.Usage{
				PromptTokens:     10,
				CompletionTokens: 2,
				TotalTokens:      12,
			},
		}

		var capturedModelID string
		var capturedRequest openaiwire.ChatCompletionRequest
		client := &providertest.Client{
			CompleteFunc: func(
				_ context.Context,
				modelID string,
				request openaiwire.ChatCompletionRequest,
			) (openaiwire.ChatCompletionResponse, error) {
				capturedModelID = modelID
				capturedRequest = request
				return providerResponse, nil
			},
		}

		handler := router.NewHandler(testConfig(), map[string]provider.Client{"groq": client})
		requestBody := `{
			"model": "meal-planner/analyst",
			"messages": [{"role": "user", "content": "Suggest a simple dinner"}],
			"response_format": {"type": "json_object"},
			"tools": [{
				"type": "function",
				"function": {
					"name": "find_recipe",
					"description": "Find a recipe",
					"parameters": {"type": "object"}
				}
			}]
		}`
		req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(requestBody))
		res := httptest.NewRecorder()

		handler.ServeHTTP(res, req)

		require.Equal(t, http.StatusOK, res.Code)
		assert.Equal(t, "application/json", res.Header().Get("Content-Type"))
		assert.Equal(t, "groq", res.Header().Get("X-Model-Fleet-Provider"))
		assert.Equal(t, "physical-model", res.Header().Get("X-Model-Fleet-ModelId"))
		assert.Equal(t, "groq-analyst", res.Header().Get("X-Model-Fleet-DeploymentId"))
		assert.Equal(t, "physical-model", capturedModelID)
		assert.Equal(t, "meal-planner/analyst", capturedRequest.Model)
		assert.Equal(t, responseFormat, capturedRequest.ResponseFormat)
		require.Len(t, capturedRequest.Tools, 1)
		assert.Equal(t, "find_recipe", capturedRequest.Tools[0].Function.Name)

		expectedResponse := providerResponse
		expectedResponse.Model = "physical-model"
		expectedBody, err := json.Marshal(expectedResponse)
		require.NoError(t, err)
		assert.JSONEq(t, string(expectedBody), res.Body.String())
	})

	t.Run("returns a JSON error for an unsupported logical model", func(t *testing.T) {
		handler := router.NewHandler(testConfig(), map[string]provider.Client{})
		req := httptest.NewRequest(
			http.MethodPost,
			"/v1/chat/completions",
			strings.NewReader(`{"model":"unknown","messages":[]}`),
		)
		res := httptest.NewRecorder()

		handler.ServeHTTP(res, req)

		assert.Equal(t, http.StatusBadRequest, res.Code)
		assert.Equal(t, "application/json", res.Header().Get("Content-Type"))
		assert.JSONEq(t, `{"error":{"message":"router.Handler.handleCompletion: unsupported model: unknown"}}`, res.Body.String())
	})

	t.Run("returns retry metadata for provider rate limits", func(t *testing.T) {
		client := &providertest.Client{
			CompleteFunc: func(
				context.Context,
				string,
				openaiwire.ChatCompletionRequest,
			) (openaiwire.ChatCompletionResponse, error) {
				return openaiwire.ChatCompletionResponse{}, &provider.ProviderError{
					Category:   provider.ErrorCategoryRateLimited,
					StatusCode: http.StatusTooManyRequests,
					RetryAfter: 30 * time.Second,
					Cause:      errors.New("quota exceeded"),
				}
			},
		}
		handler := router.NewHandler(testConfig(), map[string]provider.Client{"groq": client})
		req := httptest.NewRequest(
			http.MethodPost,
			"/v1/chat/completions",
			strings.NewReader(`{"model":"meal-planner/analyst","messages":[]}`),
		)
		res := httptest.NewRecorder()

		handler.ServeHTTP(res, req)

		assert.Equal(t, http.StatusTooManyRequests, res.Code)
		assert.Equal(t, "30", res.Header().Get("Retry-After"))
		assert.JSONEq(
			t,
			`{"error":{"message":"router.Handler.handleCompletion: failed request for model: meal-planner/analyst: provider error: rate_limited (status 429): quota exceeded"}}`,
			res.Body.String(),
		)
	})
}

func testConfig() config.Config {
	return config.Config{
		ProviderConnections: map[string]config.ProviderConnectionConfig{
			"groq": {
				Provider: "groq",
				Endpoint: "https://api.groq.com",
			},
		},
		RoleRoutes: map[string]config.RoleRouteConfig{
			"meal-planner/analyst": {DeploymentIDs: []string{"groq-analyst"}},
		},
		Deployments: map[string]config.DeploymentConfig{
			"groq-analyst": {
				Connection: "groq",
				Model:      "physical-model",
			},
		},
	}
}
