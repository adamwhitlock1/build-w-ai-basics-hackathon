package aikit

import (
	"context"
	"errors"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/firebase/genkit/go/core/status"
	"github.com/openai/openai-go"

	"porchlight/internal/llm"
)

// upstream turns an error from a Genkit call into an *llm.UpstreamError, so
// people see the same reasons and hints as with the legacy client. The
// plugins wrap the SDK's own error, which carries the HTTP status, so that
// comes first; Genkit's status codes are only a fallback.
func upstream(err error) error {
	var (
		ue     *llm.UpstreamError
		oaiErr *openai.Error
		antErr *anthropic.Error
	)
	switch {
	case errors.As(err, &ue):
		return err
	case errors.As(err, &oaiErr):
		return llm.UpstreamFor(oaiErr.StatusCode, err)
	case errors.As(err, &antErr):
		return llm.UpstreamFor(antErr.StatusCode, err)
	case errors.Is(err, context.DeadlineExceeded), errors.Is(err, context.Canceled):
		return llm.UpstreamFor(0, err)
	}
	if name, ok := status.Classified(err); ok {
		return llm.UpstreamFor(name.HTTPCode(), err)
	}
	return llm.UpstreamFor(0, err)
}
