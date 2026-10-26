package hosteddemo

import (
	"context"
	"errors"
	"sync"
	"time"

	"kinlight/internal/respond"
)

// Error codes the frontend reads to tell a demo pause from a provider failure.
const (
	CodeLimit = "demo_limit"
	CodeBusy  = "demo_busy"
)

// BusyError means every AI slot stayed taken for the whole wait.
type BusyError struct{}

func (*BusyError) Error() string {
	return "The demo is busy with other visitors' AI requests."
}

// Slots bounds how many AI operations run at once. An operation takes a slot
// on its first paid dispatch and keeps it until its request finishes, so
// cached reads and free requests never wait behind live AI.
type Slots struct {
	ch   chan struct{}
	wait time.Duration
}

func NewSlots(n int, wait time.Duration) *Slots {
	return &Slots{ch: make(chan struct{}, n), wait: wait}
}

type operationKey struct{}
type operation struct {
	mu       sync.Mutex
	err      error
	slots    *Slots
	acquire  sync.Once
	held     bool
	released bool
}

// WithOperation starts one request's operation. slots may be nil (no bound).
func WithOperation(ctx context.Context, slots *Slots) context.Context {
	return context.WithValue(ctx, operationKey{}, &operation{slots: slots})
}

// Release gives back the operation's slot, if it took one. A dispatch still
// waiting for a slot after Release hands it straight back.
func Release(ctx context.Context) {
	op, ok := ctx.Value(operationKey{}).(*operation)
	if !ok {
		return
	}
	op.mu.Lock()
	defer op.mu.Unlock()
	if op.held {
		<-op.slots.ch
		op.held = false
	}
	op.released = true
}

// takeSlot waits for the operation's slot on its first paid dispatch. Later
// dispatches in the same operation, including parallel ones, share it.
func takeSlot(ctx context.Context) error {
	op, ok := ctx.Value(operationKey{}).(*operation)
	if !ok || op.slots == nil {
		return nil
	}
	op.acquire.Do(func() {
		timer := time.NewTimer(op.slots.wait)
		defer timer.Stop()
		select {
		case op.slots.ch <- struct{}{}:
			op.mu.Lock()
			if op.released {
				<-op.slots.ch
			} else {
				op.held = true
			}
			op.mu.Unlock()
		case <-timer.C:
			failOperation(ctx, &BusyError{})
		case <-ctx.Done():
			failOperation(ctx, &BusyError{})
		}
	})
	return OperationError(ctx)
}

func OperationError(ctx context.Context) error {
	if op, ok := ctx.Value(operationKey{}).(*operation); ok {
		op.mu.Lock()
		defer op.mu.Unlock()
		return op.err
	}
	return nil
}
func failOperation(ctx context.Context, err error) {
	if op, ok := ctx.Value(operationKey{}).(*operation); ok {
		op.mu.Lock()
		defer op.mu.Unlock()
		if op.err == nil {
			op.err = err
		}
	}
}

// ErrorBody is how a handler reports an operation's demo pause. A busy demo
// gets its own code and hint; every other pause is the allowance.
func ErrorBody(err error, hint string) respond.ErrorBody {
	if busy := (*BusyError)(nil); errors.As(err, &busy) {
		return respond.ErrorBody{Error: err.Error(), Hint: "Try again in a moment.", Code: CodeBusy}
	}
	return respond.ErrorBody{Error: err.Error(), Hint: hint, Code: CodeLimit}
}
