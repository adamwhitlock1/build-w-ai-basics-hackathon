// Package chat is kid chat with the family agent: kids' conversations, the
// safety check on every message, and what parents see of them. Service
// holds the rules (who may do what, what is stored, when crisis resources
// are added); the agent itself, prompts and models, is aikit.KidAgent, and
// Handler serves it all over HTTP.
package chat

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"porchlight/internal/aikit"
	"porchlight/internal/llm"
	"porchlight/internal/store"
)

// Feature names kid chat's AI calls in the usage records.
const Feature = "kid_chat"

// MaxMessageChars is the longest message a kid can send.
const MaxMessageChars = 4000

// Context sizes: how many turns the safety check sees before the new
// message, and how many the reply sees (the agent caps it too).
const (
	checkTurns = 10
	replyTurns = 30
)

// CrisisResources ends every reply to a message the safety check marked as
// a possible crisis. The server adds it, whatever the model wrote.
const CrisisResources = "If you're thinking about hurting yourself or you feel unsafe, you can call or text 988 any time, or text HOME to 741741. If you're in danger right now, call 911."

// Errors the service returns. Handler turns them into responses.
var (
	// ErrChatOff means kid chat isn't set up: there's no no-retention
	// provider configured.
	ErrChatOff = errors.New("kid chat isn't set up")
	// ErrNotAKid means a parent tried to chat as themselves.
	ErrNotAKid = errors.New("only kids can chat with the family agent")
	// ErrNotParent means a kid tried to use what only parents can.
	ErrNotParent = errors.New("only parents can review kids' chats")
	// ErrNoSuchChat means there's no such conversation for this member:
	// it never existed, it's someone else's, or it was deleted.
	ErrNoSuchChat = errors.New("no such conversation")
	// ErrNoSuchKid means the member a parent asked about isn't a kid.
	ErrNoSuchKid = errors.New("no such kid")
	// ErrNoSuchFlag means there's no such flag in the parent's family.
	ErrNoSuchFlag = errors.New("no such flag")
	// ErrInvalidMode means a conversation was started with an unknown mode.
	ErrInvalidMode = errors.New("mode must be homework or explore")
	// ErrEmptyMessage and ErrMessageTooLong reject a message before
	// anything is stored.
	ErrEmptyMessage   = errors.New("the message is empty")
	ErrMessageTooLong = fmt.Errorf("the message is longer than %d characters", MaxMessageChars)
	// ErrInvalidBirthYear means a birth year that doesn't make the kid 5
	// to 17 this year.
	ErrInvalidBirthYear = fmt.Errorf("a kid's birth year must make them %d to %d this year", MinKidAge, MaxKidAge)
)

// The ages a kid's birth year may give them.
const (
	MinKidAge = 5
	MaxKidAge = 17
)

// Agent is the family agent: aikit.KidAgent, or a fake in tests.
type Agent interface {
	Check(ctx context.Context, recent []aikit.Turn, msg string, wantTitle bool) (aikit.Verdict, error)
	Reply(ctx context.Context, in aikit.ReplyInput, onChunk func(string)) (string, error)
}

// Store is the storage Service needs; *store.Store satisfies it.
type Store interface {
	CreateConversation(ctx context.Context, familyID, memberID int64, mode string) (store.Conversation, error)
	ListConversations(ctx context.Context, familyID, memberID int64) ([]store.Conversation, error)
	GetConversation(ctx context.Context, familyID, memberID, id int64) (store.Conversation, error)
	SetConversationTitle(ctx context.Context, id int64, title string) error
	DeleteConversation(ctx context.Context, familyID, memberID, id int64) error
	AddChatMessage(ctx context.Context, conversationID int64, role, text string) (store.ChatMessage, error)
	ListChatMessages(ctx context.Context, conversationID int64) ([]store.ChatMessage, error)
	RecentChatMessages(ctx context.Context, conversationID int64, n int) ([]store.ChatMessage, error)
	AddChatFlag(ctx context.Context, f store.ChatFlag) (store.ChatFlag, error)
	ConversationFlags(ctx context.Context, conversationID int64) ([]store.ChatFlag, error)
	ReviewQueue(ctx context.Context, familyID int64, reviewedLimit int) ([]store.ChatFlag, error)
	MarkFlagReviewed(ctx context.Context, familyID, id, parentID int64) (store.ChatFlag, error)
	UnreviewedCrises(ctx context.Context, familyID int64) ([]store.ChatFlag, error)
	CountUnreviewedFlags(ctx context.Context, familyID int64) (int, error)
	GetBirthYear(ctx context.Context, userID int64) (int, bool, error)
	SetBirthYear(ctx context.Context, userID int64, year int) error
}

// Service is kid chat. Kids use List, Start, Get and Send; parents use the
// methods in parent.go. Every method checks who may do it and scopes what
// it reads to the caller's family.
type Service struct {
	agent  Agent // nil: kid chat is off (see ErrChatOff)
	store  Store
	logger *slog.Logger
	now    func() time.Time
}

// NewService returns a Service on agent and st. A nil agent turns kid chat
// off: kids can read their conversations but can't start or continue one.
func NewService(agent Agent, st Store, logger *slog.Logger) *Service {
	return &Service{agent: agent, store: st, logger: logger, now: time.Now}
}

// Available reports whether kid chat is on.
func (s *Service) Available() bool { return s.agent != nil }

// Transcript is a conversation with its messages and, for parents, its
// flags.
type Transcript struct {
	Conversation store.Conversation
	Messages     []store.ChatMessage
	Flags        []store.ChatFlag // nil for kids
}

// List returns kid's conversations, most recently active first.
func (s *Service) List(ctx context.Context, kid store.User) ([]store.Conversation, error) {
	if kid.IsAdmin {
		return nil, ErrNotAKid
	}
	return s.store.ListConversations(ctx, kid.FamilyID, kid.ID)
}

// Start begins a conversation for kid in mode (aikit.ModeHomework or
// aikit.ModeExplore).
func (s *Service) Start(ctx context.Context, kid store.User, mode string) (store.Conversation, error) {
	if kid.IsAdmin {
		return store.Conversation{}, ErrNotAKid
	}
	if !s.Available() {
		return store.Conversation{}, ErrChatOff
	}
	if mode != aikit.ModeHomework && mode != aikit.ModeExplore {
		return store.Conversation{}, ErrInvalidMode
	}
	c, err := s.store.CreateConversation(ctx, kid.FamilyID, kid.ID, mode)
	if err != nil {
		return store.Conversation{}, err
	}
	s.logger.InfoContext(ctx, "chat started", "member_id", kid.ID, "chat_id", c.ID, "mode", mode)
	return c, nil
}

// Get returns one of kid's conversations with its messages (no flags: kids
// never see them).
func (s *Service) Get(ctx context.Context, kid store.User, id int64) (Transcript, error) {
	if kid.IsAdmin {
		return Transcript{}, ErrNotAKid
	}
	return s.transcript(ctx, kid, id, false)
}

func (s *Service) transcript(ctx context.Context, member store.User, id int64, withFlags bool) (Transcript, error) {
	c, err := s.store.GetConversation(ctx, member.FamilyID, member.ID, id)
	if errors.Is(err, store.ErrNotFound) {
		return Transcript{}, ErrNoSuchChat
	}
	if err != nil {
		return Transcript{}, err
	}
	t := Transcript{Conversation: c}
	if t.Messages, err = s.store.ListChatMessages(ctx, c.ID); err != nil {
		return Transcript{}, err
	}
	if withFlags {
		if t.Flags, err = s.store.ConversationFlags(ctx, c.ID); err != nil {
			return Transcript{}, err
		}
	}
	return t, nil
}

// Send adds kid's message text to conversation id and streams the agent's
// reply:
//  1. the kid's message is stored, and passed to onKid;
//  2. the safety check classifies it, and a flag is stored if needed (a
//     failed check is flagged "couldn't check", and the reply goes ahead);
//  3. the reply streams to onChunk; a crisis reply ends with
//     CrisisResources, streamed as its last chunk;
//  4. the complete reply is stored and returned.
//
// A failed reply stores nothing more and returns a *llm.UpstreamError,
// possibly after some chunks were passed to onChunk.
func (s *Service) Send(ctx context.Context, kid store.User, id int64, text string, onKid func(store.ChatMessage), onChunk func(string)) (store.ChatMessage, error) {
	if kid.IsAdmin {
		return store.ChatMessage{}, ErrNotAKid
	}
	if !s.Available() {
		return store.ChatMessage{}, ErrChatOff
	}
	text = strings.TrimSpace(text)
	switch {
	case text == "":
		return store.ChatMessage{}, ErrEmptyMessage
	case utf8.RuneCountInString(text) > MaxMessageChars:
		return store.ChatMessage{}, ErrMessageTooLong
	}
	c, err := s.store.GetConversation(ctx, kid.FamilyID, kid.ID, id)
	if errors.Is(err, store.ErrNotFound) {
		return store.ChatMessage{}, ErrNoSuchChat
	}
	if err != nil {
		return store.ChatMessage{}, err
	}
	ctx = llm.WithAttribution(ctx, llm.Attribution{FamilyID: kid.FamilyID, MemberID: kid.ID, ViewerID: kid.ID, Feature: Feature})

	// The context the safety check sees: the turns before this message.
	before, err := s.store.RecentChatMessages(ctx, c.ID, checkTurns)
	if err != nil {
		return store.ChatMessage{}, err
	}
	first := len(before) == 0

	kidMsg, err := s.store.AddChatMessage(ctx, c.ID, store.RoleKid, text)
	if err != nil {
		return store.ChatMessage{}, err
	}
	onKid(kidMsg)

	verdict := s.check(ctx, kid, c, kidMsg, turns(before), first)
	if first {
		title := verdict.Title
		if title == "" {
			title = fallbackTitle(text)
		}
		if err := s.store.SetConversationTitle(ctx, c.ID, title); err != nil {
			s.logger.WarnContext(ctx, "couldn't title chat", "member_id", kid.ID, "chat_id", c.ID, "err", err)
		}
	}

	history, err := s.store.RecentChatMessages(ctx, c.ID, replyTurns)
	if err != nil {
		return store.ChatMessage{}, err
	}
	in := aikit.ReplyInput{Persona: s.persona(ctx, kid), Mode: c.Mode, Crisis: verdict.Crisis, History: turns(history)}
	reply, err := s.agent.Reply(ctx, in, onChunk)
	if err != nil {
		s.logger.WarnContext(ctx, "chat reply failed", "member_id", kid.ID, "chat_id", c.ID, "err", err)
		return store.ChatMessage{}, err
	}
	if verdict.Crisis {
		resources := "\n\n" + CrisisResources
		onChunk(resources)
		reply += resources
	}
	agentMsg, err := s.store.AddChatMessage(ctx, c.ID, store.RoleAgent, reply)
	if err != nil {
		return store.ChatMessage{}, err
	}
	return agentMsg, nil
}

// check runs the safety check on msg and stores a flag when needed. It
// never fails: a check that can't be done is flagged "couldn't check", and
// the verdict it returns then has no crisis.
func (s *Service) check(ctx context.Context, kid store.User, c store.Conversation, msg store.ChatMessage, before []aikit.Turn, wantTitle bool) aikit.Verdict {
	verdict, err := s.agent.Check(ctx, before, msg.Text, wantTitle)
	if err != nil {
		s.logger.WarnContext(ctx, "chat safety check failed; flagging for review", "member_id", kid.ID, "chat_id", c.ID, "err", err)
		verdict = aikit.Verdict{Flag: true, Category: aikit.CategoryCouldntCheck, Severity: "low",
			Reason: "The safety check couldn't run on this message, so it's here for you to look at."}
	}
	if !verdict.Flag {
		return verdict
	}
	_, err = s.store.AddChatFlag(ctx, store.ChatFlag{
		MessageID: msg.ID, ConversationID: c.ID, FamilyID: kid.FamilyID, MemberID: kid.ID,
		Category: verdict.Category, Severity: verdict.Severity, Crisis: verdict.Crisis, Reason: verdict.Reason,
	})
	if err != nil {
		// The reply still goes ahead: the kid is waiting, and a crisis
		// reply still carries the resources.
		s.logger.ErrorContext(ctx, "couldn't store chat flag", "member_id", kid.ID, "chat_id", c.ID, "err", err)
		return verdict
	}
	// Category and severity only: the log never holds what a kid wrote.
	s.logger.WarnContext(ctx, "chat message flagged", "member_id", kid.ID, "chat_id", c.ID,
		"category", verdict.Category, "severity", verdict.Severity, "crisis", verdict.Crisis)
	return verdict
}

// persona picks the agent's voice for kid's age.
func (s *Service) persona(ctx context.Context, kid store.User) string {
	year, ok, err := s.store.GetBirthYear(ctx, kid.ID)
	if err != nil {
		s.logger.WarnContext(ctx, "couldn't read birth year; using the younger persona", "member_id", kid.ID, "err", err)
	}
	return PersonaFor(year, ok, s.now())
}

// PersonaFor returns the persona for a kid born in birthYear (ok false: no
// birth year set) as of now: the teen persona from 13, worked out as this
// year minus the birth year, and the younger one otherwise.
func PersonaFor(birthYear int, ok bool, now time.Time) string {
	if ok && now.Year()-birthYear >= 13 {
		return aikit.PersonaTeen
	}
	return aikit.PersonaYounger
}

// turns converts stored messages to the agent's turns.
func turns(msgs []store.ChatMessage) []aikit.Turn {
	out := make([]aikit.Turn, len(msgs))
	for i, m := range msgs {
		out[i] = aikit.Turn{FromKid: m.Role == store.RoleKid, Text: m.Text}
	}
	return out
}

// maxTitleChars is the longest fallback title.
const maxTitleChars = 60

// fallbackTitle is a conversation's title when the safety check gave none:
// the first message, cut at a word to maxTitleChars.
func fallbackTitle(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	runes := []rune(text)
	if len(runes) <= maxTitleChars {
		return text
	}
	cut := string(runes[:maxTitleChars-1])
	if i := strings.LastIndexByte(cut, ' '); i > 0 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, " .,;:-") + "…"
}
