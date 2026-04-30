package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	_ "github.com/glebarez/sqlite"
)

// AnalysisCacheEntry is the marshaled value stored in the DB.
type AnalysisCacheEntry struct {
	Title     string
	Image     string
	Video     string
	Audio     string
	BodyHTML  string
	CleanText string
	OpenGraph string
	StoredAt  int64
}

// SQLCache stores analyzed results in a sqlite DB with TTL and optional max entries.
type SQLCache struct {
	db       *sql.DB
	ttl      time.Duration
	maxItems int
	ctx      context.Context
	cancel   context.CancelFunc
	// async writer
	writeCh chan setRequest
	wg      sync.WaitGroup
}

type setRequest struct {
	key  string
	blob []byte
}

// NewSQLCache creates or opens the sqlite database at path and starts janitor.
func NewSQLCache(path string, ttl time.Duration, maxItems int) (*SQLCache, error) {
	if path == "" {
		return nil, errors.New("path required")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("mkdir cache dir: %w", err)
	}

	dsn := "file:" + path + "?_busy_timeout=5000"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// set pragmas for WAL and performance
	if _, err := db.Exec("PRAGMA journal_mode = WAL;"); err != nil {
		// non-fatal
	}
	if _, err := db.Exec("PRAGMA synchronous = NORMAL;"); err != nil {
		// non-fatal
	}

	schema := `
CREATE TABLE IF NOT EXISTS cache (
  key TEXT PRIMARY KEY,
  url TEXT,
  value BLOB,
  fetched_at INTEGER,
  expires_at INTEGER,
  last_accessed INTEGER,
  size INTEGER
);
CREATE INDEX IF NOT EXISTS idx_expires ON cache(expires_at);
CREATE INDEX IF NOT EXISTS idx_lastaccess ON cache(last_accessed);
`
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("create schema: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	c := &SQLCache{db: db, ttl: ttl, maxItems: maxItems, ctx: ctx, cancel: cancel, writeCh: make(chan setRequest, 64)}
	go c.janitor(5 * time.Minute)
	c.wg.Add(1)
	go c.runWriter()
	return c, nil
}

// Get returns raw blob and true if present and not expired.
func (c *SQLCache) Get(urlStr string) ([]byte, bool, error) {
	key := urlStr
	now := time.Now().Unix()
	row := c.db.QueryRow("SELECT value, expires_at FROM cache WHERE key = ?", key)
	var blob []byte
	var expiresAt sql.NullInt64
	if err := row.Scan(&blob, &expiresAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, false, nil
		}
		return nil, false, err
	}
	if expiresAt.Valid && expiresAt.Int64 > 0 && now > expiresAt.Int64 {
		// expired -> delete
		_, _ = c.db.Exec("DELETE FROM cache WHERE key = ?", key)
		return nil, false, nil
	}
	// update last_accessed (best effort)
	_, _ = c.db.Exec("UPDATE cache SET last_accessed = ? WHERE key = ?", now, key)
	return blob, true, nil
}

// Set stores the blob for URL
func (c *SQLCache) Set(urlStr string, blob []byte) error {
	key := urlStr
	now := time.Now().Unix()
	expires := now + int64(c.ttl.Seconds())
	_, err := c.db.Exec(`INSERT OR REPLACE INTO cache(key,url,value,fetched_at,expires_at,last_accessed,size) VALUES(?,?,?,?,?,?,?)`, key, urlStr, blob, now, expires, now, len(blob))
	if err != nil {
		return err
	}
	if c.maxItems > 0 {
		// enforce size limit: delete oldest by last_accessed if count > maxItems
		_, _ = c.db.Exec(`
		  DELETE FROM cache WHERE key IN (
		    SELECT key FROM cache ORDER BY last_accessed ASC LIMIT (
		      SELECT CASE WHEN COUNT(*) > ? THEN COUNT(*) - ? ELSE 0 END FROM cache
		    )
		  )`, c.maxItems, c.maxItems)
	}
	return nil
}

// SetAsync queues a write to the DB; non-blocking unless buffer full.
func (c *SQLCache) SetAsync(urlStr string, blob []byte) error {
	if c == nil || c.writeCh == nil {
		// fallback to sync
		return c.Set(urlStr, blob)
	}
	req := setRequest{key: urlStr, blob: blob}
	select {
	case c.writeCh <- req:
		return nil
	default:
		// buffer full, fallback to sync write to avoid data loss
		return c.Set(urlStr, blob)
	}
}

func (c *SQLCache) runWriter() {
	defer c.wg.Done()
	for req := range c.writeCh {
		_ = c.Set(req.key, req.blob)
	}
}

// Delete removes a key from the cache.
func (c *SQLCache) Delete(key string) error {
	if c == nil || c.db == nil {
		return nil
	}
	_, err := c.db.Exec("DELETE FROM cache WHERE key = ?", key)
	return err
}

// Close closes the underlying DB and stops janitor.
func (c *SQLCache) Close() error {
	// stop janitor
	c.cancel()
	// close writer and wait
	if c.writeCh != nil {
		close(c.writeCh)
		c.wg.Wait()
	}
	return c.db.Close()
}

// janitor periodically removes expired entries
func (c *SQLCache) janitor(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-ticker.C:
			// remove expired
			_, _ = c.db.Exec("DELETE FROM cache WHERE expires_at <= ?", time.Now().Unix())
			// optional: VACUUM periodically if needed
		}
	}
}
