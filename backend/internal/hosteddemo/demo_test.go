package hosteddemo

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func directory(t *testing.T) string {
	t.Helper()
	base, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(base, "kinlight-demo")
	if err = os.Mkdir(dir, 0700); err != nil {
		t.Fatal(err)
	}
	return dir
}
func buildFile(path string) error {
	return os.WriteFile(path, []byte("valid demo for storage test"), 0600)
}
func TestStorageBoundaries(t *testing.T) {
	dir := directory(t)
	if err := Validate(dir); err == nil {
		t.Fatal("accepted unmarked directory")
	}
	if err := Replace(dir, false, "", buildFile); err != nil {
		t.Fatal(err)
	}
	if err := Validate(dir); err != nil {
		t.Fatal(err)
	}
	q, err := OpenQuota(dir, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer q.Close()
	if err = q.Reserve(context.Background(), "model"); err != nil {
		t.Fatal(err)
	}
	lock, err := Lock(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err = Replace(dir, true, dir, buildFile); err == nil {
		t.Fatal("reset while running")
	}
	lock.Close()
	original, _ := os.ReadFile(filepath.Join(dir, ContentName))
	if err = Replace(dir, true, dir, func(string) error { return errors.New("failed build") }); err == nil {
		t.Fatal("expected failure")
	}
	current, _ := os.ReadFile(filepath.Join(dir, ContentName))
	if string(original) != string(current) {
		t.Fatal("failure changed content")
	}
	if err = Replace(dir, true, "wrong", buildFile); err == nil {
		t.Fatal("missing confirmation")
	}
	if err = Replace(dir, true, dir, buildFile); err != nil {
		t.Fatal(err)
	}
	if err = q.Reserve(context.Background(), "model"); err == nil {
		t.Fatal("reset gave extra credit")
	}
	if err = os.WriteFile(filepath.Join(dir, "kinlight.db"), []byte("household"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = Replace(dir, true, dir, buildFile); err == nil {
		t.Fatal("accepted household file")
	}
	data, _ := os.ReadFile(filepath.Join(dir, "kinlight.db"))
	if string(data) != "household" {
		t.Fatal("household modified")
	}
}
func TestSymlinkRejected(t *testing.T) {
	dir := directory(t)
	if err := os.Symlink(filepath.Join(dir, "outside"), filepath.Join(dir, ContentName)); err != nil {
		t.Fatal(err)
	}
	if err := Directory(dir); err == nil {
		t.Fatal("accepted symlink")
	}
}
func TestQuotaConcurrentPersistentAndDST(t *testing.T) {
	dir := directory(t)
	q, err := OpenQuota(dir, 3, 2)
	if err != nil {
		t.Fatal(err)
	}
	var passed atomic.Int64
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			if q.Reserve(context.Background(), "model") == nil {
				passed.Add(1)
			}
		})
	}
	wg.Wait()
	if passed.Load() != 3 {
		t.Fatalf("%d calls passed", passed.Load())
	}
	q.Close()
	q, err = OpenQuota(dir, 3, 2)
	if err != nil {
		t.Fatal(err)
	}
	defer q.Close()
	if err = q.Reserve(context.Background(), "model"); err == nil {
		t.Fatal("restart reset quota")
	}
	for _, test := range []struct {
		month      time.Month
		day, hours int
	}{{time.March, 8, 23}, {time.November, 1, 25}} {
		now := time.Date(2026, test.month, test.day, 0, 0, 0, 0, denver)
		_, reset := Day(now)
		if reset.Sub(now) != time.Duration(test.hours)*time.Hour {
			t.Fatal("wrong DST reset")
		}
	}
	q.Now = func() time.Time { return time.Now().AddDate(0, 0, 1) }
	if err = q.Reserve(context.Background(), "model"); err != nil {
		t.Fatal(err)
	}
	q.Close()
	if err = q.Reserve(context.Background(), "model"); err == nil {
		t.Fatal("closed ledger allowed dispatch")
	}
}
func TestTransportAttemptsOutputBoundAndStickyFailure(t *testing.T) {
	q, err := OpenQuota(directory(t), 2, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer q.Close()
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Method == "POST" {
			var body map[string]any
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				t.Error(err)
			}
			if body["max_tokens"] != float64(2048) {
				t.Errorf("uncapped: %v", body)
			}
		}
		w.Write([]byte(`{}`))
	}))
	defer server.Close()
	client := &http.Client{Transport: &Transport{Quota: q, Kind: "model"}}
	for i := range 3 {
		r, _ := http.NewRequest("POST", server.URL, strings.NewReader(`{"max_tokens":9999}`))
		res, err := client.Do(r)
		if i < 2 && err != nil {
			t.Fatal(err)
		}
		if res != nil {
			io.Copy(io.Discard, res.Body)
			res.Body.Close()
		}
		if i == 2 && err == nil {
			t.Fatal("exceeded limit")
		}
	}
	if calls.Load() != 2 {
		t.Fatal("extra paid call")
	}
	// Exhausting a tool's search allowance stops follow-up model dispatch in the same operation.
	ctx := WithOperation(context.Background())
	search := &http.Client{Transport: &Transport{Quota: q, Kind: "search"}}
	for range 2 {
		r, _ := http.NewRequestWithContext(ctx, "GET", server.URL, nil)
		res, _ := search.Do(r)
		if res != nil {
			res.Body.Close()
		}
	}
	if OperationError(ctx) == nil {
		t.Fatal("quota failure not retained")
	}
	q.Models = 10
	r, _ := http.NewRequestWithContext(ctx, "POST", server.URL, strings.NewReader(`{}`))
	if _, err = client.Do(r); err == nil {
		t.Fatal("continued after failed tool quota")
	}
	if calls.Load() != 3 {
		t.Fatal("follow-up dispatched")
	}
}

func TestFailedInitialProvisionCanRetryInsideVolume(t *testing.T) {
	dir := directory(t)
	failed := func(path string) error {
		if filepath.Dir(filepath.Dir(path)) != dir {
			t.Fatal("staging escaped demo volume")
		}
		return errors.New("interrupted build")
	}
	if err := Replace(dir, false, "", failed); err == nil {
		t.Fatal("expected build error")
	}
	if err := Replace(dir, false, "", buildFile); err != nil {
		t.Fatal(err)
	}
}
