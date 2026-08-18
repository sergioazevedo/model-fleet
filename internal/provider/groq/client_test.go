package groq_test

import (
	"context"
	"net/http"
	"testing"

	"github.com/sergioazevedo/model-fleet/internal/provider"
	"github.com/sergioazevedo/model-fleet/internal/provider/groq"
	"github.com/sergioazevedo/model-fleet/internal/provider/providertest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGroqClient_Complete_UsesConfiguredCompatibleClient(t *testing.T) {
	const response = `{"choices":[{"message":{"role":"assistant","content":"Try pasta"},"finish_reason":"stop"}],"usage":{}}`

	httpClient, spy := providertest.NewHTTPClient(response, http.StatusOK)
	client := groq.New("test-api-key", httpClient)

	result, err := client.Complete(
		context.Background(),
		provider.ModelDeployment{
			ModelID:  "openai/gpt-oss-120b",
			Endpoint: "https://groq.test",
		},
		provider.CompletionRequest{
			Messages: []provider.Message{
				{Role: "user", Content: "Suggest a simple dinner"},
			},
		},
	)

	require.NoError(t, err)
	require.NotNil(t, spy.Request)
	assert.Equal(t, "Bearer test-api-key", spy.Request.Header.Get("Authorization"))
	assert.Equal(t, "https://groq.test/chat/completions", spy.Request.URL.String())
	assert.Equal(t, "Try pasta", result.Response.Message.Content)
}
