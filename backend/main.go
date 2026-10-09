// Command porchlight runs the Porchlight API server.
//
// It reads configuration from the environment (and backend/.env), checks
// that OpenRouter will accept its requests, then serves the JSON API on
// 127.0.0.1:$PORT until it receives SIGINT or SIGTERM.
package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"porchlight/internal/llm"
	"porchlight/internal/respond"
	"porchlight/internal/sources"
	"porchlight/internal/widgets"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	slog.SetDefault(logger)

	// Keeping the real work in run, which returns an error, means deferred
	// cleanups run before we exit; os.Exit skips defers.
	if err := run(context.Background(), logger); err != nil {
		fmt.Fprintln(os.Stderr, "porchlight:", err)
		os.Exit(1)
	}
}

// config is everything the server reads from the environment.
type config struct {
	port              string
	openRouterKey     string
	openRouterModel   string
	openRouterBaseURL string
	braveKey          string
	braveBaseURL      string
}

// loadConfig reads backend/.env (if present) and then the environment.
// godotenv never overwrites variables that are already set, so values from
// the shell (even empty ones) win over the file.
func loadConfig() (config, error) {
	if err := godotenv.Load(".env"); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return config{}, fmt.Errorf("read .env: %w", err)
	}

	cfg := config{
		port:              envOr("PORT", "8080"),
		openRouterKey:     os.Getenv("OPENROUTER_API_KEY"),
		openRouterModel:   envOr("OPENROUTER_MODEL", "openai/gpt-4.1-mini"),
		openRouterBaseURL: envOr("OPENROUTER_BASE_URL", llm.DefaultBaseURL),
		braveKey:          os.Getenv("BRAVE_API_KEY"),
		braveBaseURL:      envOr("BRAVE_BASE_URL", sources.DefaultBraveBaseURL),
	}

	var missing []string
	if cfg.openRouterKey == "" {
		missing = append(missing, "OPENROUTER_API_KEY")
	}
	if cfg.braveKey == "" {
		missing = append(missing, "BRAVE_API_KEY")
	}
	if len(missing) > 0 {
		return config{}, fmt.Errorf("missing %s: copy backend/.env.example to backend/.env and fill in your keys",
			strings.Join(missing, " and "))
	}
	return cfg, nil
}

// envOr returns the environment variable key, or fallback if it is empty.
func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func run(ctx context.Context, logger *slog.Logger) error {
	// ctx is cancelled on Ctrl+C or SIGTERM; everything below watches it.
	ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
	defer stop()

	// Every request's context descends from requestsCtx, so cancelling it at
	// shutdown aborts in-flight LLM and Brave calls instead of waiting them out.
	requestsCtx, cancelRequests := context.WithCancel(context.Background())
	defer cancelRequests()

	cfg, err := loadConfig()
	if err != nil {
		return err
	}

	llmClient := llm.NewClient(cfg.openRouterKey, cfg.openRouterModel, cfg.openRouterBaseURL)
	if err := preflight(ctx, logger, llmClient); err != nil {
		return err
	}

	// The Brave key is not checked at startup: every Brave call costs credit.
	brave := sources.NewBraveClient(cfg.braveKey, cfg.braveBaseURL)
	registry := sources.NewRegistry(brave.News(), brave.Events(), brave.Web())
	widgetHandler := widgets.NewHandler(llmClient, registry, logger)

	// Go 1.22 patterns put the HTTP method in the route itself.
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", handleHealth)
	mux.HandleFunc("POST /api/widgets/from-prompt", widgetHandler.FromPrompt)

	srv := &http.Server{
		Handler:           logRequests(logger, recoverPanics(logger, mux)),
		BaseContext:       func(net.Listener) context.Context { return requestsCtx },
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		// The widget handler gives up after widgets.RequestBudget; the extra
		// time lets it write its reply before the connection is cut.
		WriteTimeout: widgets.RequestBudget + 30*time.Second,
		IdleTimeout:  time.Minute,
	}

	// Listening before serving surfaces "port already in use" right away.
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", cfg.port))
	if err != nil {
		return fmt.Errorf("listen: %w", err)
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()
	logger.Info("porchlight API listening", "addr", "http://"+ln.Addr().String(), "model", cfg.openRouterModel)

	select {
	case err := <-serveErr:
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
	}
	// Restore Go's default signal handling, so a second Ctrl+C exits at once.
	stop()

	logger.Info("shutting down")
	cancelRequests()
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Warn("graceful shutdown timed out; closing open connections", "err", err)
		_ = srv.Close() // best effort: we're exiting anyway
	}
	return nil
}

// preflight runs the OpenRouter startup checks. Problems that would break
// every request are fatal; trouble reaching OpenRouter only logs a warning,
// since the network may come back.
func preflight(ctx context.Context, logger *slog.Logger, client *llm.Client) error {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	// client.CheckModel is a method value: a func bound to client.
	checks := []func(context.Context) error{client.CheckModel, client.CheckKey}
	for _, check := range checks {
		err := check(ctx)
		switch {
		case err == nil:
		case errors.Is(err, llm.ErrUnreachable):
			logger.Warn("skipping OpenRouter startup check", "err", err)
		default:
			return err
		}
	}
	return nil
}

func handleHealth(w http.ResponseWriter, _ *http.Request) {
	respond.JSON(w, http.StatusOK, map[string]bool{"ok": true})
}
