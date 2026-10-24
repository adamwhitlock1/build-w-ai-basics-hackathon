package aikit

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/core/status"
	"github.com/firebase/genkit/go/genkit"

	"porchlight/internal/llm"
)

// callTimeout bounds a single model call, as in internal/llm.
const callTimeout = 40 * time.Second

// structuredTemperature is the sampling temperature for structured calls,
// as in internal/llm.
const structuredTemperature = 0.2

// callConfig is the model config for one call: temperature, plus, on a
// no-retention Kit, OpenRouter routing that allows only zero-data-retention
// endpoints of providers that don't collect data. A map is the one config
// shape every model plugin accepts. Every Kit call builds its config here,
// so none can leave the routing out.
func (k *Kit) callConfig(temperature float64) map[string]any {
	cfg := map[string]any{"temperature": temperature}
	if k.noRetention {
		cfg["provider"] = map[string]any{"zdr": true, "dataCollection": "deny"}
	}
	return cfg
}

// structuredCall is one request for JSON matching T's schema.
type structuredCall struct {
	step     string // usage step and log label, e.g. "morning_briefing"
	model    string // Genkit model name; "" means the Kit's default
	system   string
	messages []*ai.Message
}

func (c structuredCall) modelName(k *Kit) string {
	if c.model != "" {
		return c.model
	}
	return k.defaultModel
}

// generateJSON asks for a T, decodes the reply and runs check on it. If the
// reply doesn't parse or check rejects it, it retries once, showing the
// model its reply and what was wrong, exactly as internal/llm's callJSON
// does. Failed calls are returned as *llm.UpstreamError (a rate limit has
// already been retried once).
//
// It is a generic function rather than a method because Go methods can't
// declare their own type parameters.
func generateJSON[T any](ctx context.Context, k *Kit, call structuredCall, check func(*T) error) (T, error) {
	var zero T
	var lastErr error
	for attempt := 1; attempt <= 2; attempt++ {
		resp, err := k.generate(ctx, call.step, structuredTemperature,
			ai.WithModelName(call.modelName(k)),
			ai.WithSystem(call.system),
			ai.WithMessages(call.messages...),
			ai.WithOutputType(zero),
		)
		if err != nil {
			return zero, err
		}
		switch resp.FinishReason {
		case ai.FinishReasonLength:
			return zero, &llm.UpstreamError{Reason: "the AI model's answer was cut off", Hint: "Try again in a moment.", Err: errors.New("finish_reason=length")}
		case ai.FinishReasonBlocked:
			return zero, &llm.UpstreamError{Reason: "the AI model declined to answer", Hint: "Try describing the widget differently.", Err: errors.New("finish_reason=blocked")}
		}

		content := resp.Text()
		var out T
		if err := resp.Output(&out); err != nil {
			lastErr = &llm.UpstreamError{Reason: "the AI model sent back an answer we couldn't read", Hint: "Try again in a moment.", Err: err}
		} else if err := check(&out); err != nil {
			lastErr = err
		} else {
			return out, nil
		}

		slog.WarnContext(ctx, "llm reply rejected", "schema", call.step, "attempt", attempt, "err", lastErr)
		// Show the model its bad reply so it can correct it. APIs reject an
		// assistant message without content, so an empty reply is left out.
		if strings.TrimSpace(content) != "" {
			call.messages = append(call.messages, ai.NewModelTextMessage(content))
		}
		call.messages = append(call.messages, ai.NewUserTextMessage(
			fmt.Sprintf("That reply was invalid (%v). Reply again with only JSON that matches the schema.", lastErr)))
	}
	return zero, lastErr
}

// generate runs one Generate call with usage tracked under step and the
// Kit's call config at temperature. A rate limit (HTTP 429) is retried once
// after a short pause.
func (k *Kit) generate(ctx context.Context, step string, temperature float64, opts ...ai.GenerateOption) (*ai.ModelResponse, error) {
	opts = append(opts, ai.WithConfig(k.callConfig(temperature)))
	resp, err := k.generateOnce(ctx, step, opts...)
	var ue *llm.UpstreamError
	if !errors.As(err, &ue) || ue.Status != http.StatusTooManyRequests {
		return resp, err
	}
	slog.WarnContext(ctx, "openrouter rate limited; retrying once", "schema", step)
	// select lets the pause end early if the request is cancelled.
	select {
	case <-time.After(k.retryDelay):
	case <-ctx.Done():
		return nil, err
	}
	return k.generateOnce(ctx, step, opts...)
}

// generateOnce runs a single Generate call. A reply that doesn't match the
// output schema comes back as a response, not an error: the model did
// answer, and the caller decides whether to ask again.
func (k *Kit) generateOnce(ctx context.Context, step string, opts ...ai.GenerateOption) (*ai.ModelResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, callTimeout)
	defer cancel()

	resp, err := genkit.Generate(ctx, k.g, append(opts, k.TrackUsage(step))...)
	if err != nil {
		if errors.Is(err, status.ErrInvalidOutput) && resp != nil && resp.Message != nil {
			return resp, nil
		}
		return nil, upstream(err)
	}
	return resp, nil
}
