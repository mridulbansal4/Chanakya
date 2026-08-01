package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// GeminiExtractor calls the Google Gemini (Generative Language) API and forces
// JSON output. Like the Anthropic extractor, it produces DATA only: the model is
// told to emit the strict {"obligations":[...]} document and the compiler
// re-validates that output against the same strict JSON schema and the mandatory
// verbatim-citation rule - the model is never trusted to self-certify. It is used
// ONLY when GEMINI_API_KEY is configured; otherwise CHANAKYA runs the
// deterministic OfflineExtractor.
type GeminiExtractor struct {
	apiKey     string
	model      string
	schema     json.RawMessage // the strict extraction schema (described to the model)
	httpClient *http.Client
	baseURL    string
	maxRetries int
}

// GeminiConfig configures the extractor.
type GeminiConfig struct {
	APIKey string
	Model  string          // defaults to gemini-2.5-flash
	Schema json.RawMessage // the strict extraction schema
}

// DefaultGeminiModel is the model used when GEMINI_MODEL is unset.
const DefaultGeminiModel = "gemini-2.5-flash"

// NewGeminiExtractor builds a Gemini-backed extractor. It returns an error if no
// API key is supplied.
func NewGeminiExtractor(cfg GeminiConfig) (*GeminiExtractor, error) {
	if cfg.APIKey == "" {
		return nil, fmt.Errorf("gemini extractor: API key is required")
	}
	model := strings.TrimSpace(cfg.Model)
	if model == "" {
		model = DefaultGeminiModel
	}
	return &GeminiExtractor{
		apiKey:     cfg.APIKey,
		model:      model,
		schema:     cfg.Schema,
		httpClient: &http.Client{Timeout: 60 * time.Second},
		// The model id is a path segment; the API key travels in a header, never
		// in the URL/query string.
		baseURL:    "https://generativelanguage.googleapis.com/v1beta/models/" + model + ":generateContent",
		maxRetries: 3,
	}, nil
}

// Name identifies this extractor for provenance.
func (e *GeminiExtractor) Name() string { return "gemini:" + e.model }

// gemini request/response shapes (only the fields we use).
type geminiRequest struct {
	SystemInstruction *geminiContent  `json:"systemInstruction,omitempty"`
	Contents          []geminiContent `json:"contents"`
	GenerationConfig  geminiGenConfig `json:"generationConfig"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenConfig struct {
	ResponseMIMEType string  `json:"responseMimeType"`
	Temperature      float64 `json:"temperature"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []geminiPart `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	Error *struct {
		Code    int    `json:"code"`
		Status  string `json:"status"`
		Message string `json:"message"`
	} `json:"error"`
}

// Extract calls generateContent forcing a JSON response and returns the emitted
// {"obligations":[...]} document as raw JSON. The document is validated by the
// compiler against the strict schema before any obligation is trusted.
func (e *GeminiExtractor) Extract(ctx context.Context, req ExtractionRequest) ([]byte, error) {
	// The model receives the same instruction as the Anthropic path, plus the
	// exact schema so its JSON matches the shape the compiler enforces.
	system := extractionSystemPrompt
	if len(e.schema) > 0 {
		system += "\n\nReturn ONLY a JSON object of the form {\"obligations\": [...]} " +
			"that conforms to this JSON Schema (no prose, no markdown fences):\n" + string(e.schema)
	}

	body := geminiRequest{
		SystemInstruction: &geminiContent{Parts: []geminiPart{{Text: system}}},
		Contents: []geminiContent{{
			Role: "user",
			Parts: []geminiPart{{Text: fmt.Sprintf("Clause %s - %s\n\n%s",
				req.ClauseRef, req.Heading, req.Text)}},
		}},
		// Deterministic-as-possible: temperature 0, strict JSON mime type.
		GenerationConfig: geminiGenConfig{ResponseMIMEType: "application/json", Temperature: 0},
	}
	payload, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal gemini request: %w", err)
	}

	var lastErr error
	for attempt := 0; attempt <= e.maxRetries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("gemini extract cancelled: %w", ctx.Err())
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
	return nil, fmt.Errorf("gemini extract exhausted retries: %w", lastErr)
}

// doRequest performs one HTTP attempt. The bool reports whether the error is
// retryable (429 / 5xx / transport).
func (e *GeminiExtractor) doRequest(ctx context.Context, payload []byte) ([]byte, bool, error) {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, e.baseURL, bytes.NewReader(payload))
	if err != nil {
		return nil, false, fmt.Errorf("build request: %w", err)
	}
	httpReq.Header.Set("content-type", "application/json")
	httpReq.Header.Set("x-goog-api-key", e.apiKey)

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
		return nil, true, fmt.Errorf("gemini status %d: %s", resp.StatusCode, string(respBody))
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("gemini status %d: %s", resp.StatusCode, string(respBody))
	}

	var gr geminiResponse
	if err := json.Unmarshal(respBody, &gr); err != nil {
		return nil, false, fmt.Errorf("decode response: %w", err)
	}
	if gr.Error != nil {
		return nil, false, fmt.Errorf("gemini error %s: %s", gr.Error.Status, gr.Error.Message)
	}
	if len(gr.Candidates) == 0 {
		return nil, false, fmt.Errorf("gemini response contained no candidates")
	}

	// Concatenate the text parts of the first candidate and strip any stray
	// markdown fences the model may add despite the JSON mime type.
	var sb strings.Builder
	for _, p := range gr.Candidates[0].Content.Parts {
		sb.WriteString(p.Text)
	}
	out := stripJSONFence(strings.TrimSpace(sb.String()))
	if out == "" {
		return nil, false, fmt.Errorf("gemini response contained no text output")
	}
	return []byte(out), false, nil
}

// stripJSONFence removes an optional ```json ... ``` markdown fence around a
// JSON payload, returning the inner text.
func stripJSONFence(s string) string {
	if !strings.HasPrefix(s, "```") {
		return s
	}
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimPrefix(s, "json")
	s = strings.TrimPrefix(s, "JSON")
	if i := strings.LastIndex(s, "```"); i >= 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}
