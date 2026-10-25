// Package assistant is the parents' Assistant: each parent's private
// conversations with aikit.ParentAgent. An "Ask anything" conversation runs
// on the main key and can't read the kids' chats; an "About the kids" one
// runs on the kids' zero-retention key and can. Service holds those rules;
// Handler serves it over HTTP.
package assistant

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"kinlight/internal/aikit"
	"kinlight/internal/llm"
	"kinlight/internal/store"
)

// Feature names the Assistant's AI calls in the usage records.
const Feature = "parent_chat"

// MaxMessageChars is the longest message a parent can send.
const MaxMessageChars = 8000

// replyTurns is how much of the conversation the Assistant sees.
const replyTurns = 30

// Errors the service returns. Handler turns them into responses.
var (
	ErrNotParent          = errors.New("the Assistant is for parents")
	ErrKidsOff            = errors.New(`"About the kids" needs kid chat to be set up`)
	ErrNoSuchConversation = errors.New("no such conversation")
	ErrInvalidKind        = errors.New("kind must be anything or kids")
	ErrEmptyMessage       = errors.New("the message is empty")
	ErrMessageTooLong     = fmt.Errorf("the message is longer than %d characters", MaxMessageChars)
	errNoAgent            = errors.New("the Assistant isn't set up")
)

// Agent is the Assistant: aikit.ParentAgent, or a fake in tests.
type Agent interface {
	Reply(ctx context.Context, in aikit.ParentReplyInput, onChunk func(string)) (string, error)
}

// Store is the storage Service needs; *store.Store satisfies it.
type Store interface {
	CreateAssistantConversation(ctx context.Context, familyID, memberID int64, kind string) (store.AssistantConversation, error)
	ListAssistantConversations(ctx context.Context, familyID, memberID int64) ([]store.AssistantConversation, error)
	GetAssistantConversation(ctx context.Context, familyID, memberID, id int64) (store.AssistantConversation, error)
	SetAssistantConversationTitle(ctx context.Context, id int64, title string) error
	DeleteAssistantConversation(ctx context.Context, familyID, memberID, id int64) error
	AddAssistantMessage(ctx context.Context, conversationID int64, role, text string) (store.AssistantMessage, error)
	ListAssistantMessages(ctx context.Context, conversationID int64) ([]store.AssistantMessage, error)
	RecentAssistantMessages(ctx context.Context, conversationID int64, n int) ([]store.AssistantMessage, error)
	AddAssistantToolCalls(ctx context.Context, conversationID, messageID int64, calls []store.ChatToolCall) error
	ListAssistantToolCalls(ctx context.Context, conversationID int64) ([]store.ChatToolCall, error)
}

// Tools looks things up for one parent. Every method takes the parent, so
// it reads only their family.
type Tools interface {
	Search(ctx context.Context, query string) ([]aikit.SearchResult, error)
	FamilyDay(ctx context.Context, parent store.User, who string, loc *time.Location) (string, error)
	Weather(ctx context.Context, parent store.User) (string, error)
	MyPicks(ctx context.Context, parent store.User) (string, error)
	KidsChats(ctx context.Context, parent store.User, q aikit.KidsChatsQuery) (string, error)
}

// Service is the Assistant.
type Service struct {
	anything Agent // on the main key
	kids     Agent // on the kids' no-retention key; nil when kid chat is off
	store    Store
	tools    Tools // nil: no tools
	logger   *slog.Logger
}

// NewService returns a Service whose "Ask anything" conversations use
// anything and whose "About the kids" ones use kids (nil: unavailable).
func NewService(anything, kids Agent, st Store, logger *slog.Logger) *Service {
	return &Service{anything: anything, kids: kids, store: st, logger: logger}
}

// SetTools gives the Assistant tools that look things up through t.
func (s *Service) SetTools(t Tools) { s.tools = t }

// KidsAvailable reports whether "About the kids" conversations can start.
func (s *Service) KidsAvailable() bool { return s.kids != nil }

// Transcript is a conversation with its messages and tool notes.
type Transcript struct {
	Conversation store.AssistantConversation
	Messages     []store.AssistantMessage
	ToolCalls    []store.ChatToolCall
}

// SendEvents is what Send reports while it works.
type SendEvents struct {
	OnMessage func(store.AssistantMessage) // the parent's message, stored
	OnTool    func(aikit.ToolNote)         // a tool the Assistant used
	OnChunk   func(string)                 // a piece of the reply
}

// List returns parent's conversations, most recently active first.
func (s *Service) List(ctx context.Context, parent store.User) ([]store.AssistantConversation, error) {
	if !parent.IsAdmin {
		return nil, ErrNotParent
	}
	return s.store.ListAssistantConversations(ctx, parent.FamilyID, parent.ID)
}

// Start begins one of parent's conversations of kind.
func (s *Service) Start(ctx context.Context, parent store.User, kind string) (store.AssistantConversation, error) {
	if !parent.IsAdmin {
		return store.AssistantConversation{}, ErrNotParent
	}
	switch kind {
	case aikit.KindAnything:
	case aikit.KindKids:
		if s.kids == nil {
			return store.AssistantConversation{}, ErrKidsOff
		}
	default:
		return store.AssistantConversation{}, ErrInvalidKind
	}
	c, err := s.store.CreateAssistantConversation(ctx, parent.FamilyID, parent.ID, kind)
	if err != nil {
		return store.AssistantConversation{}, err
	}
	s.logger.InfoContext(ctx, "assistant conversation started", "member_id", parent.ID, "conversation_id", c.ID, "kind", kind)
	return c, nil
}

// Get returns one of parent's conversations.
func (s *Service) Get(ctx context.Context, parent store.User, id int64) (Transcript, error) {
	c, err := s.conversation(ctx, parent, id)
	if err != nil {
		return Transcript{}, err
	}
	t := Transcript{Conversation: c}
	if t.Messages, err = s.store.ListAssistantMessages(ctx, c.ID); err != nil {
		return Transcript{}, err
	}
	if t.ToolCalls, err = s.store.ListAssistantToolCalls(ctx, c.ID); err != nil {
		return Transcript{}, err
	}
	return t, nil
}

// Delete deletes one of parent's conversations.
func (s *Service) Delete(ctx context.Context, parent store.User, id int64) error {
	if !parent.IsAdmin {
		return ErrNotParent
	}
	err := s.store.DeleteAssistantConversation(ctx, parent.FamilyID, parent.ID, id)
	if errors.Is(err, store.ErrNotFound) {
		return ErrNoSuchConversation
	}
	return err
}

func (s *Service) conversation(ctx context.Context, parent store.User, id int64) (store.AssistantConversation, error) {
	if !parent.IsAdmin {
		return store.AssistantConversation{}, ErrNotParent
	}
	c, err := s.store.GetAssistantConversation(ctx, parent.FamilyID, parent.ID, id)
	if errors.Is(err, store.ErrNotFound) {
		return store.AssistantConversation{}, ErrNoSuchConversation
	}
	return c, err
}

// Send adds parent's message to conversation id and streams the reply,
// from the agent for the conversation's kind. The first message titles the
// conversation. Tool notes are stored with the reply, or alone if it
// failed. loc is the parent's time zone.
func (s *Service) Send(ctx context.Context, parent store.User, id int64, text string, loc *time.Location, ev SendEvents) (store.AssistantMessage, error) {
	text = strings.TrimSpace(text)
	switch {
	case text == "":
		return store.AssistantMessage{}, ErrEmptyMessage
	case utf8.RuneCountInString(text) > MaxMessageChars:
		return store.AssistantMessage{}, ErrMessageTooLong
	}
	c, err := s.conversation(ctx, parent, id)
	if err != nil {
		return store.AssistantMessage{}, err
	}
	agent := s.anything
	if c.Kind == aikit.KindKids {
		agent = s.kids
		if agent == nil {
			return store.AssistantMessage{}, ErrKidsOff
		}
	}
	if agent == nil {
		return store.AssistantMessage{}, errNoAgent
	}
	ctx = llm.WithAttribution(ctx, llm.Attribution{FamilyID: parent.FamilyID, MemberID: parent.ID, ViewerID: parent.ID, Feature: Feature})

	msg, err := s.store.AddAssistantMessage(ctx, c.ID, store.RoleParent, text)
	if err != nil {
		return store.AssistantMessage{}, err
	}
	if ev.OnMessage != nil {
		ev.OnMessage(msg)
	}
	if c.Title == "" {
		if err := s.store.SetAssistantConversationTitle(ctx, c.ID, titleFrom(text)); err != nil {
			s.logger.WarnContext(ctx, "couldn't title assistant conversation", "conversation_id", c.ID, "err", err)
		}
	}

	history, err := s.store.RecentAssistantMessages(ctx, c.ID, replyTurns)
	if err != nil {
		return store.AssistantMessage{}, err
	}
	turns := make([]aikit.Turn, len(history))
	for i, m := range history {
		turns[i] = aikit.Turn{FromUser: m.Role == store.RoleParent, Text: m.Text}
	}
	var notes []store.ChatToolCall
	tools := s.parentTools(parent, c.Kind, loc, func(n aikit.ToolNote) {
		notes = append(notes, store.ChatToolCall{Tool: n.Tool, Detail: n.Detail})
		if ev.OnTool != nil {
			ev.OnTool(n)
		}
	})
	onChunk := ev.OnChunk
	if onChunk == nil {
		onChunk = func(string) {}
	}
	reply, err := agent.Reply(ctx, aikit.ParentReplyInput{Kind: c.Kind, History: turns, Tools: tools}, onChunk)
	if err != nil {
		s.logger.WarnContext(ctx, "assistant reply failed", "member_id", parent.ID, "conversation_id", c.ID, "err", err)
		s.saveToolCalls(ctx, c.ID, 0, notes)
		return store.AssistantMessage{}, err
	}
	saved, err := s.store.AddAssistantMessage(ctx, c.ID, store.RoleAgent, reply)
	if err != nil {
		s.saveToolCalls(ctx, c.ID, 0, notes)
		return store.AssistantMessage{}, err
	}
	s.saveToolCalls(ctx, c.ID, saved.ID, notes)
	return saved, nil
}

func (s *Service) saveToolCalls(ctx context.Context, conversationID, messageID int64, notes []store.ChatToolCall) {
	if err := s.store.AddAssistantToolCalls(context.WithoutCancel(ctx), conversationID, messageID, notes); err != nil {
		s.logger.ErrorContext(ctx, "couldn't store assistant tool calls", "conversation_id", conversationID, "err", err)
	}
}

// parentTools builds the Assistant's tools for one message from parent.
// kids_chats exists only in an "About the kids" conversation, so an "Ask
// anything" one can't reach the kids' chats whatever is asked.
func (s *Service) parentTools(parent store.User, kind string, loc *time.Location, onTool func(aikit.ToolNote)) *aikit.ParentTools {
	if s.tools == nil {
		return nil
	}
	if loc == nil {
		loc = time.UTC
	}
	logged := func(ctx context.Context, tool string, err error) {
		if err != nil {
			s.logger.WarnContext(ctx, "assistant tool failed", "member_id", parent.ID, "tool", tool, "err", err)
		}
	}
	t := &aikit.ParentTools{
		Search: func(ctx context.Context, q string) ([]aikit.SearchResult, error) {
			out, err := s.tools.Search(ctx, q)
			logged(ctx, aikit.ToolSearch, err)
			return out, err
		},
		FamilyDay: func(ctx context.Context, who string) (string, error) {
			out, err := s.tools.FamilyDay(ctx, parent, who, loc)
			logged(ctx, aikit.ToolFamilyDay, err)
			return out, err
		},
		Weather: func(ctx context.Context) (string, error) {
			out, err := s.tools.Weather(ctx, parent)
			logged(ctx, aikit.ToolWeather, err)
			return out, err
		},
		MyPicks: func(ctx context.Context) (string, error) {
			out, err := s.tools.MyPicks(ctx, parent)
			logged(ctx, aikit.ToolMyPicks, err)
			return out, err
		},
		OnTool: onTool,
	}
	if kind == aikit.KindKids {
		t.KidsChats = func(ctx context.Context, q aikit.KidsChatsQuery) (string, error) {
			out, err := s.tools.KidsChats(ctx, parent, q)
			logged(ctx, aikit.ToolKidsChats, err)
			return out, err
		}
	}
	return t
}

// titleFrom is a conversation's title: its first message, cut at a word to
// 60 characters.
func titleFrom(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	runes := []rune(text)
	if len(runes) <= 60 {
		return text
	}
	cut := string(runes[:59])
	if i := strings.LastIndexByte(cut, ' '); i > 0 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, " .,;:-") + "…"
}
