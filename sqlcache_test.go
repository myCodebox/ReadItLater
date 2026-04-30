package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestSQLCache_SetGetDelete_TTL(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "sqlcache_test")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}
	defer os.RemoveAll(tmpdir)
	dbPath := filepath.Join(tmpdir, "cache.db")

	// TTL 1 second
	c, err := NewSQLCache(dbPath, 1*time.Second, 0)
	if err != nil {
		t.Fatalf("NewSQLCache: %v", err)
	}
	defer c.Close()

	key := "https://example.com/article"
	val := []byte("hello")
	if err := c.Set(key, val); err != nil {
		t.Fatalf("Set error: %v", err)
	}
	b, ok, err := c.Get(key)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if !ok {
		t.Fatalf("expected ok after set")
	}
	if string(b) != string(val) {
		t.Fatalf("value mismatch: got %q want %q", string(b), string(val))
	}

	// Delete
	if err := c.Delete(key); err != nil {
		t.Fatalf("Delete error: %v", err)
	}
	_, ok, err = c.Get(key)
	if err != nil {
		t.Fatalf("Get after delete error: %v", err)
	}
	if ok {
		t.Fatalf("expected not ok after delete")
	}

	// Set again and wait for TTL expiry
	if err := c.Set(key, val); err != nil {
		t.Fatalf("Set error: %v", err)
	}
	// wait > ttl (ensure Unix-second boundary passed)
	time.Sleep(2200 * time.Millisecond)
	_, ok, err = c.Get(key)
	if err != nil {
		t.Fatalf("Get after ttl error: %v", err)
	}
	if ok {
		t.Fatalf("expected expired entry to be not ok")
	}
}

func TestSQLCache_Eviction_MaxItems(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "sqlcache_ev")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}
	defer os.RemoveAll(tmpdir)
	dbPath := filepath.Join(tmpdir, "cache.db")

	// maxItems = 2
	c, err := NewSQLCache(dbPath, 60*time.Second, 2)
	if err != nil {
		t.Fatalf("NewSQLCache: %v", err)
	}
	defer c.Close()

	// Insert three entries sequentially
	_ = c.Set("k1", []byte("v1"))
	// small sleep so last_accessed differs
	time.Sleep(10 * time.Millisecond)
	_ = c.Set("k2", []byte("v2"))
	time.Sleep(10 * time.Millisecond)
	_ = c.Set("k3", []byte("v3"))

	// k1 should be evicted (oldest)
	if _, ok, _ := c.Get("k1"); ok {
		t.Fatalf("expected k1 evicted, but present")
	}
	// k2 and k3 should be present
	if b, ok, _ := c.Get("k2"); !ok || string(b) != "v2" {
		t.Fatalf("k2 missing or wrong: ok=%v val=%q", ok, string(b))
	}
	if b, ok, _ := c.Get("k3"); !ok || string(b) != "v3" {
		t.Fatalf("k3 missing or wrong: ok=%v val=%q", ok, string(b))
	}
}

func TestSQLCache_StoredAnalysisEntry(t *testing.T) {
	tmpdir, err := os.MkdirTemp("", "sqlcache_entry")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}
	defer os.RemoveAll(tmpdir)
	dbPath := filepath.Join(tmpdir, "cache.db")

	c, err := NewSQLCache(dbPath, 60*time.Second, 0)
	if err != nil {
		t.Fatalf("NewSQLCache: %v", err)
	}
	defer c.Close()

	entry := AnalysisCacheEntry{
		Title:     "T",
		Image:     "I",
		Video:     "V",
		Audio:     "A",
		BodyHTML:  "<p>ok</p>",
		CleanText: "ok",
		OpenGraph: `{"og:image":"I"}`,
		StoredAt:  time.Now().Unix(),
	}
	b, _ := json.Marshal(entry)
	key := "https://example.com/entry"
	if err := c.Set(key, b); err != nil {
		t.Fatalf("Set entry error: %v", err)
	}
	got, ok, err := c.Get(key)
	if err != nil {
		t.Fatalf("Get entry error: %v", err)
	}
	if !ok {
		t.Fatalf("expected entry present")
	}
	var out AnalysisCacheEntry
	if err := json.Unmarshal(got, &out); err != nil {
		t.Fatalf("unmarshal got: %v", err)
	}
	if out.Title != entry.Title || out.BodyHTML != entry.BodyHTML {
		t.Fatalf("entry mismatch: %+v vs %+v", out, entry)
	}
}
