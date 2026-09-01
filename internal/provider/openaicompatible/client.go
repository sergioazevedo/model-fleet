package openaicompatible

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/sergioazevedo/model-fleet/internal/openaiwire"
	"github.com/sergioazevedo/model-fleet/internal/provider"
)

type Client struct {
	endpoint   string
	apiKey     string
	httpClient *http.Client
}

func New(endpoint string, apiKey string, httpClient *http.Client) *Client {
	return &Client{
		endpoint:   endpoint,
		apiKey:     apiKey,
		httpClient: httpClient,
	}
}

func (c *Client) Complete(
	ctx context.Context,
	modelID string,
	request openaiwire.ChatCompletionRequest,
) (openaiwire.ChatCompletionResponse, error) {
	req, err := buildRequest(
		ctx,
		c.endpoint,
		modelID,
		request,
		map[string]string{
			"Content-Type":  "application/json",
			"Authorization": fmt.Sprintf("Bearer %s", c.apiKey),
		},
	)
	if err != nil {
		return openaiwire.ChatCompletionResponse{}, fmt.Errorf("failed to build request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return openaiwire.ChatCompletionResponse{}, fmt.Errorf("failed to complete request: %w", err)
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return openaiwire.ChatCompletionResponse{}, normalizeErrorResponse(resp)
	}

	result, err := decodeResponse(resp)
	if err != nil {
		return openaiwire.ChatCompletionResponse{}, fmt.Errorf("failed to decode response: %w", err)
	}

	return result, nil
}

func normalizeErrorResponse(resp *http.Response) *provider.ProviderError {
	var category provider.ErrorCategory

	statusCode := resp.StatusCode
	retryAfter := time.Duration(0)
	switch {
	// 429 is a special case for rate limiting, so we handle it explicitly.
	case statusCode == http.StatusTooManyRequests:
		category = provider.ErrorCategoryRateLimited
		retryAfterHeader := resp.Header.Get("Retry-After")
		if retryAfterHeader != "" {
			if d, err := time.ParseDuration(retryAfterHeader + "s"); err == nil {
				retryAfter = d
			}
		}
	// 401 and 403 as authentication errors
	case statusCode == http.StatusUnauthorized ||
		statusCode == http.StatusForbidden:
		category = provider.ErrorCategoryAuthenticationFailed
	// 404 is a special case for model unavailability, so we handle it explicitly.
	case statusCode == http.StatusNotFound:
		category = provider.ErrorCategoryModelUnavailable
	// 500 and above are considered provider errors
	case statusCode >= 500:
		category = provider.ErrorCategoryProviderUnavailable
	default:
		category = provider.ErrorCategoryInvalidRequest
	}

	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))

	var respBody openaiwire.ErrorResponse
	var cause error

	if err := json.Unmarshal(bodyBytes, &respBody); err != nil {
		cause = fmt.Errorf("failed to decode error response: %w", err)
	} else if respBody.Error.Message != "" {
		cause = errors.New(respBody.Error.Message)
	}

	return &provider.ProviderError{
		Category:   category,
		StatusCode: resp.StatusCode,
		RetryAfter: retryAfter,
		Cause:      cause,
	}
}

func buildRequest(
	ctx context.Context,
	endpoint string,
	modelID string,
	request openaiwire.ChatCompletionRequest,
	headers map[string]string,
) (*http.Request, error) {
	targetURL := strings.TrimRight(endpoint, "/") + "/chat/completions"
	body, err := encodeRequestBody(modelID, request)
	if err != nil {
		return nil, fmt.Errorf("failed to encode request body: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		targetURL,
		strings.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	for key, value := range headers {
		req.Header.Set(key, value)
	}

	return req, nil
}

func encodeRequestBody(
	modelID string,
	req openaiwire.ChatCompletionRequest,
) (string, error) {
	req.Model = modelID
	if len(req.Tools) > 0 && len(req.ToolChoice) == 0 {
		req.ToolChoice = json.RawMessage(`"auto"`)
	}

	body, err := json.Marshal(req)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request body: %w", err)
	}

	return string(body), nil
}

func decodeResponse(resp *http.Response) (openaiwire.ChatCompletionResponse, error) {
	var respBody openaiwire.ChatCompletionResponse
	if err := json.NewDecoder(resp.Body).Decode(&respBody); err != nil {
		return openaiwire.ChatCompletionResponse{}, fmt.Errorf("failed to decode response: %w", err)
	}

	// Drain remaining body
	_, _ = io.Copy(io.Discard, resp.Body)

	if len(respBody.Choices) == 0 {
		return respBody, fmt.Errorf("no content generated")
	}

	return respBody, nil
}
