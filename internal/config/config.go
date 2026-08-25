package config

import (
	"fmt"
	"io"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ProviderConnections map[string]ProviderConnectionConfig `yaml:"provider_connections"`
	Deployments         map[string]DeploymentConfig         `yaml:"deployments"`
	RoleRoutes          map[string]RoleRouteConfig          `yaml:"routes"`
}

type ProviderConnectionConfig struct {
	Provider      string `yaml:"provider"`
	Endpoint      string `yaml:"endpoint"`
	CredentialRef string `yaml:"credential_ref"`
	QuotaPool     string `yaml:"quota_pool"`
}

type DeploymentConfig struct {
	Connection string `yaml:"connection"`
	Model      string `yaml:"model"`
}
type RoleRouteConfig struct {
	DeploymentIDs []string `yaml:"deployments"`
}

func Load(reader io.Reader) (Config, error) {
	var config Config
	decoder := yaml.NewDecoder(reader)
	decoder.KnownFields(true)

	if err := decoder.Decode(&config); err != nil {
		return Config{}, fmt.Errorf("failed decoding configuration: %w", err)
	}

	if err := config.validate(); err != nil {
		return Config{}, fmt.Errorf("failed validating configuration: %w", err)
	}

	return config, nil
}

func (c Config) validate() error {
	for routeName, routeConfig := range c.RoleRoutes {
		if len(routeConfig.DeploymentIDs) == 0 {
			return fmt.Errorf("route %q has no deployments", routeName)
		}

		for _, deploymentID := range routeConfig.DeploymentIDs {
			if _, exists := c.Deployments[deploymentID]; !exists {
				return fmt.Errorf(
					"route %q references unknown deployment %q",
					routeName,
					deploymentID,
				)
			}

			//deployment must have a connection specified
			if c.Deployments[deploymentID].Connection == "" {
				return fmt.Errorf(
					"deployment %q has no connection specified",
					deploymentID,
				)
			}

			//connection must exist in provider connections
			connectionName := c.Deployments[deploymentID].Connection
			if _, exists := c.ProviderConnections[connectionName]; !exists {
				return fmt.Errorf(
					"deployment %q references unknown connection %q",
					deploymentID,
					connectionName,
				)
			}
		}
	}

	return nil
}
