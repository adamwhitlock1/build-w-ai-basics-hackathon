package aikit_test

import (
	"context"
	"errors"
	"kinlight/internal/aikit"
	"kinlight/internal/hosteddemo"
	"kinlight/internal/llm"
	"kinlight/internal/sources"
	"net/http"
	"testing"
	"time"
)

func TestDemoQuotaAcrossLegacyAndGenkit(t *testing.T) {
	q, err := hosteddemo.OpenQuota(t.TempDir(), 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer q.Close()
	fake, url := newFakeOpenRouter(t, completion(`{"briefing":"Sunny today."}`, 10, 5), completion(`{"briefing":"Sunny today."}`, 10, 5))
	transport := &hosteddemo.Transport{Quota: q, Kind: "model"}
	legacy := llm.NewClient("test-key", "test/model", url)
	legacy.WrapTransport(func(base http.RoundTripper) http.RoundTripper {
		return &hosteddemo.Transport{Base: base, Quota: q, Kind: "model"}
	})
	kit, err := aikit.New(context.Background(), aikit.Config{OpenRouterKey: "test-key", OpenRouterModel: "test/model", OpenRouterBaseURL: url, HTTPClient: &http.Client{Transport: transport}})
	if err != nil {
		t.Fatal(err)
	}
	for _, briefer := range []interface {
		WriteBriefing(context.Context, llm.BriefingFacts, sources.Audience) (string, error)
	}{legacy, aikit.NewBriefer(kit)} {
		if _, err = briefer.WriteBriefing(hosteddemo.WithOperation(context.Background(), nil), testFacts(), sources.Everyone); err != nil {
			t.Fatal(err)
		}
	}
	ctx := hosteddemo.WithOperation(context.Background(), nil)
	_, err = aikit.NewBriefer(kit).WriteBriefing(ctx, testFacts(), sources.Everyone)
	var limit *hosteddemo.LimitError
	if !errors.As(err, &limit) {
		t.Fatalf("lost quota error: %v", err)
	}
	if len(fake.requests()) != 2 {
		t.Fatal("exhausted request reached provider")
	}
	// The same Kit also powers kid safety; exhaustion must not invoke its model.
	_, err = aikit.NewKidAgent(kit, "openrouter/test/model", "openrouter/test/model").Check(hosteddemo.WithOperation(context.Background(), nil), nil, "help with math", true)
	if !errors.As(err, &limit) {
		t.Fatalf("safety quota error: %v", err)
	}
	if len(fake.requests()) != 2 {
		t.Fatal("safety call bypassed quota")
	}
}

func TestDemoQuotaStopsRetryAndChatDispatch(t *testing.T) {
	q, err := hosteddemo.OpenQuota(t.TempDir(), 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer q.Close()
	fake, url := newFakeOpenRouter(t, apiFailure(429))
	kit, err := aikit.New(context.Background(), aikit.Config{OpenRouterKey: "test-key", OpenRouterModel: "test/model", OpenRouterBaseURL: url, HTTPClient: &http.Client{Transport: &hosteddemo.Transport{Quota: q, Kind: "model"}}})
	if err != nil {
		t.Fatal(err)
	}
	aikit.SetRetryDelay(kit, time.Millisecond)
	_, err = aikit.NewBriefer(kit).WriteBriefing(hosteddemo.WithOperation(context.Background(), nil), testFacts(), sources.Everyone)
	if err == nil || len(fake.requests()) != 1 {
		t.Fatalf("retry bypass: %v, %d requests", err, len(fake.requests()))
	}
	for name, call := range map[string]func(context.Context) error{
		"kid generation": func(ctx context.Context) error {
			_, err := aikit.NewKidAgent(kit, "", "").Reply(ctx, kidAsks("help"), func(string) {})
			return err
		},
		"parent generation": func(ctx context.Context) error {
			_, err := aikit.NewParentAgent(kit, "").Reply(ctx, asks(aikit.KindAnything, "help", nil), func(string) {})
			return err
		},
	} {
		t.Run(name, func(t *testing.T) {
			ctx := hosteddemo.WithOperation(context.Background(), nil)
			if call(ctx) == nil || hosteddemo.OperationError(ctx) == nil {
				t.Fatal("quota failure lost")
			}
			if len(fake.requests()) != 1 {
				t.Fatal("exhausted request dispatched")
			}
		})
	}
}
