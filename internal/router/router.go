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
		"POST /v1/completions",
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
		http.Error(w, fmt.Sprintf("router.Handler.handleCompletion: failed to parse request: %v", err), http.StatusBadRequest)
		return
	}

	// TODO: move this behaviour to Config and respect Demeter's Law.
	// Check if the application is "meal-planner" and return an error if it's not
	if completionReq.Application != "meal-planner" {
		http.Error(w, fmt.Sprintf("router.Handler.handleCompletion: unsupported application: %s", completionReq.Application), http.StatusBadRequest)
		return
	}

	roleRoute := fmt.Sprintf(
		"%s/%s",
		completionReq.Application,
		completionReq.Role,
	)

	roleConfig, exists := h.fleetConfig.RoleRoutes[roleRoute]
	if !exists {
		http.Error(w, fmt.Sprintf("router.Handler.handleCompletion: unsupported role: %s", completionReq.Role), http.StatusBadRequest)
		return
	}

	if len(roleConfig.DeploymentIDs) == 0 {
		http.Error(w, fmt.Sprintf("router.Handler.handleCompletion: no deployments configured for role: %s", completionReq.Role), http.StatusPreconditionFailed)
		return
	}

	//Route pick deployment
	candidateDeploymentID := roleConfig.DeploymentIDs[0]
	deploymentConfig := h.fleetConfig.Deployments[candidateDeploymentID]

	targetClient, exists := h.providerClients[deploymentConfig.Connection]
	if !exists {
		http.Error(w, fmt.Sprintf("router.Handler.handleCompletion: no provider configured for role: %s", completionReq.Role), http.StatusPreconditionFailed)
		return
	}

	providerReq, err := mapRequest(completionReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("router.Handler.handleCompletion: failed parsing request for role: %s", completionReq.Role), http.StatusBadRequest)
		return
	}

	providerResp, err := targetClient.Complete(
		r.Context(),
		deploymentConfig.Model,
		providerReq,
	)

	if err != nil {
		var providerErr *provider.ProviderError

		code := http.StatusInternalServerError
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

		http.Error(
			w,
			fmt.Sprintf("router.Handler.handleCompletion: failed request for role: %s: %v", completionReq.Role, err),
			code,
		)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(providerResp)
}

func mapRequest(r completionRequest) (provider.CompletionRequest, error) {
	var responseFormat *provider.ResponseFormat
	if r.ResponseFormat != nil {
		format := provider.ResponseFormat(*r.ResponseFormat)
		switch format {
		case provider.ResponseFormatText, provider.ResponseFormatJSON:
			responseFormat = &format
		default:
			return provider.CompletionRequest{}, fmt.Errorf(
				"unsupported response format %q",
				*r.ResponseFormat,
			)
		}
	}

	messages := make([]provider.Message, len(r.Messages))
	for messageIndex, message := range r.Messages {
		toolCalls := make([]provider.ToolCall, len(message.ToolCalls))
		for callIndex, call := range message.ToolCalls {
			toolCalls[callIndex] = provider.ToolCall{
				ID:        call.ID,
				Name:      call.Name,
				Arguments: call.Arguments,
			}
		}

		messages[messageIndex] = provider.Message{
			Role:       message.Role,
			Content:    message.Content,
			ToolCalls:  toolCalls,
			ToolCallID: message.ToolCallID,
		}
	}

	tools := make([]provider.Tool, len(r.Tools))
	for toolIndex, tool := range r.Tools {
		tools[toolIndex] = provider.Tool{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  tool.Parameters,
		}
	}

	return provider.CompletionRequest{
		Messages:        messages,
		Tools:           tools,
		Temperature:     r.Temperature,
		ReasoningEffort: r.ReasoningEffort,
		ResponseFormat:  responseFormat,
	}, nil
}

func parseRequest(r *http.Request) (completionRequest, error) {
	var providerReq completionRequest

	defer r.Body.Close()

	if err := json.NewDecoder(r.Body).Decode(&providerReq); err != nil {
		return completionRequest{}, fmt.Errorf(
			"failed to decode provider request body: %w",
			err,
		)
	}

	// Discard any remaining data in the request body to ensure it's fully read
	io.Copy(io.Discard, r.Body)

	return providerReq, nil
}
