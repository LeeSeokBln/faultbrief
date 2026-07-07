package baseline

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCacheRoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "patterns.json") // parent dir must be created
	c, err := LoadCache(path)
	if err != nil {
		t.Fatal(err)
	}
	if c.Has("abc") {
		t.Fatal("fresh cache must be empty")
	}
	now := time.Date(2026, 7, 7, 10, 0, 0, 0, time.UTC)
	c.Remember("abc", "conn to <IP> failed", now)
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}
	c2, err := LoadCache(path)
	if err != nil {
		t.Fatal(err)
	}
	if !c2.Has("abc") {
		t.Fatal("cache did not persist")
	}
}

func TestCacheCorruptFileStartsEmpty(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "patterns.json")
	os.WriteFile(path, []byte("{corrupt"), 0o644)
	c, err := LoadCache(path)
	if err != nil {
		t.Fatalf("corrupt cache should not be fatal: %v", err)
	}
	if c.Has("anything") {
		t.Fatal("corrupt cache should start empty")
	}
}
