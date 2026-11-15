package view

import (
	"bytes"
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/banorton/gote/internal/cli"
	"github.com/banorton/gote/internal/client"
	"github.com/banorton/gote/internal/config"
	"github.com/banorton/gote/internal/index"
	"github.com/banorton/gote/internal/ops"
	_ "github.com/banorton/gote/internal/ops/diagram"
	_ "github.com/banorton/gote/internal/ops/notes"
	"github.com/banorton/gote/internal/store"
	"github.com/banorton/gote/internal/tui/pager"
	"github.com/banorton/gote/internal/web"
)

var opened []string
var openedHook func(url string)

func init() {
	cli.Register(cli.Command{Name: "open", NoConfig: true, Run: func(c *cli.Ctx, args []string) error {
		openCalls = append(openCalls, args)
		return nil
	}})
}

var openCalls [][]string

type env struct {
	dir, state string
	ops        *ops.Env
	ctx        *cli.Ctx
	out, err   *bytes.Buffer
}

const scene = `{"type":"excalidraw","version":2,"elements":[{"id":"r1","type":"rectangle","x":0,"y":0,"width":10,"height":10,"isDeleted":false,"boundElements":[]},{"id":"t1","type":"text","text":"hello box","x":0,"y":0,"width":5,"height":5,"isDeleted":false,"containerId":null}],"appState":{},"files":{}}`

func setup(t *testing.T, server bool) *env {
	t.Helper()
	root := t.TempDir()
	e := &env{dir: filepath.Join(root, "notes"), state: filepath.Join(root, "state"), out: &bytes.Buffer{}, err: &bytes.Buffer{}}
	t.Setenv("XDG_STATE_HOME", e.state)
	for rel, body := range map[string]string{
		"a.md":                               "---\ntags: [x]\n---\n# Alpha\n\nlink to [[b]] and [[b|bee]] and [rel](sub/c.md) ![[pic.png]]\n",
		"b.md":                               "beta body\n",
		"sub/c.md":                           "---\ntitle: Sea\n---\nsee [[../b]]\n",
		"diagrams/260101000000_d.excalidraw": scene,
	} {
		p := filepath.Join(e.dir, filepath.FromSlash(rel))
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte(body), 0o644)
	}
	st, err := store.Open(filepath.Join(root, "gote.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	cfg := &config.Config{NotesDir: e.dir, DiagramDir: "diagrams", Server: server}
	e.ops = &ops.Env{Cfg: cfg, Store: st, Index: index.New(st, e.dir), Now: time.Now}
	if _, err := e.ops.Index.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	e.ctx = &cli.Ctx{Client: client.Local(e.ops), Cfg: cfg, Out: e.out, Err: e.err}

	origOpener, origWait, origTTY, origPager := Opener, WaitContext, isTTY, runPager
	opened, openedHook, openCalls = nil, nil, nil
	t.Cleanup(func() { Opener, WaitContext, isTTY, runPager = origOpener, origWait, origTTY, origPager })
	Opener = func(url string) error {
		opened = append(opened, url)
		if openedHook != nil {
			openedHook(url)
		}
		return nil
	}
	return e
}

func TestServerlessViewStartsAndStopsItsListener(t *testing.T) {
	e := setup(t, false)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	WaitContext = func() (context.Context, context.CancelFunc) { return ctx, cancel }
	var gotStatus int
	var addr string
	openedHook = func(url string) {
		resp, err := http.Get(url)
		if err != nil {
			t.Error(err)
		} else {
			resp.Body.Close()
			gotStatus = resp.StatusCode
		}
		addr = strings.TrimPrefix(strings.SplitN(url, "/n/", 2)[0], "http://")
		cancel()
	}
	if err := command.Run(e.ctx, []string{"a"}); err != nil {
		t.Fatal(err)
	}
	if len(opened) != 1 || !strings.Contains(opened[0], "/n/a.md?t=") {
		t.Fatalf("opened %v", opened)
	}
	if gotStatus != 200 {
		t.Fatalf("page status %d", gotStatus)
	}
	if !strings.Contains(e.out.String(), opened[0]) {
		t.Fatalf("URL not printed: %q", e.out.String())
	}
	if conn, err := net.DialTimeout("tcp", addr, 300*time.Millisecond); err == nil {
		conn.Close()
		t.Fatal("listener still open after cancel")
	}
	if _, err := os.Stat(filepath.Join(e.state, "gote", "web.json")); !os.IsNotExist(err) {
		t.Fatal("serverless view must not write web.json")
	}
}

func TestViewDrawingRoutesToViewerPage(t *testing.T) {
	e := setup(t, false)
	ctx, cancel := context.WithCancel(context.Background())
	WaitContext = func() (context.Context, context.CancelFunc) { return ctx, cancel }
	openedHook = func(string) { cancel() }
	if err := command.Run(e.ctx, []string{"260101000000_d.excalidraw"}); err != nil {
		t.Fatal(err)
	}
	if len(opened) != 1 || !strings.Contains(opened[0], "/d/diagrams/260101000000_d.excalidraw?t=") {
		t.Fatalf("opened %v", opened)
	}
}

func TestJSONOutput(t *testing.T) {
	e := setup(t, false)
	e.ctx.JSON = true
	ctx, cancel := context.WithCancel(context.Background())
	WaitContext = func() (context.Context, context.CancelFunc) { return ctx, cancel }
	openedHook = func(string) { cancel() }
	if err := command.Run(e.ctx, []string{"a"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(e.out.String(), `"id": "a.md"`) || !strings.Contains(e.out.String(), `"url": "http://127.0.0.1:`) {
		t.Fatalf("%s", e.out.String())
	}
}

func TestAttachesToRunningServerWithoutBlocking(t *testing.T) {
	e := setup(t, true)
	dir := filepath.Join(e.state, "gote")
	os.MkdirAll(dir, 0o700)
	ws, err := web.Start(context.Background(), e.ops, web.WithStateDir(dir))
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()
	WaitContext = func() (context.Context, context.CancelFunc) {
		t.Error("attached view must not block on its own listener")
		return context.WithCancel(context.Background())
	}
	done := make(chan error, 1)
	go func() { done <- command.Run(e.ctx, []string{"a"}) }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("view blocked while a server is running")
	}
	if len(opened) != 1 || opened[0] != ws.URL("/n/a.md") {
		t.Fatalf("opened %v want %s", opened, ws.URL("/n/a.md"))
	}
}

func TestStaleWebJSONFallsBackToServerless(t *testing.T) {
	e := setup(t, true)
	dir := filepath.Join(e.state, "gote")
	os.MkdirAll(dir, 0o700)
	l, _ := net.Listen("tcp", "127.0.0.1:0")
	dead := l.Addr().(*net.TCPAddr).Port
	l.Close()
	os.WriteFile(filepath.Join(dir, "web.json"), []byte(`{"port":`+itoa(dead)+`,"token":"abc"}`), 0o600)
	ctx, cancel := context.WithCancel(context.Background())
	WaitContext = func() (context.Context, context.CancelFunc) { return ctx, cancel }
	openedHook = func(string) { cancel() }
	if err := command.Run(e.ctx, []string{"a"}); err != nil {
		t.Fatal(err)
	}
	if len(opened) != 1 || strings.Contains(opened[0], ":"+itoa(dead)+"/") {
		t.Fatalf("used stale web.json: %v", opened)
	}
}

func TestNoServerFlagIgnoresRunningServer(t *testing.T) {
	e := setup(t, true)
	e.ctx.NoServer = true
	dir := filepath.Join(e.state, "gote")
	os.MkdirAll(dir, 0o700)
	ws, err := web.Start(context.Background(), e.ops, web.WithStateDir(dir))
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Close()
	ctx, cancel := context.WithCancel(context.Background())
	WaitContext = func() (context.Context, context.CancelFunc) { return ctx, cancel }
	openedHook = func(string) { cancel() }
	if err := command.Run(e.ctx, []string{"a"}); err != nil {
		t.Fatal(err)
	}
	if len(opened) != 1 || strings.HasPrefix(opened[0], "http://"+ws.Addr()) {
		t.Fatalf("--no-server attached anyway: %v", opened)
	}
}

func itoa(n int) string {
	b := []byte{}
	for ; n > 0; n /= 10 {
		b = append([]byte{byte('0' + n%10)}, b...)
	}
	return string(b)
}

func TestUsageAndUnknown(t *testing.T) {
	e := setup(t, false)
	for _, args := range [][]string{{}, {"-T"}, {"--bogus", "a"}} {
		var ue *cli.UsageError
		if err := command.Run(e.ctx, args); err == nil || !asUsage(err, &ue) {
			t.Errorf("%v: want usage error, got %v", args, err)
		}
	}
	if err := command.Run(e.ctx, []string{"nope"}); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("unknown note: %v", err)
	}
	if len(opened) != 0 {
		t.Fatal("opened a browser on failure")
	}
}

func asUsage(err error, target **cli.UsageError) bool {
	ue, ok := err.(*cli.UsageError)
	*target = ue
	return ok
}

func TestTerminalNonTTYPrintsPlainRenderedText(t *testing.T) {
	e := setup(t, false)
	isTTY = func(io.Writer) bool { return false }
	for _, flag := range []string{"-T", "--terminal"} {
		e.out.Reset()
		if err := command.Run(e.ctx, []string{flag, "a"}); err != nil {
			t.Fatal(err)
		}
		out := e.out.String()
		for _, want := range []string{"Alpha", "[[b]]", "bee", "x"} {
			if !strings.Contains(out, want) {
				t.Errorf("%s: missing %q in:\n%s", flag, want, out)
			}
		}
		if strings.Contains(out, "\x1b") || strings.Contains(out, "tags: [") {
			t.Errorf("%s: output must be plain, no frontmatter:\n%q", flag, out)
		}
	}
	if len(opened) != 0 {
		t.Fatal("-T opened a browser")
	}
}

func TestTerminalDrawingPrintsSummary(t *testing.T) {
	e := setup(t, false)
	isTTY = func(io.Writer) bool { return false }
	if err := command.Run(e.ctx, []string{"-T", "260101000000_d.excalidraw"}); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"shapes (1)", "r1 rectangle", "hello box"} {
		if !strings.Contains(e.out.String(), want) {
			t.Errorf("missing %q in:\n%s", want, e.out.String())
		}
	}
}

func TestTerminalJSON(t *testing.T) {
	e := setup(t, false)
	e.ctx.JSON = true
	isTTY = func(io.Writer) bool { return false }
	if err := command.Run(e.ctx, []string{"-T", "b"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(e.out.String(), `"id": "b.md"`) || !strings.Contains(e.out.String(), "beta body") {
		t.Fatalf("%s", e.out.String())
	}
}

func TestBuildDocLinks(t *testing.T) {
	e := setup(t, false)
	d, err := loadDoc(e.ctx, "a", 80, false)
	if err != nil {
		t.Fatal(err)
	}
	if d.Ref != "a.md" || d.Title != "a" {
		t.Fatalf("%+v", d)
	}
	var got []string
	for _, l := range d.Links {
		got = append(got, l.Label+"=>"+l.Target)
	}
	want := "[[b]]=>b|bee=>b|sub/c.md=>sub/c.md"
	if strings.Join(got, "|") != want {
		t.Fatalf("links %v want %s", got, want)
	}
	c, err := loadDoc(e.ctx, "sub/c.md", 80, false)
	if err != nil || c.Title != "Sea" {
		t.Fatalf("%+v %v", c, err)
	}
	if len(c.Links) != 1 || c.Links[0].Target != "../b" {
		t.Fatalf("%+v", c.Links)
	}
}

func TestPagerEditLoop(t *testing.T) {
	e := setup(t, false)
	isTTY = func(io.Writer) bool { return true }
	var runs int
	var docs []string
	runPager = func(c *cli.Ctx, m *pager.Model) error {
		runs++
		docs = append(docs, m.Doc().Ref)
		if runs == 1 {
			m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("e")})
		}
		return nil
	}
	if err := command.Run(e.ctx, []string{"-T", "a"}); err != nil {
		t.Fatal(err)
	}
	if runs != 2 || len(openCalls) != 1 || len(openCalls[0]) != 1 || openCalls[0][0] != "a.md" {
		t.Fatalf("runs %d open calls %v", runs, openCalls)
	}
	if strings.Join(docs, ",") != "a.md,a.md" {
		t.Fatalf("docs %v", docs)
	}
}

func TestPagerFollowsLinks(t *testing.T) {
	e := setup(t, false)
	isTTY = func(io.Writer) bool { return true }
	runPager = func(c *cli.Ctx, m *pager.Model) error {
		m.Update(tea.KeyMsg{Type: tea.KeyTab})
		m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		if m.Doc().Ref != "b.md" || m.Depth() != 1 {
			t.Errorf("follow failed: %s depth %d", m.Doc().Ref, m.Depth())
		}
		m.Update(tea.KeyMsg{Type: tea.KeyTab})
		m.Update(tea.KeyMsg{Type: tea.KeyEnter})
		return nil
	}
	if err := command.Run(e.ctx, []string{"-T", "a"}); err != nil {
		t.Fatal(err)
	}
}
