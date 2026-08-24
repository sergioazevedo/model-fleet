package config

import (
	_ "embed"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

//go:embed testdata/valid.yaml
var testConfig string

//go:embed testdata/unknown-deployment.yaml
var missingDeploymentConfig string

//go:embed testdata/no_deployments.yaml
var routeWithNoDeploymentsConfig string

func TestLoad(t *testing.T) {
	t.Run("loads logical route with ordered deployments", func(t *testing.T) {
		testReader := strings.NewReader(testConfig)
		config, err := Load(testReader)

		assert.NoError(t, err)

		expectedConfig := Config{
			Deployments: map[string]DeploymentConfig{
				"groq-analyst": {
					Provider:      "groq",
					Model:         "openai/gpt-oss-120b",
					Endpoint:      "https://api.groq.com/openai/v1",
					CredentialRef: "GROQ_API_KEY",
					QuotaPool:     "groq-personal",
				},
				"mistral-analyst": {
					Provider:      "mistral",
					Model:         "mistral-small-2603",
					Endpoint:      "https://api.mistral.ai/v1",
					CredentialRef: "MISTRAL_API_KEY",
					QuotaPool:     "mistral-personal",
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

}
