// Command seed fills a local database with test families, so there's always
// an account to log in with when trying things out:
//
//	make seed                                  # into kinlight.db (or $DB_PATH)
//	go run ./cmd/seed -db kinlight.db.test   # into another file
//	make demo                                  # the demo family, into kinlight.db.demo
//
// Without -demo it creates a parent and a kid in one family, through the
// same store calls the app uses. If the parent already exists it changes
// nothing, so it's safe to run again.
//
// With -demo it rebuilds the demo families from scratch (see demo.go): a
// whole family with profiles, calendars, described widgets, a custom page
// and remembered results, plus a second family for isolation checks.
//
// LOCAL ONLY. The credentials are in this file, so they're public with the
// repository. Never run this against a deployed database.
package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"kinlight/internal/auth"
	"kinlight/internal/store"
)

// The test accounts. The kid uses a "+kid" address, which Gmail delivers to
// the same inbox, though nothing here sends email.
const (
	parentEmail = "contact.codestuff@gmail.com"
	kidEmail    = "contact.codestuff+kid@gmail.com"
	password    = "password1"
	familyName  = "Codestuff Test Family"
)

func main() {
	defaultDB := os.Getenv("DB_PATH")
	if defaultDB == "" {
		defaultDB = "kinlight.db"
	}
	dbPath := flag.String("db", "", "SQLite database file to seed (default $DB_PATH or kinlight.db; kinlight.db.demo with -demo)")
	demo := flag.Bool("demo", false, "rebuild the demo families instead of adding the test family")
	hostedDir := flag.String("hosted-dir", "", "provision a dedicated kinlight-demo directory")
	reset := flag.Bool("hosted-reset", false, "reset stopped hosted demo content")
	confirm := flag.String("confirm-dir", "", "exact directory confirmation for hosted reset")
	flag.Parse()

	var err error
	if *hostedDir != "" {
		err = provisionHosted(context.Background(), *hostedDir, *reset, *confirm, os.Getenv("KINLIGHT_DEMO_PARENT_PASSWORD"), os.Getenv("KINLIGHT_DEMO_KID_PASSWORD"))
	} else if *demo {
		if *dbPath == "" {
			*dbPath = defaultDemoDB
		}
		err = seedDemo(context.Background(), *dbPath, os.Stdout)
	} else {
		if *dbPath == "" {
			*dbPath = defaultDB
		}
		err = seed(context.Background(), *dbPath)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "seed:", err)
		os.Exit(1)
	}
}

func seed(ctx context.Context, dbPath string) error {
	st, err := store.Open(ctx, dbPath) // applies any migrations it hasn't seen
	if err != nil {
		return err
	}
	defer st.Close()

	switch _, err := st.GetUserByEmail(ctx, parentEmail); {
	case err == nil:
		fmt.Printf("%s already has the test family; nothing to do.\n", dbPath)
		return nil
	case !errors.Is(err, store.ErrNotFound):
		return err
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return err
	}

	// The parent, as sign-up and the create-family intro make them.
	parent, err := st.CreateFamilyWithAdmin(ctx, parentEmail, hash)
	if err != nil {
		return fmt.Errorf("create parent: %w", err)
	}
	err = st.CompleteFamilySetup(ctx, store.FamilySetup{
		FamilyID: parent.FamilyID,
		Name:     familyName,
		Profile:  store.MemberProfile{UserID: parent.ID, Interests: "space launches, soccer, easy weeknight dinners", Location: "Denver, CO"},
	})
	if err != nil {
		return fmt.Errorf("set up family: %w", err)
	}

	// The kid, as an accepted invite makes them.
	token, err := randomToken()
	if err != nil {
		return err
	}
	_, _, err = st.InviteToFamily(ctx, store.Invite{
		FamilyID:  parent.FamilyID,
		Email:     kidEmail,
		Token:     token,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	})
	if err != nil {
		return fmt.Errorf("invite kid: %w", err)
	}
	if _, err := st.AcceptInvite(ctx, token, hash); err != nil {
		return fmt.Errorf("accept kid's invite: %w", err)
	}

	fmt.Printf("Seeded %s with %q:\n  parent  %s\n  kid     %s\n  (password %q for both; local testing only)\n",
		dbPath, familyName, parentEmail, kidEmail, password)
	return nil
}

func randomToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
