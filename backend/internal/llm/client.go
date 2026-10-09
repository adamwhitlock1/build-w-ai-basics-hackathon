// Package llm talks to an LLM through OpenRouter's OpenAI-compatible API.
// It turns a user's sentence into a WidgetSpec and turns search results into
// widget Items, always asking for structured (JSON Schema) output and always
// re-validating that output in Go.
package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	openai "github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
)

// DefaultBaseURL is OpenRouter's OpenAI-compatible API root.
const DefaultBaseURL = "https://openrouter.ai/api/v1"

// callTimeout bounds a single chat completion.
const callTimeout = 40 * time.Second

// Errors returned by Client methods. Callers match them with errors.Is.
var (
	// ErrInvalidSpec means the model answered, but even after a retry its
	// answer named an unknown source or archetype, or left out a title or query.
	ErrInvalidSpec = errors.New("model did not produce a usable widget spec")
	// ErrNoItems means the model could not shape any usable items from the
	// search results.
	ErrNoItems = errors.New("model produced no usable items")
)

// UpstreamError reports a failed call to OpenRouter or the model behind it.
// Reason and Hint are short and safe to show to users; Err holds the detail
// for logs.
type UpstreamError struct {
	Reason string // what went wrong, e.g. "OpenRouter is rate limiting requests"
	Hint   string // what to try next; may be empty
	Err    error
}

func (e *UpstreamError) Error() string { return "openrouter: " + e.Reason + ": " + e.Err.Error() }

// Unwrap lets errors.Is and errors.As see the underlying cause.
func (e *UpstreamError) Unwrap() error { return e.Err }

// Client calls OpenRouter. It is safe for concurrent use.
type Client struct {
	api        *openai.Client
	http       *http.Client
	apiKey     string
	baseURL    string
	model      string
	retryDelay time.Duration // pause before retrying a 429
}

// NewClient returns a Client that uses model through OpenRouter. An empty
// baseURL means DefaultBaseURL; tests point it at a fake server.
func NewClient(apiKey, model, baseURL string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	baseURL = strings.TrimRight(baseURL, "/")

	httpClient := &http.Client{
		Timeout: 60 * time.Second, // backstop; each call also has its own context deadline
		Transport: &headerTransport{
			base: http.DefaultTransport,
			// Optional OpenRouter attribution headers.
			headers: map[string]string{
				"HTTP-Referer":       "http://localhost:5174",
				"X-Title":            "Porchlight",
				"X-OpenRouter-Title": "Porchlight",
			},
		},
	}

	cfg := openai.DefaultConfig(apiKey)
	cfg.BaseURL = baseURL
	cfg.HTTPClient = httpClient

	return &Client{
		api:        openai.NewClientWithConfig(cfg),
		http:       httpClient,
		apiKey:     apiKey,
		baseURL:    baseURL,
		model:      model,
		retryDelay: time.Second,
	}
}

// headerTransport is an http.RoundTripper that adds fixed headers to every
// request before handing it to the base transport.
type headerTransport struct {
	base    http.RoundTripper
	headers map[string]string
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	// A RoundTripper must not modify the caller's request, so work on a clone.
	req = req.Clone(req.Context())
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}
	return t.base.RoundTrip(req)
}

// structuredCall is one request for JSON that must match schema.
type structuredCall struct {
	name     string // schema name sent to the API, e.g. "widget_spec"
	schema   *jsonschema.Definition
	messages []openai.ChatCompletionMessage
}

// callJSON sends call, decodes the reply into a T, and runs check on it. If
// the reply doesn't parse or check rejects it, callJSON retries once, telling
// the model what was wrong. API failures are returned as they are (complete
// has already retried a rate limit once).
//
// It is a generic function rather than a method because Go methods can't
// declare their own type parameters.
func callJSON[T any](ctx context.Context, c *Client, call structuredCall, check func(*T) error) (T, error) {
	var zero T
	var lastErr error
	for attempt := 1; attempt <= 2; attempt++ {
		content, err := c.complete(ctx, call)
		if err != nil {
			return zero, err
		}

		var out T
		if err := json.Unmarshal([]byte(stripCodeFence(content)), &out); err != nil {
			lastErr = &UpstreamError{Reason: "the AI model sent back an answer we couldn't read", Hint: "Try again in a moment.", Err: err}
		} else if err := check(&out); err != nil {
			lastErr = err
		} else {
			return out, nil
		}

		slog.WarnContext(ctx, "llm reply rejected", "schema", call.name, "attempt", attempt, "err", lastErr)
		// Show the model its bad reply so it can correct it. APIs reject an
		// assistant message without content, so an empty reply is left out.
		if strings.TrimSpace(content) != "" {
			call.messages = append(call.messages, openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: content})
		}
		call.messages = append(call.messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: fmt.Sprintf("That reply was invalid (%v). Reply again with only JSON that matches the schema.", lastErr),
		})
	}
	return zero, lastErr
}

// complete runs one chat completion and returns the reply text. A rate limit
// (HTTP 429, common on OpenRouter's free models) is retried once after a
// short pause.
func (c *Client) complete(ctx context.Context, call structuredCall) (string, error) {
	content, err := c.completeOnce(ctx, call)
	if httpStatus(err) != http.StatusTooManyRequests {
		return content, err
	}
	slog.WarnContext(ctx, "openrouter rate limited; retrying once", "schema", call.name)
	// select lets the pause end early if the request is cancelled.
	select {
	case <-time.After(c.retryDelay):
	case <-ctx.Done():
		return "", err
	}
	return c.completeOnce(ctx, call)
}

// completeOnce runs a single chat completion request.
func (c *Client) completeOnce(ctx context.Context, call structuredCall) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()

	resp, err := c.api.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model:       c.model,
		Messages:    call.messages,
		Temperature: 0.2,
		ResponseFormat: &openai.ChatCompletionResponseFormat{
			Type: openai.ChatCompletionResponseFormatTypeJSONSchema,
			JSONSchema: &openai.ChatCompletionResponseFormatJSONSchema{
				Name:   call.name,
				Schema: call.schema,
				Strict: true,
			},
		},
	})
	if err != nil {
		return "", upstreamError(err)
	}
	// OpenRouter can answer HTTP 200 with an error object and no choices.
	// go-openai reports that as success, so check for it ourselves.
	if len(resp.Choices) == 0 {
		return "", &UpstreamError{
			Reason: "OpenRouter returned an empty answer (the model's provider may be having trouble)",
			Hint:   "Try again in a moment. If it keeps happening, try a different OPENROUTER_MODEL.",
			Err:    errors.New("response had no choices"),
		}
	}
	choice := resp.Choices[0]
	switch choice.FinishReason {
	case openai.FinishReasonLength:
		return "", &UpstreamError{Reason: "the AI model's answer was cut off", Hint: "Try again in a moment.", Err: errors.New("finish_reason=length")}
	case openai.FinishReasonContentFilter:
		return "", &UpstreamError{Reason: "the AI model declined to answer", Hint: "Try describing the widget differently.", Err: errors.New("finish_reason=content_filter")}
	}
	return choice.Message.Content, nil
}

// httpStatus returns the HTTP status code carried by a go-openai error, or 0
// if err didn't come from an HTTP response.
func httpStatus(err error) int {
	var apiErr *openai.APIError
	var reqErr *openai.RequestError
	switch {
	case errors.As(err, &apiErr):
		return apiErr.HTTPStatusCode
	case errors.As(err, &reqErr):
		return reqErr.HTTPStatusCode
	}
	return 0
}

// upstreamError converts a go-openai error into an *UpstreamError with a
// reason and hint a person can act on.
func upstreamError(err error) error {
	e := &UpstreamError{Reason: "couldn't reach OpenRouter", Hint: "Check your internet connection, then try again.", Err: err}
	switch status := httpStatus(err); {
	case status == http.StatusUnauthorized:
		e.Reason, e.Hint = "OpenRouter rejected the API key", "Check OPENROUTER_API_KEY in backend/.env, then restart the backend."
	case status == http.StatusPaymentRequired:
		e.Reason, e.Hint = "the OpenRouter account is out of credits", "Add credits at https://openrouter.ai/settings/credits, or switch OPENROUTER_MODEL."
	case status == http.StatusTooManyRequests:
		e.Reason, e.Hint = "OpenRouter is rate limiting requests", "Wait a minute, then try again."
	case status >= 500:
		e.Reason, e.Hint = "OpenRouter or the model's provider is having trouble", "Try again in a moment."
	case status != 0:
		e.Reason, e.Hint = fmt.Sprintf("OpenRouter returned HTTP %d", status), "Try again in a moment. If it keeps happening, check the backend console."
	case errors.Is(err, context.DeadlineExceeded):
		e.Reason, e.Hint = "the AI model took too long to answer", "Try again in a moment."
	case errors.Is(err, context.Canceled):
		e.Reason, e.Hint = "the request was cancelled", ""
	}
	return e
}

// stripCodeFence removes a ```json ... ``` wrapper, which some providers add
// when they ignore response_format.
func stripCodeFence(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimPrefix(s, "json")
	s = strings.TrimSuffix(s, "```")
	return strings.TrimSpace(s)
}
