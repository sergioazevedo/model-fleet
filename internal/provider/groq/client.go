package groq

import (
	"context"
	"net/http"

	"github.com/sergioazevedo/model-fleet/internal/openaiwire"
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
	request openaiwire.ChatCompletionRequest,
) (openaiwire.ChatCompletionResponse, error) {
	return c.client.Complete(ctx, modelID, request)
}
