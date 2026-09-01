package provider

import (
	"context"

	"github.com/sergioazevedo/model-fleet/internal/openaiwire"
)

type Client interface {
	Complete(
		ctx context.Context,
		modelID string,
		request openaiwire.ChatCompletionRequest,
	) (openaiwire.ChatCompletionResponse, error)
}
