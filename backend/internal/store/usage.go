package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

// LLMUsage is one AI model answer's token usage. MemberID, ViewerID and
// FamilyID are zero when the call couldn't be attributed to anyone; they are
// stored as NULL.
type LLMUsage struct {
	CreatedAt        time.Time // zero means now
	FamilyID         int64
	MemberID         int64 // whose content the call produced
	ViewerID         int64 // who caused it
	Feature          string
	Step             string
	Model            string
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// InsertLLMUsage saves one usage entry.
func (s *Store) InsertLLMUsage(ctx context.Context, u LLMUsage) error {
	createdAt := u.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO llm_usage (created_at, family_id, member_id, viewer_id, feature, step, model,
		                        prompt_tokens, completion_tokens, total_tokens)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		formatTime(createdAt), nullID(u.FamilyID), nullID(u.MemberID), nullID(u.ViewerID),
		u.Feature, u.Step, u.Model, u.PromptTokens, u.CompletionTokens, u.TotalTokens)
	if err != nil {
		return fmt.Errorf("insert llm usage: %w", err)
	}
	return nil
}

// MemberUsage is one member's usage of one feature over a time window.
type MemberUsage struct {
	MemberID         int64
	Feature          string
	Calls            int
	PromptTokens     int
	CompletionTokens int
	TotalTokens      int
}

// UsageSummary totals familyID's usage since since, per member and feature,
// ordered by member then feature. Unattributed rows (no member) aren't
// included: they belong to no one in the family.
func (s *Store) UsageSummary(ctx context.Context, familyID int64, since time.Time) ([]MemberUsage, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT member_id, feature, count(*), sum(prompt_tokens), sum(completion_tokens), sum(total_tokens)
		   FROM llm_usage
		  WHERE family_id = ? AND member_id IS NOT NULL AND created_at >= ?
		  GROUP BY member_id, feature
		  ORDER BY member_id, feature`,
		familyID, formatTime(since))
	if err != nil {
		return nil, fmt.Errorf("summarize llm usage: %w", err)
	}
	defer rows.Close()

	var out []MemberUsage
	for rows.Next() {
		var u MemberUsage
		if err := rows.Scan(&u.MemberID, &u.Feature, &u.Calls, &u.PromptTokens, &u.CompletionTokens, &u.TotalTokens); err != nil {
			return nil, fmt.Errorf("scan llm usage: %w", err)
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// nullID stores a zero id as NULL, which the foreign keys allow, rather than
// as 0, which they'd reject.
func nullID(id int64) sql.NullInt64 {
	return sql.NullInt64{Int64: id, Valid: id != 0}
}
