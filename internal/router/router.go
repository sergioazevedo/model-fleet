package router

import (
	"net/http"

	"github.com/sergioazevedo/model-fleet/internal/config"
	"github.com/sergioazevedo/model-fleet/internal/provider"
)

func NewHandler(
	fleetConfig config.Config,
	providerClients map[string]provider.Client,
) http.Handler {
	return nil
}
