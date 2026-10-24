// Package aikit is Porchlight's one way into Genkit for Go. It builds the
// Genkit instance with the model plugins Porchlight uses, knows which models
// exist, records every model answer's token usage, and turns Genkit's errors
// into llm.UpstreamError, so the rest of the backend never imports a Genkit
// plugin.
package aikit

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	anthropicopt "github.com/anthropics/anthropic-sdk-go/option"
	"github.com/firebase/genkit/go/core/api"
	"github.com/firebase/genkit/go/genkit"
	"github.com/firebase/genkit/go/plugins/anthropic"
	"github.com/firebase/genkit/go/plugins/compat_oai/openrouter"
	"github.com/openai/openai-go/option"

	"porchlight/internal/llm"
)

// DefaultMiniMaxModel is the MiniMax model registered when MINIMAX_MODEL
// isn't set.
const DefaultMiniMaxModel = "MiniMax-M3.1-Flash"

// Config is what New needs to build a Kit.
type Config struct {
	// OpenRouterKey is required.
	OpenRouterKey string
	// OpenRouterBaseURL overrides OpenRouter's API root; tests point it at
	// a fake server. Empty means llm.DefaultBaseURL.
	OpenRouterBaseURL string
	// OpenRouterModel is the OpenRouter model id, e.g. "openai/gpt-4.1-mini".
	OpenRouterModel string
	// DefaultModel is the Genkit model name calls use unless they pick
	// another. Empty means "openrouter/" + OpenRouterModel.
	DefaultModel string

	// MiniMaxKey turns on MiniMax's Anthropic-compatible API. Empty leaves
	// MiniMax out entirely.
	MiniMaxKey     string
	MiniMaxBaseURL string // e.g. "https://api.minimax.io/anthropic"
	MiniMaxModel   string // empty means DefaultMiniMaxModel
}

// Kit wraps a Genkit instance with Porchlight's choices: the default model,
// where usage goes, and how failed calls are retried. It is safe for
// concurrent use once set up.
type Kit struct {
	g            *genkit.Genkit
	defaultModel string
	miniMaxModel string            // "anthropic/<id>", or "" without a MiniMax key
	usage        llm.UsageRecorder // nil: usage isn't recorded
	retryDelay   time.Duration     // pause before retrying a 429
}

// New builds a Kit from cfg. Missing or broken configuration is returned as
// an error: New never panics, though genkit.Init and its plugins do.
func New(ctx context.Context, cfg Config) (kit *Kit, err error) {
	if cfg.OpenRouterKey == "" {
		return nil, errors.New("aikit: an OpenRouter API key is required")
	}
	if cfg.OpenRouterModel == "" && cfg.DefaultModel == "" {
		return nil, errors.New("aikit: an OpenRouter model or a default model is required")
	}
	baseURL := cfg.OpenRouterBaseURL
	if baseURL == "" {
		baseURL = llm.DefaultBaseURL
	}
	defaultModel := cfg.DefaultModel
	if defaultModel == "" {
		defaultModel = "openrouter/" + cfg.OpenRouterModel
	}

	// The SDKs' own retries are off (WithMaxRetries(0)): the Kit retries
	// exactly as internal/llm does, so latency and cost don't change.
	plugins := []api.Plugin{&openrouter.OpenRouter{
		APIKey:  cfg.OpenRouterKey,
		SiteURL: "http://localhost:5174",
		AppName: "Porchlight",
		Opts:    []option.RequestOption{option.WithBaseURL(strings.TrimRight(baseURL, "/") + "/"), option.WithMaxRetries(0)},
	}}
	miniMaxModel := ""
	if cfg.MiniMaxKey != "" {
		// Registered only with a key: the anthropic plugin panics without one.
		plugins = append(plugins, &anthropic.Anthropic{
			APIKey:  cfg.MiniMaxKey,
			BaseURL: cfg.MiniMaxBaseURL,
			Opts:    []anthropicopt.RequestOption{anthropicopt.WithMaxRetries(0)},
		})
		id := cfg.MiniMaxModel
		if id == "" {
			id = DefaultMiniMaxModel
		}
		miniMaxModel = "anthropic/" + id
	}

	// genkit.Init reports bad settings (such as an invalid GENKIT_ENV) by
	// panicking. recover turns that back into an error; it works only in a
	// deferred function.
	defer func() {
		if r := recover(); r != nil {
			kit, err = nil, fmt.Errorf("aikit: start Genkit: %v", r)
		}
	}()
	g := genkit.Init(ctx, genkit.WithPlugins(plugins...), genkit.WithDefaultModel(defaultModel))

	k := FromGenkit(g, defaultModel)
	k.miniMaxModel = miniMaxModel
	return k, nil
}

// FromGenkit wraps a Genkit instance someone else built, such as a test kit
// with a fake model (see the aitest package), using defaultModel unless a
// call picks another.
func FromGenkit(g *genkit.Genkit, defaultModel string) *Kit {
	return &Kit{g: g, defaultModel: defaultModel, retryDelay: time.Second}
}

// Genkit returns the Genkit instance, for code that defines flows and tools
// on it.
func (k *Kit) Genkit() *genkit.Genkit { return k.g }

// DefaultModel returns the Genkit name of the model calls use by default,
// e.g. "openrouter/openai/gpt-4.1-mini".
func (k *Kit) DefaultModel() string { return k.defaultModel }

// MiniMaxModel returns the Genkit name of the MiniMax model, or "" when no
// MiniMax key was configured.
func (k *Kit) MiniMaxModel() string { return k.miniMaxModel }

// SetUsageRecorder makes k report every model answer's usage to r. Call it
// before k is used; a nil r (the default) records nothing.
func (k *Kit) SetUsageRecorder(r llm.UsageRecorder) { k.usage = r }
