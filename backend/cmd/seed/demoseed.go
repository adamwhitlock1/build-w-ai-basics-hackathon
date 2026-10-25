package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/joho/godotenv"

	"kinlight/internal/auth"
	"kinlight/internal/calendar"
	"kinlight/internal/store"
	"kinlight/internal/widgets"
)

// seedDemo rebuilds the demo families (demoFamilies) in the database at
// dbPath and writes a summary of the accounts to out. Any calendar link set
// in a SEED_CALENDAR_<NAME> variable (from the environment or backend/.env)
// replaces that member's fixture link; those links are never printed.
func seedDemo(ctx context.Context, dbPath string, out io.Writer) error {
	if err := resetDemoDB(dbPath); err != nil {
		return err
	}
	// godotenv never overwrites variables already set, so the shell wins.
	if err := godotenv.Load(".env"); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("read .env: %w", err)
	}

	st, err := store.Open(ctx, dbPath)
	if err != nil {
		return err
	}
	defer st.Close()

	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}
	now := time.Now()

	type row struct{ email, role, calendar string }
	var rows []row
	var inviteLinks []string
	for _, fam := range demoFamilies {
		users, err := createFamily(ctx, st, fam, hash)
		if err != nil {
			return err
		}
		for i, m := range fam.members {
			cal, err := fillMember(ctx, st, users[i], m, now)
			if err != nil {
				return fmt.Errorf("%s: %w", m.email, err)
			}
			rows = append(rows, row{m.email, m.role, cal})
		}
		for _, inv := range fam.invites {
			token, err := randomToken()
			if err != nil {
				return err
			}
			_, _, err = st.InviteToFamily(ctx, store.Invite{
				FamilyID: users[0].FamilyID, Email: inv.email, Token: token, IsAdmin: inv.parent,
				ExpiresAt: now.Add(30 * 24 * time.Hour),
			})
			if err != nil {
				return fmt.Errorf("invite %s: %w", inv.email, err)
			}
			rows = append(rows, row{inv.email, inv.role, "-"})
			inviteLinks = append(inviteLinks, fmt.Sprintf("  %s  %s/invite/%s", inv.email, webOrigin(), token))
		}
	}

	fmt.Fprintf(out, "Rebuilt %s with the demo families (local testing only).\n\n", dbPath)
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "  ACCOUNT\tCALENDAR\tFOR")
	for _, r := range rows {
		fmt.Fprintf(tw, "  %s\t%s\t%s\n", r.email, r.calendar, r.role)
	}
	tw.Flush()
	fmt.Fprintf(out, "\nPassword for every account: %q\n", password)
	if len(inviteLinks) > 0 {
		fmt.Fprintln(out, "\nPending invites (open the link to join):")
		for _, l := range inviteLinks {
			fmt.Fprintln(out, l)
		}
	}
	fmt.Fprintln(out, "\nRun the API with fixture calendars on: make demo-serve")
	return nil
}

// createFamily creates fam's members as the app does: the first through
// sign-up and the create-family intro, the rest through accepted invites.
// It returns their users, in fam.members order.
func createFamily(ctx context.Context, st *store.Store, fam demoFamily, hash []byte, kidHash ...[]byte) ([]store.User, error) {
	creator := fam.members[0]
	if !creator.parent {
		return nil, fmt.Errorf("%s: a family's first member must be a parent", fam.name)
	}
	first, err := st.CreateFamilyWithAdmin(ctx, creator.email, hash)
	if err != nil {
		return nil, fmt.Errorf("create %s: %w", creator.email, err)
	}
	err = st.CompleteFamilySetup(ctx, store.FamilySetup{
		FamilyID: first.FamilyID,
		Name:     fam.name,
		Profile:  store.MemberProfile{UserID: first.ID, Interests: creator.interests, Location: creator.location},
	})
	if err != nil {
		return nil, fmt.Errorf("set up %s: %w", fam.name, err)
	}

	users := []store.User{first}
	for _, m := range fam.members[1:] {
		token, err := randomToken()
		if err != nil {
			return nil, err
		}
		_, _, err = st.InviteToFamily(ctx, store.Invite{
			FamilyID: first.FamilyID, Email: m.email, Token: token, IsAdmin: m.parent,
			ExpiresAt: time.Now().Add(24 * time.Hour),
		})
		if err != nil {
			return nil, fmt.Errorf("invite %s: %w", m.email, err)
		}
		memberHash := hash
		if !m.parent && len(kidHash) > 0 {
			memberHash = kidHash[0]
		}
		u, err := st.AcceptInvite(ctx, token, memberHash)
		if err != nil {
			return nil, fmt.Errorf("accept %s's invite: %w", m.email, err)
		}
		users = append(users, u)
	}
	return users, nil
}

// fillMember gives u everything m describes: profile, calendar, widgets,
// page layout and remembered results, all dated now. It returns how the
// member's calendar was set, for the summary.
func fillMember(ctx context.Context, st *store.Store, u store.User, m demoMember, now time.Time) (calendarNote string, err error) {
	return fillMemberWithCalendar(ctx, st, u, m, now, calendarLink)
}

func fillMemberWithCalendar(ctx context.Context, st *store.Store, u store.User, m demoMember, now time.Time, linkFor func(demoMember) (string, string, error)) (calendarNote string, err error) {
	profile := store.MemberProfile{UserID: u.ID, Interests: m.interests, Location: m.location}
	if err := st.UpsertMemberProfile(ctx, profile); err != nil {
		return "", err
	}

	calendarNote = "none"
	if m.calendar != "" {
		link, note, err := linkFor(m)
		if err != nil {
			return "", err
		}
		if err := st.SetCalendarLink(ctx, u.ID, link); err != nil {
			return "", err
		}
		calendarNote = note
	}

	var widgetIDs []int64
	for _, w := range m.widgets {
		cw, err := widgets.NewCustomWidget(u, w.title, w.source, w.query, w.prompt, w.archetype, w.items(now))
		if err != nil {
			return "", err
		}
		saved, err := st.CreateCustomWidget(ctx, cw)
		if err != nil {
			return "", err
		}
		widgetIDs = append(widgetIDs, saved.ID)
	}
	if m.layout != nil {
		data, err := json.Marshal(m.layout(widgetIDs))
		if err != nil {
			return "", err
		}
		if err := st.UpsertLayout(ctx, u.ID, data); err != nil {
			return "", err
		}
	}

	if m.articles != nil {
		if err := widgets.PrefillArticles(ctx, st, u, profile, m.articles(now), now); err != nil {
			return "", err
		}
	}
	if m.events != nil {
		if err := widgets.PrefillEvents(ctx, st, u, profile, m.events(now), now); err != nil {
			return "", err
		}
	}

	if m.age != 0 {
		if err := st.SetBirthYear(ctx, u.ID, now.Year()-m.age); err != nil {
			return "", err
		}
	}
	for _, c := range m.chats {
		if err := addDemoChat(ctx, st, u, c); err != nil {
			return "", fmt.Errorf("chat %q: %w", c.title, err)
		}
	}
	return calendarNote, nil
}

// addDemoChat stores c as one of u's conversations, flagging the kid's
// turn c.flag names. Nothing is sent to any AI model.
func addDemoChat(ctx context.Context, st *store.Store, u store.User, c demoChat) error {
	conv, err := st.CreateConversation(ctx, u.FamilyID, u.ID, c.mode)
	if err != nil {
		return err
	}
	if err := st.SetConversationTitle(ctx, conv.ID, c.title); err != nil {
		return err
	}
	for i, text := range c.turns {
		role := store.RoleKid
		if i%2 == 1 {
			role = store.RoleAgent
		}
		msg, err := st.AddChatMessage(ctx, conv.ID, role, text)
		if err != nil {
			return err
		}
		if c.flag == nil || c.flag.turn != i {
			continue
		}
		if role != store.RoleKid {
			return fmt.Errorf("turn %d is the agent's; only a kid's message can be flagged", i)
		}
		_, err = st.AddChatFlag(ctx, store.ChatFlag{
			MessageID: msg.ID, ConversationID: conv.ID, FamilyID: u.FamilyID, MemberID: u.ID,
			Category: c.flag.category, Severity: c.flag.severity, Reason: c.flag.reason,
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// calendarLink is m's calendar link: the one in SEED_CALENDAR_<NAME> if
// that's set, or else m's fixture feed. note says which, without the link.
func calendarLink(m demoMember) (link, note string, err error) {
	name := strings.ToUpper(m.calendar)
	variable := "SEED_CALENDAR_" + name
	raw := strings.TrimSpace(os.Getenv(variable))
	if raw == "" {
		return calendar.FixtureLink(m.calendar), "fixture " + m.calendar, nil
	}
	link, err = calendar.NormalizeLink(raw)
	if err != nil {
		// The error names the variable, never its value: it's a secret.
		return "", "", fmt.Errorf("%s isn't an https or webcal calendar link", variable)
	}
	return link, "from " + variable, nil
}

// webOrigin is where the frontend dev server runs, for invite links. It
// reads KINLIGHT_WEB_PORT, as frontend/vite.config.ts does.
func webOrigin() string {
	port := os.Getenv("KINLIGHT_WEB_PORT")
	if port == "" {
		port = "5174"
	}
	return "http://localhost:" + port
}

// resetDemoDB makes dbPath ready for a fresh demo. A file whose name ends in
// ".demo" is disposable, so it's deleted, with SQLite's -wal and -shm side
// files. Any other file is only used if it has no users yet: the demo must
// never be written over a database someone is using.
func resetDemoDB(dbPath string) error {
	if strings.HasSuffix(dbPath, ".demo") {
		for _, p := range []string{dbPath, dbPath + "-wal", dbPath + "-shm"} {
			if err := os.Remove(p); err != nil && !errors.Is(err, fs.ErrNotExist) {
				return fmt.Errorf("remove old demo database: %w", err)
			}
		}
		return nil
	}

	if _, err := os.Stat(dbPath); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	users, err := countUsers(dbPath)
	if err != nil {
		return err
	}
	if users > 0 {
		return fmt.Errorf("%s already has %d users; the demo only replaces files ending in .demo (try -db kinlight.db.demo)", dbPath, users)
	}
	return nil
}

// countUsers counts the accounts in the database at dbPath, opening it read
// only so that checking changes nothing. A database without a users table
// has none.
func countUsers(dbPath string) (int, error) {
	// The same escaping as store.Open: in a "file:" URI, '?', '#' and '%'
	// are special.
	escaped := strings.NewReplacer("%", "%25", "?", "%3f", "#", "%23").Replace(dbPath)
	db, err := sql.Open("sqlite", "file:"+escaped+"?mode=ro")
	if err != nil {
		return 0, err
	}
	defer db.Close()
	var n int
	err = db.QueryRow(`SELECT count(*) FROM users`).Scan(&n)
	if err != nil && strings.Contains(err.Error(), "no such table") {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("check %s for users: %w", dbPath, err)
	}
	return n, nil
}
