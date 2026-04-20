package reasoning

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"tars/internal/foundation/secrets"
)

func TestDecodeProviderModelListProtocols(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name     string
		protocol string
		body     string
		want     []ProviderModelInfo
	}{
		{
			name:     "openai compatible",
			protocol: ModelProtocolOpenAICompatible,
			body:     `{"data":[{"id":"gpt-4o-mini"},{"id":"gpt-4.1"}]}`,
			want: []ProviderModelInfo{
				{ID: "gpt-4o-mini", Name: "gpt-4o-mini"},
				{ID: "gpt-4.1", Name: "gpt-4.1"},
			},
		},
		{
			name:     "anthropic",
			protocol: ModelProtocolAnthropic,
			body:     `{"data":[{"id":"claude-sonnet-4-5","display_name":"Claude Sonnet 4.5"}]}`,
			want: []ProviderModelInfo{
				{ID: "claude-sonnet-4-5", Name: "Claude Sonnet 4.5"},
			},
		},
		{
			name:     "ollama",
			protocol: ModelProtocolOllama,
			body:     `{"models":[{"name":"qwen2.5:14b"}]}`,
			want: []ProviderModelInfo{
				{ID: "qwen2.5:14b", Name: "qwen2.5:14b"},
			},
		},
		{
			name:     "gemini",
			protocol: ModelProtocolGemini,
			body:     `{"models":[{"name":"models/gemini-2.5-flash","displayName":"Gemini 2.5 Flash"}]}`,
			want: []ProviderModelInfo{
				{ID: "gemini-2.5-flash", Name: "Gemini 2.5 Flash"},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := decodeProviderModelList(tc.protocol, bytes.NewBufferString(tc.body))
			if err != nil {
				t.Fatalf("decodeProviderModelList: %v", err)
			}
			if len(got) != len(tc.want) {
				t.Fatalf("expected %d models, got %+v", len(tc.want), got)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("model %d = %+v, want %+v", i, got[i], tc.want[i])
				}
			}
		})
	}
}

func TestDecodeProviderModelListErrors(t *testing.T) {
	t.Parallel()

	if _, err := decodeProviderModelList(ModelProtocolOpenAICompatible, bytes.NewBufferString(`{`)); err == nil {
		t.Fatalf("expected invalid json to fail")
	}
	if _, err := decodeProviderModelList("unsupported", bytes.NewBufferString(`{}`)); err == nil {
		t.Fatalf("expected unsupported protocol to fail")
	}
}

func TestNormalizeGeminiModelsEndpoint(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name    string
		baseURL string
		apiKey  string
		want    string
	}{
		{
			name:    "appends default path and key",
			baseURL: "https://generativelanguage.googleapis.com",
			apiKey:  "secret",
			want:    "https://generativelanguage.googleapis.com/v1beta/models?key=secret",
		},
		{
			name:    "preserves models endpoint without key",
			baseURL: "https://generativelanguage.googleapis.com/v1beta/models",
			want:    "https://generativelanguage.googleapis.com/v1beta/models",
		},
		{
			name:    "appends models to v1beta root",
			baseURL: "https://generativelanguage.googleapis.com/v1beta",
			apiKey:  "secret",
			want:    "https://generativelanguage.googleapis.com/v1beta/models?key=secret",
		},
		{
			name:    "appends key to models endpoint",
			baseURL: "https://generativelanguage.googleapis.com/v1beta/models",
			apiKey:  "secret",
			want:    "https://generativelanguage.googleapis.com/v1beta/models?key=secret",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := normalizeGeminiModelsEndpoint(tc.baseURL, tc.apiKey); got != tc.want {
				t.Fatalf("normalizeGeminiModelsEndpoint(%q, %q) = %q, want %q", tc.baseURL, tc.apiKey, got, tc.want)
			}
		})
	}
}

func TestNewProviderListRequestCoversProtocolsAndHeaders(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		protocol    string
		entry       ProviderEntry
		wantURL     string
		wantHeader  map[string]string
		expectError bool
	}{
		{
			name:     "openai compatible",
			protocol: ModelProtocolOpenAICompatible,
			entry: ProviderEntry{
				BaseURL: "https://api.example.test/v1",
				APIKey:  "secret",
			},
			wantURL:    "https://api.example.test/v1/models",
			wantHeader: map[string]string{"Authorization": "Bearer secret"},
		},
		{
			name:     "anthropic",
			protocol: ModelProtocolAnthropic,
			entry: ProviderEntry{
				BaseURL: "https://api.anthropic.com",
				APIKey:  "secret",
			},
			wantURL: "https://api.anthropic.com/v1/models",
			wantHeader: map[string]string{
				"anthropic-version": "2023-06-01",
				"x-api-key":         "secret",
			},
		},
		{
			name:     "ollama",
			protocol: ModelProtocolOllama,
			entry: ProviderEntry{
				BaseURL: "http://127.0.0.1:11434",
			},
			wantURL: "http://127.0.0.1:11434/api/tags",
		},
		{
			name:     "gemini",
			protocol: ModelProtocolGemini,
			entry: ProviderEntry{
				BaseURL: "https://generativelanguage.googleapis.com",
				APIKey:  "secret",
			},
			wantURL: "https://generativelanguage.googleapis.com/v1beta/models?key=secret",
		},
		{
			name:     "openrouter",
			protocol: ModelProtocolOpenRouter,
			entry: ProviderEntry{
				BaseURL: "https://openrouter.ai",
				APIKey:  "secret",
			},
			wantURL:    "https://openrouter.ai/api/v1/models",
			wantHeader: map[string]string{"Authorization": "Bearer secret"},
		},
		{
			name:        "unsupported",
			protocol:    "unsupported",
			entry:       ProviderEntry{BaseURL: "https://api.example.test"},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req, err := newProviderListRequest(context.Background(), tc.protocol, tc.entry)
			if tc.expectError {
				if err == nil {
					t.Fatalf("expected error for protocol %q", tc.protocol)
				}
				return
			}
			if err != nil {
				t.Fatalf("newProviderListRequest: %v", err)
			}
			if req.URL.String() != tc.wantURL {
				t.Fatalf("unexpected url: %s", req.URL.String())
			}
			for key, want := range tc.wantHeader {
				if got := req.Header.Get(key); got != want {
					t.Fatalf("header %s = %q, want %q", key, got, want)
				}
			}
		})
	}
}

func TestCheckProviderAvailabilityUsesListResults(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"id":"gpt-4o-mini"},{"id":"gpt-4.1"}]}`))
	}))
	defer server.Close()

	entry := ProviderEntry{
		ID:       "openai-main",
		Vendor:   "openai",
		Protocol: ModelProtocolOpenAICompatible,
		BaseURL:  server.URL,
		Enabled:  true,
	}

	result, err := CheckProviderAvailability(context.Background(), entry, "")
	if err != nil {
		t.Fatalf("check provider availability without model: %v", err)
	}
	if !result.Available || !strings.Contains(result.Detail, "list_models ok (2 models)") {
		t.Fatalf("expected successful list-models check, got %+v", result)
	}

	result, err = CheckProviderAvailability(context.Background(), entry, "gpt-4.1")
	if err != nil {
		t.Fatalf("check provider availability for existing model: %v", err)
	}
	if !result.Available || result.Detail != "model found in provider listing" {
		t.Fatalf("expected found model result, got %+v", result)
	}

	result, err = CheckProviderAvailability(context.Background(), entry, "missing-model")
	if err != nil {
		t.Fatalf("check provider availability for missing model: %v", err)
	}
	if result.Available || result.Detail != "model not found in provider listing" {
		t.Fatalf("expected missing model result, got %+v", result)
	}
}

func TestCheckProviderAvailabilityReturnsListErrorWhenModelIsBlank(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"listing unavailable"}`))
	}))
	defer server.Close()

	result, err := CheckProviderAvailability(context.Background(), ProviderEntry{
		ID:       "openai-main",
		Vendor:   "openai",
		Protocol: ModelProtocolOpenAICompatible,
		BaseURL:  server.URL,
		Enabled:  true,
	}, "")
	if err != nil {
		t.Fatalf("check provider availability: %v", err)
	}
	if result.Available {
		t.Fatalf("expected unavailable result, got %+v", result)
	}
	if !strings.Contains(result.Detail, "status=500") {
		t.Fatalf("expected list-model failure detail, got %+v", result)
	}
}

func TestCheckProviderAvailabilityReturnsBuildErrorForUnsupportedProtocol(t *testing.T) {
	t.Parallel()

	_, err := CheckProviderAvailability(context.Background(), ProviderEntry{
		ID:       "bad-provider",
		Protocol: "unsupported",
		BaseURL:  "https://api.example.test",
		Enabled:  true,
	}, "some-model")
	if err == nil {
		t.Fatalf("expected unsupported protocol to return an error")
	}
}

func TestBuildModelInvocationCoversOpenRouterAndUnsupportedProtocol(t *testing.T) {
	t.Parallel()

	invocation, err := buildModelInvocation(ModelProtocolOpenRouter, "https://openrouter.ai", "secret", "openai/gpt-4.1-mini", "system", "user")
	if err != nil {
		t.Fatalf("buildModelInvocation openrouter: %v", err)
	}
	if invocation.Endpoint != "https://openrouter.ai/api/v1/chat/completions" {
		t.Fatalf("unexpected openrouter endpoint: %s", invocation.Endpoint)
	}
	if invocation.Headers["Authorization"] != "Bearer secret" {
		t.Fatalf("unexpected openrouter auth header: %+v", invocation.Headers)
	}

	if _, err := buildModelInvocation("unsupported", "https://api.example.test", "", "model", "system", "user"); err == nil {
		t.Fatalf("expected unsupported protocol to fail")
	}
}

func TestExtractModelResponseContentErrorBranches(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name       string
		protocol   string
		statusCode int
		body       string
		wantErr    string
	}{
		{
			name:       "openai status error",
			protocol:   ModelProtocolOpenAICompatible,
			statusCode: http.StatusBadGateway,
			body:       `{"choices":[{"message":{"content":"ignored"}}]}`,
			wantErr:    "status=502",
		},
		{
			name:       "anthropic error message",
			protocol:   ModelProtocolAnthropic,
			statusCode: http.StatusTooManyRequests,
			body:       `{"error":{"message":"rate limited"}}`,
			wantErr:    "message=rate limited",
		},
		{
			name:       "ollama missing content",
			protocol:   ModelProtocolOllama,
			statusCode: http.StatusOK,
			body:       `{"message":{"content":""}}`,
			wantErr:    "does not contain message content",
		},
		{
			name:       "gemini missing text",
			protocol:   ModelProtocolGemini,
			statusCode: http.StatusOK,
			body:       `{"candidates":[{"content":{"parts":[{"text":"   "}]} }]}`,
			wantErr:    "does not contain text content",
		},
		{
			name:       "unsupported protocol",
			protocol:   "unsupported",
			statusCode: http.StatusOK,
			body:       `{}`,
			wantErr:    "unsupported model protocol",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := extractModelResponseContent(tc.protocol, tc.statusCode, bytes.NewBufferString(tc.body))
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("extractModelResponseContent error = %v, want substring %q", err, tc.wantErr)
			}
		})
	}
}

func TestNormalizeProviderHelpersAndResolveBoundTarget(t *testing.T) {
	t.Parallel()

	if got := normalizeProviderProtocol("", "claude"); got != ModelProtocolAnthropic {
		t.Fatalf("expected vendor fallback to anthropic, got %q", got)
	}
	if got := normalizeProviderVendor("", ModelProtocolOpenAICompatible); got != "openai" {
		t.Fatalf("expected default openai vendor, got %q", got)
	}
	if got := normalizeProviderVendor("Anthropic", ""); got != "claude" {
		t.Fatalf("expected anthropic vendor normalization, got %q", got)
	}

	cfg := &ProvidersConfig{
		Primary: ProviderBinding{ProviderID: "primary", Model: "gpt-4o-mini"},
		Entries: []ProviderEntry{
			{ID: "primary", Protocol: ModelProtocolOpenAICompatible, BaseURL: "https://primary.example.test", Enabled: false},
			{ID: "assist", Protocol: ModelProtocolOpenAICompatible, BaseURL: "https://assist.example.test", Enabled: true, APIKey: "inline-key"},
		},
	}
	if got := resolveBoundTarget(cfg, ProviderBinding{ProviderID: "primary", Model: "gpt-4o-mini"}, nil); got != nil {
		t.Fatalf("expected disabled provider to be ignored, got %+v", got)
	}

	dir := t.TempDir()
	store, err := secrets.NewStore(filepath.Join(dir, "secrets.yaml"))
	if err != nil {
		t.Fatalf("new secret store: %v", err)
	}
	if _, err := store.Apply(map[string]string{"provider/assist": "secret-key"}, nil, time.Now().UTC()); err != nil {
		t.Fatalf("seed secrets: %v", err)
	}

	cfg.Entries[1].APIKey = ""
	cfg.Entries[1].APIKeyRef = "provider/assist"
	target := resolveBoundTarget(cfg, ProviderBinding{ProviderID: "assist", Model: "gpt-4o-mini"}, store)
	if target == nil || target.APIKey != "secret-key" {
		t.Fatalf("expected resolved target with secret api key, got %+v", target)
	}
	if got := resolveBoundTarget(cfg, ProviderBinding{ProviderID: "assist", Model: ""}, store); got != nil {
		t.Fatalf("expected blank model to disable target resolution, got %+v", got)
	}
}

func TestProviderManagerSaveConfigWithoutPathClonesTemplatesAndPersists(t *testing.T) {
	t.Parallel()

	manager, err := NewProviderManager("")
	if err != nil {
		t.Fatalf("new provider manager: %v", err)
	}

	var persisted ProvidersConfig
	manager.SetPersistence(func(cfg ProvidersConfig) error {
		persisted = cfg
		return nil
	})

	err = manager.SaveConfig(ProvidersConfig{
		Primary: ProviderBinding{ProviderID: " primary ", Model: " gpt-4o-mini "},
		Entries: []ProviderEntry{
			{
				ID:      " primary ",
				Vendor:  " openai-compatible ",
				BaseURL: " https://api.example.test ",
				Enabled: true,
				APIKey:  " inline-key ",
				Templates: []ProviderTemplate{{ID: "tpl-1", Name: "Default", Values: map[string]string{
					" region ": " us-east-1 ",
					" ":        "ignored",
				}}},
			},
		},
	})
	if err != nil {
		t.Fatalf("save config without path: %v", err)
	}

	snapshot := manager.Snapshot()
	if snapshot.Config.Primary.ProviderID != "primary" || snapshot.Config.Primary.Model != "gpt-4o-mini" {
		t.Fatalf("expected normalized primary binding, got %+v", snapshot.Config.Primary)
	}
	if len(snapshot.Config.Entries) != 1 || snapshot.Config.Entries[0].Templates[0].Values["region"] != "us-east-1" {
		t.Fatalf("expected template values to be cloned and normalized, got %+v", snapshot.Config.Entries)
	}
	if persisted.Entries[0].Templates[0].Values["region"] != "us-east-1" {
		t.Fatalf("expected persisted config to receive normalized template values, got %+v", persisted.Entries)
	}

	snapshot.Config.Entries[0].Templates[0].Values["region"] = "mutated"
	if manager.Snapshot().Config.Entries[0].Templates[0].Values["region"] != "us-east-1" {
		t.Fatalf("expected snapshot mutation not to leak back into manager state")
	}
}

func TestProviderManagerSaveWithoutPathParsesAndNormalizesContent(t *testing.T) {
	t.Parallel()

	manager, err := NewProviderManager("")
	if err != nil {
		t.Fatalf("new provider manager: %v", err)
	}

	content := `
providers:
  primary:
    provider_id: " provider-a "
    model: " gpt-4.1 "
  entries:
    - id: "provider-a"
      vendor: " openrouter "
      base_url: " https://openrouter.ai "
      enabled: true
`
	if err := manager.Save(content); err != nil {
		t.Fatalf("save content without path: %v", err)
	}

	var persisted ProvidersConfig
	manager.SetPersistence(func(cfg ProvidersConfig) error {
		persisted = cfg
		return nil
	})
	if err := manager.Save(content); err != nil {
		t.Fatalf("save content without path with persistence: %v", err)
	}

	snapshot := manager.Snapshot()
	if snapshot.Config.Primary.ProviderID != "provider-a" || snapshot.Config.Primary.Model != "gpt-4.1" {
		t.Fatalf("expected normalized saved content, got %+v", snapshot.Config)
	}
	if len(snapshot.Config.Entries) != 1 || snapshot.Config.Entries[0].Protocol != ModelProtocolOpenRouter {
		t.Fatalf("expected protocol normalization from saved content, got %+v", snapshot.Config.Entries)
	}
	if persisted.Primary.ProviderID != "provider-a" || len(persisted.Entries) != 1 {
		t.Fatalf("expected raw-save persistence to receive normalized config, got %+v", persisted)
	}
}

func TestEndpointNormalizersAndProviderWriteHelper(t *testing.T) {
	t.Parallel()

	if got := normalizeOpenAIEndpoint("https://api.example.test"); got != "https://api.example.test/v1/chat/completions" {
		t.Fatalf("unexpected openai endpoint: %s", got)
	}
	if got := normalizeAnthropicEndpoint("https://api.anthropic.com/v1"); got != "https://api.anthropic.com/v1/messages" {
		t.Fatalf("unexpected anthropic endpoint: %s", got)
	}
	if got := normalizeOpenRouterEndpoint("https://openrouter.ai"); got != "https://openrouter.ai/api/v1/chat/completions" {
		t.Fatalf("unexpected openrouter endpoint: %s", got)
	}
	if got := normalizeOllamaEndpoint("http://127.0.0.1:11434"); got != "http://127.0.0.1:11434/api/chat" {
		t.Fatalf("unexpected ollama endpoint: %s", got)
	}
	if got := normalizeGeminiEndpoint("https://generativelanguage.googleapis.com/v1beta", "gemini-2.5-flash", "secret"); got != "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent?key=secret" {
		t.Fatalf("unexpected gemini endpoint: %s", got)
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "providers.yaml")
	if err := writeProvidersFileAtomically(path, "providers:\n  entries: []\n"); err != nil {
		t.Fatalf("write providers file: %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read providers file: %v", err)
	}
	if !strings.Contains(string(content), "entries") {
		t.Fatalf("expected providers file to be written, got %q", string(content))
	}
	if err := writeProvidersFileAtomically(filepath.Join(dir, "missing", "providers.yaml"), "x"); err == nil {
		t.Fatalf("expected write helper to fail for missing directory")
	}
}

func TestMergeProviderSecretsPreservesExistingSecretsAndRefs(t *testing.T) {
	t.Parallel()

	merged := mergeProviderSecrets(&ProvidersConfig{
		Entries: []ProviderEntry{
			{ID: "primary", APIKey: "primary-key", APIKeyRef: "secret/primary"},
			{ID: "assist", APIKeyRef: "secret/assist"},
		},
	}, &ProvidersConfig{
		Entries: []ProviderEntry{
			{ID: "primary", Vendor: "openai", BaseURL: "https://primary.example.test"},
			{ID: "assist", Vendor: "anthropic", BaseURL: "https://assist.example.test"},
		},
	})

	if len(merged.Entries) != 2 {
		t.Fatalf("expected merged entries, got %+v", merged.Entries)
	}
	if merged.Entries[1].ID != "primary" || merged.Entries[1].APIKey != "primary-key" || merged.Entries[1].APIKeyRef != "secret/primary" {
		t.Fatalf("expected existing primary secret and ref to be preserved, got %+v", merged.Entries)
	}
	if merged.Entries[0].ID != "assist" || merged.Entries[0].APIKeyRef != "secret/assist" {
		t.Fatalf("expected existing assist ref to be preserved, got %+v", merged.Entries)
	}
}

func TestNormalizeGeminiEndpointAndExtractModelResponseAdditionalBranches(t *testing.T) {
	t.Parallel()

	testEndpoints := []struct {
		baseURL string
		model   string
		apiKey  string
		want    string
	}{
		{
			baseURL: "https://generativelanguage.googleapis.com/v1beta",
			model:   "gemini-2.5-flash",
			apiKey:  "",
			want:    "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent",
		},
		{
			baseURL: "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent",
			model:   "gemini-2.5-flash",
			apiKey:  "secret",
			want:    "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent?key=secret",
		},
		{
			baseURL: "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent?key=existing",
			model:   "gemini-2.5-flash",
			apiKey:  "secret",
			want:    "https://generativelanguage.googleapis.com/v1beta/models/gemini-2.5-flash:generateContent?key=existing",
		},
	}
	for _, tc := range testEndpoints {
		if got := normalizeGeminiEndpoint(tc.baseURL, tc.model, tc.apiKey); got != tc.want {
			t.Fatalf("normalizeGeminiEndpoint(%q, %q, %q) = %q, want %q", tc.baseURL, tc.model, tc.apiKey, got, tc.want)
		}
	}

	testResponses := []struct {
		name       string
		protocol   string
		statusCode int
		body       string
		want       string
		wantErr    string
	}{
		{
			name:       "openai empty choices",
			protocol:   ModelProtocolOpenAICompatible,
			statusCode: http.StatusOK,
			body:       `{"choices":[]}`,
			wantErr:    "status=200",
		},
		{
			name:       "openai invalid json",
			protocol:   ModelProtocolOpenAICompatible,
			statusCode: http.StatusOK,
			body:       `{`,
			wantErr:    "unexpected EOF",
		},
		{
			name:       "anthropic no text",
			protocol:   ModelProtocolAnthropic,
			statusCode: http.StatusOK,
			body:       `{"content":[{"type":"tool_use","text":""}]}`,
			wantErr:    "does not contain text content",
		},
		{
			name:       "ollama error message",
			protocol:   ModelProtocolOllama,
			statusCode: http.StatusBadGateway,
			body:       `{"error":"gateway down"}`,
			wantErr:    "message=gateway down",
		},
		{
			name:       "gemini error message",
			protocol:   ModelProtocolGemini,
			statusCode: http.StatusBadGateway,
			body:       `{"error":{"message":"quota exceeded"}}`,
			wantErr:    "message=quota exceeded",
		},
	}
	for _, tc := range testResponses {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := extractModelResponseContent(tc.protocol, tc.statusCode, bytes.NewBufferString(tc.body))
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("extractModelResponseContent error = %v, want substring %q", err, tc.wantErr)
			}
		})
	}
}

func TestProviderManagerErrorBranches(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "providers.yaml")
	if err := os.WriteFile(path, []byte("providers:\n  entries: []\n"), 0o600); err != nil {
		t.Fatalf("write providers file: %v", err)
	}
	manager, err := NewProviderManager(path)
	if err != nil {
		t.Fatalf("new provider manager: %v", err)
	}
	if err := manager.Save(":\n"); err == nil {
		t.Fatalf("expected invalid providers yaml to fail")
	}
	if err := (&ProviderManager{path: filepath.Join(dir, "missing", "providers.yaml")}).Reload(); err == nil {
		t.Fatalf("expected reload from missing providers file to fail")
	}
}

func TestCheckProviderAvailabilityFallsBackToInferenceWhenListModelsUnsupported(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			http.NotFound(w, r)
		case "/v1/chat/completions":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"summary\":\"ok\",\"execution_hint\":\"\"}"}}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	result, err := CheckProviderAvailability(context.Background(), ProviderEntry{
		ID:       "lmstudio-local",
		Vendor:   "lmstudio",
		Protocol: "lmstudio",
		BaseURL:  server.URL,
		Enabled:  true,
	}, "qwen/qwen3-4b-2507")
	if err != nil {
		t.Fatalf("check provider availability: %v", err)
	}
	if !result.Available {
		t.Fatalf("expected provider to be available, got %+v", result)
	}
	if !strings.Contains(result.Detail, "minimal inference succeeded") {
		t.Fatalf("expected inference fallback detail, got %+v", result)
	}
}

func TestCheckProviderAvailabilityReturnsUnavailableWhenListAndInferenceFail(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/models":
			http.NotFound(w, r)
		case "/v1/chat/completions":
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`{"error":{"message":"gateway down"}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	result, err := CheckProviderAvailability(context.Background(), ProviderEntry{
		ID:       "lmstudio-local",
		Vendor:   "lmstudio",
		Protocol: "lmstudio",
		BaseURL:  server.URL,
		Enabled:  true,
	}, "qwen/qwen3-4b-2507")
	if err != nil {
		t.Fatalf("check provider availability: %v", err)
	}
	if result.Available {
		t.Fatalf("expected unavailable result, got %+v", result)
	}
	if !strings.Contains(result.Detail, "status=502") {
		t.Fatalf("expected fallback failure detail, got %+v", result)
	}
}
