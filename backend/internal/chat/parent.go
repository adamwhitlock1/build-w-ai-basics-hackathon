package chat

import (
	"context"
	"errors"

	"porchlight/internal/store"
)

// reviewedShown is how many reviewed flags the review queue includes, after
// every unreviewed one.
const reviewedShown = 50

// What parents can do. parent is the logged-in user; kid is the family
// member the request is about, already checked to be in parent's family
// (auth.RequireMember does that for the HTTP routes).

// checkParentOfKid refuses unless parent is a parent and kid is a kid in
// parent's family.
func checkParentOfKid(parent, kid store.User) error {
	switch {
	case !parent.IsAdmin:
		return ErrNotParent
	case kid.IsAdmin || kid.FamilyID != parent.FamilyID:
		return ErrNoSuchKid
	}
	return nil
}

// ListFor returns kid's conversations for parent, most recently active
// first, with each one's unreviewed flag count.
func (s *Service) ListFor(ctx context.Context, parent, kid store.User) ([]store.Conversation, error) {
	if err := checkParentOfKid(parent, kid); err != nil {
		return nil, err
	}
	return s.store.ListConversations(ctx, kid.FamilyID, kid.ID)
}

// Transcript returns one of kid's conversations for parent, with every
// message and its flags. Reading changes nothing.
func (s *Service) Transcript(ctx context.Context, parent, kid store.User, id int64) (Transcript, error) {
	if err := checkParentOfKid(parent, kid); err != nil {
		return Transcript{}, err
	}
	return s.transcript(ctx, kid, id, true)
}

// Delete permanently deletes one of kid's conversations, with its messages
// and flags. AI usage records stay: they hold no content.
func (s *Service) Delete(ctx context.Context, parent, kid store.User, id int64) error {
	if err := checkParentOfKid(parent, kid); err != nil {
		return err
	}
	err := s.store.DeleteConversation(ctx, kid.FamilyID, kid.ID, id)
	if errors.Is(err, store.ErrNotFound) {
		return ErrNoSuchChat
	}
	if err != nil {
		return err
	}
	s.logger.InfoContext(ctx, "chat deleted", "member_id", kid.ID, "chat_id", id, "parent_id", parent.ID)
	return nil
}

// Reviews returns parent's family's review queue: unreviewed flags first
// (crises, then by severity, newest first), then the most recently
// reviewed ones.
func (s *Service) Reviews(ctx context.Context, parent store.User) ([]store.ChatFlag, error) {
	if !parent.IsAdmin {
		return nil, ErrNotParent
	}
	return s.store.ReviewQueue(ctx, parent.FamilyID, reviewedShown)
}

// MarkReviewed records that parent reviewed flag id. A flag outside
// parent's family is ErrNoSuchFlag.
func (s *Service) MarkReviewed(ctx context.Context, parent store.User, id int64) (store.ChatFlag, error) {
	if !parent.IsAdmin {
		return store.ChatFlag{}, ErrNotParent
	}
	f, err := s.store.MarkFlagReviewed(ctx, parent.FamilyID, id, parent.ID)
	if errors.Is(err, store.ErrNotFound) {
		return store.ChatFlag{}, ErrNoSuchFlag
	}
	if err != nil {
		return store.ChatFlag{}, err
	}
	s.logger.InfoContext(ctx, "chat flag reviewed", "flag_id", id, "member_id", f.MemberID, "parent_id", parent.ID)
	return f, nil
}

// Alerts is what every parent page shows: how many flags are waiting, and
// any crisis no parent has reviewed yet.
type Alerts struct {
	Unreviewed int
	Crises     []store.ChatFlag // newest first
}

// Alerts returns parent's family's alerts.
func (s *Service) Alerts(ctx context.Context, parent store.User) (Alerts, error) {
	if !parent.IsAdmin {
		return Alerts{}, ErrNotParent
	}
	n, err := s.store.CountUnreviewedFlags(ctx, parent.FamilyID)
	if err != nil {
		return Alerts{}, err
	}
	crises, err := s.store.UnreviewedCrises(ctx, parent.FamilyID)
	if err != nil {
		return Alerts{}, err
	}
	return Alerts{Unreviewed: n, Crises: crises}, nil
}

// BirthYear returns kid's birth year for parent, and false if none is set.
func (s *Service) BirthYear(ctx context.Context, parent, kid store.User) (int, bool, error) {
	if err := checkParentOfKid(parent, kid); err != nil {
		return 0, false, err
	}
	return s.store.GetBirthYear(ctx, kid.ID)
}

// SetBirthYear sets kid's birth year (0 clears it), which picks the
// agent's persona. Only a parent can: kids can't change their own age.
func (s *Service) SetBirthYear(ctx context.Context, parent, kid store.User, year int) error {
	if err := checkParentOfKid(parent, kid); err != nil {
		return err
	}
	if year != 0 {
		age := s.now().Year() - year
		if age < MinKidAge || age > MaxKidAge {
			return ErrInvalidBirthYear
		}
	}
	if err := s.store.SetBirthYear(ctx, kid.ID, year); err != nil {
		return err
	}
	s.logger.InfoContext(ctx, "birth year set", "member_id", kid.ID, "parent_id", parent.ID, "cleared", year == 0)
	return nil
}
