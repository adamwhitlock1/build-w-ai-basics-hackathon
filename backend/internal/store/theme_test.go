package store_test

import (
	"context"
	"errors"
	"testing"

	"kinlight/internal/store"
)

func TestSetTheme(t *testing.T) {
	ctx := context.Background()
	s := openTestStore(t)
	maya := signUp(t, s, "maya@example.com")
	sam := signUp(t, s, "sam@example.com")

	if maya.Theme != store.ThemeAuto {
		t.Fatalf("a new member's theme = %q, want %q", maya.Theme, store.ThemeAuto)
	}

	got, err := s.SetTheme(ctx, maya.ID, store.ThemeDark)
	if err != nil || got.Theme != store.ThemeDark || got.ID != maya.ID {
		t.Fatalf("SetTheme(dark) = %+v, %v", got, err)
	}
	if again, err := s.GetUser(ctx, maya.ID); err != nil || again.Theme != store.ThemeDark {
		t.Errorf("after saving, the theme reads back as %q, %v", again.Theme, err)
	}
	if other, err := s.GetUser(ctx, sam.ID); err != nil || other.Theme != store.ThemeAuto {
		t.Errorf("another member's theme became %q, %v", other.Theme, err)
	}

	if _, err := s.SetTheme(ctx, maya.ID, "sepia"); err == nil {
		t.Error("an unknown theme was saved")
	}
	if again, _ := s.GetUser(ctx, maya.ID); again.Theme != store.ThemeDark {
		t.Errorf("a refused theme changed the saved one to %q", again.Theme)
	}

	if _, err := s.SetTheme(ctx, 9999, store.ThemeLight); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("SetTheme for no such user: %v, want ErrNotFound", err)
	}
}
