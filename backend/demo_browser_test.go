package main

// Opt-in, local-only browser fixture. No real AI or search provider is contacted.
import (
	"context"
	"kinlight/internal/aikit"
	"kinlight/internal/assistant"
	"kinlight/internal/calendar"
	"kinlight/internal/chat"
	"kinlight/internal/hosteddemo"
	"os"
	"os/signal"
	"path/filepath"
	"testing"
	"time"
)

func TestDemoBrowser(t *testing.T) {
	dir := os.Getenv("KINLIGHT_BROWSER_DEMO_DIR")
	if dir == "" {
		t.Skip("opt-in local browser fixture")
	}
	if err := hosteddemo.Validate(dir); err != nil {
		t.Fatal(err)
	}
	lock, err := hosteddemo.Lock(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer lock.Close()
	q, err := hosteddemo.OpenQuota(dir, 100, 50)
	if err != nil {
		t.Fatal(err)
	}
	defer q.Close()
	srv, _ := startServerOpts(t, filepath.Join(dir, hosteddemo.ContentName), func(d *deps) {
		d.DemoQuota = q
		d.WebDir = "../frontend/dist"
		d.DemoURL = "https://demo.example.test"
		d.Calendars = calendar.NewHostedDemoFetcher(time.Now)
		agent := &scriptedAgent{}
		agent.set(aikit.Verdict{}, nil, "Let's work through it together. What would you subtract from both sides first?")
		d.Chat = chat.NewService(&browserDemoAgent{scriptedAgent: agent, quota: q}, d.Store, d.Logger)
		d.Assistant = assistant.NewService(&parentAgent{reply: "Jordan has soccer today. Everyone has a daily checklist."}, nil, d.Store, d.Logger)
	})
	if err = os.WriteFile("/tmp/kinlight-judge-browser-url", []byte(srv.URL), 0600); err != nil {
		t.Fatal(err)
	}
	t.Log("Browser fixture running", srv.URL)
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	defer signal.Stop(stop)
	select {
	case <-stop:
	case <-time.After(25 * time.Minute):
	}
}

// The fake consumes the real allowance but never contacts a provider.
type browserDemoAgent struct {
	*scriptedAgent
	quota *hosteddemo.Quota
}

func (a *browserDemoAgent) Check(ctx context.Context, recent []aikit.Turn, text string, title bool) (aikit.Verdict, error) {
	if err := a.quota.Reserve(ctx, "model"); err != nil {
		return aikit.Verdict{}, err
	}
	return a.scriptedAgent.Check(ctx, recent, text, title)
}
func (a *browserDemoAgent) Reply(ctx context.Context, in aikit.ReplyInput, onChunk func(string)) (string, error) {
	onChunk("Let's look at the first step. ")
	if err := a.quota.Reserve(ctx, "model"); err != nil {
		return "", err
	}
	return a.scriptedAgent.Reply(ctx, in, onChunk)
}
