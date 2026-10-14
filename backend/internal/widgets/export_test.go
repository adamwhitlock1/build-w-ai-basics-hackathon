package widgets

import "time"

// SetBriefingClock makes h read the time from now, so tests can move the
// clock. Files named export_test.go are compiled only by "go test".
func SetBriefingClock(h *BriefingHandler, now func() time.Time) { h.now = now }
