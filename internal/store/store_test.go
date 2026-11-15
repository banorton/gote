package store

import (
	"path/filepath"
	"testing"
)

func open(t *testing.T) (*Store, string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "gote.db")
	s, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, p
}

func TestPragmas(t *testing.T) {
	s, _ := open(t)
	var mode string
	s.DB.QueryRow("PRAGMA journal_mode").Scan(&mode)
	var fk, busy int
	s.DB.QueryRow("PRAGMA foreign_keys").Scan(&fk)
	s.DB.QueryRow("PRAGMA busy_timeout").Scan(&busy)
	if mode != "wal" || fk != 1 || busy != 5000 {
		t.Fatal(mode, fk, busy)
	}
}

func TestSchemaAndCascade(t *testing.T) {
	s, _ := open(t)
	for _, q := range []string{
		"INSERT INTO notes VALUES('a.md','note','A','a',NULL,1,1,'h','{}',1,NULL)",
		"INSERT INTO tags VALUES('a.md','x')",
		"INSERT INTO links VALUES('a.md','b',NULL,'',  '','wiki',1)",
		"INSERT INTO notes_fts VALUES('a.md','A','','x','body text')",
		"DELETE FROM notes WHERE id='a.md'",
	} {
		if _, err := s.DB.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	var n int
	s.DB.QueryRow("SELECT count(*) FROM tags").Scan(&n)
	if n != 0 {
		t.Fatal("tags not cascaded")
	}
}

func TestCacheVersionChangeKeepsState(t *testing.T) {
	s, p := open(t)
	s.DB.Exec("INSERT INTO notes VALUES('a.md','note','A','a',NULL,1,1,'h','{}',1,NULL)")
	s.DB.Exec("INSERT INTO pins VALUES('a.md', 1)")
	s.DB.Exec("UPDATE meta SET value='0' WHERE key='cache_version'")
	s.Close()
	s2, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	var notes, pins int
	s2.DB.QueryRow("SELECT count(*) FROM notes").Scan(&notes)
	s2.DB.QueryRow("SELECT count(*) FROM pins").Scan(&pins)
	if notes != 0 || pins != 1 {
		t.Fatal(notes, pins)
	}
}

func TestResetCache(t *testing.T) {
	s, _ := open(t)
	s.DB.Exec("INSERT INTO notes VALUES('a.md','note','A','a',NULL,1,1,'h','{}',1,NULL)")
	s.DB.Exec("INSERT INTO visits VALUES('a.md', 1)")
	if err := s.ResetCache(); err != nil {
		t.Fatal(err)
	}
	var notes, visits int
	s.DB.QueryRow("SELECT count(*) FROM notes").Scan(&notes)
	s.DB.QueryRow("SELECT count(*) FROM visits").Scan(&visits)
	if notes != 0 || visits != 1 {
		t.Fatal(notes, visits)
	}
}

func TestChunksCacheAndEmbeddingsState(t *testing.T) {
	s, p := open(t)
	for _, q := range []string{
		"INSERT INTO notes VALUES('a.md','note','A','a',NULL,1,1,'h','{}',1,NULL)",
		"INSERT INTO chunks(note_id, ord, heading_path, start_line, text, hash) VALUES('a.md',0,'',1,'t','h1')",
		"INSERT INTO embeddings VALUES('m','h1',2,x'0000803f00000000')",
	} {
		if _, err := s.DB.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	if _, err := s.DB.Exec("DELETE FROM notes WHERE id='a.md'"); err != nil {
		t.Fatal(err)
	}
	var n int
	s.DB.QueryRow("SELECT count(*) FROM chunks").Scan(&n)
	if n != 0 {
		t.Fatal("chunks not cascaded")
	}
	s.DB.Exec("INSERT INTO notes VALUES('a.md','note','A','a',NULL,1,1,'h','{}',1,NULL)")
	s.DB.Exec("INSERT INTO chunks(note_id, ord, heading_path, start_line, text, hash) VALUES('a.md',0,'',1,'t','h1')")
	if err := s.ResetCache(); err != nil {
		t.Fatal(err)
	}
	s.DB.QueryRow("SELECT count(*) FROM chunks").Scan(&n)
	if n != 0 {
		t.Fatal("chunks survived cache reset")
	}
	s.DB.QueryRow("SELECT count(*) FROM embeddings").Scan(&n)
	if n != 1 {
		t.Fatal("embeddings lost on cache reset")
	}
	s.Close()
	if _, err := Open(p); err != nil {
		t.Fatal(err)
	}
}
