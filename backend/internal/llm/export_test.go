package llm

import "time"

// SetRetryDelay shortens the rate-limit pause so tests run fast. Files named
// export_test.go are compiled only by "go test", so this never ships, yet the
// external llm_test package can still call it.
func SetRetryDelay(c *Client, d time.Duration) { c.retryDelay = d }
