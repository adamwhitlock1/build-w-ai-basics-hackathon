package assistant

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"kinlight/internal/aikit"
	"kinlight/internal/auth"
	"kinlight/internal/hosteddemo"
	"kinlight/internal/respond"
	"kinlight/internal/store"
)

// maxBodyBytes caps a request body: a message of MaxMessageChars in UTF-8,
// with room for the JSON around it.
const maxBodyBytes = 4*MaxMessageChars + 1024

// Handler serves the Assistant over HTTP, under /api/me/assistant.
type Handler struct {
	svc    *Service
	logger *slog.Logger
}

// NewHandler returns a Handler for svc.
func NewHandler(svc *Service, logger *slog.Logger) *Handler {
	return &Handler{svc: svc, logger: logger}
}

type conversationJSON struct {
	ID        int64     `json:"id"`
	Kind      string    `json:"kind"`
	Title     string    `json:"title"`
	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type messageJSON struct {
	ID        int64     `json:"id"`
	Role      string    `json:"role"` // "parent" or "agent"
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"createdAt"`
}

type toolCallJSON struct {
	Tool      string    `json:"tool"`
	Detail    string    `json:"detail"`
	MessageID *int64    `json:"messageId"`
	CreatedAt time.Time `json:"createdAt"`
}

func toConversation(c store.AssistantConversation) conversationJSON {
	return conversationJSON{ID: c.ID, Kind: c.Kind, Title: c.Title, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt}
}

func toMessage(m store.AssistantMessage) messageJSON {
	return messageJSON{ID: m.ID, Role: m.Role, Text: m.Text, CreatedAt: m.CreatedAt}
}

// List serves GET /api/me/assistant: the parent's conversations, and
// whether "About the kids" is available.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	parent, ok := h.user(w, r)
	if !ok {
		return
	}
	list, err := h.svc.List(r.Context(), parent)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	out := make([]conversationJSON, len(list))
	for i, c := range list {
		out[i] = toConversation(c)
	}
	respond.JSON(w, http.StatusOK, map[string]any{"conversations": out, "kidsAvailable": h.svc.KidsAvailable()})
}

// Start serves POST /api/me/assistant with {"kind": "anything" | "kids"}.
func (h *Handler) Start(w http.ResponseWriter, r *http.Request) {
	parent, ok := h.user(w, r)
	if !ok {
		return
	}
	var req struct {
		Kind string `json:"kind"`
	}
	if !decodeJSON(w, r, &req, `Send {"kind": "anything"} or {"kind": "kids"}.`) {
		return
	}
	c, err := h.svc.Start(r.Context(), parent, req.Kind)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toConversation(c))
}

// Get serves GET /api/me/assistant/{id}.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	parent, ok := h.user(w, r)
	if !ok {
		return
	}
	id, ok := pathID(r)
	if !ok {
		h.writeError(w, r, ErrNoSuchConversation)
		return
	}
	t, err := h.svc.Get(r.Context(), parent, id)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	msgs := make([]messageJSON, len(t.Messages))
	for i, m := range t.Messages {
		msgs[i] = toMessage(m)
	}
	calls := make([]toolCallJSON, len(t.ToolCalls))
	for i, c := range t.ToolCalls {
		calls[i] = toolCallJSON{Tool: c.Tool, Detail: c.Detail, CreatedAt: c.CreatedAt}
		if c.MessageID != 0 {
			id := c.MessageID
			calls[i].MessageID = &id
		}
	}
	respond.JSON(w, http.StatusOK, map[string]any{"conversation": toConversation(t.Conversation), "messages": msgs, "toolCalls": calls})
}

// Delete serves DELETE /api/me/assistant/{id}.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	parent, ok := h.user(w, r)
	if !ok {
		return
	}
	id, ok := pathID(r)
	if !ok {
		h.writeError(w, r, ErrNoSuchConversation)
		return
	}
	if err := h.svc.Delete(r.Context(), parent, id); err != nil {
		h.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// Send serves POST /api/me/assistant/{id}/messages with {"text": "...",
// "timeZone": "America/Denver"}, answering with server-sent events exactly
// as kid chat does: message, tool…, delta…, then done or error. Anything
// refused before the message is stored is an ordinary JSON error.
func (h *Handler) Send(w http.ResponseWriter, r *http.Request) {
	parent, ok := h.user(w, r)
	if !ok {
		return
	}
	id, ok := pathID(r)
	if !ok {
		h.writeError(w, r, ErrNoSuchConversation)
		return
	}
	var req struct {
		Text     string `json:"text"`
		TimeZone string `json:"timeZone"`
	}
	if !decodeJSON(w, r, &req, `Send {"text": "your message"}.`) {
		return
	}

	rc := http.NewResponseController(w)
	streaming := false
	event := func(name string, v any) {
		b, err := json.Marshal(v)
		if err != nil {
			return
		}
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", name, b)
		_ = rc.Flush()
	}
	reply, err := h.svc.Send(r.Context(), parent, id, req.Text, locationOf(req.TimeZone), SendEvents{
		OnMessage: func(m store.AssistantMessage) {
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("X-Accel-Buffering", "no")
			w.WriteHeader(http.StatusOK)
			streaming = true
			event("message", toMessage(m))
		},
		OnTool:  func(n aikit.ToolNote) { event("tool", map[string]string{"tool": n.Tool, "detail": n.Detail}) },
		OnChunk: func(text string) { event("delta", map[string]string{"text": text}) },
	})
	switch {
	case err == nil:
		event("done", toMessage(reply))
	case hosteddemo.OperationError(r.Context()) != nil:
		paused := hosteddemo.OperationError(r.Context())
		if streaming {
			event("error", hosteddemo.ErrorBody(paused, "This answer is incomplete. Existing content is still available."))
		} else {
			respond.JSON(w, 429, hosteddemo.ErrorBody(paused, "Existing content is still available."))
		}
	case streaming:
		event("error", respond.ErrorBody{Error: "The Assistant couldn't finish its answer.", Hint: "Try again in a moment."})
	default:
		h.writeError(w, r, err)
	}
}

func (h *Handler) user(w http.ResponseWriter, r *http.Request) (store.User, bool) {
	u, ok := auth.UserFrom(r.Context())
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "Please log in.", "")
	}
	return u, ok
}

func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, context.Canceled) && r.Context().Err() != nil:
		return // the browser gave up; nobody is waiting
	case errors.Is(err, ErrNotParent):
		respond.Error(w, http.StatusForbidden, "The Assistant is for parents.", "")
	case errors.Is(err, ErrNoSuchConversation):
		respond.Error(w, http.StatusNotFound, "That conversation doesn't exist.", "")
	case errors.Is(err, ErrKidsOff):
		respond.Error(w, http.StatusConflict, `"About the kids" needs kid chat to be set up.`, "Set OPENROUTER_KIDS_API_KEY in backend/.env (see the README), then restart.")
	case errors.Is(err, ErrInvalidKind):
		respond.Error(w, http.StatusBadRequest, `Send {"kind": "anything"} or {"kind": "kids"}.`, "")
	case errors.Is(err, ErrEmptyMessage):
		respond.Error(w, http.StatusBadRequest, "Type a message first.", "")
	case errors.Is(err, ErrMessageTooLong):
		respond.Error(w, http.StatusBadRequest, "That message is too long.", fmt.Sprintf("Keep it under %d characters.", MaxMessageChars))
	default:
		h.logger.ErrorContext(r.Context(), "assistant request failed", "path", r.URL.Path, "err", err)
		respond.Error(w, http.StatusInternalServerError, "Something went wrong on our side.", "Try again in a moment.")
	}
}

func pathID(r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	return id, err == nil && id > 0
}

// locationOf returns the IANA zone tz, or UTC if it's missing or unknown.
func locationOf(tz string) *time.Location {
	if tz == "" || tz == "Local" {
		return time.UTC
	}
	if loc, err := time.LoadLocation(tz); err == nil {
		return loc
	}
	return time.UTC
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any, example string) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	err := dec.Decode(dst)
	if err == nil {
		if err = dec.Decode(&struct{}{}); errors.Is(err, io.EOF) {
			err = nil
		} else if err == nil {
			err = errors.New("more than one JSON value")
		}
	}
	var tooBig *http.MaxBytesError
	switch {
	case errors.As(err, &tooBig):
		respond.Error(w, http.StatusBadRequest, "That message is too long.", fmt.Sprintf("Keep it under %d characters.", MaxMessageChars))
		return false
	case err != nil:
		respond.Error(w, http.StatusBadRequest, example, "")
		return false
	}
	return true
}
