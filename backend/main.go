// Command porchlight runs the Porchlight API server.
//
// It reads configuration from the environment (and backend/.env), checks
// that OpenRouter will accept its requests, opens the SQLite database, then
// serves the JSON API on 127.0.0.1:$PORT until it receives SIGINT or
// SIGTERM.
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
	"path/filepath"
	"strings"
	"syscall"
	"time"
	// Imported for its side effect: the time zone database is built into the
	// binary, so members' time zones work on hosts that don't ship one.
	_ "time/tzdata"

	"github.com/alexedwards/scs/sqlite3store"
	"github.com/joho/godotenv"

	"kinlight/internal/aikit"
	"kinlight/internal/assistant"
	"kinlight/internal/auth"
	"kinlight/internal/calendar"
	"kinlight/internal/chat"
	"kinlight/internal/llm"
	"kinlight/internal/sources"
	"kinlight/internal/store"
	"kinlight/internal/usage"
	"kinlight/internal/widgets"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	slog.SetDefault(logger)

	// Keeping the real work in run, which returns an error, means deferred
	// cleanups run before we exit; os.Exit skips defers.
	if err := run(context.Background(), logger); err != nil {
		fmt.Fprintln(os.Stderr, "kinlight:", err)
		os.Exit(1)
	}
}

// config is everything the server reads from the environment.
type config struct {
	port              string
	dbPath            string
	openRouterKey     string
	openRouterModel   string
	openRouterBaseURL string
	braveKey          string
	braveBaseURL      string
	// genkitModel is the Genkit model name the Genkit briefer uses; empty
	// means "openrouter/" + openRouterModel.
	genkitModel string
	// MiniMax's Anthropic-compatible API, for the owner's own use only.
	// Without a key it isn't registered at all.
	miniMaxKey     string
	miniMaxBaseURL string
	miniMaxModel   string
	// briefer picks the morning briefing's implementation: "genkit" or
	// "legacy" (internal/llm). A temporary switch, until the agent work has
	// run on Genkit for a while.
	briefer string
	// Kid chat runs on its own OpenRouter key, restricted to
	// zero-data-retention providers. Without the key it's off.
	kidsKey         string
	kidsChatModel   string
	kidsSafetyModel string
	// kidsAllowRetention lets kid chat run on any provider, with the main
	// key if there's no kids' key, for prototyping with the demo family.
	// Development only: KINLIGHT_KIDS_ALLOW_RETENTION=1.
	kidsAllowRetention bool
	// parentChatModel is the parents' Assistant's OpenRouter model, on
	// both the main and the kids' key.
	parentChatModel string
	// devFixtures makes the server answer the demo family's fixture
	// calendar links itself (see calendar.FixtureFetcher). Local
	// development only: KINLIGHT_DEV_FIXTURES=1.
	devFixtures bool
}

// loadConfig reads backend/.env (if present) and then the environment.
// godotenv never overwrites variables that are already set, so values from
// the shell (even empty ones) win over the file.
func loadConfig() (config, error) {
	if err := godotenv.Load(".env"); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return config{}, fmt.Errorf("read .env: %w", err)
	}
	if err := checkOldNames(); err != nil {
		return config{}, err
	}

	cfg := config{
		port:               envOr("PORT", "8080"),
		dbPath:             envOr("DB_PATH", "kinlight.db"),
		openRouterKey:      os.Getenv("OPENROUTER_API_KEY"),
		openRouterModel:    envOr("OPENROUTER_MODEL", "openai/gpt-4.1-mini"),
		openRouterBaseURL:  envOr("OPENROUTER_BASE_URL", llm.DefaultBaseURL),
		braveKey:           os.Getenv("BRAVE_API_KEY"),
		braveBaseURL:       envOr("BRAVE_BASE_URL", sources.DefaultBraveBaseURL),
		genkitModel:        os.Getenv("GENKIT_MODEL"),
		miniMaxKey:         os.Getenv("MINIMAX_API_KEY"),
		miniMaxBaseURL:     envOr("MINIMAX_BASE_URL", defaultMiniMaxBaseURL),
		miniMaxModel:       envOr("MINIMAX_MODEL", aikit.DefaultMiniMaxModel),
		kidsKey:            os.Getenv("OPENROUTER_KIDS_API_KEY"),
		kidsChatModel:      envOr("KIDS_CHAT_MODEL", defaultKidsChatModel),
		kidsSafetyModel:    envOr("KIDS_SAFETY_MODEL", defaultKidsSafetyModel),
		kidsAllowRetention: os.Getenv("KINLIGHT_KIDS_ALLOW_RETENTION") == "1",
		parentChatModel:    envOr("PARENT_CHAT_MODEL", defaultParentChatModel),
		devFixtures:        os.Getenv("KINLIGHT_DEV_FIXTURES") == "1",
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

	briefer, err := parseBriefer(os.Getenv("KINLIGHT_BRIEFER"))
	if err != nil {
		return config{}, err
	}
	cfg.briefer = briefer
	return cfg, nil
}

// The OpenRouter models kid chat uses unless KIDS_CHAT_MODEL and
// KIDS_SAFETY_MODEL say otherwise. Each must have a zero-data-retention
// endpoint on OpenRouter (make ai-smoke checks) that doesn't filter out
// what kids in trouble write (make ai-eval-kids checks): the OpenAI models'
// zero-retention endpoints block self-harm messages, so they don't qualify.
const (
	defaultKidsChatModel   = "anthropic/claude-haiku-5.5"
	defaultKidsSafetyModel = "anthropic/claude-haiku-5.5"
	// defaultParentChatModel also runs on the kids' key for "About the
	// kids", so it too needs a zero-retention endpoint that doesn't block.
	defaultParentChatModel = "anthropic/claude-haiku-5.5"
)

// defaultMiniMaxBaseURL is MiniMax's Anthropic-compatible API root.
const defaultMiniMaxBaseURL = "https://api.minimax.io/anthropic"

// parseBriefer checks KINLIGHT_BRIEFER: "genkit" (also the default when
// it's empty) or "legacy".
func parseBriefer(v string) (string, error) {
	switch v {
	case "", "genkit":
		return "genkit", nil
	case "legacy":
		return "legacy", nil
	}
	return "", fmt.Errorf("KINLIGHT_BRIEFER is %q; set it to genkit (the default) or legacy in backend/.env", v)
}

// oldSettings are the settings renamed when Porchlight became Kinlight,
// each with its new name.
var oldSettings = []struct{ old, now string }{
	{"PORCHLIGHT_KIDS_ALLOW_RETENTION", "KINLIGHT_KIDS_ALLOW_RETENTION"},
	{"PORCHLIGHT_DEV_FIXTURES", "KINLIGHT_DEV_FIXTURES"},
	{"PORCHLIGHT_BRIEFER", "KINLIGHT_BRIEFER"},
	{"PORCHLIGHT_LIVE_AI", "KINLIGHT_LIVE_AI"},
	{"PORCHLIGHT_WEB_PORT", "KINLIGHT_WEB_PORT"},
	{"PORCHLIGHT_API", "KINLIGHT_API"},
}

// checkOldNames refuses settings that still use the Porchlight names, so a
// stale backend/.env fails loudly rather than being quietly ignored (or, for
// DB_PATH, quietly starting an empty database). It reads only names, never
// values, so its errors can't reveal a secret.
func checkOldNames() error {
	var renames []string
	for _, s := range oldSettings {
		if _, set := os.LookupEnv(s.old); set {
			renames = append(renames, fmt.Sprintf("%s is now %s", s.old, s.now))
		}
	}
	if strings.HasPrefix(filepath.Base(os.Getenv("DB_PATH")), "porchlight.db") {
		renames = append(renames, "DB_PATH names a porchlight.db file: rename the file to kinlight.db and set DB_PATH to match")
	}
	if len(renames) > 0 {
		return fmt.Errorf("Porchlight is now Kinlight; update backend/.env or your shell: %s", strings.Join(renames, "; "))
	}
	return nil
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

	// Genkit's reflection server (for the Dev UI) runs any flow without a
	// login. It starts only with these set: fine on a laptop, never on the
	// homelab.
	if os.Getenv("GENKIT_ENV") == "dev" || os.Getenv("GENKIT_REFLECTION_ENABLED") != "" {
		logger.Warn("Genkit's reflection server is on: it runs AI flows without a login; never enable it in a deployment",
			"GENKIT_ENV", os.Getenv("GENKIT_ENV"), "GENKIT_REFLECTION_ENABLED", os.Getenv("GENKIT_REFLECTION_ENABLED"))
	}
	kit, err := aikit.New(ctx, aikit.Config{
		OpenRouterKey:     cfg.openRouterKey,
		OpenRouterBaseURL: cfg.openRouterBaseURL,
		OpenRouterModel:   cfg.openRouterModel,
		DefaultModel:      cfg.genkitModel,
		MiniMaxKey:        cfg.miniMaxKey,
		MiniMaxBaseURL:    cfg.miniMaxBaseURL,
		MiniMaxModel:      cfg.miniMaxModel,
	})
	if err != nil {
		return err
	}

	// The Brave key is not checked at startup: every Brave call costs credit.
	brave := sources.NewBraveClient(cfg.braveKey, cfg.braveBaseURL)
	// Open-Meteo is free and needs no key.
	openMeteo := sources.NewOpenMeteoClient("", "")
	registry := sources.NewRegistry(brave.News(), brave.Events(), brave.Web(), openMeteo.Forecast())

	st, err := store.Open(ctx, cfg.dbPath)
	if err != nil {
		return err
	}
	defer st.Close()

	// Sessions live in the same database. The session store deletes expired
	// sessions in a background goroutine; StopCleanup ends it. Deferred calls
	// run last-in, first-out, so it stops before st.Close runs.
	sessionStore := sqlite3store.New(st.DB())
	defer sessionStore.StopCleanup()
	sessions := auth.NewSessionManager(sessionStore, logger)

	// Token usage of every AI call goes to the llm_usage table. Requests only
	// queue an entry; this one goroutine writes them, so recording never
	// slows a request down. It's drained after the server stops (below).
	usageRecorder := usage.New(st, logger, 256)
	go usageRecorder.Run(ctx)
	llmClient.SetUsageRecorder(usageRecorder)
	kit.SetUsageRecorder(usageRecorder)

	// Kid chat, on its own Kit (see buildKidAgent). A Service with no
	// agent is kid chat turned off.
	chatService := chat.NewService(nil, st, logger)
	kids, err := buildKidAgent(ctx, logger, cfg)
	if err != nil {
		return err
	}
	if kids != nil {
		kids.kit.SetUsageRecorder(usageRecorder)
		chatService = chat.NewService(kids.agent, st, logger)
	}

	// widgets.Briefer is an interface, so either implementation fits.
	var briefer widgets.Briefer = aikit.NewBriefer(kit)
	if cfg.briefer == "legacy" {
		briefer = llmClient
	}

	// The Today card fetches calendars through this. With dev fixtures on,
	// links on calendar.FixtureHost are answered in-process and every other
	// link still goes to the real, checked fetcher.
	var calendars widgets.CalendarFetcher = calendar.NewFetcher()
	if cfg.devFixtures {
		calendars = calendar.NewFixtureFetcher(calendar.NewFetcher(), time.Now)
		logger.Warn("dev fixture calendars are on; never enable this in a deployment", "host", calendar.FixtureHost)
	}

	// The kid agent's tools look things up on the same data and sources as
	// the kid's cards: their calendar (through the same fetcher), their
	// remembered picks, the forecast, and kid-safe web search.
	lookups := &chat.ToolSources{
		Profiles: st,
		Agenda:   widgets.NewAgenda(calendars, st, logger),
		Results:  st,
		Forecast: openMeteo.Forecast(),
		Web:      brave.Web(),
	}
	chatService.SetTools(lookups)

	// The parents' Assistant: "Ask anything" on the main Kit, "About the
	// kids" on the kids' Kit (no-retention), which only exists when kid chat
	// is on. Its tools reuse kid chat's lookups for any member, search the
	// web unfiltered, and read the kids' chats through chatService.
	parentModel := "openrouter/" + cfg.parentChatModel
	var kidsParentAgent assistant.Agent
	if kids != nil {
		kidsParentAgent = aikit.NewParentAgent(kids.kit, parentModel)
	}
	assistantService := assistant.NewService(aikit.NewParentAgent(kit, parentModel), kidsParentAgent, st, logger)
	assistantService.SetTools(&assistant.ToolSources{Members: st, Lookups: lookups, Web: brave.Web(), Chats: chatService})
	logger.Info("parents' Assistant is on", "model", cfg.parentChatModel, "about_the_kids", kids != nil)

	srv := &http.Server{
		Handler: routes(deps{
			Logger:    logger,
			Store:     st,
			Sessions:  sessions,
			Planner:   llmClient,
			Curator:   llmClient,
			Briefer:   briefer,
			Sources:   registry,
			Calendars: calendars,
			Chat:      chatService,
			Assistant: assistantService,
		}),
		BaseContext:       func(net.Listener) context.Context { return requestsCtx },
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       15 * time.Second,
		// The widget handlers give up after widgets.RequestBudget; the extra
		// time lets them write their reply before the connection is cut.
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
	logger.Info("kinlight API listening", "addr", "http://"+ln.Addr().String(), "model", cfg.openRouterModel,
		"briefer", cfg.briefer, "genkit_model", kit.DefaultModel(), "db", cfg.dbPath)

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
	// No request can queue usage any more: write what's left.
	if err := usageRecorder.Close(5 * time.Second); err != nil {
		logger.Warn("some AI usage entries weren't recorded", "err", err)
	}
	return nil
}

// kidAgent is kid chat's agent and the Kit it runs on.
type kidAgent struct {
	kit   *aikit.Kit
	agent *aikit.KidAgent
}

// buildKidAgent builds kid chat's agent on its own Kit, and logs which way
// kid chat runs:
//   - on: the kids' key, every call restricted to zero-data-retention
//     providers that don't collect data;
//   - prototyping (KINLIGHT_KIDS_ALLOW_RETENTION=1): any provider, with a
//     warning;
//   - off: no kids' key. It returns nil, and kids see "Chat isn't set up".
func buildKidAgent(ctx context.Context, logger *slog.Logger, cfg config) (*kidAgent, error) {
	key, noRetention := cfg.kidsKey, true
	switch {
	case cfg.kidsAllowRetention:
		if key == "" {
			key = cfg.openRouterKey
		}
		noRetention = false
		logger.Warn("kid chat isn't restricted to no-retention providers (KINLIGHT_KIDS_ALLOW_RETENTION=1); for prototyping only, never once kids use it")
	case key == "":
		logger.Info("kid chat is off: set OPENROUTER_KIDS_API_KEY to turn it on")
		return nil, nil
	}
	kit, err := aikit.New(ctx, aikit.Config{
		OpenRouterKey:     key,
		OpenRouterBaseURL: cfg.openRouterBaseURL,
		OpenRouterModel:   cfg.kidsChatModel,
		NoRetention:       noRetention,
	})
	if err != nil {
		return nil, fmt.Errorf("kid chat: %w", err)
	}
	agent := aikit.NewKidAgent(kit, "openrouter/"+cfg.kidsChatModel, "openrouter/"+cfg.kidsSafetyModel)
	logger.Info("kid chat is on", "no_retention", noRetention, "chat_model", cfg.kidsChatModel, "safety_model", cfg.kidsSafetyModel)
	return &kidAgent{kit: kit, agent: agent}, nil
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
