package provider_test

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/sergioazevedo/model-fleet/internal/openaiwire"
	"github.com/sergioazevedo/model-fleet/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type connectivityTestConfig struct {
	APIKeyEnv string
	ModelID   string
	NewClient func(string, *http.Client) provider.Client
}

func runConnectivityTest(t *testing.T, config connectivityTestConfig) {
	t.Helper()

	if os.Getenv("MODEL_FLEET_LIVE_TESTS") != "1" {
		t.Skip("set MODEL_FLEET_LIVE_TESTS=1 to run live provider tests")
	}

	apiKey := os.Getenv(config.APIKeyEnv)
	require.NotEmpty(t, apiKey, "%s must be set for live provider tests", config.APIKeyEnv)

	httpClient := &http.Client{Timeout: 30 * time.Second}
	client := config.NewClient(apiKey, httpClient)
	temperature := 0.0

	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	result, err := client.Complete(
		ctx,
		config.ModelID,
		openaiwire.ChatCompletionRequest{
			Messages: []openaiwire.Message{
				{Role: "user", Content: "Reply with one word: connected"},
			},
			Temperature: &temperature,
		},
	)

	require.NoError(t, err)
	require.NotEmpty(t, result.Choices)
	assert.Equal(t, "assistant", result.Choices[0].Message.Role)
	assert.NotEmpty(t, result.Choices[0].Message.Content)
	assert.NotEmpty(t, result.Choices[0].FinishReason)
	assert.Positive(t, result.Usage.TotalTokens)
}
