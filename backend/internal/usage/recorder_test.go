package usage_test

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"
	"time"

	"porchlight/internal/llm"
	"porchlight/internal/store"
	"porchlight/internal/usage"
)

// fakeStore records inserts. If block is set, every insert waits for it to
// be closed; if err is set, every insert fails.
type fakeStore struct {
	mu    sync.Mutex
	rows  []store.LLMUsage
	block chan struct{}
	err   error
}

func (f *fakeStore) InsertLLMUsage(ctx context.Context, u store.LLMUsage) error {
	if f.block != nil {
		<-f.block
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return f.err
	}
	f.rows = append(f.rows, u)
	return nil
}

func (f *fakeStore) got() []store.LLMUsage {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]store.LLMUsage(nil), f.rows...)
}

var discard = slog.New(slog.DiscardHandler)

func start(t *testing.T, st usage.Store, capacity int) *usage.Recorder {
	t.Helper()
	rec := usage.New(st, discard, capacity)
	go rec.Run(context.Background())
	t.Cleanup(func() { rec.Close(5 * time.Second) })
	return rec
}

var tokens = llm.Usage{Model: "test/model", Step: "widget_spec", PromptTokens: 10, CompletionTokens: 2, TotalTokens: 12}

func TestRecorderWritesAttributedEntries(t *testing.T) {
	st := &fakeStore{}
	rec := start(t, st, 8)

	ctx := llm.WithAttribution(context.Background(), llm.Attribution{FamilyID: 1, MemberID: 3, ViewerID: 2, Feature: "describe_widget"})
	rec.RecordUsage(ctx, tokens)
	rec.RecordUsage(context.Background(), tokens) // no attribution
	rec.Flush()

	rows := st.got()
	if len(rows) != 2 {
		t.Fatalf("wrote %d rows, want 2", len(rows))
	}
	if r := rows[0]; r.FamilyID != 1 || r.MemberID != 3 || r.ViewerID != 2 || r.Feature != "describe_widget" || r.Step != "widget_spec" || r.TotalTokens != 12 || r.CreatedAt.IsZero() {
		t.Errorf("attributed row = %+v", r)
	}
	if r := rows[1]; r.MemberID != 0 || r.FamilyID != 0 || r.Feature != usage.Unattributed {
		t.Errorf("unattributed row = %+v, want no member and feature %q", r, usage.Unattributed)
	}
}

func TestRecorderNeverBlocksWhenFull(t *testing.T) {
	st := &fakeStore{block: make(chan struct{})}
	rec := start(t, st, 2)
	defer close(st.block) // let the writer finish before Close in Cleanup

	// The writer takes one entry and blocks in the store; two more fill the
	// queue; the rest must be dropped, not wait.
	began := time.Now()
	for range 50 {
		rec.RecordUsage(context.Background(), tokens)
	}
	if elapsed := time.Since(began); elapsed > time.Second {
		t.Errorf("50 RecordUsage calls took %v with the store blocked; they must not wait", elapsed)
	}
}

func TestRecorderCloseDrainsTheQueue(t *testing.T) {
	st := &fakeStore{}
	rec := usage.New(st, discard, 16)
	for range 5 {
		rec.RecordUsage(context.Background(), tokens) // queued before Run even starts
	}
	go rec.Run(context.Background())
	if err := rec.Close(5 * time.Second); err != nil {
		t.Fatal(err)
	}
	if n := len(st.got()); n != 5 {
		t.Errorf("wrote %d rows after Close, want all 5", n)
	}
	rec.RecordUsage(context.Background(), tokens) // after Close: ignored, no panic
	if err := rec.Close(time.Second); err != nil {
		t.Errorf("second Close: %v", err)
	}
}

func TestRecorderKeepsGoingAfterAnInsertError(t *testing.T) {
	st := &fakeStore{err: errors.New("database is locked")}
	rec := start(t, st, 8)
	rec.RecordUsage(context.Background(), tokens)
	rec.Flush() // the failure is logged, not fatal

	st.mu.Lock()
	st.err = nil
	st.mu.Unlock()
	rec.RecordUsage(context.Background(), tokens)
	rec.Flush()
	if n := len(st.got()); n != 1 {
		t.Errorf("wrote %d rows, want 1 (the one after the error)", n)
	}
}
