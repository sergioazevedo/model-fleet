package mistral

import (
	"context"
	"net/http"

	"github.com/sergioazevedo/model-fleet/internal/openaiwire"
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
	request openaiwire.ChatCompletionRequest,
) (openaiwire.ChatCompletionResponse, error) {
	return c.client.Complete(ctx, modelID, request)
}
