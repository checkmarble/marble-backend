package ai_agent

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/checkmarble/llmberjack"
	"github.com/checkmarble/llmberjack/llms/aistudio"
	"github.com/checkmarble/marble-backend/infra"
	"github.com/checkmarble/marble-backend/usecases/tracfin"
	"github.com/invopop/jsonschema"
	"github.com/stretchr/testify/require"
)

type tracfinPayloadTransport func(*http.Request) (*http.Response, error)

func (f tracfinPayloadTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestTracfinOutputBudgetSentToAIStudio(t *testing.T) {
	calls := 0
	client := &http.Client{Transport: tracfinPayloadTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		var payload struct {
			GenerationConfig struct {
				MaxOutputTokens    int            `json:"maxOutputTokens"`
				ResponseMIMEType   string         `json:"responseMimeType"`
				ResponseJsonSchema map[string]any `json:"responseJsonSchema"`
			} `json:"generationConfig"`
		}
		require.NoError(t, json.NewDecoder(r.Body).Decode(&payload))
		require.Equal(t, 65536, payload.GenerationConfig.MaxOutputTokens)
		require.Equal(t, "application/json", payload.GenerationConfig.ResponseMIMEType)
		require.NotEmpty(t, payload.GenerationConfig.ResponseJsonSchema)
		schema := payload.GenerationConfig.ResponseJsonSchema
		if ref, ok := schema["$ref"].(string); ok {
			require.True(t, strings.HasPrefix(ref, "#/$defs/"))
			definitions, ok := schema["$defs"].(map[string]any)
			require.True(t, ok)
			schema, ok = definitions[strings.TrimPrefix(ref, "#/$defs/")].(map[string]any)
			require.True(t, ok)
		}
		properties, ok := schema["properties"].(map[string]any)
		require.True(t, ok)
		require.Contains(t, properties, "has_more")
		require.Contains(t, schema["required"], "has_more")
		return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(`{"candidates":[{"content":{"role":"model","parts":[{"text":"raw response"}]},"finishReason":"STOP"}]}`)), Request: r}, nil
	})}
	provider, err := aistudio.New(aistudio.WithApiKey("test-key"))
	require.NoError(t, err)
	adapter, err := llmberjack.New(llmberjack.WithProvider("main", provider), llmberjack.WithHttpClient(client))
	require.NoError(t, err)
	uc := &AiAgentUsecase{config: infra.AIAgentConfiguration{MainAgentProviderType: infra.AIAgentProviderTypeAIStudio}, caseReviewAdapter: adapter}
	response, err := uc.TracfinAICompleteWithModel(context.Background(), "system", "prompt", *jsonschema.Reflect(tracfin.AISectionBatch{}), "main", "gemini-3.5-flash")
	require.NoError(t, err)
	require.Equal(t, "raw response", response.Text)
	require.Equal(t, 1, calls)
}
