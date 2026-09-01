package config

import (
	_ "embed"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

//go:embed testdata/valid.yaml
var testConfig string

//go:embed testdata/unknown_deployment.yaml
var missingDeploymentConfig string

//go:embed testdata/no_deployments.yaml
var routeWithNoDeploymentsConfig string

//go:embed testdata/unknown_connection.yaml
var missingConnectionForDeploymentConfig string

func TestLoad(t *testing.T) {
	t.Run("loads logical route with ordered deployments", func(t *testing.T) {
		testReader := strings.NewReader(testConfig)
		config, err := Load(testReader)

		require.NoError(t, err)

		expectedConfig := Config{
			ProviderConnections: map[string]ProviderConnectionConfig{
				"groq": {
					Provider:      "groq",
					Endpoint:      "https://api.groq.com/openai/v1",
					CredentialRef: "GROQ_API_KEY",
					QuotaPool:     "groq-personal",
				},
				"mistral": {
					Provider:      "mistral",
					Endpoint:      "https://api.mistral.ai/v1",
					CredentialRef: "MISTRAL_API_KEY",
					QuotaPool:     "mistral-personal",
				},
			},
			Deployments: map[string]DeploymentConfig{
				"groq-analyst": {
					Model:      "openai/gpt-oss-120b",
					Connection: "groq",
				},
				"mistral-analyst": {
					Model:      "mistral-small-2603",
					Connection: "mistral",
				},
			},
			RoleRoutes: map[string]RoleRouteConfig{
				"meal-planner/analyst": {
					DeploymentIDs: []string{"groq-analyst", "mistral-analyst"},
				},
			},
		}

		assert.Equal(t, expectedConfig, config)
	})

	t.Run("errors when missing a Deployment", func(t *testing.T) {
		testReader := strings.NewReader(missingDeploymentConfig)
		_, err := Load(testReader)

		assert.Error(t, err)
	})

	t.Run("errors when a route has no Deployments", func(t *testing.T) {
		testReader := strings.NewReader(routeWithNoDeploymentsConfig)
		_, err := Load(testReader)

		assert.Error(t, err)
	})

	t.Run("errors when Deployment has missing connection", func(t *testing.T) {
		testReader := strings.NewReader(missingConnectionForDeploymentConfig)
		_, err := Load(testReader)

		assert.Error(t, err)
	})

}
