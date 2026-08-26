package mistral

import (
	"context"
	"net/http"

	"github.com/sergioazevedo/model-fleet/internal/provider"
	"github.com/sergioazevedo/model-fleet/internal/provider/openaicompatible"
)

type MistralClient struct {
	client *openaicompatible.Client
}

func New(endpoint string, apiKey string, httpClient *http.Client) *MistralClient {
	return &MistralClient{
		client: openaicompatible.New(
			endpoint,
			apiKey,
			httpClient,
		),
	}
}

func (c *MistralClient) Complete(
	ctx context.Context,
	modelID string,
	request provider.CompletionRequest,
) (provider.CompletionResult, error) {
	return c.client.Complete(ctx, modelID, request)
}
