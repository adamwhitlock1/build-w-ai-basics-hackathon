// Package usage records how many AI tokens each family member's use of
// Porchlight consumes, without ever slowing a request down: entries go on a
// queue, and one background goroutine writes them to the database.
package usage

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"time"

	"porchlight/internal/llm"
	"porchlight/internal/store"
)

// Unattributed is the feature recorded for an AI call whose context said
// nothing about whom it was for.
const Unattributed = "unattributed"

// writeTimeout bounds one database insert.
const writeTimeout = 5 * time.Second

// dropWarnEvery limits "queue full" warnings to one per this long.
const dropWarnEvery = time.Minute

// Store is the one database method the Recorder needs.
type Store interface {
	InsertLLMUsage(ctx context.Context, u store.LLMUsage) error
}

// Recorder is an llm.UsageRecorder that queues entries and writes them in
// the background. Start it with Run and stop it with Close. It is safe for
// concurrent use.
//
// This is Go's producer/consumer pattern: many request goroutines send on a
// buffered channel, and one goroutine receives and does the slow part.
type Recorder struct {
	store  Store
	logger *slog.Logger
	queue  chan entry
	done   chan struct{} // closed when Run returns

	// pending counts entries queued but not yet written, for Flush.
	pending sync.WaitGroup

	mu       sync.Mutex // guards closed, dropped and lastWarn
	closed   bool
	dropped  int
	lastWarn time.Time
}

// entry is one queued usage record.
type entry struct {
	usage       llm.Usage
	attribution llm.Attribution
	attributed  bool
	at          time.Time
}

// New returns a Recorder whose queue holds up to capacity entries.
func New(st Store, logger *slog.Logger, capacity int) *Recorder {
	return &Recorder{
		store:  st,
		logger: logger,
		queue:  make(chan entry, capacity),
		done:   make(chan struct{}),
	}
}

// RecordUsage queues u, with the Attribution in ctx, and returns at once.
// If the queue is full the entry is dropped (and counted) rather than
// making the caller wait. After Close it does nothing.
func (r *Recorder) RecordUsage(ctx context.Context, u llm.Usage) {
	a, ok := llm.AttributionFrom(ctx)
	e := entry{usage: u, attribution: a, attributed: ok, at: time.Now()}

	// Holding mu while sending means Close can't close the channel between
	// the closed check and the send (sending on a closed channel panics).
	// The send never blocks, so neither does holding the lock.
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.closed {
		return
	}
	r.pending.Add(1)
	select { // a select with a default case never waits
	case r.queue <- e:
	default:
		r.pending.Done()
		r.dropped++
		if time.Since(r.lastWarn) >= dropWarnEvery {
			r.logger.Warn("usage queue full; dropping entries", "dropped_so_far", r.dropped)
			r.lastWarn = time.Now()
		}
	}
}

// Run writes queued entries until Close is called and the queue is empty.
// Run it on its own goroutine: go rec.Run(ctx). ctx's values are kept but
// not its cancellation, so entries queued during shutdown are still written.
func (r *Recorder) Run(ctx context.Context) {
	defer close(r.done)
	base := context.WithoutCancel(ctx)
	// Ranging over a channel receives until it's closed and drained.
	for e := range r.queue {
		r.write(base, e)
		r.pending.Done()
	}
}

func (r *Recorder) write(ctx context.Context, e entry) {
	row := store.LLMUsage{
		CreatedAt:        e.at,
		Feature:          e.attribution.Feature,
		Step:             e.usage.Step,
		Model:            e.usage.Model,
		PromptTokens:     e.usage.PromptTokens,
		CompletionTokens: e.usage.CompletionTokens,
		TotalTokens:      e.usage.TotalTokens,
	}
	if e.attributed {
		row.FamilyID, row.MemberID, row.ViewerID = e.attribution.FamilyID, e.attribution.MemberID, e.attribution.ViewerID
	}
	if !e.attributed || row.Feature == "" {
		row.Feature = Unattributed
		r.logger.Warn("AI call had no attribution; recording it as unattributed", "step", e.usage.Step)
	}

	ctx, cancel := context.WithTimeout(ctx, writeTimeout)
	defer cancel()
	if err := r.store.InsertLLMUsage(ctx, row); err != nil {
		r.logger.Warn("couldn't record AI usage", "feature", row.Feature, "step", row.Step, "err", err)
	}
}

// Flush waits until every entry queued so far has been written (or failed).
// Tests use it to read entries back.
func (r *Recorder) Flush() { r.pending.Wait() }

// ErrCloseTimeout means Close gave up before every queued entry was written.
var ErrCloseTimeout = errors.New("usage recorder: timed out writing queued entries")

// Close stops accepting entries and waits up to timeout for Run to write the
// ones already queued. It's safe to call more than once.
func (r *Recorder) Close(timeout time.Duration) error {
	r.mu.Lock()
	if !r.closed {
		r.closed = true
		close(r.queue)
	}
	r.mu.Unlock()

	select {
	case <-r.done:
		return nil
	case <-time.After(timeout):
		return ErrCloseTimeout
	}
}
