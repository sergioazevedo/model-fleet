package provider_test

import (
	"net/http"
	"testing"

	"github.com/sergioazevedo/model-fleet/internal/provider"
	"github.com/sergioazevedo/model-fleet/internal/provider/groq"
)

func TestGroqClient_Connectivity(t *testing.T) {
	runConnectivityTest(t, connectivityTestConfig{
		APIKeyEnv: "GROQ_API_KEY",
		Endpoint:  "https://api.groq.com/openai/v1",
		ModelID:   "openai/gpt-oss-120b",
		NewClient: func(apiKey string, httpClient *http.Client) provider.Client {
			return groq.New(apiKey, httpClient)
		},
	})
}
