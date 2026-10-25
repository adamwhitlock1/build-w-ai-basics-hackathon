// Package chores owns daily household assignments and their access rules.
package chores

import (
	"context"
	"errors"
	"time"

	"kinlight/internal/store"
)

const TimeZone = "America/Denver"

var ErrForbidden = errors.New("Only parents can manage family chores.")
var ErrStale = errors.New("A new chore day has started. Reload today and check again.")
var ErrDate = errors.New("A valid displayed chore date is required.")

type Day struct {
	Date        string    `json:"date"`
	TimeZone    string    `json:"timeZone"`
	ServerNow   time.Time `json:"serverNow"`
	NextResetAt time.Time `json:"nextResetAt"`
}
type List struct {
	Day
	Items          []store.Chore `json:"items"`
	CompletedCount int           `json:"completedCount"`
	TotalCount     int           `json:"totalCount"`
}
type Summary struct {
	Day
	Members []store.ChoreProgress `json:"members"`
}
type Service struct {
	store *store.Store
	now   func() time.Time
	zone  *time.Location
}

func New(st *store.Store, now func() time.Time) *Service {
	zone, err := time.LoadLocation(TimeZone)
	if err != nil {
		panic(err)
	}
	if now == nil {
		now = time.Now
	}
	return &Service{st, now, zone}
}
func (s *Service) day() Day {
	now := s.now().In(s.zone)
	midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, s.zone)
	return Day{now.Format(time.DateOnly), TimeZone, now, midnight.AddDate(0, 0, 1)}
}
func authorize(actor, subject store.User, manage bool) error {
	if actor.ID == 0 || subject.ID == 0 || actor.FamilyID != subject.FamilyID {
		return store.ErrNotFound
	}
	if !actor.IsAdmin && (manage || actor.ID != subject.ID) {
		return ErrForbidden
	}
	return nil
}
func (s *Service) List(ctx context.Context, actor, subject store.User) (List, error) {
	if err := authorize(actor, subject, false); err != nil {
		return List{}, err
	}
	return s.list(ctx, subject, s.day())
}
func (s *Service) list(ctx context.Context, subject store.User, day Day) (List, error) {
	items, err := s.store.ListChores(ctx, subject.FamilyID, subject.ID, day.Date)
	r := List{Day: day, Items: items, TotalCount: len(items)}
	for _, c := range items {
		if c.Completed {
			r.CompletedCount++
		}
	}
	return r, err
}
func (s *Service) Mutate(ctx context.Context, actor, subject store.User, id int64, operation, title, date string, completed bool) (List, error) {
	if err := authorize(actor, subject, operation != "completion"); err != nil {
		return List{}, err
	}
	if operation == "completion" {
		if _, err := time.Parse(time.DateOnly, date); err != nil {
			return List{}, ErrDate
		}
	}
	day := s.day()
	err := s.store.MutateChore(ctx, actor.FamilyID, subject.ID, actor.ID, id, operation, title, completed, func() (string, error) {
		day = s.day()
		if day.Date != date {
			return "", ErrStale
		}
		return day.Date, nil
	})
	if err != nil {
		return List{}, err
	}
	return s.list(ctx, subject, day)
}
func (s *Service) Summary(ctx context.Context, actor store.User) (Summary, error) {
	if !actor.IsAdmin {
		return Summary{}, ErrForbidden
	}
	day := s.day()
	members, err := s.store.ChoreSummary(ctx, actor.FamilyID, day.Date)
	return Summary{day, members}, err
}
