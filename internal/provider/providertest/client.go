package providertest

import (
	"context"

	"github.com/sergioazevedo/model-fleet/internal/openaiwire"
)

type Client struct {
	CompleteFunc func(
		context.Context,
		string,
		openaiwire.ChatCompletionRequest,
	) (openaiwire.ChatCompletionResponse, error)
}

func (c *Client) Complete(
	ctx context.Context,
	modelID string,
	request openaiwire.ChatCompletionRequest,
) (openaiwire.ChatCompletionResponse, error) {
	return c.CompleteFunc(ctx, modelID, request)
}
