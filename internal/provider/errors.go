package provider

import (
	"fmt"
	"time"
)

type ErrorCategory string

const (
	ErrorCategoryRateLimited          ErrorCategory = "rate_limited"
	ErrorCategoryModelUnavailable     ErrorCategory = "model_unavailable"
	ErrorCategoryInvalidRequest       ErrorCategory = "invalid_request"
	ErrorCategoryAuthenticationFailed ErrorCategory = "authentication_failed"
	ErrorCategoryProviderUnavailable  ErrorCategory = "provider_unavailable"
)

type ProviderError struct {
	Category   ErrorCategory
	StatusCode int
	RetryAfter time.Duration
	Cause      error
}

func (e *ProviderError) Error() string {
	if e == nil {
		return ""
	}

	message := fmt.Sprintf("provider error: %s", e.Category)
	if e.StatusCode != 0 {
		message += fmt.Sprintf(" (status %d)", e.StatusCode)
	}

	if e.Cause != nil {
		message += fmt.Sprintf(": %v", e.Cause)
	}

	return message
}

func (e *ProviderError) Unwrap() error {
	if e == nil {
		return nil
	}

	return e.Cause
}
