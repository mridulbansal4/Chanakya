package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// JSONCompleter is a general strict-JSON completion, used where the caller needs
// a shape other than {"obligations": [...]} - today, Stage 3 circular metadata.
//
// SAFETY: identical model to Extractor. The returned bytes are DATA that the
// caller validates against a strict JSON Schema before trusting any field. A
// completer never enforces anything and its output is never executed.
type JSONCompleter interface {
	Name() string
	// Complete returns raw JSON conforming to schema, given a system instruction
	// and user content.
	Complete(ctx context.Context, system, user string, schema json.RawMessage) ([]byte, error)
}

// SelectJSONCompleter mirrors SelectExtractor's precedence: Gemini, then
// Anthropic, then none. Returning nil (rather than an offline stand-in) is
// deliberate: there is no deterministic way to infer a circular number that the
// regex pass could not find, and a stub that returned plausible-looking metadata
// would be worse than an admitted gap.
func SelectJSONCompleter() JSONCompleter {
	if key := strings.TrimSpace(os.Getenv("GEMINI_API_KEY")); key != "" {
		return &geminiCompleter{apiKey: key, model: modelOr(os.Getenv("GEMINI_MODEL"), DefaultGeminiModel)}
	}
	if key := strings.TrimSpace(os.Getenv("CHANAKYA_LLM_API_KEY")); key != "" {
		return &anthropicCompleter{apiKey: key, model: modelOr(os.Getenv("CHANAKYA_LLM_MODEL"), defaultAnthropicModel)}
	}
	return nil
}

func modelOr(v, def string) string {
	if v = strings.TrimSpace(v); v != "" {
		return v
	}
	return def
}

const defaultAnthropicModel = "claude-sonnet-4-5"

// completionTimeout bounds a single completion. Stage 3 is one call per
// document, so a generous-but-finite budget is right.
const completionTimeout = 60 * time.Second

// --- Gemini ------------------------------------------------------------------

type geminiCompleter struct {
	apiKey string
	model  string
}

func (c *geminiCompleter) Name() string { return "gemini:" + c.model }

func (c *geminiCompleter) Complete(ctx context.Context, system, user string, schema json.RawMessage) ([]byte, error) {
	if len(schema) > 0 {
		system += "\n\nReturn ONLY a JSON object conforming to this JSON Schema " +
			"(no prose, no markdown fences):\n" + string(schema)
	}
	body := geminiRequest{
		SystemInstruction: &geminiContent{Parts: []geminiPart{{Text: system}}},
		Contents:          []geminiContent{{Role: "user", Parts: []geminiPart{{Text: user}}}},
		GenerationConfig:  geminiGenConfig{ResponseMIMEType: "application/json", Temperature: 0},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal gemini completion request: %w", err)
	}
	url := "https://generativelanguage.googleapis.com/v1beta/models/" + c.model + ":generateContent"
	raw, err := postJSON(ctx, url, payload, map[string]string{"x-goog-api-key": c.apiKey})
	if err != nil {
		return nil, err
	}
	var gr geminiResponse
	if err := json.Unmarshal(raw, &gr); err != nil {
		return nil, fmt.Errorf("decode gemini completion: %w", err)
	}
	if gr.Error != nil {
		return nil, fmt.Errorf("gemini error %s: %s", gr.Error.Status, gr.Error.Message)
	}
	if len(gr.Candidates) == 0 {
		return nil, fmt.Errorf("gemini completion contained no candidates")
	}
	var sb strings.Builder
	for _, p := range gr.Candidates[0].Content.Parts {
		sb.WriteString(p.Text)
	}
	out := stripJSONFence(strings.TrimSpace(sb.String()))
	if out == "" {
		return nil, fmt.Errorf("gemini completion contained no text")
	}
	return []byte(out), nil
}

// --- Anthropic ---------------------------------------------------------------

type anthropicCompleter struct {
	apiKey string
	model  string
}

func (c *anthropicCompleter) Name() string { return "anthropic:" + c.model }

func (c *anthropicCompleter) Complete(ctx context.Context, system, user string, schema json.RawMessage) ([]byte, error) {
	// Strict tool use forces the model's output into the schema's shape, the
	// same mechanism the obligation extractor uses.
	tool := map[string]any{
		"name":         "emit_metadata",
		"description":  "Emit the requested document metadata fields.",
		"input_schema": json.RawMessage(schema),
	}
	body := map[string]any{
		"model":       c.model,
		"max_tokens":  1024,
		"system":      system,
		"messages":    []map[string]any{{"role": "user", "content": user}},
		"tools":       []any{tool},
		"tool_choice": map[string]any{"type": "tool", "name": "emit_metadata"},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal anthropic completion request: %w", err)
	}
	raw, err := postJSON(ctx, "https://api.anthropic.com/v1/messages", payload, map[string]string{
		"x-api-key":         c.apiKey,
		"anthropic-version": "2023-06-01",
	})
	if err != nil {
		return nil, err
	}
	var ar struct {
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
	if err := json.Unmarshal(raw, &ar); err != nil {
		return nil, fmt.Errorf("decode anthropic completion: %w", err)
	}
	if ar.Error != nil {
		return nil, fmt.Errorf("anthropic error %s: %s", ar.Error.Type, ar.Error.Message)
	}
	for _, blk := range ar.Content {
		if blk.Type == "tool_use" && len(blk.Input) > 0 {
			return blk.Input, nil
		}
	}
	return nil, fmt.Errorf("anthropic completion contained no tool_use block")
}

// --- shared transport --------------------------------------------------------

// postJSON performs one JSON POST with retry-and-backoff on 429/5xx/transport
// errors, using the same attempt*attempt*500ms + shape as gemini.go.
func postJSON(ctx context.Context, url string, payload []byte, headers map[string]string) ([]byte, error) {
	const maxRetries = 2
	client := &http.Client{Timeout: completionTimeout}

	var lastErr error
	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("completion cancelled: %w", ctx.Err())
			case <-time.After(time.Duration(attempt*attempt) * 500 * time.Millisecond):
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
		if err != nil {
			return nil, fmt.Errorf("build completion request: %w", err)
		}
		req.Header.Set("content-type", "application/json")
		for k, v := range headers {
			req.Header.Set(k, v)
		}

		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("http do: %w", err)
			continue
		}
		respBody, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = fmt.Errorf("read body: %w", readErr)
			continue
		}
		if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
			lastErr = fmt.Errorf("completion status %d: %s", resp.StatusCode, string(respBody))
			continue
		}
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("completion status %d: %s", resp.StatusCode, string(respBody))
		}
		return respBody, nil
	}
	return nil, fmt.Errorf("completion exhausted retries: %w", lastErr)
}
