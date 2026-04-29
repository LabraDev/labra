package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type openAIResponsesProvider struct {
	apiKey  string
	baseURL string
	client  *http.Client
}

func newOpenAIResponsesProvider(apiKey string, baseURL string) AIProvider {
	trimmedBaseURL := strings.TrimSpace(baseURL)
	if trimmedBaseURL == "" {
		trimmedBaseURL = "https://api.openai.com/v1"
	}
	trimmedBaseURL = strings.TrimRight(trimmedBaseURL, "/")

	return openAIResponsesProvider{
		apiKey:  strings.TrimSpace(apiKey),
		baseURL: trimmedBaseURL,
		client:  &http.Client{},
	}
}

func (p openAIResponsesProvider) Generate(ctx context.Context, in AIProviderInput) (AIProviderOutput, error) {
	if strings.TrimSpace(p.apiKey) == "" {
		return AIProviderOutput{}, fmt.Errorf("OPENAI_API_KEY is not configured")
	}

	model := strings.TrimSpace(in.Model)
	if model == "" {
		model = "gpt-4.1-mini"
	}

	reqBody := map[string]any{
		"model": model,
		"input": []map[string]any{
			{
				"role": "system",
				"content": []map[string]string{
					{
						"type": "input_text",
						"text": "You are Labra's deployment assistant. Be concise, factual, and use only provided context.",
					},
				},
			},
			{
				"role": "user",
				"content": []map[string]string{
					{
						"type": "input_text",
						"text": strings.TrimSpace(in.Prompt),
					},
				},
			},
		},
	}

	encoded, err := json.Marshal(reqBody)
	if err != nil {
		return AIProviderOutput{}, fmt.Errorf("marshal openai request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/responses", bytes.NewReader(encoded))
	if err != nil {
		return AIProviderOutput{}, fmt.Errorf("build openai request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return AIProviderOutput{}, fmt.Errorf("call openai responses: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return AIProviderOutput{}, fmt.Errorf("read openai response: %w", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(bodyBytes, &payload); err != nil {
		return AIProviderOutput{}, fmt.Errorf("decode openai response: %w", err)
	}

	if resp.StatusCode >= 300 {
		if apiErr := extractOpenAIError(payload); apiErr != "" {
			return AIProviderOutput{}, fmt.Errorf("openai error (%d): %s", resp.StatusCode, apiErr)
		}
		return AIProviderOutput{}, fmt.Errorf("openai error (%d)", resp.StatusCode)
	}

	text := strings.TrimSpace(extractOpenAIOutputText(payload))
	if text == "" {
		return AIProviderOutput{}, fmt.Errorf("openai response contained no output text")
	}

	return AIProviderOutput{
		Text:       text,
		Provider:   "openai",
		Model:      model,
		Confidence: "medium",
	}, nil
}

func extractOpenAIError(payload map[string]any) string {
	errRaw, ok := payload["error"]
	if !ok {
		return ""
	}
	errObj, ok := errRaw.(map[string]any)
	if !ok {
		return ""
	}
	return strings.TrimSpace(toString(errObj["message"]))
}

func extractOpenAIOutputText(payload map[string]any) string {
	if v := strings.TrimSpace(toString(payload["output_text"])); v != "" {
		return v
	}

	outputItems, ok := payload["output"].([]any)
	if !ok {
		return ""
	}
	segments := make([]string, 0)
	for _, item := range outputItems {
		itemObj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		contentItems, ok := itemObj["content"].([]any)
		if !ok {
			continue
		}
		for _, content := range contentItems {
			contentObj, ok := content.(map[string]any)
			if !ok {
				continue
			}
			if text := strings.TrimSpace(toString(contentObj["text"])); text != "" {
				segments = append(segments, text)
				continue
			}
			if typeName := strings.TrimSpace(toString(contentObj["type"])); typeName == "output_text" {
				if textObj, ok := contentObj["text"].(map[string]any); ok {
					if text := strings.TrimSpace(toString(textObj["value"])); text != "" {
						segments = append(segments, text)
					}
				}
			}
		}
	}
	return strings.Join(segments, "\n")
}

func toString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	default:
		return ""
	}
}
