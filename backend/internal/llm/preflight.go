package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
)

// ErrUnreachable marks a startup check that couldn't reach OpenRouter or got
// an unexpected answer. Such failures may be temporary, so the server can
// warn and start anyway.
var ErrUnreachable = errors.New("couldn't complete the OpenRouter check")

// CheckModel confirms the configured model exists on OpenRouter and supports
// structured outputs, which Porchlight relies on.
func (c *Client) CheckModel(ctx context.Context) error {
	var body struct {
		Data []struct {
			ID                  string   `json:"id"`
			SupportedParameters []string `json:"supported_parameters"`
		} `json:"data"`
	}
	// The model list is public, so this request needs no key.
	status, err := c.getJSON(ctx, "/models", false, &body)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("%w: GET /models returned HTTP %d", ErrUnreachable, status)
	}

	for _, m := range body.Data {
		if m.ID != c.model {
			continue
		}
		if !slices.Contains(m.SupportedParameters, "structured_outputs") {
			return fmt.Errorf("model %q doesn't support structured outputs, which Porchlight needs; "+
				"set OPENROUTER_MODEL in backend/.env to one that does, such as openai/gpt-4.1-mini or openai/gpt-4o-mini", c.model)
		}
		return nil
	}
	return fmt.Errorf("model %q isn't available on OpenRouter; set OPENROUTER_MODEL in backend/.env "+
		"to a model id from https://openrouter.ai/models, such as openai/gpt-4.1-mini", c.model)
}

// CheckKey confirms OpenRouter accepts the API key. It doesn't spend credit.
func (c *Client) CheckKey(ctx context.Context) error {
	status, err := c.getJSON(ctx, "/key", true, nil)
	if err != nil {
		return err
	}
	switch status {
	case http.StatusOK:
		return nil
	case http.StatusUnauthorized:
		return errors.New("OpenRouter rejected your key; check OPENROUTER_API_KEY in backend/.env (keys: https://openrouter.ai/settings/keys)")
	default:
		return fmt.Errorf("%w: GET /key returned HTTP %d", ErrUnreachable, status)
	}
}

// getJSON GETs path under the base URL and, on HTTP 200, decodes the body into
// out (if out is non-nil). Network and decode failures wrap ErrUnreachable.
func (c *Client) getJSON(ctx context.Context, path string, auth bool, out any) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return 0, fmt.Errorf("build request: %w", err)
	}
	if auth {
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("%w: %w", ErrUnreachable, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusOK && out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return 0, fmt.Errorf("%w: decode %s: %w", ErrUnreachable, path, err)
		}
	}
	return resp.StatusCode, nil
}
