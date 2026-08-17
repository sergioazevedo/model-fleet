package groq_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/sergioazevedo/model-fleet/internal/provider"
	"github.com/sergioazevedo/model-fleet/internal/provider/groq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type requestSpy struct {
	responseBody   string
	responseStatus int
	request        *http.Request
	requestBody    string
}

var testDeployment = provider.ModelDeployment{
	ModelID:  "openai/gpt-oss-120b",
	Endpoint: "https://groq.test",
}

func TestGroqClient_Complete(t *testing.T) {
	t.Run("encodes the request", func(t *testing.T) {
		expectedJSON := `{
			"model": "openai/gpt-oss-120b",
			"messages": [{"role": "user","content": "Suggest a simple dinner"}],
			"temperature": 0.2
		}`

		resultBody := `{"choices": [{"message": {"role": "assistant","content": "ok"},"finish_reason": "stop"}],"usage": {}}`

		httpClient, spy := stubHTTPClient(resultBody, http.StatusOK)
		client := groq.New("test-api-key", httpClient)

		_, err := client.Complete(
			context.Background(),
			testDeployment,
			provider.CompletionRequest{
				Messages: []provider.Message{
					{Role: "user", Content: "Suggest a simple dinner"},
				},
				Temperature: ptr(0.2),
			},
		)

		require.NoError(t, err)
		require.NotNil(t, spy.request)
		assert.Equal(t, http.MethodPost, spy.request.Method)
		assert.Equal(t, "https://groq.test/chat/completions", spy.request.URL.String())
		assert.Equal(t, "application/json", spy.request.Header.Get("Content-Type"))
		assert.Equal(t, "Bearer test-api-key", spy.request.Header.Get("Authorization"))
		assert.JSONEq(t, expectedJSON, spy.requestBody)
	})

	t.Run("decodes response", func(t *testing.T) {
		const response = `{
			"choices": [{
				"message": {
					"role": "assistant",
					"content": "Try pasta"
				},
				"finish_reason": "stop"
			}],
			"usage": {
				"prompt_tokens": 10,
				"completion_tokens": 2,
				"total_tokens": 12
			}
		}`

		httpClient, _ := stubHTTPClient(response, http.StatusOK)
		client := groq.New("test-api-key", httpClient)

		result, err := client.Complete(
			context.Background(),
			testDeployment,
			provider.CompletionRequest{
				Messages: []provider.Message{
					{Role: "user", Content: "Suggest a simple dinner"},
				},
				Temperature: ptr(0.2),
			},
		)
		require.NoError(t, err)

		want := provider.CompletionResult{
			Response: provider.CompletionResponse{
				Message: provider.Message{
					Role:    "assistant",
					Content: "Try pasta",
				},
				FinishReason: "stop",
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
			{
				name:         "bad request",
				status:       http.StatusBadRequest,
				wantCategory: provider.ErrorCategoryInvalidRequest,
			},
			{
				name:         "unprocessable entity",
				status:       http.StatusUnprocessableEntity,
				wantCategory: provider.ErrorCategoryInvalidRequest,
			},
			{
				name:         "unauthorized",
				status:       http.StatusUnauthorized,
				wantCategory: provider.ErrorCategoryAuthenticationFailed,
			},
			{
				name:         "forbidden",
				status:       http.StatusForbidden,
				wantCategory: provider.ErrorCategoryAuthenticationFailed,
			},
			{
				name:         "model not found",
				status:       http.StatusNotFound,
				wantCategory: provider.ErrorCategoryModelUnavailable,
			},
			{
				name:         "rate limited",
				status:       http.StatusTooManyRequests,
				wantCategory: provider.ErrorCategoryRateLimited,
			},
			{
				name:         "internal server error",
				status:       http.StatusInternalServerError,
				wantCategory: provider.ErrorCategoryProviderUnavailable,
			},
			{
				name:         "bad gateway",
				status:       http.StatusBadGateway,
				wantCategory: provider.ErrorCategoryProviderUnavailable,
			},
			{
				name:         "service unavailable",
				status:       http.StatusServiceUnavailable,
				wantCategory: provider.ErrorCategoryProviderUnavailable,
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				httpClient, _ := stubHTTPClient(response, tt.status)
				client := groq.New("test-api-key", httpClient)

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

func (s *requestSpy) RoundTrip(req *http.Request) (*http.Response, error) {
	defer req.Body.Close()

	reqBody, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}

	s.request = req
	s.requestBody = string(reqBody)

	return &http.Response{
		StatusCode: s.responseStatus,
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(strings.NewReader(s.responseBody)),
	}, nil
}

func stubHTTPClient(responseBody string, responseStatus int) (*http.Client, *requestSpy) {
	spy := &requestSpy{
		responseBody:   responseBody,
		responseStatus: responseStatus,
	}

	return &http.Client{
		Transport: spy,
	}, spy
}

func ptr[T any](value T) *T {
	return &value
}
