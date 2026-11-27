package tasks

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/banorton/gote/internal/cli"
	"github.com/banorton/gote/internal/client"
	"github.com/banorton/gote/internal/config"
	"github.com/banorton/gote/internal/index"
	"github.com/banorton/gote/internal/ops"
	_ "github.com/banorton/gote/internal/ops/search"
	optasks "github.com/banorton/gote/internal/ops/tasks"
	"github.com/banorton/gote/internal/store"
)

var files = map[string]string{
	"a.md": "---\ntags: [work]\n---\n- [ ] write spec\n- [x] ship it\n",
	"b.md": "# B\n\n- [ ] buy milk\n",
	"z.md": "---\ntags: [Archive]\n---\n- [ ] archived task\n",
}

func setup(t *testing.T, jsonOut bool) (*cli.Ctx, *bytes.Buffer) {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	mt := time.Now().Add(-time.Hour)
	if err := os.Chtimes(filepath.Join(dir, "a.md"), mt, mt); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(t.TempDir(), "gote.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	ix := index.New(st, dir)
	if _, err := ix.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	return &cli.Ctx{Client: client.Local(&ops.Env{Cfg: &config.Config{NotesDir: dir}, Store: st, Index: ix, Now: time.Now}), Out: &out, Err: &out, JSON: jsonOut}, &out
}

func TestTasksText(t *testing.T) {
	c, out := setup(t, false)
	if err := run(c, nil); err != nil {
		t.Fatal(err)
	}
	if want := "b (b.md:3)  [ ] buy milk\na (a.md:4)  [ ] write spec\n"; out.String() != want {
		t.Fatalf("got %q", out)
	}
	out.Reset()
	if err := run(c, []string{"--all"}); err != nil {
		t.Fatal(err)
	}
	if want := "b (b.md:3)  [ ] buy milk\nz (z.md:4)  [ ] archived task\na (a.md:4)  [ ] write spec\na (a.md:5)  [x] ship it\n"; out.String() != want && out.String() != "z (z.md:4)  [ ] archived task\nb (b.md:3)  [ ] buy milk\na (a.md:4)  [ ] write spec\na (a.md:5)  [x] ship it\n" {
		t.Fatalf("got %q", out)
	}
}

func TestTasksQueryAndNegatedTerm(t *testing.T) {
	c, out := setup(t, false)
	if err := run(c, []string{"tag:work"}); err != nil {
		t.Fatal(err)
	}
	if want := "a (a.md:4)  [ ] write spec\n"; out.String() != want {
		t.Fatalf("got %q", out)
	}
	out.Reset()
	if err := run(c, []string{"-tag:work", "--all"}); err != nil {
		t.Fatal(err)
	}
	if got := out.String(); !strings.Contains(got, "buy milk") || !strings.Contains(got, "archived task") || strings.Contains(got, "write spec") {
		t.Fatalf("got %q", out)
	}
}

func TestTasksJSON(t *testing.T) {
	c, out := setup(t, true)
	if err := run(c, []string{"--all"}); err != nil {
		t.Fatal(err)
	}
	var got []optasks.Task
	if err := json.Unmarshal(out.Bytes(), &got); err != nil || len(got) != 4 || !got[3].Done {
		t.Fatalf("%v %s", err, out)
	}
	out.Reset()
	if err := run(c, []string{"tag:nothing"}); err != nil || out.String() != "[]\n" {
		t.Fatalf("%v %q", err, out)
	}
}

func TestTasksBadQueryIsInvalid(t *testing.T) {
	c, _ := setup(t, false)
	err := run(c, []string{"created:123"})
	var oe *ops.Error
	if !errors.As(err, &oe) || oe.Code != ops.CodeInvalid {
		t.Fatalf("%v", err)
	}
}
