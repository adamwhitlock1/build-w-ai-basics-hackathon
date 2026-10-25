// Package hosteddemo owns the opt-in judge sandbox, never household storage.
package hosteddemo

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

const marker = "Kinlight hosted judge demo v1\n"
const ContentName = "judge-content.sqlite"

// Directory accepts only a dedicated directory whose ancestry contains no symlinks.
func Directory(dir string) error {
	if !filepath.IsAbs(dir) || filepath.Clean(dir) != dir || filepath.Base(dir) != "kinlight-demo" {
		return errors.New("demo directory must be an absolute clean path ending in kinlight-demo")
	}
	for p := dir; ; p = filepath.Dir(p) {
		st, err := os.Lstat(p)
		if err != nil {
			return err
		}
		if st.Mode()&os.ModeSymlink != 0 || !st.IsDir() {
			return errors.New("demo directory and ancestors must be real directories")
		}
		if p == filepath.Dir(p) {
			break
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.Type()&os.ModeSymlink != 0 {
			return errors.New("demo directory must not contain symlinks")
		}
		switch e.Name() {
		case ".judge-demo", ".lock", ContentName, ContentName + "-wal", ContentName + "-shm", "quota.sqlite", "quota.sqlite-wal", "quota.sqlite-shm":
		default:
			return fmt.Errorf("unexpected file in demo directory: %s", e.Name())
		}
	}
	return nil
}

func Validate(dir string) error {
	if err := Directory(dir); err != nil {
		return err
	}
	b, err := os.ReadFile(filepath.Join(dir, ".judge-demo"))
	if err != nil {
		return err
	}
	if string(b) != marker {
		return errors.New("not a marked hosted demo directory")
	}
	st, err := os.Stat(filepath.Join(dir, ContentName))
	if err != nil {
		return err
	}
	if !st.Mode().IsRegular() {
		return errors.New("demo content is not a regular database file")
	}
	return nil
}

// Lock is held for the entire server lifetime, or the complete provisioning operation.
func Lock(dir string) (*os.File, error) {
	if err := Directory(dir); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(dir, ".lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		return nil, errors.New("demo is in use; stop its server before provisioning or resetting")
	}
	return f, nil
}

// Replace builds off to the side, then swaps only the marked demo content.
// build must close/checkpoint its SQLite connection before returning.
func Replace(dir string, reset bool, confirm string, build func(string) error) error {
	if err := Directory(dir); err != nil {
		return err
	}
	if reset {
		if confirm != dir {
			return errors.New("reset requires confirmation of the exact demo directory")
		}
		if err := Validate(dir); err != nil {
			return err
		}
	} else {
		entries, _ := os.ReadDir(dir)
		if len(entries) != 0 && !(len(entries) == 1 && entries[0].Name() == ".lock") {
			return errors.New("initial provisioning requires an empty demo directory")
		}
	}
	lock, err := Lock(dir)
	if err != nil {
		return err
	}
	defer lock.Close()
	// A stopped, clean SQLite database has no outstanding WAL. Refuse rather than discard it.
	for _, suffix := range []string{"-wal", "-shm"} {
		if st, err := os.Stat(filepath.Join(dir, ContentName) + suffix); err == nil && st.Size() > 0 {
			return errors.New("demo has SQLite sidecars; recover/checkpoint it before reset")
		}
	}
	stage, err := os.MkdirTemp(dir, ".judge-build-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	fresh := filepath.Join(stage, ContentName)
	if err = build(fresh); err != nil {
		return err
	}
	if st, err := os.Stat(fresh); err != nil || st.Size() == 0 {
		return errors.New("builder did not produce demo content")
	}
	if err = os.Rename(fresh, filepath.Join(dir, ContentName)); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, ".judge-demo"), []byte(marker), 0600)
}
