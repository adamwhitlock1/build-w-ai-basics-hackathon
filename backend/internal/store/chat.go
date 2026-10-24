package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Conversation modes and message roles, as stored.
const (
	ModeHomework = "homework"
	ModeExplore  = "explore"

	RoleKid   = "kid"
	RoleAgent = "agent"
)

// Conversation is one kid's chat with the family agent.
type Conversation struct {
	ID        int64
	FamilyID  int64
	MemberID  int64
	Mode      string // ModeHomework or ModeExplore
	Title     string // "" until the first message
	CreatedAt time.Time
	UpdatedAt time.Time // the last message's time
	// UnreviewedFlags counts the conversation's flags no parent has marked
	// reviewed. Only the list queries fill it in.
	UnreviewedFlags int
}

// ChatMessage is one message in a conversation.
type ChatMessage struct {
	ID             int64
	ConversationID int64
	Role           string // RoleKid or RoleAgent
	Text           string
	CreatedAt      time.Time
}

// ChatFlag is a kid message the safety check flagged, for parents to review.
type ChatFlag struct {
	ID             int64
	MessageID      int64
	ConversationID int64
	FamilyID       int64
	MemberID       int64
	Category       string
	Severity       string // "low", "medium" or "high"
	Crisis         bool
	Reason         string
	CreatedAt      time.Time
	ReviewedAt     time.Time // zero until reviewed
	ReviewedBy     int64     // 0 until reviewed

	// Filled in by the queue queries only: the flagged message and the
	// conversation's title, so a queue entry needs no second lookup.
	MessageText       string
	ConversationTitle string
}

const conversationColumns = `c.id, c.family_id, c.member_id, c.mode, c.title, c.created_at, c.updated_at`

// SQLite's RETURNING can't name a table alias, so inserts return these.
const (
	conversationReturning = `id, family_id, member_id, mode, title, created_at, updated_at`
	chatFlagReturning     = `id, message_id, conversation_id, family_id, member_id, category, severity,
	crisis, reason, created_at, reviewed_at, reviewed_by`
)

// unreviewedCount is a subquery counting conversation c's unreviewed flags.
const unreviewedCount = `(SELECT count(*) FROM chat_flags f WHERE f.conversation_id = c.id AND f.reviewed_at IS NULL)`

func scanConversation(row scanner, withCount bool) (Conversation, error) {
	var c Conversation
	dest := []any{&c.ID, &c.FamilyID, &c.MemberID, &c.Mode, &c.Title, &c.CreatedAt, &c.UpdatedAt}
	if withCount {
		dest = append(dest, &c.UnreviewedFlags)
	}
	err := row.Scan(dest...)
	if errors.Is(err, sql.ErrNoRows) {
		return Conversation{}, ErrNotFound
	}
	return c, err
}

// CreateConversation starts a conversation for memberID in familyID.
func (s *Store) CreateConversation(ctx context.Context, familyID, memberID int64, mode string) (Conversation, error) {
	c, err := scanConversation(s.db.QueryRowContext(ctx,
		`INSERT INTO chat_conversations (family_id, member_id, mode) VALUES (?, ?, ?)
		 RETURNING `+conversationReturning, familyID, memberID, mode), false)
	if err != nil {
		return Conversation{}, fmt.Errorf("create conversation for member %d: %w", memberID, err)
	}
	return c, nil
}

// ListConversations returns memberID's conversations in familyID, most
// recently active first, with their unreviewed flag counts.
func (s *Store) ListConversations(ctx context.Context, familyID, memberID int64) ([]Conversation, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+conversationColumns+`, `+unreviewedCount+`
		   FROM chat_conversations c
		  WHERE c.family_id = ? AND c.member_id = ?
		  -- Times have one-second resolution, so ties go to the conversation
		  -- with the newest message.
		  ORDER BY c.updated_at DESC,
		           (SELECT max(m.id) FROM chat_messages m WHERE m.conversation_id = c.id) DESC NULLS LAST,
		           c.id DESC`, familyID, memberID)
	if err != nil {
		return nil, fmt.Errorf("list conversations of member %d: %w", memberID, err)
	}
	defer rows.Close()
	var out []Conversation
	for rows.Next() {
		c, err := scanConversation(rows, true)
		if err != nil {
			return nil, fmt.Errorf("scan conversation: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetConversation returns conversation id if it belongs to memberID in
// familyID, and ErrNotFound otherwise, so ids can't be probed.
func (s *Store) GetConversation(ctx context.Context, familyID, memberID, id int64) (Conversation, error) {
	c, err := scanConversation(s.db.QueryRowContext(ctx,
		`SELECT `+conversationColumns+`, `+unreviewedCount+`
		   FROM chat_conversations c
		  WHERE c.id = ? AND c.family_id = ? AND c.member_id = ?`, id, familyID, memberID), true)
	if err != nil {
		return Conversation{}, fmt.Errorf("get conversation %d: %w", id, err)
	}
	return c, nil
}

// SetConversationTitle sets conversation id's title.
func (s *Store) SetConversationTitle(ctx context.Context, id int64, title string) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE chat_conversations SET title = ? WHERE id = ?`, title, id); err != nil {
		return fmt.Errorf("set title of conversation %d: %w", id, err)
	}
	return nil
}

// DeleteConversation deletes conversation id, with its messages and flags,
// if it belongs to memberID in familyID; otherwise it returns ErrNotFound.
// Usage records aren't linked to conversations, so they stay.
func (s *Store) DeleteConversation(ctx context.Context, familyID, memberID, id int64) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM chat_conversations WHERE id = ? AND family_id = ? AND member_id = ?`, id, familyID, memberID)
	if err != nil {
		return fmt.Errorf("delete conversation %d: %w", id, err)
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		return fmt.Errorf("delete conversation %d: %w", id, ErrNotFound)
	}
	return nil
}

const chatMessageColumns = `id, conversation_id, role, text, created_at`

func scanChatMessage(row scanner) (ChatMessage, error) {
	var m ChatMessage
	err := row.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Text, &m.CreatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return ChatMessage{}, ErrNotFound
	}
	return m, err
}

// AddChatMessage appends a message to conversation id and marks the
// conversation as active now.
func (s *Store) AddChatMessage(ctx context.Context, conversationID int64, role, text string) (ChatMessage, error) {
	var m ChatMessage
	err := withTx(ctx, s.db, func(tx *sql.Tx) error {
		var err error
		m, err = scanChatMessage(tx.QueryRowContext(ctx,
			`INSERT INTO chat_messages (conversation_id, role, text) VALUES (?, ?, ?) RETURNING `+chatMessageColumns,
			conversationID, role, text))
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE chat_conversations SET updated_at = ? WHERE id = ?`,
			formatTime(m.CreatedAt), conversationID)
		return err
	})
	if err != nil {
		return ChatMessage{}, fmt.Errorf("add message to conversation %d: %w", conversationID, err)
	}
	return m, nil
}

// ListChatMessages returns every message in conversation id, oldest first.
func (s *Store) ListChatMessages(ctx context.Context, conversationID int64) ([]ChatMessage, error) {
	return s.queryChatMessages(ctx,
		`SELECT `+chatMessageColumns+` FROM chat_messages WHERE conversation_id = ? ORDER BY id`, conversationID)
}

// RecentChatMessages returns the last n messages of conversation id, oldest
// first.
func (s *Store) RecentChatMessages(ctx context.Context, conversationID int64, n int) ([]ChatMessage, error) {
	return s.queryChatMessages(ctx,
		`SELECT * FROM (SELECT `+chatMessageColumns+` FROM chat_messages WHERE conversation_id = ? ORDER BY id DESC LIMIT ?)
		 ORDER BY id`, conversationID, n)
}

func (s *Store) queryChatMessages(ctx context.Context, query string, args ...any) ([]ChatMessage, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list chat messages: %w", err)
	}
	defer rows.Close()
	var out []ChatMessage
	for rows.Next() {
		m, err := scanChatMessage(rows)
		if err != nil {
			return nil, fmt.Errorf("scan chat message: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

const chatFlagColumns = `f.id, f.message_id, f.conversation_id, f.family_id, f.member_id, f.category, f.severity,
	f.crisis, f.reason, f.created_at, f.reviewed_at, f.reviewed_by`

func scanChatFlag(row scanner, withContext bool) (ChatFlag, error) {
	var (
		f          ChatFlag
		reviewedAt sql.NullTime
		reviewedBy sql.NullInt64
	)
	dest := []any{&f.ID, &f.MessageID, &f.ConversationID, &f.FamilyID, &f.MemberID, &f.Category, &f.Severity,
		&f.Crisis, &f.Reason, &f.CreatedAt, &reviewedAt, &reviewedBy}
	if withContext {
		dest = append(dest, &f.MessageText, &f.ConversationTitle)
	}
	err := row.Scan(dest...)
	if errors.Is(err, sql.ErrNoRows) {
		return ChatFlag{}, ErrNotFound
	}
	if err != nil {
		return ChatFlag{}, err
	}
	if reviewedAt.Valid {
		f.ReviewedAt = reviewedAt.Time
	}
	f.ReviewedBy = reviewedBy.Int64
	return f, nil
}

// AddChatFlag saves f (ID, CreatedAt and the review fields are ignored).
func (s *Store) AddChatFlag(ctx context.Context, f ChatFlag) (ChatFlag, error) {
	saved, err := scanChatFlag(s.db.QueryRowContext(ctx,
		`INSERT INTO chat_flags (message_id, conversation_id, family_id, member_id, category, severity, crisis, reason)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?) RETURNING `+chatFlagReturning,
		f.MessageID, f.ConversationID, f.FamilyID, f.MemberID, f.Category, f.Severity, f.Crisis, f.Reason), false)
	if err != nil {
		return ChatFlag{}, fmt.Errorf("add flag to message %d: %w", f.MessageID, err)
	}
	return saved, nil
}

// flagQueueOrder puts unreviewed flags first: crises, then by severity,
// newest first. Reviewed flags follow, most recently reviewed first.
const flagQueueOrder = `
	ORDER BY f.reviewed_at IS NOT NULL,
	         CASE WHEN f.reviewed_at IS NULL THEN f.crisis ELSE 0 END DESC,
	         CASE WHEN f.reviewed_at IS NULL THEN CASE f.severity WHEN 'high' THEN 3 WHEN 'medium' THEN 2 ELSE 1 END ELSE 0 END DESC,
	         CASE WHEN f.reviewed_at IS NULL THEN f.created_at ELSE f.reviewed_at END DESC,
	         f.id DESC`

// ReviewQueue returns familyID's flags in review order (see flagQueueOrder),
// each with its message's text and its conversation's title. At most
// reviewedLimit reviewed flags are included.
func (s *Store) ReviewQueue(ctx context.Context, familyID int64, reviewedLimit int) ([]ChatFlag, error) {
	return s.queryChatFlags(ctx, true,
		`SELECT `+chatFlagColumns+`, m.text, c.title
		   FROM chat_flags f
		   JOIN chat_messages m ON m.id = f.message_id
		   JOIN chat_conversations c ON c.id = f.conversation_id
		  WHERE f.family_id = ?
		    AND (f.reviewed_at IS NULL OR f.id IN (
		          SELECT id FROM chat_flags WHERE family_id = ? AND reviewed_at IS NOT NULL
		           ORDER BY reviewed_at DESC, id DESC LIMIT ?))`+flagQueueOrder,
		familyID, familyID, reviewedLimit)
}

// ConversationFlags returns conversation id's flags, oldest first.
func (s *Store) ConversationFlags(ctx context.Context, conversationID int64) ([]ChatFlag, error) {
	return s.queryChatFlags(ctx, false,
		`SELECT `+chatFlagColumns+` FROM chat_flags f WHERE f.conversation_id = ? ORDER BY f.id`, conversationID)
}

// UnreviewedCrises returns familyID's unreviewed crisis flags, newest first.
func (s *Store) UnreviewedCrises(ctx context.Context, familyID int64) ([]ChatFlag, error) {
	return s.queryChatFlags(ctx, false,
		`SELECT `+chatFlagColumns+` FROM chat_flags f
		  WHERE f.family_id = ? AND f.crisis AND f.reviewed_at IS NULL
		  ORDER BY f.created_at DESC, f.id DESC`, familyID)
}

// CountUnreviewedFlags counts familyID's flags no parent has reviewed.
func (s *Store) CountUnreviewedFlags(ctx context.Context, familyID int64) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx,
		`SELECT count(*) FROM chat_flags WHERE family_id = ? AND reviewed_at IS NULL`, familyID).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("count unreviewed flags of family %d: %w", familyID, err)
	}
	return n, nil
}

// MarkFlagReviewed records that parentID reviewed flag id, now. A flag
// outside familyID is ErrNotFound. Marking a reviewed flag again keeps the
// first review.
func (s *Store) MarkFlagReviewed(ctx context.Context, familyID, id, parentID int64) (ChatFlag, error) {
	_, err := s.db.ExecContext(ctx,
		`UPDATE chat_flags SET reviewed_at = ?, reviewed_by = ?
		  WHERE id = ? AND family_id = ? AND reviewed_at IS NULL`,
		formatTime(time.Now()), parentID, id, familyID)
	if err != nil {
		return ChatFlag{}, fmt.Errorf("mark flag %d reviewed: %w", id, err)
	}
	f, err := scanChatFlag(s.db.QueryRowContext(ctx,
		`SELECT `+chatFlagColumns+` FROM chat_flags f WHERE f.id = ? AND f.family_id = ?`, id, familyID), false)
	if err != nil {
		return ChatFlag{}, fmt.Errorf("get flag %d: %w", id, err)
	}
	return f, nil
}

func (s *Store) queryChatFlags(ctx context.Context, withContext bool, query string, args ...any) ([]ChatFlag, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list chat flags: %w", err)
	}
	defer rows.Close()
	var out []ChatFlag
	for rows.Next() {
		f, err := scanChatFlag(rows, withContext)
		if err != nil {
			return nil, fmt.Errorf("scan chat flag: %w", err)
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// GetBirthYear returns userID's birth year, and false if none is set.
func (s *Store) GetBirthYear(ctx context.Context, userID int64) (int, bool, error) {
	var year sql.NullInt64
	err := s.db.QueryRowContext(ctx, `SELECT birth_year FROM member_profiles WHERE user_id = ?`, userID).Scan(&year)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, fmt.Errorf("get birth year of user %d: %w", userID, err)
	}
	return int(year.Int64), year.Valid, nil
}

// SetBirthYear saves userID's birth year; 0 clears it. It leaves the rest
// of the profile alone.
func (s *Store) SetBirthYear(ctx context.Context, userID int64, year int) error {
	var v any // NULL
	if year != 0 {
		v = year
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO member_profiles (user_id, birth_year) VALUES (?, ?)
		ON CONFLICT (user_id) DO UPDATE SET
			birth_year = excluded.birth_year,
			updated_at = CURRENT_TIMESTAMP`,
		userID, v)
	if err != nil {
		return fmt.Errorf("set birth year of user %d: %w", userID, err)
	}
	return nil
}
