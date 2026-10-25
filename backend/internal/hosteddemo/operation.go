package hosteddemo

import (
	"context"
	"sync"
)

type operationKey struct{}
type operation struct {
	mu  sync.Mutex
	err error
}

func WithOperation(ctx context.Context) context.Context {
	return context.WithValue(ctx, operationKey{}, &operation{})
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
