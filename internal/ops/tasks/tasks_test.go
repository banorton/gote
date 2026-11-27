package tasks

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/banorton/gote/internal/client"
	"github.com/banorton/gote/internal/config"
	"github.com/banorton/gote/internal/index"
	"github.com/banorton/gote/internal/ops"
	"github.com/banorton/gote/internal/store"
)

var fakeIDs []string
var fakeReq struct {
	Q     string `json:"q"`
	Limit int    `json:"limit"`
}

func init() {
	ops.Register(ops.Op{Name: "search.query", Run: func(ctx context.Context, env *ops.Env, raw json.RawMessage) (any, error) {
		if err := json.Unmarshal(raw, &fakeReq); err != nil {
			return nil, err
		}
		type res struct {
			ID string `json:"id"`
		}
		out := struct {
			Results []res `json:"results"`
		}{Results: []res{}}
		for _, id := range fakeIDs {
			out.Results = append(out.Results, res{ID: id})
		}
		return out, nil
	}})
}

var fixture = map[string]string{
	"old.md":  "# Old\n\n- [ ] old open\n- [x] old done\n",
	"new.md":  "---\ntitle: Shiny\n---\n\ntext\n- [ ] new first\n- [x] new done\n- [ ] new second\n",
	"none.md": "# None\n\nno tasks\n",
	"mid.md":  "# Mid\n\n- [ ] mid open\n",
}

func setup(t *testing.T) client.Client {
	t.Helper()
	dir := t.TempDir()
	base := time.Now().Add(-time.Hour)
	for i, name := range []string{"old.md", "mid.md", "new.md", "none.md"} {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte(fixture[name]), 0o644); err != nil {
			t.Fatal(err)
		}
		mt := base.Add(time.Duration(i) * time.Minute)
		if err := os.Chtimes(p, mt, mt); err != nil {
			t.Fatal(err)
		}
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
	return client.Local(&ops.Env{Cfg: &config.Config{}, Store: st, Index: ix, Now: time.Now})
}

func list(t *testing.T, c client.Client, req ListReq) []Task {
	t.Helper()
	var got []Task
	if err := c.Call(context.Background(), "tasks.list", req, &got); err != nil {
		t.Fatal(err)
	}
	return got
}

func TestListOpenOrderedByMtimeThenLine(t *testing.T) {
	c := setup(t)
	got := list(t, c, ListReq{})
	want := []Task{
		{ID: "new.md", Title: "Shiny", Line: 6, Text: "new first"},
		{ID: "new.md", Title: "Shiny", Line: 8, Text: "new second"},
		{ID: "mid.md", Title: "mid", Line: 3, Text: "mid open"},
		{ID: "old.md", Title: "old", Line: 3, Text: "old open"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v", got)
	}
}

func TestListAllIncludesDone(t *testing.T) {
	c := setup(t)
	got := list(t, c, ListReq{All: true})
	if len(got) != 6 {
		t.Fatalf("got %+v", got)
	}
	if got[1].Text != "new done" || !got[1].Done || got[0].Done {
		t.Fatalf("got %+v", got)
	}
}

func TestListQueryRestrictsToSearchResults(t *testing.T) {
	c := setup(t)
	fakeIDs = []string{"old.md", "none.md"}
	got := list(t, c, ListReq{Q: "tag:x"})
	if fakeReq.Q != "tag:x" || fakeReq.Limit != 1000 {
		t.Fatalf("search req %+v", fakeReq)
	}
	if len(got) != 1 || got[0].ID != "old.md" {
		t.Fatalf("got %+v", got)
	}
	fakeIDs = nil
	if got := list(t, c, ListReq{Q: "zzz"}); got == nil || len(got) != 0 {
		t.Fatalf("want empty non-nil, got %#v", got)
	}
}

func TestListEmptyIsNonNil(t *testing.T) {
	dir := t.TempDir()
	st, _ := store.Open(filepath.Join(t.TempDir(), "gote.db"))
	t.Cleanup(func() { st.Close() })
	c := client.Local(&ops.Env{Cfg: &config.Config{}, Store: st, Index: index.New(st, dir), Now: time.Now})
	if got := list(t, c, ListReq{}); got == nil || len(got) != 0 {
		t.Fatalf("%#v", got)
	}
}
