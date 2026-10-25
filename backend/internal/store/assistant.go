package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Assistant conversation kinds and message roles, as stored.
const (
	KindAnything = "anything"
	KindKids     = "kids"

	RoleParent = "parent"
)

// AssistantConversation is one of a parent's private Assistant
// conversations.
type AssistantConversation struct {
	ID        int64
	FamilyID  int64
	MemberID  int64
	Kind      string // KindAnything or KindKids
	Title     string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// AssistantMessage is one message: the parent's (RoleParent) or the
// assistant's (RoleAgent).
type AssistantMessage struct {
	ID             int64
	ConversationID int64
	Role           string
	Text           string
	CreatedAt      time.Time
}

const assistantConversationColumns = `id, family_id, member_id, kind, title, created_at, updated_at`

func scanAssistantConversation(row scanner) (AssistantConversation, error) {
	var c AssistantConversation
	err := row.Scan(&c.ID, &c.FamilyID, &c.MemberID, &c.Kind, &c.Title, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return AssistantConversation{}, ErrNotFound
	}
	return c, err
}

// CreateAssistantConversation starts one of memberID's conversations.
func (s *Store) CreateAssistantConversation(ctx context.Context, familyID, memberID int64, kind string) (AssistantConversation, error) {
	c, err := scanAssistantConversation(s.db.QueryRowContext(ctx,
		`INSERT INTO assistant_conversations (family_id, member_id, kind) VALUES (?, ?, ?)
		 RETURNING `+assistantConversationColumns, familyID, memberID, kind))
	if err != nil {
		return AssistantConversation{}, fmt.Errorf("create assistant conversation for member %d: %w", memberID, err)
	}
	return c, nil
}

// ListAssistantConversations returns memberID's conversations, most
// recently active first.
func (s *Store) ListAssistantConversations(ctx context.Context, familyID, memberID int64) ([]AssistantConversation, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT `+assistantConversationColumns+` FROM assistant_conversations c
		  WHERE family_id = ? AND member_id = ?
		  -- Times have one-second resolution: ties go to the newest message.
		  ORDER BY updated_at DESC,
		           (SELECT max(m.id) FROM assistant_messages m WHERE m.conversation_id = c.id) DESC NULLS LAST,
		           id DESC`, familyID, memberID)
	if err != nil {
		return nil, fmt.Errorf("list assistant conversations of member %d: %w", memberID, err)
	}
	defer rows.Close()
	var out []AssistantConversation
	for rows.Next() {
		c, err := scanAssistantConversation(rows)
		if err != nil {
			return nil, fmt.Errorf("scan assistant conversation: %w", err)
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// GetAssistantConversation returns conversation id if it's memberID's, in
// familyID; otherwise ErrNotFound.
func (s *Store) GetAssistantConversation(ctx context.Context, familyID, memberID, id int64) (AssistantConversation, error) {
	c, err := scanAssistantConversation(s.db.QueryRowContext(ctx,
		`SELECT `+assistantConversationColumns+` FROM assistant_conversations
		  WHERE id = ? AND family_id = ? AND member_id = ?`, id, familyID, memberID))
	if err != nil {
		return AssistantConversation{}, fmt.Errorf("get assistant conversation %d: %w", id, err)
	}
	return c, nil
}

// SetAssistantConversationTitle sets conversation id's title.
func (s *Store) SetAssistantConversationTitle(ctx context.Context, id int64, title string) error {
	if _, err := s.db.ExecContext(ctx, `UPDATE assistant_conversations SET title = ? WHERE id = ?`, title, id); err != nil {
		return fmt.Errorf("set title of assistant conversation %d: %w", id, err)
	}
	return nil
}

// DeleteAssistantConversation deletes conversation id, with its messages
// and tool calls, if it's memberID's; otherwise ErrNotFound.
func (s *Store) DeleteAssistantConversation(ctx context.Context, familyID, memberID, id int64) error {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM assistant_conversations WHERE id = ? AND family_id = ? AND member_id = ?`, id, familyID, memberID)
	if err != nil {
		return fmt.Errorf("delete assistant conversation %d: %w", id, err)
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		return fmt.Errorf("delete assistant conversation %d: %w", id, ErrNotFound)
	}
	return nil
}

const assistantMessageColumns = `id, conversation_id, role, text, created_at`

// AddAssistantMessage appends a message to conversation conversationID and
// marks it active now.
func (s *Store) AddAssistantMessage(ctx context.Context, conversationID int64, role, text string) (AssistantMessage, error) {
	var m AssistantMessage
	err := withTx(ctx, s.db, func(tx *sql.Tx) error {
		err := tx.QueryRowContext(ctx,
			`INSERT INTO assistant_messages (conversation_id, role, text) VALUES (?, ?, ?) RETURNING `+assistantMessageColumns,
			conversationID, role, text).Scan(&m.ID, &m.ConversationID, &m.Role, &m.Text, &m.CreatedAt)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `UPDATE assistant_conversations SET updated_at = ? WHERE id = ?`, formatTime(m.CreatedAt), conversationID)
		return err
	})
	if err != nil {
		return AssistantMessage{}, fmt.Errorf("add message to assistant conversation %d: %w", conversationID, err)
	}
	return m, nil
}

// ListAssistantMessages returns every message of conversationID, oldest
// first.
func (s *Store) ListAssistantMessages(ctx context.Context, conversationID int64) ([]AssistantMessage, error) {
	return s.queryAssistantMessages(ctx,
		`SELECT `+assistantMessageColumns+` FROM assistant_messages WHERE conversation_id = ? ORDER BY id`, conversationID)
}

// RecentAssistantMessages returns the last n messages of conversationID,
// oldest first.
func (s *Store) RecentAssistantMessages(ctx context.Context, conversationID int64, n int) ([]AssistantMessage, error) {
	return s.queryAssistantMessages(ctx,
		`SELECT * FROM (SELECT `+assistantMessageColumns+` FROM assistant_messages WHERE conversation_id = ? ORDER BY id DESC LIMIT ?)
		 ORDER BY id`, conversationID, n)
}

func (s *Store) queryAssistantMessages(ctx context.Context, query string, args ...any) ([]AssistantMessage, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list assistant messages: %w", err)
	}
	defer rows.Close()
	var out []AssistantMessage
	for rows.Next() {
		var m AssistantMessage
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Role, &m.Text, &m.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan assistant message: %w", err)
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

// AddAssistantToolCalls saves calls for conversationID, attached to reply
// messageID (0: the reply failed). It saves all or none.
func (s *Store) AddAssistantToolCalls(ctx context.Context, conversationID, messageID int64, calls []ChatToolCall) error {
	if len(calls) == 0 {
		return nil
	}
	var msg any // NULL
	if messageID != 0 {
		msg = messageID
	}
	err := withTx(ctx, s.db, func(tx *sql.Tx) error {
		for _, c := range calls {
			if _, err := tx.ExecContext(ctx,
				`INSERT INTO assistant_tool_calls (conversation_id, message_id, tool, detail) VALUES (?, ?, ?, ?)`,
				conversationID, msg, c.Tool, c.Detail); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("add tool calls to assistant conversation %d: %w", conversationID, err)
	}
	return nil
}

// ListAssistantToolCalls returns conversationID's tool calls in order.
func (s *Store) ListAssistantToolCalls(ctx context.Context, conversationID int64) ([]ChatToolCall, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, conversation_id, message_id, tool, detail, created_at
		   FROM assistant_tool_calls WHERE conversation_id = ? ORDER BY id`, conversationID)
	if err != nil {
		return nil, fmt.Errorf("list tool calls of assistant conversation %d: %w", conversationID, err)
	}
	defer rows.Close()
	var out []ChatToolCall
	for rows.Next() {
		var (
			c   ChatToolCall
			msg sql.NullInt64
		)
		if err := rows.Scan(&c.ID, &c.ConversationID, &msg, &c.Tool, &c.Detail, &c.CreatedAt); err != nil {
			return nil, fmt.Errorf("scan assistant tool call: %w", err)
		}
		c.MessageID = msg.Int64
		out = append(out, c)
	}
	return out, rows.Err()
}
