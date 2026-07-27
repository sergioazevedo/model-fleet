package provider

import (
	"errors"
	"testing"
)

func TestProviderError(t *testing.T) {
	tests := []struct {
		name          string
		cause         error
		statusCode    int
		ErrorCategory ErrorCategory
		want          string
	}{
		{
			name:          "category only",
			cause:         nil,
			statusCode:    0,
			ErrorCategory: ErrorCategoryInvalidRequest,
			want:          "provider error: invalid_request",
		},
		{
			name:          "category + status code",
			cause:         nil,
			statusCode:    429,
			ErrorCategory: ErrorCategoryRateLimited,
			want:          "provider error: rate_limited (status 429)",
		},
		{
			name:          "full error",
			cause:         errors.New("exceeded quota"),
			statusCode:    429,
			ErrorCategory: ErrorCategoryRateLimited,
			want:          "provider error: rate_limited (status 429): exceeded quota",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			providerErr := &ProviderError{
				Category:   tt.ErrorCategory,
				StatusCode: tt.statusCode,
				Cause:      tt.cause,
			}

			if got := providerErr.Error(); got != tt.want {
				t.Fatalf("Error() = %v, want %v", got, tt.want)
			}
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

	if !errors.Is(providerErr, cause) {
		t.Fatal("errors.Is() = false, want true for wrapped cause")
	}

	unrelated := errors.New("unrelated error")
	if errors.Is(providerErr, unrelated) {
		t.Fatal("errors.Is() = true, want false for unrelated error")
	}
}
