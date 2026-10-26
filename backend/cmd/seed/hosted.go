package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"kinlight/internal/auth"
	"kinlight/internal/calendar"
	"kinlight/internal/hosteddemo"
	"kinlight/internal/store"
	"kinlight/internal/widgets"
)

func provisionHosted(ctx context.Context, dir string, reset bool, confirm, parentPassword, kidPassword string) error {
	if len(parentPassword) < 12 || len(kidPassword) < 12 || parentPassword == kidPassword {
		return fmt.Errorf("provide distinct demo parent and kid passwords of at least 12 characters")
	}
	parentHash, err := auth.HashPassword(parentPassword)
	if err != nil {
		return err
	}
	kidHash, err := auth.HashPassword(kidPassword)
	if err != nil {
		return err
	}
	return hosteddemo.Replace(dir, reset, confirm, func(path string) error {
		st, err := store.Open(ctx, path)
		if err != nil {
			return err
		}
		defer st.Close()
		fam := demoFamilies[0]
		users, err := createFamily(ctx, st, fam, parentHash, kidHash)
		if err != nil {
			return err
		}
		now := time.Now()
		date, _ := hosteddemo.Day(now)
		for i, m := range fam.members {
			// Copy sample conversations before labeling: the shared local seed stays unchanged.
			m.chats = append([]demoChat(nil), m.chats...)
			for j := range m.chats {
				m.chats[j].title = "Sample: " + m.chats[j].title
			}
			u := users[i]
			if _, err = fillMemberWithCalendar(ctx, st, u, m, now, func(m demoMember) (string, string, error) { return calendar.FixtureLink(m.calendar), "synthetic", nil }); err != nil {
				return err
			}
			if err = widgets.PrefillDemoBriefing(ctx, st, u, now); err != nil {
				return err
			}
			chores := []string{"Put away clean dishes", "Tidy your space"}
			if !m.parent {
				chores = []string{"Pack your school bag", "Put laundry in the basket"}
			}
			for _, title := range chores {
				if err = st.MutateChore(ctx, u.FamilyID, u.ID, users[0].ID, 0, "create", title, false, nil); err != nil {
					return err
				}
			}
			list, err := st.ListChores(ctx, u.FamilyID, u.ID, date)
			if err != nil {
				return err
			}
			if i%2 == 0 && len(list) > 0 {
				if err = st.MutateChore(ctx, u.FamilyID, u.ID, u.ID, list[0].ID, "completion", "", true, func() (string, error) { return date, nil }); err != nil {
					return err
				}
			}
		}
		var integrity string
		if err = st.DB().QueryRowContext(ctx, "PRAGMA integrity_check").Scan(&integrity); err != nil {
			return err
		}
		if strings.TrimSpace(integrity) != "ok" {
			return fmt.Errorf("demo integrity check failed")
		}
		if _, err = st.DB().ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil {
			return err
		}
		return st.Close()
	})
}
