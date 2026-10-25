package hosteddemo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Transport enforces before every paid HTTP attempt, including SDK/tool retries.
// Metadata GETs on the model API do not consume inference allowance.
type Transport struct {
	Base  http.RoundTripper
	Quota *Quota
	Kind  string
}

func (t *Transport) RoundTrip(r *http.Request) (*http.Response, error) {
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	if t.Kind == "model" && r.Method != http.MethodPost {
		return base.RoundTrip(r)
	}
	if t.Kind == "model" {
		data, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
		r.Body.Close()
		if err != nil {
			return nil, err
		}
		var body map[string]any
		if err = json.Unmarshal(data, &body); err != nil {
			return nil, err
		}
		// Use the provider-supported completion cap, respecting any smaller explicit limit.
		cap := float64(2048)
		for _, key := range []string{"max_tokens", "max_completion_tokens"} {
			if n, ok := body[key].(float64); ok && n > 0 && n < cap {
				cap = n
			}
			delete(body, key)
		}
		body["max_tokens"] = cap
		data, err = json.Marshal(body)
		if err != nil {
			return nil, err
		}
		r = r.Clone(r.Context())
		r.Body = io.NopCloser(bytes.NewReader(data))
		r.ContentLength = int64(len(data))
		r.GetBody = func() (io.ReadCloser, error) { return io.NopCloser(bytes.NewReader(data)), nil }
	}
	if err := OperationError(r.Context()); err != nil {
		return nil, err
	}
	if err := t.Quota.Reserve(r.Context(), t.Kind); err != nil {
		failOperation(r.Context(), err)
		return nil, fmt.Errorf("judge demo: %w", err)
	}
	return base.RoundTrip(r)
}
