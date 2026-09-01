package router

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"github.com/sergioazevedo/model-fleet/internal/config"
	"github.com/sergioazevedo/model-fleet/internal/openaiwire"
	"github.com/sergioazevedo/model-fleet/internal/provider"
)

type Handler struct {
	fleetConfig     config.Config
	providerClients map[string]provider.Client
}

func NewHandler(
	fleetConfig config.Config,
	providerClients map[string]provider.Client,
) http.Handler {

	handler := &Handler{
		fleetConfig:     fleetConfig,
		providerClients: providerClients,
	}

	mux := http.NewServeMux()
	mux.HandleFunc(
		"POST /v1/chat/completions",
		handler.handleCompletion,
	)

	return mux
}

func (h *Handler) handleCompletion(
	w http.ResponseWriter,
	r *http.Request,
) {
	completionReq, err := parseRequest(r)
	if err != nil {
		writeError(
			w,
			http.StatusBadRequest,
			fmt.Sprintf("router.Handler.handleCompletion: failed to parse request: %v", err),
		)
		return
	}

	roleRoute := completionReq.Model
	roleConfig, exists := h.fleetConfig.RoleRoutes[roleRoute]
	if !exists {
		writeError(
			w,
			http.StatusBadRequest,
			fmt.Sprintf("router.Handler.handleCompletion: unsupported model: %s", roleRoute),
		)
		return
	}

	if len(roleConfig.DeploymentIDs) == 0 {
		writeError(
			w,
			http.StatusPreconditionFailed,
			fmt.Sprintf("router.Handler.handleCompletion: no deployments configured for model: %s", roleRoute),
		)
		return
	}

	// TODO: route across all configured deployments instead of always selecting the first.
	candidateDeploymentID := roleConfig.DeploymentIDs[0]
	deploymentConfig := h.fleetConfig.Deployments[candidateDeploymentID]

	targetClient, exists := h.providerClients[deploymentConfig.Connection]
	if !exists {
		writeError(
			w,
			http.StatusPreconditionFailed,
			fmt.Sprintf("router.Handler.handleCompletion: no provider configured for model: %s", roleRoute),
		)
		return
	}

	providerResp, err := targetClient.Complete(
		r.Context(),
		deploymentConfig.Model,
		completionReq,
	)

	if err != nil {
		var providerErr *provider.ProviderError

		code := http.StatusBadGateway
		if errors.As(err, &providerErr) {
			switch providerErr.Category {
			case provider.ErrorCategoryRateLimited:
				code = http.StatusTooManyRequests
				if providerErr.RetryAfter > 0 {
					w.Header().Set(
						"Retry-After",
						strconv.FormatInt(
							int64(providerErr.RetryAfter/time.Second),
							10,
						),
					)
				}
			case provider.ErrorCategoryModelUnavailable:
				code = http.StatusServiceUnavailable
			case provider.ErrorCategoryAuthenticationFailed:
				code = http.StatusServiceUnavailable
			case provider.ErrorCategoryProviderUnavailable:
				code = http.StatusServiceUnavailable
			case provider.ErrorCategoryInvalidRequest:
				code = http.StatusBadRequest
			}
		}

		writeError(
			w,
			code,
			fmt.Sprintf("router.Handler.handleCompletion: failed request for model: %s: %v", roleRoute, err),
		)
		return
	}

	writeJSON(w, http.StatusOK, providerResp)
}

func parseRequest(r *http.Request) (openaiwire.ChatCompletionRequest, error) {
	var providerReq openaiwire.ChatCompletionRequest

	defer r.Body.Close()

	if err := json.NewDecoder(r.Body).Decode(&providerReq); err != nil {
		return openaiwire.ChatCompletionRequest{}, fmt.Errorf(
			"failed to decode provider request body: %w",
			err,
		)
	}

	// Discard any remaining data in the request body to ensure it's fully read
	io.Copy(io.Discard, r.Body)

	return providerReq, nil
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(
		w,
		status,
		openaiwire.ErrorResponse{
			Error: openaiwire.Error{Message: message},
		},
	)
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
