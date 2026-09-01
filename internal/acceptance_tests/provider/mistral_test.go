package provider_test

import (
	"net/http"
	"testing"

	"github.com/sergioazevedo/model-fleet/internal/provider"
	"github.com/sergioazevedo/model-fleet/internal/provider/mistral"
)

func TestMistralClient_Connectivity(t *testing.T) {
	runConnectivityTest(t, connectivityTestConfig{
		APIKeyEnv: "MISTRAL_API_KEY",
		ModelID:   "mistral-small-2603",
		NewClient: func(apiKey string, httpClient *http.Client) provider.Client {
			return mistral.New("https://api.mistral.ai/v1", apiKey, httpClient)
		},
	})
}
