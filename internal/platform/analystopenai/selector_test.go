package analystopenai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

type transportFunc func(*http.Request) (*http.Response, error)

func (f transportFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestSelectorSendsOnlyQuestionAndStructuredContract(t *testing.T) {
	s, err := New("synthetic-key", "synthetic-model")
	if err != nil {
		t.Fatal(err)
	}
	s.client.Transport = transportFunc(func(r *http.Request) (*http.Response, error) {
		var data map[string]any
		if r.URL.String() != "https://api.openai.com/v1/responses" || r.Header.Get("Authorization") != "Bearer synthetic-key" ||
			json.NewDecoder(r.Body).Decode(&data) != nil || data["input"] != "How long?" || data["store"] != false || data["tools"] != nil {
			t.Fatalf("invalid provider request: %+v", data)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(
			`{"status":"completed","model":"synthetic-model-1","output":[{"type":"message","content":[{"type":"output_text","text":"{\"fields\":[\"episodeCount\"]}"}]}]}`,
		))}, nil
	})
	fields, model, err := s.Select(context.Background(), "How long?", []string{"episodeCount"})
	if err != nil || model != "synthetic-model-1" || len(fields) != 1 || fields[0] != "episodeCount" {
		t.Fatalf("unexpected selection: %v %q %v", fields, model, err)
	}
}
