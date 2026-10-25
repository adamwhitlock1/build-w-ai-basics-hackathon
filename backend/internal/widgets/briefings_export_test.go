package widgets

import (
	"context"
	"time"

	"kinlight/internal/sources"
	"kinlight/internal/store"
)

// Like export_test.go, this file is compiled only by "go test".

// SetBriefingsClock makes b read the time from now, as SetBriefingClock
// does for a BriefingHandler.
func SetBriefingsClock(b *Briefings, now func() time.Time) { b.now = now }

// The facts and weather tests in briefing_internal_test.go build a
// BriefingHandler and call gatherFacts and weatherFact on it, from before
// that work moved into Briefings. These hand the calls on to the Briefings
// the handler is (see BriefingHandler).

func (h *BriefingHandler) gatherFacts(ctx context.Context, userID int64, profile store.MemberProfile, aud sources.Audience, loc *time.Location, now time.Time) briefingFacts {
	return (*Briefings)(h).gatherFacts(ctx, userID, profile, aud, loc, now)
}

func (h *BriefingHandler) weatherFact(ctx context.Context, userID int64, location string, aud sources.Audience, date string, now time.Time) (today, tomorrow string) {
	return (*Briefings)(h).weatherFact(ctx, userID, location, aud, date, now)
}
