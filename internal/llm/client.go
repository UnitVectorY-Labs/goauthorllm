package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Client communicates with an OpenAI-compatible chat completions endpoint.
type Client struct {
	baseURL    string
	model      string
	apiKey     string
	httpClient *http.Client
}

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"`
	Name    string `json:"name,omitempty"`
	Content string `json:"content"`
}

// StreamEvent represents a single event from a streaming response.
type StreamEvent struct {
	Delta string
	Done  bool
	Err   error
}

type chatRequest struct {
	Model          string          `json:"model"`
	Messages       []Message       `json:"messages"`
	Temperature    float64         `json:"temperature,omitempty"`
	Stream         bool            `json:"stream,omitempty"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type responseFormat struct {
	Type       string      `json:"type"`
	JSONSchema *jsonSchema `json:"json_schema,omitempty"`
}

type jsonSchema struct {
	Name   string `json:"name"`
	Schema any    `json:"schema"`
	Strict bool   `json:"strict,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content any `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

type streamResponse struct {
	Choices []struct {
		Delta struct {
			Content any `json:"content"`
		} `json:"delta"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// NewClient creates a client for the given endpoint.
func NewClient(baseURL, model, apiKey string, timeout time.Duration) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		model:   model,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

// StreamChat sends a streaming chat request and dispatches events through send.
func (c *Client) StreamChat(ctx context.Context, messages []Message, send func(StreamEvent) error) error {
	payload, err := json.Marshal(chatRequest{
		Model:       c.model,
		Messages:    messages,
		Temperature: 0.8,
		Stream:      true,
	})
	if err != nil {
		return err
	}

	resp, err := c.doChatCompletion(ctx, payload, "text/event-stream, application/json")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return fmt.Errorf("llm request failed: status %d", resp.StatusCode)
		}
		return fmt.Errorf("llm request failed: %s", strings.TrimSpace(string(body)))
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(strings.ToLower(contentType), "text/event-stream") {
		return c.consumeNonStreamingResponse(resp.Body, send)
	}

	return c.consumeSSE(resp.Body, send)
}

// StructuredChat sends a non-streaming request with a JSON schema response format.
func (c *Client) StructuredChat(ctx context.Context, messages []Message, schemaName string, schema any) (string, error) {
	schemaName = strings.TrimSpace(schemaName)
	if schemaName == "" {
		return "", fmt.Errorf("schema name is required")
	}

	payload, err := json.Marshal(chatRequest{
		Model:       c.model,
		Messages:    messages,
		Temperature: 0,
		ResponseFormat: &responseFormat{
			Type: "json_schema",
			JSONSchema: &jsonSchema{
				Name:   schemaName,
				Schema: schema,
				Strict: true,
			},
		},
	})
	if err != nil {
		return "", err
	}

	resp, err := c.doChatCompletion(ctx, payload, "application/json")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return "", fmt.Errorf("llm request failed: status %d", resp.StatusCode)
		}
		return "", fmt.Errorf("llm request failed: %s", strings.TrimSpace(string(body)))
	}

	return c.consumeStructuredResponse(resp.Body)
}

func (c *Client) doChatCompletion(ctx context.Context, payload []byte, accept string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", accept)
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	return c.httpClient.Do(req)
}

func (c *Client) consumeNonStreamingResponse(body io.Reader, send func(StreamEvent) error) error {
	payload, err := io.ReadAll(body)
	if err != nil {
		return err
	}

	var decoded chatResponse
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return err
	}
	if decoded.Error != nil && decoded.Error.Message != "" {
		return fmt.Errorf("llm error: %s", decoded.Error.Message)
	}
	if len(decoded.Choices) == 0 {
		return fmt.Errorf("llm response did not include any choices")
	}

	content := extractContent(decoded.Choices[0].Message.Content)
	if strings.TrimSpace(content) != "" {
		if err := send(StreamEvent{Delta: content}); err != nil {
			return err
		}
	}
	return send(StreamEvent{Done: true})
}

func (c *Client) consumeStructuredResponse(body io.Reader) (string, error) {
	payload, err := io.ReadAll(body)
	if err != nil {
		return "", err
	}

	var decoded chatResponse
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return "", err
	}
	if decoded.Error != nil && decoded.Error.Message != "" {
		return "", fmt.Errorf("llm error: %s", decoded.Error.Message)
	}
	if len(decoded.Choices) == 0 {
		return "", fmt.Errorf("llm response did not include any choices")
	}

	content := extractContent(decoded.Choices[0].Message.Content)
	if strings.TrimSpace(content) == "" {
		return "", fmt.Errorf("llm response did not include any content")
	}
	return content, nil
}

func (c *Client) consumeSSE(body io.Reader, send func(StreamEvent) error) error {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	var dataLines []string
	streamDone := false
	dispatch := func() error {
		if len(dataLines) == 0 {
			return nil
		}

		payload := strings.Join(dataLines, "\n")
		dataLines = dataLines[:0]

		if payload == "[DONE]" {
			streamDone = true
			return send(StreamEvent{Done: true})
		}

		var chunk streamResponse
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			return err
		}
		if chunk.Error != nil && chunk.Error.Message != "" {
			return fmt.Errorf("llm error: %s", chunk.Error.Message)
		}
		if len(chunk.Choices) == 0 {
			return nil
		}

		delta := extractContent(chunk.Choices[0].Delta.Content)
		if delta != "" {
			if err := send(StreamEvent{Delta: delta}); err != nil {
				return err
			}
		}
		if chunk.Choices[0].FinishReason != "" {
			streamDone = true
			return send(StreamEvent{Done: true})
		}
		return nil
	}

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if err := dispatch(); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}

	if err := scanner.Err(); err != nil {
		return err
	}
	if err := dispatch(); err != nil {
		return err
	}
	if streamDone {
		return nil
	}
	return send(StreamEvent{Done: true})
}

func extractContent(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case []any:
		var parts []string
		for _, item := range typed {
			fragment, ok := item.(map[string]any)
			if !ok {
				continue
			}
			text, _ := fragment["text"].(string)
			if text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "")
	default:
		return ""
	}
}
