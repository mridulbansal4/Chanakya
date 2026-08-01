package llm

import (
	"encoding/json"
	"os"
	"strings"
)

// SelectExtractor chooses the extractor by which API key is present in the
// environment, in precedence order:
//
//  1. GEMINI_API_KEY        -> Google Gemini extractor (model GEMINI_MODEL)
//  2. CHANAKYA_LLM_API_KEY  -> Anthropic extractor (model CHANAKYA_LLM_MODEL)
//  3. (none)                -> deterministic offline extractor (the default)
//
// schema is the strict extraction schema handed to a real LLM so its output
// matches the shape the compiler enforces; the offline extractor ignores it.
// Whichever extractor is returned, its output is DATA that the compiler
// re-validates against the same strict schema and the mandatory verbatim-citation
// rule - the choice of extractor never weakens the safety model.
func SelectExtractor(schema json.RawMessage) (Extractor, error) {
	if key := strings.TrimSpace(os.Getenv("GEMINI_API_KEY")); key != "" {
		return NewGeminiExtractor(GeminiConfig{
			APIKey: key,
			Model:  os.Getenv("GEMINI_MODEL"),
			Schema: schema,
		})
	}
	if key := strings.TrimSpace(os.Getenv("CHANAKYA_LLM_API_KEY")); key != "" {
		return NewAnthropicExtractor(AnthropicConfig{
			APIKey: key,
			Model:  os.Getenv("CHANAKYA_LLM_MODEL"),
			Schema: schema,
		})
	}
	return NewOfflineExtractor(), nil
}
