package llm

import "testing"

func TestNewGeminiExtractorRequiresKey(t *testing.T) {
	if _, err := NewGeminiExtractor(GeminiConfig{}); err == nil {
		t.Fatal("expected an error when no API key is supplied")
	}
}

func TestNewGeminiExtractorDefaultsModel(t *testing.T) {
	ex, err := NewGeminiExtractor(GeminiConfig{APIKey: "test-key"})
	if err != nil {
		t.Fatalf("NewGeminiExtractor: %v", err)
	}
	if ex.model != DefaultGeminiModel {
		t.Errorf("model = %q, want default %q", ex.model, DefaultGeminiModel)
	}
	if got, want := ex.Name(), "gemini:"+DefaultGeminiModel; got != want {
		t.Errorf("Name() = %q, want %q", got, want)
	}
}

func TestNewGeminiExtractorHonoursModel(t *testing.T) {
	ex, err := NewGeminiExtractor(GeminiConfig{APIKey: "k", Model: "gemini-2.5-pro"})
	if err != nil {
		t.Fatalf("NewGeminiExtractor: %v", err)
	}
	if ex.Name() != "gemini:gemini-2.5-pro" {
		t.Errorf("Name() = %q", ex.Name())
	}
}

func TestStripJSONFence(t *testing.T) {
	cases := map[string]string{
		"{\"a\":1}":               "{\"a\":1}",
		"```json\n{\"a\":1}\n```": "{\"a\":1}",
		"```\n{\"a\":1}\n```":     "{\"a\":1}",
	}
	for in, want := range cases {
		if got := stripJSONFence(in); got != want {
			t.Errorf("stripJSONFence(%q) = %q, want %q", in, got, want)
		}
	}
}
