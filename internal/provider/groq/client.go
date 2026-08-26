package groq

import (
	"context"
	"net/http"

	"github.com/sergioazevedo/model-fleet/internal/provider"
	"github.com/sergioazevedo/model-fleet/internal/provider/openaicompatible"
)

type GroqClient struct {
	client *openaicompatible.Client
}

func New(endpoint string, apiKey string, httpClient *http.Client) *GroqClient {
	return &GroqClient{
		client: openaicompatible.New(
			endpoint,
			apiKey,
			httpClient,
		),
	}
}

func (c *GroqClient) Complete(
	ctx context.Context,
	modelID string,
	request provider.CompletionRequest,
) (provider.CompletionResult, error) {
	return c.client.Complete(ctx, modelID, request)
}
