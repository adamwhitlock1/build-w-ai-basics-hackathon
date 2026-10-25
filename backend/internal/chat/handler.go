package chat

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
	"kinlight/internal/respond"
	"kinlight/internal/store"
)

// maxBodyBytes caps a request body: a message of MaxMessageChars, in UTF-8,
// with room for the JSON around it.
const maxBodyBytes = 4*MaxMessageChars + 1024

// Handler serves kid chat over HTTP. Its handlers only decode the request,
// call the Service, and turn the result or error into a response.
type Handler struct {
	svc    *Service
	logger *slog.Logger
}

// NewHandler returns a Handler for svc.
func NewHandler(svc *Service, logger *slog.Logger) *Handler {
	return &Handler{svc: svc, logger: logger}
}

// JSON shapes. Kids never see flags or flag counts.

type conversationJSON struct {
	ID              int64     `json:"id"`
	Mode            string    `json:"mode"`
	Title           string    `json:"title"`
	CreatedAt       time.Time `json:"createdAt"`
	UpdatedAt       time.Time `json:"updatedAt"`
	UnreviewedFlags *int      `json:"unreviewedFlags,omitempty"` // parents only
}

type messageJSON struct {
	ID        int64     `json:"id"`
	Role      string    `json:"role"` // "kid" or "agent"
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"createdAt"`
}

type flagJSON struct {
	ID                int64      `json:"id"`
	MessageID         int64      `json:"messageId"`
	ConversationID    int64      `json:"conversationId"`
	MemberID          int64      `json:"memberId"`
	Category          string     `json:"category"`
	Severity          string     `json:"severity"`
	Crisis            bool       `json:"crisis"`
	Reason            string     `json:"reason"`
	CreatedAt         time.Time  `json:"createdAt"`
	ReviewedAt        *time.Time `json:"reviewedAt"`
	ReviewedBy        *int64     `json:"reviewedBy"`
	MessageText       string     `json:"messageText,omitempty"`
	ConversationTitle string     `json:"conversationTitle,omitempty"`
}

// toolCallJSON is one tool the agent used. Kids and parents both see these
// notes, search queries included.
type toolCallJSON struct {
	Tool      string    `json:"tool"`
	Detail    string    `json:"detail"`
	MessageID *int64    `json:"messageId"` // the reply it informed; null if that reply failed
	CreatedAt time.Time `json:"createdAt"`
}

type transcriptJSON struct {
	Conversation conversationJSON `json:"conversation"`
	Messages     []messageJSON    `json:"messages"`
	ToolCalls    []toolCallJSON   `json:"toolCalls"`
	Flags        []flagJSON       `json:"flags,omitempty"` // parents only
}

func toConversation(c store.Conversation, forParent bool) conversationJSON {
	out := conversationJSON{ID: c.ID, Mode: c.Mode, Title: c.Title, CreatedAt: c.CreatedAt, UpdatedAt: c.UpdatedAt}
	if forParent {
		n := c.UnreviewedFlags
		out.UnreviewedFlags = &n
	}
	return out
}

func toConversations(cs []store.Conversation, forParent bool) []conversationJSON {
	out := make([]conversationJSON, len(cs))
	for i, c := range cs {
		out[i] = toConversation(c, forParent)
	}
	return out
}

func toMessage(m store.ChatMessage) messageJSON {
	return messageJSON{ID: m.ID, Role: m.Role, Text: m.Text, CreatedAt: m.CreatedAt}
}

func toFlag(f store.ChatFlag) flagJSON {
	out := flagJSON{
		ID: f.ID, MessageID: f.MessageID, ConversationID: f.ConversationID, MemberID: f.MemberID,
		Category: f.Category, Severity: f.Severity, Crisis: f.Crisis, Reason: f.Reason, CreatedAt: f.CreatedAt,
		MessageText: f.MessageText, ConversationTitle: f.ConversationTitle,
	}
	if !f.ReviewedAt.IsZero() {
		at, by := f.ReviewedAt, f.ReviewedBy
		out.ReviewedAt, out.ReviewedBy = &at, &by
	}
	return out
}

func toFlags(fs []store.ChatFlag) []flagJSON {
	out := make([]flagJSON, len(fs))
	for i, f := range fs {
		out[i] = toFlag(f)
	}
	return out
}

func toTranscript(t Transcript, forParent bool) transcriptJSON {
	out := transcriptJSON{
		Conversation: toConversation(t.Conversation, forParent),
		Messages:     make([]messageJSON, len(t.Messages)),
		ToolCalls:    make([]toolCallJSON, len(t.ToolCalls)),
	}
	for i, m := range t.Messages {
		out.Messages[i] = toMessage(m)
	}
	for i, c := range t.ToolCalls {
		out.ToolCalls[i] = toolCallJSON{Tool: c.Tool, Detail: c.Detail, CreatedAt: c.CreatedAt}
		if c.MessageID != 0 {
			id := c.MessageID
			out.ToolCalls[i].MessageID = &id
		}
	}
	if forParent {
		out.Flags = toFlags(t.Flags)
	}
	return out
}

// --- Kids: /api/me/chats ---

// List serves GET /api/me/chats: whether chat is on, and the kid's
// conversations.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	kid, ok := auth.UserFrom(r.Context())
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "Please log in.", "")
		return
	}
	list, err := h.svc.List(r.Context(), kid)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{"available": h.svc.Available(), "conversations": toConversations(list, false)})
}

// Start serves POST /api/me/chats with {"mode": "homework" | "explore"}.
func (h *Handler) Start(w http.ResponseWriter, r *http.Request) {
	kid, ok := auth.UserFrom(r.Context())
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "Please log in.", "")
		return
	}
	var req struct {
		Mode string `json:"mode"`
	}
	if !decodeJSON(w, r, &req, `Send {"mode": "homework"} or {"mode": "explore"}.`) {
		return
	}
	c, err := h.svc.Start(r.Context(), kid, req.Mode)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	respond.JSON(w, http.StatusCreated, toConversation(c, false))
}

// Get serves GET /api/me/chats/{id}: one conversation with its messages.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	kid, ok := auth.UserFrom(r.Context())
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "Please log in.", "")
		return
	}
	id, ok := pathID(r, "id")
	if !ok {
		h.writeError(w, r, ErrNoSuchChat)
		return
	}
	t, err := h.svc.Get(r.Context(), kid, id)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	respond.JSON(w, http.StatusOK, toTranscript(t, false))
}

// Send serves POST /api/me/chats/{id}/messages with {"text": "...",
// "timeZone": "America/Denver"} (an IANA zone; missing or unknown means
// UTC). The answer is a stream of server-sent events:
//
//	event: message   the kid's stored message
//	event: tool      {"tool": "search", "detail": "..."}: a tool the agent used
//	event: delta     {"text": "..."}, a piece of the reply (repeated)
//	event: done      the agent's stored message
//	event: error     {"error": "...", "hint": "..."}: the reply failed
//
// Anything refused before the kid's message is stored (chat off, not a
// kid, no such conversation, an empty or over-long message) is an ordinary
// JSON error with its status instead.
func (h *Handler) Send(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	kid, ok := auth.UserFrom(ctx)
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "Please log in.", "")
		return
	}
	id, ok := pathID(r, "id")
	if !ok {
		h.writeError(w, r, ErrNoSuchChat)
		return
	}
	var req struct {
		Text     string `json:"text"`
		TimeZone string `json:"timeZone"`
	}
	if !decodeJSON(w, r, &req, `Send {"text": "your message"}.`) {
		return
	}
	loc := locationOf(req.TimeZone)

	// http.ResponseController reaches Flush through the logging
	// middleware's response wrapper (it has an Unwrap method).
	rc := http.NewResponseController(w)
	streaming := false
	event := func(name string, v any) {
		b, err := json.Marshal(v)
		if err != nil {
			return
		}
		// A failed write means the kid has gone; the request's context is
		// cancelled too, which stops the reply.
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", name, b)
		_ = rc.Flush()
	}

	reply, err := h.svc.Send(ctx, kid, id, req.Text, loc, SendEvents{
		OnKid: func(m store.ChatMessage) {
			// The kid's message is stored: from here on, the answer is a
			// stream.
			w.Header().Set("Content-Type", "text/event-stream")
			w.Header().Set("Cache-Control", "no-cache")
			w.Header().Set("X-Accel-Buffering", "no") // ask any proxy not to buffer
			w.WriteHeader(http.StatusOK)
			streaming = true
			event("message", toMessage(m))
		},
		OnTool: func(n aikit.ToolNote) {
			event("tool", map[string]string{"tool": n.Tool, "detail": n.Detail})
		},
		OnChunk: func(text string) { event("delta", map[string]string{"text": text}) },
	})
	switch {
	case err == nil:
		event("done", toMessage(reply))
	case streaming:
		// The service has logged the cause. Kids get a plain, kind
		// message: the technical reason and hint are for the log.
		event("error", respond.ErrorBody{
			Error: "Sorry, something went wrong while I was answering.",
			Hint:  "Try sending your message again in a moment.",
		})
	default:
		h.writeError(w, r, err)
	}
}

// --- Parents: /api/members/{id}/chats, behind asParent ---

// ListFor serves GET /api/members/{id}/chats.
func (h *Handler) ListFor(w http.ResponseWriter, r *http.Request) {
	parent, kid, ok := h.parentAndKid(w, r)
	if !ok {
		return
	}
	list, err := h.svc.ListFor(r.Context(), parent, kid)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{"conversations": toConversations(list, true)})
}

// Transcript serves GET /api/members/{id}/chats/{chatId}.
func (h *Handler) Transcript(w http.ResponseWriter, r *http.Request) {
	parent, kid, ok := h.parentAndKid(w, r)
	if !ok {
		return
	}
	id, ok := pathID(r, "chatId")
	if !ok {
		h.writeError(w, r, ErrNoSuchChat)
		return
	}
	t, err := h.svc.Transcript(r.Context(), parent, kid, id)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	respond.JSON(w, http.StatusOK, toTranscript(t, true))
}

// Delete serves DELETE /api/members/{id}/chats/{chatId}.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	parent, kid, ok := h.parentAndKid(w, r)
	if !ok {
		return
	}
	id, ok := pathID(r, "chatId")
	if !ok {
		h.writeError(w, r, ErrNoSuchChat)
		return
	}
	if err := h.svc.Delete(r.Context(), parent, kid, id); err != nil {
		h.writeError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// BirthYear serves GET /api/members/{id}/birth-year.
func (h *Handler) BirthYear(w http.ResponseWriter, r *http.Request) {
	parent, kid, ok := h.parentAndKid(w, r)
	if !ok {
		return
	}
	year, set, err := h.svc.BirthYear(r.Context(), parent, kid)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	respond.JSON(w, http.StatusOK, birthYearJSON(year, set))
}

// SetBirthYear serves PUT /api/members/{id}/birth-year with
// {"birthYear": 2011} or {"birthYear": null} to clear it.
func (h *Handler) SetBirthYear(w http.ResponseWriter, r *http.Request) {
	parent, kid, ok := h.parentAndKid(w, r)
	if !ok {
		return
	}
	var req struct {
		BirthYear *int `json:"birthYear"`
	}
	if !decodeJSON(w, r, &req, `Send {"birthYear": 2011}, or {"birthYear": null} to clear it.`) {
		return
	}
	year := 0
	if req.BirthYear != nil {
		year = *req.BirthYear
		if year == 0 {
			h.writeError(w, r, ErrInvalidBirthYear)
			return
		}
	}
	if err := h.svc.SetBirthYear(r.Context(), parent, kid, year); err != nil {
		h.writeError(w, r, err)
		return
	}
	respond.JSON(w, http.StatusOK, birthYearJSON(year, year != 0))
}

func birthYearJSON(year int, set bool) map[string]any {
	if !set {
		return map[string]any{"birthYear": nil}
	}
	return map[string]any{"birthYear": year}
}

// --- The family's parents: /api/family/..., behind parentsOnly ---

// Reviews serves GET /api/family/reviews: the review queue.
func (h *Handler) Reviews(w http.ResponseWriter, r *http.Request) {
	parent, ok := auth.UserFrom(r.Context())
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "Please log in.", "")
		return
	}
	flags, err := h.svc.Reviews(r.Context(), parent)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	respond.JSON(w, http.StatusOK, map[string]any{"flags": toFlags(flags)})
}

// MarkReviewed serves POST /api/family/reviews/{flagId}/reviewed.
func (h *Handler) MarkReviewed(w http.ResponseWriter, r *http.Request) {
	parent, ok := auth.UserFrom(r.Context())
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "Please log in.", "")
		return
	}
	id, ok := pathID(r, "flagId")
	if !ok {
		h.writeError(w, r, ErrNoSuchFlag)
		return
	}
	f, err := h.svc.MarkReviewed(r.Context(), parent, id)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	respond.JSON(w, http.StatusOK, toFlag(f))
}

// Alerts serves GET /api/family/alerts: the unreviewed count and any
// unreviewed crisis, for every parent page.
func (h *Handler) Alerts(w http.ResponseWriter, r *http.Request) {
	parent, ok := auth.UserFrom(r.Context())
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "Please log in.", "")
		return
	}
	a, err := h.svc.Alerts(r.Context(), parent)
	if err != nil {
		h.writeError(w, r, err)
		return
	}
	type crisisJSON struct {
		FlagID         int64     `json:"flagId"`
		MemberID       int64     `json:"memberId"`
		ConversationID int64     `json:"conversationId"`
		CreatedAt      time.Time `json:"createdAt"`
	}
	crises := make([]crisisJSON, len(a.Crises))
	for i, f := range a.Crises {
		crises[i] = crisisJSON{FlagID: f.ID, MemberID: f.MemberID, ConversationID: f.ConversationID, CreatedAt: f.CreatedAt}
	}
	respond.JSON(w, http.StatusOK, map[string]any{"unreviewed": a.Unreviewed, "crises": crises})
}

// parentAndKid returns the logged-in parent and the member in the path,
// which auth.RequireMember has already checked is in their family.
func (h *Handler) parentAndKid(w http.ResponseWriter, r *http.Request) (parent, kid store.User, ok bool) {
	parent, ok = auth.UserFrom(r.Context())
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "Please log in.", "")
		return store.User{}, store.User{}, false
	}
	kid, ok = auth.SubjectFrom(r.Context())
	if !ok {
		respond.Error(w, http.StatusUnauthorized, "Please log in.", "")
		return store.User{}, store.User{}, false
	}
	return parent, kid, true
}

// writeError turns a Service error into a response. Unexpected errors are
// logged here; the expected ones need no log line.
func (h *Handler) writeError(w http.ResponseWriter, r *http.Request, err error) {
	switch {
	case errors.Is(err, context.Canceled) && r.Context().Err() != nil:
		// The browser gave up on the request (a page change, a preload it
		// didn't need): nobody is waiting for an answer, and it's no fault.
		return
	case errors.Is(err, ErrChatOff):
		respond.Error(w, http.StatusServiceUnavailable, "Chat isn't set up yet. Ask a parent.", "")
	case errors.Is(err, ErrNotAKid):
		respond.Error(w, http.StatusForbidden, "Kid chat is for kids. Try your Assistant.", "")
	case errors.Is(err, ErrNotParent):
		respond.Error(w, http.StatusForbidden, "Only parents can review kids' chats.", "Ask a parent in your family.")
	case errors.Is(err, ErrNoSuchChat):
		respond.Error(w, http.StatusNotFound, "That conversation doesn't exist.", "")
	case errors.Is(err, ErrNoSuchKid):
		respond.Error(w, http.StatusNotFound, "There's no kid like that in your family.", "")
	case errors.Is(err, ErrNoSuchFlag):
		respond.Error(w, http.StatusNotFound, "That flag doesn't exist.", "")
	case errors.Is(err, ErrInvalidMode):
		respond.Error(w, http.StatusBadRequest, `Send {"mode": "homework"} or {"mode": "explore"}.`, "")
	case errors.Is(err, ErrEmptyMessage):
		respond.Error(w, http.StatusBadRequest, "Type a message first.", "")
	case errors.Is(err, ErrMessageTooLong):
		respond.Error(w, http.StatusBadRequest, "That message is too long.", fmt.Sprintf("Keep it under %d characters.", MaxMessageChars))
	case errors.Is(err, ErrInvalidBirthYear):
		respond.Error(w, http.StatusBadRequest, fmt.Sprintf("Pick a birth year that makes them %d to %d this year.", MinKidAge, MaxKidAge), "")
	default:
		h.logger.ErrorContext(r.Context(), "chat request failed", "path", r.URL.Path, "err", err)
		respond.Error(w, http.StatusInternalServerError, "Something went wrong on our side.", "Try again in a moment.")
	}
}

// locationOf returns the IANA time zone tz, or UTC if it's missing or
// unknown, as the widget routes do. "Local" would be the server's zone, not
// the kid's, so it counts as unknown.
func locationOf(tz string) *time.Location {
	if tz == "" || tz == "Local" {
		return time.UTC
	}
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return time.UTC
	}
	return loc
}

// pathID reads a positive integer id from the path wildcard name.
func pathID(r *http.Request, name string) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue(name), 10, 64)
	return id, err == nil && id > 0
}

// decodeJSON decodes the request body into dst, answering 400 with example
// when it isn't one JSON value of the right shape.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any, example string) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	err := dec.Decode(dst)
	if err == nil {
		// A second Decode must hit EOF, or the body had trailing junk.
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
