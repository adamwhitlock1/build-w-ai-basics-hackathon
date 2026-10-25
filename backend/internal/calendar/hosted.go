package calendar

import (
	"context"
	"time"
)

type rejectExternal struct{}

func (rejectExternal) Fetch(context.Context, string) ([]byte, error) { return nil, ErrLinkGone }

// NewHostedDemoFetcher serves only built-in identities; it never accesses the network.
// This is separate from the development adapter that permits external feeds.
func NewHostedDemoFetcher(now func() time.Time) *FixtureFetcher {
	return NewFixtureFetcher(rejectExternal{}, now)
}
