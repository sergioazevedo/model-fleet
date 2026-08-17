package provider

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestProviderError(t *testing.T) {
	tests := []struct {
		name       string
		cause      error
		statusCode int
		category   ErrorCategory
		want       string
	}{
		{
			name:     "category only",
			category: ErrorCategoryInvalidRequest,
			want:     "provider error: invalid_request",
		},
		{
			name:       "category + status code",
			statusCode: 429,
			category:   ErrorCategoryRateLimited,
			want:       "provider error: rate_limited (status 429)",
		},
		{
			name:       "full error",
			cause:      errors.New("exceeded quota"),
			statusCode: 429,
			category:   ErrorCategoryRateLimited,
			want:       "provider error: rate_limited (status 429): exceeded quota",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			providerErr := &ProviderError{
				Category:   tt.category,
				StatusCode: tt.statusCode,
				Cause:      tt.cause,
			}

			assert.Equal(t, tt.want, providerErr.Error())
		})
	}
}

func TestProviderError_Is(t *testing.T) {
	cause := errors.New("exceeded quota")
	providerErr := &ProviderError{
		Category:   ErrorCategoryRateLimited,
		StatusCode: 429,
		Cause:      cause,
	}

	unrelated := errors.New("unrelated error")

	assert.ErrorIs(t, providerErr, cause)
	assert.NotErrorIs(t, providerErr, unrelated)
}
