package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// AnthropicExtractor calls the Claude Messages API and forces strictly
// schema-valid JSON via a single strict tool. It is used ONLY when an API key
// is configured; otherwise CHANAKYA runs the deterministic OfflineExtractor.
//
// Determinism note: even here the model produces DATA only. The tool's
// input_schema is the compiler's strict schema, tool_choice forces that tool,
// and the compiler re-validates the returned input against the same schema —
// the model is never trusted to self-certify.
type AnthropicExtractor struct {
	apiKey     string
	model      string
	schema     json.RawMessage // the tool input_schema (compiler's strict schema)
	httpClient *http.Client
	baseURL    string
	maxRetries int
}

// AnthropicConfig configures the extractor.
type AnthropicConfig struct {
	APIKey string
	Model  string          // defaults to claude-opus-4-8
	Schema json.RawMessage // the strict extraction schema
}

// NewAnthropicExtractor builds an Anthropic-backed extractor. It returns an
// error if no API key is supplied.
func NewAnthropicExtractor(cfg AnthropicConfig) (*AnthropicExtractor, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("anthropic extractor: API key is required")
	}
	model := cfg.Model
	if model == "" {
		model = "claude-opus-4-8"
	}
	return &AnthropicExtractor{
		apiKey:     cfg.APIKey,
		model:      model,
		schema:     cfg.Schema,
		httpClient: &http.Client{Timeout: 60 * time.Second},
		baseURL:    "https://api.anthropic.com/v1/messages",
		maxRetries: 3,
	}, nil
}

// Name identifies this extractor for provenance.
func (e *AnthropicExtractor) Name() string { return "anthropic:" + e.model }

const extractionSystemPrompt = `You are a regulation compiler for the Indian securities market. ` +
	`Extract every distinct legal obligation from the given clause as DATA only. ` +
	`For each obligation you MUST provide source_clause_ref (the clause id) and ` +
	`source_sentence (the EXACT verbatim sentence from the clause text that supports it — ` +
	`copy it character-for-character, do not paraphrase). If the clause states no ` +
	`obligation (e.g. it is a definition or heading), return an empty obligations array. ` +
	`Never invent a citation. Report your confidence in [0,1] for each obligation.`

// anthropic request/response shapes (only the fields we use).
type anthropicRequest struct {
	Model      string              `json:"model"`
	MaxTokens  int                 `json:"max_tokens"`
	System     string              `json:"system"`
	Messages   []anthropicMessage  `json:"messages"`
	Tools      []anthropicTool     `json:"tools"`
	ToolChoice anthropicToolChoice `json:"tool_choice"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"input_schema"`
	Strict      bool            `json:"strict"`
}

type anthropicToolChoice struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

type anthropicResponse struct {
	Content []struct {
		Type  string          `json:"type"`
		Name  string          `json:"name"`
		Input json.RawMessage `json:"input"`
	} `json:"content"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error"`
}

// Extract calls the Messages API, forcing the emit_obligations tool, and
// returns the tool_use input (the {"obligations":[...]} document) as raw JSON.
func (e *AnthropicExtractor) Extract(ctx context.Context, req ExtractionRequest) ([]byte, error) {
	body := anthropicRequest{
		Model:     e.model,
		MaxTokens: 4096,
		System:    extractionSystemPrompt,
		Messages: []anthropicMessage{{
			Role: "user",
			Content: fmt.Sprintf("Clause %s — %s\n\n%s",
				req.ClauseRef, req.Heading, req.Text),
		}},
		Tools: []anthropicTool{{
			Name:        "emit_obligations",
			Description: "Emit the obligations extracted from the clause, as structured data.",
			InputSchema: e.schema,
			Strict:      true,
		}},
		ToolChoice: anthropicToolChoice{Type: "tool", Name: "emit_obligations"},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal anthropic request: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= e.maxRetries; attempt++ {
		if attempt > 0 {
			// Exponential backoff, cancellable via ctx.
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("anthropic extract cancelled: %w", ctx.Err())
			case <-time.After(time.Duration(attempt*attempt) * 500 * time.Millisecond):
			}
		}
		out, retry, err := e.doRequest(ctx, payload)
		if err == nil {
			return out, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("anthropic extract exhausted retries: %w", lastErr)
}

// doRequest performs one HTTP attempt. The bool reports whether the error is
// retryable (429 / 5xx / transport).
func (e *AnthropicExtractor) doRequest(ctx context.Context, payload []byte) ([]byte, bool, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL, bytes.NewReader(payload))
	if err != nil {
		return nil, false, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("content-type", "application/json")
	httpReq.Header.Set("x-api-key", e.apiKey)
	httpReq.Header.Set("anthropic-version", "2023-06-01")

	resp, err := e.httpClient.Do(httpReq)
	if err != nil {
		return nil, true, fmt.Errorf("http do: %w", err)
	}
	defer resp.Body.Close()
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, fmt.Errorf("read body: %w", err)
	}

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("anthropic status %d: %s", resp.StatusCode, string(respBody))
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("anthropic status %d: %s", resp.StatusCode, string(respBody))
	}

	var ar anthropicResponse
	if err := json.Unmarshal(respBody, &ar); err != nil {
		return nil, false, fmt.Errorf("decode response: %w", err)
	}
	if ar.Error != nil {
		return nil, false, fmt.Errorf("anthropic error %s: %s", ar.Error.Type, ar.Error.Message)
	}
	for _, block := range ar.Content {
		if block.Type == "tool_use" && block.Name == "emit_obligations" {
			return []byte(block.Input), false, nil
		}
	}
	return nil, false, fmt.Errorf("anthropic response contained no emit_obligations tool_use block")
}
