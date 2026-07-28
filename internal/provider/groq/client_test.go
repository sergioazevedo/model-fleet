package groq_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/sergioazevedo/model-fleet/internal/provider"
	"github.com/sergioazevedo/model-fleet/internal/provider/groq"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroqClient_Complete_DecodesResponse(t *testing.T) {
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

	server := newJSONServer(t, response)
	client := groq.New("test-api-key", server.Client())
	deployment := provider.ModelDeployment{
		ModelID:  "openai/gpt-oss-120b",
		Endpoint: server.URL,
	}

	request := provider.CompletionRequest{
		Messages: []provider.Message{
			{
				Role:    "user",
				Content: "Suggest a simple dinner",
			},
		},
	}

	result, err := client.Complete(context.Background(), deployment, request)
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
}

func newJSONServer(t *testing.T, body string) *httptest.Server {
	t.Helper()

	server := httptest.NewServer(http.HandlerFunc(func(
		w http.ResponseWriter,
		_ *http.Request,
	) {
		w.Header().Set("Content-Type", "application/json")

		_, err := w.Write([]byte(body))
		assert.NoError(t, err)
	}))
	t.Cleanup(server.Close)

	return server
}
