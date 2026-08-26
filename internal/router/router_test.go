package router

import (
	"encoding/json"
	"testing"

	"github.com/sergioazevedo/model-fleet/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMapRequest(t *testing.T) {
	t.Run("maps a completion request", func(t *testing.T) {
		temperature := 0.2
		reasoningEffort := "medium"
		responseFormat := "json_object"
		request := completionRequest{
			Application: "meal-planner",
			Role:        "analyst",
			Messages: []message{
				{Role: "user", Content: "Suggest a simple dinner"},
				{
					Role: "assistant",
					ToolCalls: []toolCall{
						{
							ID:        "call-1",
							Name:      "find_recipe",
							Arguments: json.RawMessage(`{"query":"pasta"}`),
						},
					},
				},
				{Role: "tool", Content: "Pasta primavera", ToolCallID: "call-1"},
			},
			Tools: []tool{
				{
					Name:        "find_recipe",
					Description: "Find a recipe",
					Parameters:  json.RawMessage(`{"type":"object"}`),
				},
			},
			Temperature:     &temperature,
			ReasoningEffort: &reasoningEffort,
			ResponseFormat:  &responseFormat,
		}

		got, err := mapRequest(request)
		require.NoError(t, err)

		wantResponseFormat := provider.ResponseFormatJSON
		want := provider.CompletionRequest{
			Messages: []provider.Message{
				{Role: "user", Content: "Suggest a simple dinner", ToolCalls: []provider.ToolCall{}},
				{
					Role: "assistant",
					ToolCalls: []provider.ToolCall{
						{
							ID:        "call-1",
							Name:      "find_recipe",
							Arguments: json.RawMessage(`{"query":"pasta"}`),
						},
					},
				},
				{Role: "tool", Content: "Pasta primavera", ToolCalls: []provider.ToolCall{}, ToolCallID: "call-1"},
			},
			Tools: []provider.Tool{
				{
					Name:        "find_recipe",
					Description: "Find a recipe",
					Parameters:  json.RawMessage(`{"type":"object"}`),
				},
			},
			Temperature:     &temperature,
			ReasoningEffort: &reasoningEffort,
			ResponseFormat:  &wantResponseFormat,
		}

		assert.Equal(t, want, got)
	})

	t.Run("preserves omitted optional fields", func(t *testing.T) {
		got, err := mapRequest(completionRequest{})
		require.NoError(t, err)

		assert.Nil(t, got.Temperature)
		assert.Nil(t, got.ReasoningEffort)
		assert.Nil(t, got.ResponseFormat)
	})

	t.Run("rejects an unsupported response format", func(t *testing.T) {
		responseFormat := "xml"

		_, err := mapRequest(completionRequest{
			ResponseFormat: &responseFormat,
		})

		require.EqualError(t, err, `unsupported response format "xml"`)
	})
}
