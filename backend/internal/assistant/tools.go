package assistant

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"porchlight/internal/aikit"
	"porchlight/internal/chat"
	"porchlight/internal/sources"
	"porchlight/internal/store"
)

// Limits on what kids_chats hands the model.
const (
	maxTranscriptMessages = 40
	maxMessageChars       = 500
	maxConversationsShown = 10
)

// ToolSources is Tools on Porchlight's own data: the family's members, each
// member's calendar, weather and picks (through kid chat's ToolSources,
// which work for any member), unfiltered web search, and the kids' chats
// through chat.Service's parent operations, which check permissions.
type ToolSources struct {
	Members interface {
		ListFamilyMembers(ctx context.Context, familyID int64) ([]store.User, error)
	}
	Lookups *chat.ToolSources
	Web     sources.Source // searched with sources.Everyone
	Chats   *chat.Service
}

// Search runs an unfiltered web search, keeping links.
func (t *ToolSources) Search(ctx context.Context, query string) ([]aikit.SearchResult, error) {
	results, err := t.Web.Search(ctx, query, sources.Everyone)
	if err != nil {
		return nil, err
	}
	out := make([]aikit.SearchResult, len(results))
	for i, r := range results {
		out[i] = aikit.SearchResult{Title: r.Title, URL: r.URL, Site: r.Site, Date: r.Date, Snippet: r.Snippet}
	}
	return out, nil
}

// Weather returns the forecast at parent's saved location.
func (t *ToolSources) Weather(ctx context.Context, parent store.User) (string, error) {
	return t.Lookups.Weather(ctx, parent)
}

// MyPicks returns parent's interests and page picks.
func (t *ToolSources) MyPicks(ctx context.Context, parent store.User) (string, error) {
	return t.Lookups.MyPicks(ctx, parent)
}

// FamilyDay returns today's and tomorrow's calendar, in loc, for who in
// parent's family: a member's first name, "me", or "everyone".
func (t *ToolSources) FamilyDay(ctx context.Context, parent store.User, who string, loc *time.Location) (string, error) {
	members, err := t.pick(ctx, parent, who)
	if err != nil {
		return "", err
	}
	if len(members) == 0 {
		return fmt.Sprintf("There's no one named %q in your family.", who), nil
	}
	var b strings.Builder
	for _, m := range members {
		day, err := t.Lookups.MyDay(ctx, m, loc)
		if err != nil {
			day = "Their calendar isn't available right now."
		}
		name := FirstName(m.Email)
		if m.ID == parent.ID {
			name += " (you)"
		}
		fmt.Fprintf(&b, "%s:\n%s\n\n", name, day)
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

// pick finds who among parent's family: everyone, the parent ("me"), or
// the members whose first name matches.
func (t *ToolSources) pick(ctx context.Context, parent store.User, who string) ([]store.User, error) {
	members, err := t.Members.ListFamilyMembers(ctx, parent.FamilyID)
	if err != nil {
		return nil, err
	}
	who = strings.ToLower(strings.TrimSpace(who))
	switch who {
	case "", "everyone", "all", "family", "the family":
		return members, nil
	case "me", "myself", "i":
		return []store.User{parent}, nil
	}
	var out []store.User
	for _, m := range members {
		if strings.ToLower(FirstName(m.Email)) == who {
			out = append(out, m)
		}
	}
	return out, nil
}

// KidsChats reads the kids' chats for parent (see aikit.KidsChatsQuery).
// Everything goes through chat.Service's parent operations, so only
// parent's own family's kids can be read.
func (t *ToolSources) KidsChats(ctx context.Context, parent store.User, q aikit.KidsChatsQuery) (string, error) {
	members, err := t.Members.ListFamilyMembers(ctx, parent.FamilyID)
	if err != nil {
		return "", err
	}
	var kids []store.User
	for _, m := range members {
		if !m.IsAdmin {
			kids = append(kids, m)
		}
	}
	queue, err := t.Chats.Reviews(ctx, parent)
	if err != nil {
		return "", err
	}

	name := strings.TrimSpace(q.Kid)
	if name == "" {
		return t.overview(ctx, parent, kids, queue)
	}
	var kid *store.User
	for i := range kids {
		if strings.EqualFold(FirstName(kids[i].Email), name) {
			kid = &kids[i]
		}
	}
	if kid == nil {
		return fmt.Sprintf("There's no kid named %q in your family.", name), nil
	}
	if strings.TrimSpace(q.Conversation) != "" {
		return t.transcript(ctx, parent, *kid, q.Conversation)
	}
	return t.kidSummary(ctx, parent, *kid, queue)
}

// overview is the family's unreviewed flags and each kid's conversations.
func (t *ToolSources) overview(ctx context.Context, parent store.User, kids []store.User, queue []store.ChatFlag) (string, error) {
	var b strings.Builder
	writeFlags(&b, "Unreviewed flags in the family", queue, func(f store.ChatFlag) bool { return f.ReviewedAt.IsZero() }, kids)
	for _, kid := range kids {
		if err := t.writeConversations(ctx, &b, parent, kid); err != nil {
			return "", err
		}
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

// kidSummary is one kid's flags and conversations.
func (t *ToolSources) kidSummary(ctx context.Context, parent, kid store.User, queue []store.ChatFlag) (string, error) {
	var b strings.Builder
	writeFlags(&b, FirstName(kid.Email)+"'s flags", queue, func(f store.ChatFlag) bool { return f.MemberID == kid.ID }, []store.User{kid})
	if err := t.writeConversations(ctx, &b, parent, kid); err != nil {
		return "", err
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

func (t *ToolSources) writeConversations(ctx context.Context, b *strings.Builder, parent, kid store.User) error {
	convs, err := t.Chats.ListFor(ctx, parent, kid)
	if err != nil {
		return err
	}
	fmt.Fprintf(b, "%s's conversations (%d):\n", FirstName(kid.Email), len(convs))
	for i, c := range convs {
		if i == maxConversationsShown {
			fmt.Fprintf(b, "- and %d older\n", len(convs)-i)
			break
		}
		title := c.Title
		if title == "" {
			title = "(untitled)"
		}
		line := fmt.Sprintf("- [id %d] %q, %s, last active %s", c.ID, title, c.Mode, c.UpdatedAt.Format("Mon Jan 2"))
		if c.UnreviewedFlags > 0 {
			line += fmt.Sprintf(", %d unreviewed flag(s)", c.UnreviewedFlags)
		}
		b.WriteString(line + "\n")
	}
	b.WriteString("\n")
	return nil
}

func writeFlags(b *strings.Builder, label string, queue []store.ChatFlag, keep func(store.ChatFlag) bool, kids []store.User) {
	names := map[int64]string{}
	for _, k := range kids {
		names[k.ID] = FirstName(k.Email)
	}
	var lines []string
	for _, f := range queue {
		if !keep(f) || names[f.MemberID] == "" {
			continue
		}
		status := "unreviewed"
		if !f.ReviewedAt.IsZero() {
			status = "reviewed"
		}
		crisis := ""
		if f.Crisis {
			crisis = ", possible crisis"
		}
		lines = append(lines, fmt.Sprintf("- %s, %s: %s, %s severity%s, %s. %s Message: %q (in conversation id %d, %q)",
			names[f.MemberID], f.CreatedAt.Format("Mon Jan 2"), f.Category, f.Severity, crisis, status, f.Reason, cut(f.MessageText), f.ConversationID, f.ConversationTitle))
	}
	if len(lines) == 0 {
		fmt.Fprintf(b, "%s: none.\n\n", label)
		return
	}
	fmt.Fprintf(b, "%s:\n%s\n\n", label, strings.Join(lines, "\n"))
}

// transcript is one of kid's conversations, found by id (as the listings
// show it) or else by title, with flagged messages marked. Flagged chats
// share the neutral title "A conversation", so the id is what's reliable.
func (t *ToolSources) transcript(ctx context.Context, parent, kid store.User, which string) (string, error) {
	convs, err := t.Chats.ListFor(ctx, parent, kid)
	if err != nil {
		return "", err
	}
	want := strings.ToLower(strings.TrimSpace(which))
	id, idErr := strconv.ParseInt(strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(want, "id"), "#")), 10, 64)
	var found *store.Conversation
	for i := range convs {
		if (idErr == nil && convs[i].ID == id) || (idErr != nil && strings.Contains(strings.ToLower(convs[i].Title), want)) {
			found = &convs[i]
			break
		}
	}
	if found == nil {
		return fmt.Sprintf("%s has no conversation like %q.", FirstName(kid.Email), which), nil
	}
	tr, err := t.Chats.Transcript(ctx, parent, kid, found.ID)
	if err != nil {
		return "", err
	}
	flagged := map[int64]store.ChatFlag{}
	for _, f := range tr.Flags {
		flagged[f.MessageID] = f
	}
	msgs := tr.Messages
	if len(msgs) > maxTranscriptMessages {
		msgs = msgs[len(msgs)-maxTranscriptMessages:]
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%s's conversation %q (%s):\n", FirstName(kid.Email), found.Title, found.Mode)
	for _, m := range msgs {
		who := FirstName(kid.Email)
		if m.Role == store.RoleAgent {
			who = "Porchlight"
		}
		line := fmt.Sprintf("%s: %s", who, cut(m.Text))
		if f, ok := flagged[m.ID]; ok {
			line += fmt.Sprintf(" [flagged: %s, %s]", f.Category, f.Severity)
		}
		b.WriteString(line + "\n")
	}
	return strings.TrimRight(b.String(), "\n"), nil
}

// cut shortens a message for the model.
func cut(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if r := []rune(s); len(r) > maxMessageChars {
		return string(r[:maxMessageChars-1]) + "…"
	}
	return s
}

// FirstName is what Porchlight calls a member: the part of their email
// before the "@", capitalized, as the web app shows it.
func FirstName(email string) string {
	local, _, _ := strings.Cut(email, "@")
	if local == "" {
		return email
	}
	r := []rune(local)
	return strings.ToUpper(string(r[0])) + string(r[1:])
}
