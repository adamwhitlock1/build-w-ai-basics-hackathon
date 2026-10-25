package main

import (
	"strings"
	"testing"
)

func TestCheckOldNames(t *testing.T) {
	t.Run("new names pass", func(t *testing.T) {
		t.Setenv("KINLIGHT_DEV_FIXTURES", "1")
		t.Setenv("DB_PATH", "kinlight.db.demo")
		if err := checkOldNames(); err != nil {
			t.Fatal(err)
		}
	})

	for _, s := range oldSettings {
		t.Run(s.old, func(t *testing.T) {
			t.Setenv(s.old, "secret-value")
			err := checkOldNames()
			if err == nil || !strings.Contains(err.Error(), s.now) {
				t.Fatalf("err = %v, want it to name %s", err, s.now)
			}
			if strings.Contains(err.Error(), "secret-value") {
				t.Errorf("err = %v; it shows the setting's value", err)
			}
		})
	}

	t.Run("set but empty still counts", func(t *testing.T) {
		t.Setenv("PORCHLIGHT_BRIEFER", "")
		if err := checkOldNames(); err == nil {
			t.Fatal("an empty old variable passed")
		}
	})

	for _, path := range []string{"porchlight.db", "porchlight.db.demo", "/srv/data/porchlight.db"} {
		t.Run("DB_PATH="+path, func(t *testing.T) {
			t.Setenv("DB_PATH", path)
			if err := checkOldNames(); err == nil || !strings.Contains(err.Error(), "kinlight.db") {
				t.Fatalf("err = %v, want it to say to rename to kinlight.db", err)
			}
		})
	}
}
