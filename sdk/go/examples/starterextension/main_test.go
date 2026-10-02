package main

import (
	"context"
	"encoding/json"
	"testing"
)

func TestInterceptInputPreservesTextAndAppendsMarker(t *testing.T) {
	for _, text := range []string{
		"explain sidecars",
		"starter: explain sidecars",
		"<response-language>\nUse English.\n</response-language>\n\nexplain sidecars",
		"<available-skills>\nreview\n</available-skills>\n\nexplain sidecars\n\n<memory-recall>\ncontext\n</memory-recall>",
	} {
		payload, err := json.Marshal(map[string]string{"text": text})
		if err != nil {
			t.Fatal(err)
		}
		result, err := interceptInput(context.Background(), "input.receive", payload)
		if err != nil {
			t.Fatal(err)
		}
		if result == nil || result.Decision != "replace" {
			t.Fatalf("text=%q, result = %#v, want replace", text, result)
		}
		var replacement struct {
			Text string `json:"text"`
		}
		if err := json.Unmarshal(result.Replacement, &replacement); err != nil {
			t.Fatal(err)
		}
		if want := text + inputSuffix; replacement.Text != want {
			t.Fatalf("replacement = %q, want %q", replacement.Text, want)
		}
	}
}

func TestInterceptInputContinuesEmptyOrMalformedPayload(t *testing.T) {
	for _, payload := range []json.RawMessage{[]byte(`{"text":""}`), []byte(`{}`), []byte(`null`), []byte(`{"text":1}`), []byte(`{`)} {
		result, err := interceptInput(context.Background(), "input.receive", payload)
		if err != nil {
			t.Fatal(err)
		}
		if result == nil || result.Decision != "continue" {
			t.Fatalf("payload=%s, result = %#v, want continue", payload, result)
		}
	}
}
