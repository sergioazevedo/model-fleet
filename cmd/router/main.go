package main

import (
	_ "embed"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/sergioazevedo/model-fleet/internal/config"
	"github.com/sergioazevedo/model-fleet/internal/provider"
	"github.com/sergioazevedo/model-fleet/internal/provider/groq"
	"github.com/sergioazevedo/model-fleet/internal/provider/mistral"
	"github.com/sergioazevedo/model-fleet/internal/router"
)

//go:embed config.yaml
var modelFleetConfig string

func main() {
	// Load the model fleet configuration from the embedded YAML file
	fleetConfig, err := config.Load(strings.NewReader(modelFleetConfig))
	if err != nil {
		log.Fatalf("failed to parse model fleet config data: %v", err)
	}

	// Create an HTTP client with a timeout for provider requests
	transport := http.DefaultTransport.(*http.Transport).Clone()
	httpClient := &http.Client{
		Transport: transport,
		Timeout:   60 * time.Second,
	}

	// Build provider clients based on the configuration
	providers, err := buildProviderClients(
		fleetConfig.ProviderConnections,
		httpClient,
	)
	if err != nil {
		log.Fatalf("failed to build model providers: %v", err)
	}

	// Create the router handler with the fleet configuration and provider clients
	handler := router.NewHandler(
		fleetConfig,
		providers,
	)

	server := &http.Server{
		Addr:    ":4545",
		Handler: handler,
	}

	// Start the server and log any errors
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}
}

func buildProviderClients(
	providerConnections map[string]config.ProviderConnectionConfig,
	httpClient *http.Client,
) (map[string]provider.Client, error) {
	clients := map[string]provider.Client{}

	for connectionID, connConfig := range providerConnections {
		var client provider.Client
		apiKey, err := requiredEnv(connConfig.CredentialRef)
		if err != nil {
			return nil, fmt.Errorf(
				"failed to get API key for provider %q: %w",
				connConfig.Provider,
				err,
			)
		}

		switch connConfig.Provider {
		case "groq":
			client = groq.New(apiKey, httpClient)
		case "mistral":
			client = mistral.New(apiKey, httpClient)
		default:
			return nil, fmt.Errorf(
				"connection %q uses unsupported provider: %q",
				connectionID,
				connConfig.Provider,
			)
		}

		clients[connectionID] = client
	}

	return clients, nil
}

func requiredEnv(name string) (string, error) {
	value, exists := os.LookupEnv(name)
	if !exists || value == "" {
		return "", fmt.Errorf(
			"required environment variable %q is not set",
			name,
		)
	}

	return value, nil
}
