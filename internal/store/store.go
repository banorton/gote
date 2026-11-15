package store

import (
	"database/sql"
	"fmt"
	"net/url"
	"strconv"

	_ "modernc.org/sqlite"
)

// CacheVersion invalidates all cache tables when the schema or indexing
// semantics change.
const CacheVersion = 3

type Store struct {
	DB   *sql.DB
	Path string
}

var cacheTables = []string{"attachments", "chunks", "notes_fts", "tasks", "headings", "links", "tags", "aliases", "notes"}

const cacheSchema = `
CREATE TABLE notes(id TEXT PRIMARY KEY, kind TEXT NOT NULL, title TEXT NOT NULL, stem TEXT NOT NULL,
  created TEXT, mtime_ns INTEGER NOT NULL, size INTEGER NOT NULL, hash TEXT NOT NULL,
  fields TEXT NOT NULL, semantic INTEGER NOT NULL, parse_error TEXT);
CREATE TABLE aliases(note_id TEXT REFERENCES notes(id) ON DELETE CASCADE, alias_lc TEXT, alias TEXT);
CREATE TABLE tags(note_id TEXT REFERENCES notes(id) ON DELETE CASCADE, tag TEXT);
CREATE TABLE links(src TEXT REFERENCES notes(id) ON DELETE CASCADE, dst_raw TEXT, dst_id TEXT,
  fragment TEXT, alias TEXT, kind TEXT, line INTEGER, attach INTEGER NOT NULL DEFAULT 0);
CREATE TABLE attachments(path TEXT PRIMARY KEY, path_lc TEXT NOT NULL);
CREATE TABLE headings(note_id TEXT REFERENCES notes(id) ON DELETE CASCADE, level INTEGER, text TEXT, line INTEGER);
CREATE TABLE tasks(note_id TEXT REFERENCES notes(id) ON DELETE CASCADE, line INTEGER, text TEXT, done INTEGER);
CREATE VIRTUAL TABLE notes_fts USING fts5(note_id UNINDEXED, title, aliases, tags, body, tokenize='porter unicode61');
CREATE TABLE chunks(id INTEGER PRIMARY KEY, note_id TEXT NOT NULL REFERENCES notes(id) ON DELETE CASCADE,
  ord INTEGER NOT NULL, heading_path TEXT NOT NULL, start_line INTEGER NOT NULL, text TEXT NOT NULL, hash TEXT NOT NULL);
CREATE INDEX chunks_note ON chunks(note_id);
CREATE INDEX chunks_hash ON chunks(hash);
CREATE INDEX notes_stem ON notes(stem);
CREATE INDEX aliases_alias_lc ON aliases(alias_lc);
CREATE INDEX tags_tag ON tags(tag);
CREATE INDEX links_dst_id ON links(dst_id);
CREATE INDEX links_src ON links(src);
`

// stateMigrations are applied in order, once, and never dropped.
var stateMigrations = []string{
	`CREATE TABLE pins(note_id TEXT PRIMARY KEY, pinned_at INTEGER NOT NULL);
	 CREATE TABLE visits(note_id TEXT PRIMARY KEY, last_visited INTEGER NOT NULL);`,
	`CREATE TABLE embeddings(model TEXT NOT NULL, hash TEXT NOT NULL, dim INTEGER NOT NULL, vec BLOB NOT NULL, PRIMARY KEY(model, hash));`,
}

func Open(path string) (*Store, error) {
	q := url.Values{}
	for _, p := range []string{"journal_mode(WAL)", "busy_timeout(30000)", "foreign_keys(ON)", "synchronous(NORMAL)"} {
		q.Add("_pragma", p)
	}
	db, err := sql.Open("sqlite", "file:"+path+"?"+q.Encode())
	if err != nil {
		return nil, fmt.Errorf("open db %s: %w", path, err)
	}
	s := &Store{DB: db, Path: path}
	if err := s.init(); err != nil {
		db.Close()
		return nil, fmt.Errorf("init db %s: %w", path, err)
	}
	return s, nil
}

func (s *Store) Close() error { return s.DB.Close() }

func (s *Store) meta(key string) (string, error) {
	var v string
	err := s.DB.QueryRow("SELECT value FROM meta WHERE key=?", key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return v, err
}

func (s *Store) init() error {
	if _, err := s.DB.Exec("CREATE TABLE IF NOT EXISTS meta(key TEXT PRIMARY KEY, value TEXT)"); err != nil {
		return err
	}
	sv, err := s.meta("state_version")
	if err != nil {
		return err
	}
	have, _ := strconv.Atoi(sv)
	for i := have; i < len(stateMigrations); i++ {
		tx, err := s.DB.Begin()
		if err != nil {
			return err
		}
		if _, err := tx.Exec(stateMigrations[i]); err != nil {
			tx.Rollback()
			return fmt.Errorf("state migration %d: %w", i+1, err)
		}
		if _, err := tx.Exec("INSERT OR REPLACE INTO meta VALUES('state_version', ?)", strconv.Itoa(i+1)); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	cv, err := s.meta("cache_version")
	if err != nil {
		return err
	}
	if cv == strconv.Itoa(CacheVersion) {
		return nil
	}
	return s.ResetCache()
}

// ResetCache drops and recreates every cache table. State tables are untouched.
func (s *Store) ResetCache() error {
	tx, err := s.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, t := range cacheTables {
		if _, err := tx.Exec("DROP TABLE IF EXISTS " + t); err != nil {
			return fmt.Errorf("drop %s: %w", t, err)
		}
	}
	if _, err := tx.Exec(cacheSchema); err != nil {
		return fmt.Errorf("create cache schema: %w", err)
	}
	if _, err := tx.Exec("INSERT OR REPLACE INTO meta VALUES('cache_version', ?)", strconv.Itoa(CacheVersion)); err != nil {
		return err
	}
	return tx.Commit()
}
