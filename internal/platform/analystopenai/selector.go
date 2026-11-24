// Package analystopenai lets a model select catalog fields. The provider
// receives a question and the allowed keys, with no database tools or records.
package analystopenai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"
)

var ErrSelection = errors.New("model field selection failed")

var modelName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.:-]{0,127}$`)

type Selector struct {
	client *http.Client
	key    string
	model  string
}

func New(key, model string) (*Selector, error) {
	if key == "" || len(key) > 4096 || strings.ContainsAny(key, " \r\n\t") || !modelName.MatchString(model) {
		return nil, errors.New("invalid analyst provider configuration")
	}
	return &Selector{
		key: key, model: model,
		client: &http.Client{
			Timeout: 8 * time.Second,
			CheckRedirect: func(*http.Request, []*http.Request) error {
				return http.ErrUseLastResponse
			},
		},
	}, nil
}

func (s *Selector) Select(ctx context.Context, question string, allowed []string) ([]string, string, error) {
	const endpoint = "https://api.openai.com/v1/responses"
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{"fields": map[string]any{
			"type": "array", "items": map[string]any{"type": "string", "enum": allowed},
		}},
		"required": []string{"fields"}, "additionalProperties": false,
	}
	payload, err := json.Marshal(map[string]any{
		"model": s.model, "input": question, "store": false, "max_output_tokens": 128,
		"instructions": "Select the catalog field keys that would help answer the question. Select an empty list for an unsupported topic. Follow the schema and emit no answer text.",
		"text": map[string]any{"format": map[string]any{
			"type": "json_schema", "name": "catalog_fields", "strict": true, "schema": schema,
		}},
	})
	if err != nil {
		return nil, "", ErrSelection
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, "", ErrSelection
	}
	request.Header.Set("Authorization", "Bearer "+s.key)
	request.Header.Set("Content-Type", "application/json")
	response, err := s.client.Do(request)
	if err != nil {
		return nil, "", ErrSelection
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, "", ErrSelection
	}
	const maxResponse = 256 << 10
	body, err := io.ReadAll(io.LimitReader(response.Body, maxResponse+1))
	if err != nil || len(body) > maxResponse {
		return nil, "", ErrSelection
	}
	var envelope struct {
		Status string `json:"status"`
		Model  string `json:"model"`
		Output []struct {
			Type    string `json:"type"`
			Content []struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}
	if json.Unmarshal(body, &envelope) != nil || envelope.Status != "completed" || !modelName.MatchString(envelope.Model) {
		return nil, "", ErrSelection
	}
	var result struct {
		Fields []string `json:"fields"`
	}
	var decoded bool
	for _, item := range envelope.Output {
		if item.Type != "message" {
			continue
		}
		for _, part := range item.Content {
			if part.Type != "output_text" || decoded {
				return nil, "", ErrSelection
			}
			decoder := json.NewDecoder(strings.NewReader(part.Text))
			decoder.DisallowUnknownFields()
			if decoder.Decode(&result) != nil || decoder.Decode(new(any)) != io.EOF || result.Fields == nil {
				return nil, "", ErrSelection
			}
			decoded = true
		}
	}
	if !decoded {
		return nil, "", ErrSelection
	}
	return result.Fields, envelope.Model, nil
}
