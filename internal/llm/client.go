package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// Client communicates with an OpenAI-compatible chat completions endpoint.
type Client struct {
	baseURL           string
	model             string
	apiKey            string
	userAgent         string
	requestHTTPClient *http.Client
	streamHTTPClient  *http.Client
}

const defaultUserAgent = "goauthorllm/dev"

// Message represents a chat message.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// StreamEvent represents a single event from a streaming response.
type StreamEvent struct {
	Delta    string
	JoinMode string // populated when structured streaming reports join_mode
	Intent   string // populated when structured streaming reports intent
	Done     bool
	Err      error
}

type chatRequest struct {
	Model          string          `json:"model"`
	Messages       []Message       `json:"messages"`
	Temperature    *float64        `json:"temperature,omitempty"`
	Stream         bool            `json:"stream,omitzero"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type responseFormat struct {
	Type       string      `json:"type"`
	JSONSchema *jsonSchema `json:"json_schema,omitempty"`
}

type jsonSchema struct {
	Name   string `json:"name"`
	Schema any    `json:"schema"`
	Strict bool   `json:"strict,omitzero"`
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

type modelsResponse struct {
	Data []struct {
		ID string `json:"id"`
	} `json:"data"`
}

// httpStatusError wraps an HTTP status error from the LLM endpoint so callers
// can decide whether to fall back (e.g., if structured streaming is rejected).
type httpStatusError struct {
	StatusCode int
	Body       string
}

func (e *httpStatusError) Error() string {
	if e.Body != "" {
		return fmt.Sprintf("llm request failed (%d): %s", e.StatusCode, e.Body)
	}
	return fmt.Sprintf("llm request failed: status %d", e.StatusCode)
}

// NewClient creates a client for the given endpoint.
func NewClient(baseURL, model, apiKey string, timeout time.Duration, userAgent ...string) *Client {
	requestUserAgent := defaultUserAgent
	if len(userAgent) > 0 && strings.TrimSpace(userAgent[0]) != "" {
		requestUserAgent = userAgent[0]
	}

	return &Client{
		baseURL:   strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		model:     strings.TrimSpace(model),
		apiKey:    apiKey,
		userAgent: requestUserAgent,
		requestHTTPClient: &http.Client{
			Timeout: timeout,
		},
		// Streaming requests are canceled explicitly via context rather than a
		// fixed total timeout.
		streamHTTPClient: &http.Client{},
	}
}

// Model returns the configured model ID.
func (c *Client) Model() string { return c.model }

// BaseURL returns the configured endpoint base URL.
func (c *Client) BaseURL() string { return c.baseURL }

// ListModels queries GET <base_url>/models and returns sorted model IDs.
func (c *Client) ListModels(ctx context.Context) ([]string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/models", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.requestHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		body, _ := io.ReadAll(resp.Body)
		return nil, &httpStatusError{StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(body))}
	}

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var decoded modelsResponse
	if err := json.Unmarshal(payload, &decoded); err != nil {
		return nil, fmt.Errorf("parse models response: %w", err)
	}
	models := make([]string, 0, len(decoded.Data))
	seen := make(map[string]struct{}, len(decoded.Data))
	for _, entry := range decoded.Data {
		id := strings.TrimSpace(entry.ID)
		if id == "" {
			continue
		}
		if _, exists := seen[id]; exists {
			continue
		}
		seen[id] = struct{}{}
		models = append(models, id)
	}
	sort.Strings(models)
	return models, nil
}

// StreamChat sends a plain streaming chat request and dispatches events through send.
func (c *Client) StreamChat(ctx context.Context, messages []Message, send func(StreamEvent) error) error {
	temp := 0.7
	return c.streamRequest(ctx, chatRequest{
		Model:       c.model,
		Messages:    messages,
		Temperature: &temp,
		Stream:      true,
	}, send)
}

// StreamStructuredGeneration streams a JSON-schema-constrained generation
// response (`{join_mode, intent, content}`), incrementally extracting and
// unescaping "content" as tokens arrive. If the server rejects response_format
// with HTTP 400/422, it falls back automatically to plain streaming.
func (c *Client) StreamStructuredGeneration(
	ctx context.Context,
	messages []Message,
	schemaName string,
	schema any,
	fallbackMessages []Message,
	send func(StreamEvent) error,
) error {
	temp := 0.7
	req := chatRequest{
		Model:       c.model,
		Messages:    messages,
		Temperature: &temp,
		Stream:      true,
		ResponseFormat: &responseFormat{
			Type: "json_schema",
			JSONSchema: &jsonSchema{
				Name:   schemaName,
				Schema: schema,
				Strict: true,
			},
		},
	}

	extractor := NewJSONStreamExtractor("content")
	emittedContent := false
	rawCollected := strings.Builder{}

	err := c.streamRequest(ctx, req, func(ev StreamEvent) error {
		if ev.Err != nil {
			return ev.Err
		}
		if ev.Delta != "" {
			rawCollected.WriteString(ev.Delta)
			for _, fe := range extractor.Feed(ev.Delta) {
				switch fe.Field {
				case "join_mode":
					if fe.Done && fe.Value != "" {
						if err := send(StreamEvent{JoinMode: strings.TrimSpace(fe.Value)}); err != nil {
							return err
						}
					}
				case "intent":
					if fe.Done && fe.Value != "" {
						if err := send(StreamEvent{Intent: strings.TrimSpace(fe.Value)}); err != nil {
							return err
						}
					}
				case "content":
					if fe.Delta != "" {
						emittedContent = true
						if err := send(StreamEvent{Delta: fe.Delta}); err != nil {
							return err
						}
					}
				}
			}
		}
		if ev.Done {
			// Safety net: if the endpoint ignored response_format and streamed
			// plain non-JSON markdown directly, forward it rather than dropping it.
			if !emittedContent {
				raw := strings.TrimSpace(rawCollected.String())
				if raw != "" && !strings.HasPrefix(raw, "{") {
					if err := send(StreamEvent{Delta: stripOuterCodeFence(raw)}); err != nil {
						return err
					}
				}
			}
			return send(StreamEvent{Done: true})
		}
		return nil
	})
	if err != nil {
		var statusErr *httpStatusError
		if !emittedContent && errors.As(err, &statusErr) &&
			(statusErr.StatusCode == http.StatusBadRequest || statusErr.StatusCode == http.StatusUnprocessableEntity) &&
			len(fallbackMessages) > 0 {
			return c.StreamChat(ctx, fallbackMessages, send)
		}
		return err
	}
	return nil
}

// StructuredChat sends a non-streaming request with a JSON schema response format.
func (c *Client) StructuredChat(ctx context.Context, messages []Message, schemaName string, schema any) (string, error) {
	schemaName = strings.TrimSpace(schemaName)
	if schemaName == "" {
		return "", fmt.Errorf("schema name is required")
	}

	temp := 0.0
	payload, err := json.Marshal(chatRequest{
		Model:       c.model,
		Messages:    messages,
		Temperature: &temp,
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

	resp, err := c.doChatCompletion(c.requestHTTPClient, ctx, payload, "application/json")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return "", &httpStatusError{StatusCode: resp.StatusCode}
		}
		return "", &httpStatusError{StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(body))}
	}

	return c.consumeStructuredResponse(resp.Body)
}

func (c *Client) streamRequest(ctx context.Context, reqBody chatRequest, send func(StreamEvent) error) error {
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	resp, err := c.doChatCompletion(c.streamHTTPClient, ctx, payload, "text/event-stream, application/json")
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		body, readErr := io.ReadAll(resp.Body)
		if readErr != nil {
			return &httpStatusError{StatusCode: resp.StatusCode}
		}
		return &httpStatusError{StatusCode: resp.StatusCode, Body: strings.TrimSpace(string(body))}
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.Contains(strings.ToLower(contentType), "text/event-stream") {
		return c.consumeNonStreamingResponse(resp.Body, send)
	}

	return c.consumeSSE(resp.Body, send)
}

func (c *Client) doChatCompletion(httpClient *http.Client, ctx context.Context, payload []byte, accept string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", accept)
	req.Header.Set("User-Agent", c.userAgent)
	if c.apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	return httpClient.Do(req)
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
	if content != "" {
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
			if streamDone {
				return nil
			}
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
		if chunk.Choices[0].FinishReason != "" && !streamDone {
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
		if after, ok := strings.CutPrefix(line, "data:"); ok {
			dataLines = append(dataLines, strings.TrimSpace(after))
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
	case map[string]any:
		text, _ := typed["text"].(string)
		return text
	default:
		return ""
	}
}

func stripOuterCodeFence(s string) string {
	trimmed := strings.TrimSpace(s)
	if !strings.HasPrefix(trimmed, "```") {
		return s
	}
	idx := strings.IndexByte(trimmed, '\n')
	if idx < 0 {
		return s
	}
	inner := trimmed[idx+1:]
	if strings.HasSuffix(strings.TrimSpace(inner), "```") {
		inner = strings.TrimSpace(inner)
		inner = strings.TrimSuffix(inner, "```")
		return strings.TrimRight(inner, "\n")
	}
	return s
}
