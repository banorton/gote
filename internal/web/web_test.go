package web

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/banorton/gote/internal/config"
	"github.com/banorton/gote/internal/excal"
	"github.com/banorton/gote/internal/index"
	"github.com/banorton/gote/internal/ops"
	"github.com/banorton/gote/internal/store"
)

const drawingID = "diagrams/261114000000_diagram_d1.excalidraw"

const sceneJSON = `{"type":"excalidraw","version":2,"source":"test","elements":[{"id":"r1","type":"rectangle","x":0,"y":0,"width":10,"height":10,"isDeleted":false}],"appState":{"viewBackgroundColor":"#ffffff"},"files":{}}`

type fixture struct {
	t      *testing.T
	dir    string
	state  string
	env    *ops.Env
	srv    *Server
	cancel context.CancelFunc
}

func put(t *testing.T, dir, rel, content string) string {
	t.Helper()
	p := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func newFixture(t *testing.T, opts ...Option) *fixture {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "notes")
	state := filepath.Join(root, "state")
	t.Setenv("XDG_STATE_HOME", state)
	put(t, dir, "a.md", "---\ntags: [alpha, beta]\n---\n# Heading\n\nsee [[b]] and [[ghost]] and [[b|the b note]]\n\n![[261114000000_diagram_d1.excalidraw]]\n\n![[pic.png]]\n\n[rel](sub/c.md)\n")
	put(t, dir, "b.md", "bee\n")
	put(t, dir, "sub/c.md", "sea\n")
	put(t, dir, drawingID, sceneJSON)
	put(t, dir, "pic.png", "PNGDATA")
	put(t, dir, ".hidden/x.txt", "hidden")
	put(t, root, "secret.txt", "secret")
	if err := os.Symlink(filepath.Join(root, "secret.txt"), filepath.Join(dir, "leak.txt")); err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(filepath.Join(root, "gote.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	env := &ops.Env{Cfg: &config.Config{NotesDir: dir, DiagramDir: "diagrams"}, Store: st, Index: index.New(st, dir), Now: time.Now}
	if _, err := env.Index.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	srv, err := Start(ctx, env, opts...)
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	t.Cleanup(func() { cancel(); srv.Close() })
	return &fixture{t: t, dir: dir, state: state, env: env, srv: srv, cancel: cancel}
}

func (f *fixture) do(method, route string, body io.Reader, mod func(*http.Request)) *http.Response {
	f.t.Helper()
	req, err := http.NewRequest(method, "http://"+f.srv.Addr()+route, body)
	if err != nil {
		f.t.Fatal(err)
	}
	if mod != nil {
		mod(req)
	}
	resp, err := (&http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}).Do(req)
	if err != nil {
		f.t.Fatal(err)
	}
	return resp
}

func (f *fixture) tok(route string) string {
	sep := "?"
	if strings.Contains(route, "?") {
		sep = "&"
	}
	return route + sep + "t=" + f.srv.Token()
}

func (f *fixture) get(route string) (int, string) {
	f.t.Helper()
	resp := f.do("GET", f.tok(route), nil, nil)
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func status(r *http.Response) int { r.Body.Close(); return r.StatusCode }

func TestStateFileWrittenAndRemoved(t *testing.T) {
	f := newFixture(t)
	p := filepath.Join(f.state, "gote", "web.json")
	fi, err := os.Stat(p)
	if err != nil {
		t.Fatal(err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("web.json mode %v", fi.Mode().Perm())
	}
	var got struct {
		Port  int    `json:"port"`
		Token string `json:"token"`
	}
	b, _ := os.ReadFile(p)
	if err := json.Unmarshal(b, &got); err != nil {
		t.Fatal(err)
	}
	if got.Port != f.srv.Port() || got.Token != f.srv.Token() || len(got.Token) != 64 {
		t.Fatalf("web.json %+v", got)
	}
	f.cancel()
	deadline := time.Now().Add(2 * time.Second)
	for {
		if _, err := os.Stat(p); os.IsNotExist(err) {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("web.json not removed after cancel")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := http.Get("http://" + f.srv.Addr() + "/healthz"); err == nil {
		t.Fatal("listener still accepting after cancel")
	}
}

func TestNoStateFileOption(t *testing.T) {
	f := newFixture(t, NoStateFile())
	if _, err := os.Stat(filepath.Join(f.state, "web.json")); !os.IsNotExist(err) {
		t.Fatalf("web.json written: %v", err)
	}
}

func TestTokenRequired(t *testing.T) {
	f := newFixture(t)
	for _, route := range []string{"/n/a.md", "/n/a.md?t=wrong", "/n/a.md?t=", "/assets/editor.js", "/events?id=a.md", "/healthz", "/api/drawing/" + drawingID, "/f/pic.png"} {
		if c := status(f.do("GET", route, nil, nil)); c != 403 {
			t.Errorf("%s without token: %d", route, c)
		}
	}
	resp := f.do("GET", f.tok("/n/a.md"), nil, nil)
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("valid token: %d", resp.StatusCode)
	}
	var ck *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "gote_t" {
			ck = c
		}
	}
	if ck == nil || ck.Value != f.srv.Token() || !ck.HttpOnly || ck.SameSite != http.SameSiteStrictMode {
		t.Fatalf("cookie %+v", ck)
	}
	r := f.do("GET", "/n/a.md", nil, func(r *http.Request) { r.AddCookie(&http.Cookie{Name: "gote_t", Value: f.srv.Token()}) })
	if r.Body.Close(); r.StatusCode != 200 {
		t.Fatalf("cookie auth: %d", r.StatusCode)
	}
	r = f.do("GET", "/n/a.md", nil, func(r *http.Request) { r.AddCookie(&http.Cookie{Name: "gote_t", Value: "nope"}) })
	if r.Body.Close(); r.StatusCode != 403 {
		t.Fatalf("bad cookie: %d", r.StatusCode)
	}
}

func TestHostCheck(t *testing.T) {
	f := newFixture(t)
	for host, want := range map[string]int{
		"evil.example": 403,
		"127.0.0.1":    403,
		fmt.Sprintf("127.0.0.1:%d", f.srv.Port()+1): 403,
		fmt.Sprintf("localhost:%d", f.srv.Port()):   200,
		fmt.Sprintf("127.0.0.1:%d", f.srv.Port()):   200,
	} {
		host := host
		c := status(f.do("GET", f.tok("/n/a.md"), nil, func(r *http.Request) { r.Host = host }))
		if c != want {
			t.Errorf("Host %q: %d want %d", host, c, want)
		}
	}
}

func TestWriteOriginCheck(t *testing.T) {
	f := newFixture(t)
	route := f.tok("/api/drawing/" + drawingID)
	mtime := f.mtime(drawingID)
	try := func(origin string) int {
		return status(f.do("PUT", route, strings.NewReader(sceneJSON), func(r *http.Request) {
			r.Header.Set("If-Match", mtime)
			if origin != "" {
				r.Header.Set("Origin", origin)
			}
		}))
	}
	if c := try("http://evil.example"); c != 403 {
		t.Errorf("foreign origin: %d", c)
	}
	if c := try("http://" + fmt.Sprintf("127.0.0.1:%d", f.srv.Port()+1)); c != 403 {
		t.Errorf("foreign port origin: %d", c)
	}
	if c := try(""); c != 403 {
		t.Errorf("missing origin: %d", c)
	}
	if c := try("http://" + f.srv.Addr()); c != 200 {
		t.Errorf("same origin: %d", c)
	}
}

func (f *fixture) mtime(id string) string {
	f.t.Helper()
	fi, err := os.Stat(filepath.Join(f.dir, filepath.FromSlash(id)))
	if err != nil {
		f.t.Fatal(err)
	}
	return fmt.Sprint(fi.ModTime().UnixNano())
}

func TestFilesServeAndTraversal(t *testing.T) {
	f := newFixture(t)
	if c, b := f.get("/f/pic.png"); c != 200 || b != "PNGDATA" {
		t.Fatalf("pic: %d %q", c, b)
	}
	resp := f.do("GET", f.tok("/f/pic.png"), nil, nil)
	resp.Body.Close()
	if !strings.Contains(resp.Header.Get("Content-Security-Policy"), "sandbox") || resp.Header.Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("files must be sandboxed: %v", resp.Header)
	}
	for _, route := range []string{
		"/f/../x", "/f/../secret.txt", "/f/%2e%2e/secret.txt", "/f/sub/../../secret.txt",
		"/f/.hidden/x.txt", "/f/leak.txt", "/f/missing.png", "/f/sub", "/f/",
	} {
		if c, _ := f.get(route); c != 404 {
			t.Errorf("%s: %d want 404", route, c)
		}
	}
	if c, _ := f.get("/assets/../../go.mod"); c != 404 {
		t.Errorf("assets traversal: %d", c)
	}
}

func TestRenderNote(t *testing.T) {
	f := newFixture(t)
	c, body := f.get("/n/a.md")
	if c != 200 {
		t.Fatalf("%d %s", c, body)
	}
	for _, want := range []string{
		`href="/n/b.md"`,
		`>the b note</a>`,
		`class="broken"`,
		`class="gote-drawing" data-id="` + drawingID + `"`,
		`<img src="/f/pic.png"`,
		`href="/n/sub/c.md"`,
		`<h1`,
		`class="tag">alpha<`,
		`class="tag">beta<`,
		`/assets/viewer.js`,
		`prefers-color-scheme: dark`,
		`data-events="/events?id=a.md"`,
		`/assets/reload.js`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("missing %q in:\n%s", want, body)
		}
	}
	if strings.Contains(body, "tags: [") {
		t.Error("frontmatter leaked into body")
	}
	if strings.Contains(body, `class="broken">b<`) {
		t.Error("resolved link marked broken")
	}
	if c, body := f.get("/n/b.md"); c != 200 || strings.Contains(body, "viewer.js") {
		t.Errorf("plain note: %d viewer loaded=%v", c, strings.Contains(body, "viewer.js"))
	}
	if c, _ := f.get("/n/nope.md"); c != 404 {
		t.Errorf("unknown id: %d", c)
	}
	if c, _ := f.get("/n/../secret.txt"); c != 404 {
		t.Errorf("traversal id: %d", c)
	}
	resp := f.do("GET", f.tok("/n/"+drawingID), nil, nil)
	resp.Body.Close()
	if resp.StatusCode != 302 || !strings.HasPrefix(resp.Header.Get("Location"), "/d/"+drawingID) {
		t.Errorf("drawing under /n/: %d %s", resp.StatusCode, resp.Header.Get("Location"))
	}
}

func TestRenderEscapesHTML(t *testing.T) {
	f := newFixture(t)
	put(t, f.dir, "x.md", "---\ntags: ['<script>alert(1)</script>']\n---\n<script>alert(2)</script>\n\n[[<img src=x onerror=alert(3)>]]\n\n[bad](javascript:alert(4))\n")
	if _, err := f.env.Index.Reconcile(context.Background()); err != nil {
		t.Fatal(err)
	}
	_, body := f.get("/n/x.md")
	for _, bad := range []string{"<script>alert", "<img src=x", `href="javascript:`} {
		if strings.Contains(body, bad) {
			t.Errorf("unescaped %q in:\n%s", bad, body)
		}
	}
}

func TestEditorPageAndAssets(t *testing.T) {
	f := newFixture(t)
	c, body := f.get("/d/" + drawingID)
	if c != 200 || !strings.Contains(body, `data-id="`+drawingID+`"`) || !strings.Contains(body, "/assets/editor.js") {
		t.Fatalf("%d %s", c, body)
	}
	if c, _ := f.get("/d/a.md"); c != 404 {
		t.Errorf("note under /d/: %d", c)
	}
	if c, _ := f.get("/d/diagrams/missing.excalidraw"); c != 404 {
		t.Errorf("missing drawing: %d", c)
	}
	for _, p := range []string{"/assets/editor.js", "/assets/viewer.js", "/assets/editor.css"} {
		if c, b := f.get(p); c != 200 || len(b) == 0 {
			t.Errorf("%s: %d len %d", p, c, len(b))
		}
	}
	entries, err := os.ReadDir(filepath.Join("..", "..", "web", "dist", "assets", "fonts"))
	if err != nil || len(entries) == 0 {
		t.Fatalf("fonts missing from web/dist: %v", err)
	}
	if c, _ := f.get("/assets/fonts/Excalifont/" + firstFile(t, filepath.Join("..", "..", "web", "dist", "assets", "fonts", "Excalifont"))); c != 200 {
		t.Errorf("self-hosted font: %d", c)
	}
	if c, _ := f.get("/assets/missing.js"); c != 404 {
		t.Errorf("missing asset: %d", c)
	}
}

func firstFile(t *testing.T, dir string) string {
	t.Helper()
	es, err := os.ReadDir(dir)
	if err != nil || len(es) == 0 {
		t.Fatalf("read %s: %v", dir, err)
	}
	return es[0].Name()
}

func TestDrawingRoundTrip(t *testing.T) {
	f := newFixture(t)
	route := "/api/drawing/" + drawingID
	resp := f.do("GET", f.tok(route), nil, nil)
	var got struct {
		Scene   json.RawMessage `json:"scene"`
		MtimeNS string          `json:"mtime_ns"`
	}
	json.NewDecoder(resp.Body).Decode(&got)
	resp.Body.Close()
	if resp.StatusCode != 200 || got.MtimeNS != f.mtime(drawingID) {
		t.Fatalf("get: %d %+v", resp.StatusCode, got)
	}
	if !bytes.Equal(bytes.TrimSpace(got.Scene), []byte(sceneJSON)) {
		t.Fatalf("scene altered: %s", got.Scene)
	}

	edited := strings.Replace(sceneJSON, `"width":10`, `"width":99`, 1)
	origin := func(r *http.Request) { r.Header.Set("Origin", "http://"+f.srv.Addr()) }
	resp = f.do("PUT", f.tok(route), strings.NewReader(edited), func(r *http.Request) {
		origin(r)
		r.Header.Set("If-Match", got.MtimeNS)
	})
	var put struct {
		MtimeNS string `json:"mtime_ns"`
	}
	json.NewDecoder(resp.Body).Decode(&put)
	resp.Body.Close()
	if resp.StatusCode != 200 || put.MtimeNS != f.mtime(drawingID) {
		t.Fatalf("put: %d %+v", resp.StatusCode, put)
	}
	disk, _ := os.ReadFile(filepath.Join(f.dir, filepath.FromSlash(drawingID)))
	if string(disk) != edited {
		t.Fatalf("file bytes changed:\n%s", disk)
	}
	if _, err := excal.Decode(disk); err != nil {
		t.Fatal(err)
	}

	// stale If-Match: 409, file untouched
	resp = f.do("PUT", f.tok(route), strings.NewReader(sceneJSON), func(r *http.Request) {
		origin(r)
		r.Header.Set("If-Match", got.MtimeNS)
	})
	if resp.Body.Close(); resp.StatusCode != 409 {
		t.Fatalf("stale: %d", resp.StatusCode)
	}
	disk, _ = os.ReadFile(filepath.Join(f.dir, filepath.FromSlash(drawingID)))
	if string(disk) != edited {
		t.Fatal("stale PUT modified the file")
	}

	for name, tc := range map[string]struct {
		body, match string
		want        int
	}{
		"no if-match": {sceneJSON, "", 428},
		"bad json":    {"{nope", put.MtimeNS, 400},
		"not a scene": {`{"type":"other"}`, put.MtimeNS, 400},
		"non-numeric": {sceneJSON, "abc", 400},
	} {
		resp = f.do("PUT", f.tok(route), strings.NewReader(tc.body), func(r *http.Request) {
			origin(r)
			if tc.match != "" {
				r.Header.Set("If-Match", tc.match)
			}
		})
		if resp.Body.Close(); resp.StatusCode != tc.want {
			t.Errorf("%s: %d want %d", name, resp.StatusCode, tc.want)
		}
	}
	for _, id := range []string{"a.md", "diagrams/nope.excalidraw", "../secret.txt"} {
		resp = f.do("PUT", f.tok("/api/drawing/"+id), strings.NewReader(sceneJSON), func(r *http.Request) {
			origin(r)
			r.Header.Set("If-Match", "1")
		})
		if resp.Body.Close(); resp.StatusCode != 404 {
			t.Errorf("PUT %s: %d want 404", id, resp.StatusCode)
		}
	}

	var n int
	f.env.Store.DB.QueryRow(`SELECT COUNT(*) FROM notes WHERE id=?`, drawingID).Scan(&n)
	if n != 1 {
		t.Fatal("drawing not indexed")
	}
}

func TestPutReindexes(t *testing.T) {
	f := newFixture(t)
	withText := strings.Replace(sceneJSON, `"elements":[`, `"elements":[{"id":"t1","type":"text","text":"zebrafish","x":0,"y":0,"width":5,"height":5,"isDeleted":false},`, 1)
	resp := f.do("PUT", f.tok("/api/drawing/"+drawingID), strings.NewReader(withText), func(r *http.Request) {
		r.Header.Set("Origin", "http://"+f.srv.Addr())
		r.Header.Set("If-Match", f.mtime(drawingID))
	})
	if resp.Body.Close(); resp.StatusCode != 200 {
		t.Fatalf("%d", resp.StatusCode)
	}
	var n int
	f.env.Store.DB.QueryRow(`SELECT COUNT(*) FROM notes_fts WHERE notes_fts MATCH 'zebrafish'`).Scan(&n)
	if n != 1 {
		t.Fatalf("PUT did not call IndexFile: fts hits %d", n)
	}
}

type sse struct {
	lines chan string
	close func()
}

func (f *fixture) events(id string) *sse {
	f.t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	req, _ := http.NewRequestWithContext(ctx, "GET", "http://"+f.srv.Addr()+f.tok("/events?id="+id), nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		cancel()
		f.t.Fatal(err)
	}
	if resp.StatusCode != 200 || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		cancel()
		f.t.Fatalf("events: %d %s", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	s := &sse{lines: make(chan string, 16), close: func() { cancel(); resp.Body.Close() }}
	go func() {
		sc := bufio.NewScanner(resp.Body)
		for sc.Scan() {
			s.lines <- sc.Text()
		}
		close(s.lines)
	}()
	f.t.Cleanup(s.close)
	return s
}

func (s *sse) expectReload(t *testing.T, within time.Duration) {
	t.Helper()
	timeout := time.After(within)
	for {
		select {
		case l, ok := <-s.lines:
			if !ok {
				t.Fatal("stream closed")
			}
			if l == "event: reload" {
				return
			}
		case <-timeout:
			t.Fatalf("no reload event within %v", within)
		}
	}
}

func (s *sse) expectQuiet(t *testing.T, d time.Duration) {
	t.Helper()
	timeout := time.After(d)
	for {
		select {
		case l := <-s.lines:
			if l == "event: reload" {
				t.Fatal("unexpected reload")
			}
		case <-timeout:
			return
		}
	}
}

func bump(t *testing.T, p, content string) {
	t.Helper()
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	future := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(p, future, future); err != nil {
		t.Fatal(err)
	}
}

func TestEventsNoteChangeReloadsWithinASecond(t *testing.T) {
	f := newFixture(t)
	s := f.events("a.md")
	time.Sleep(100 * time.Millisecond)
	s.expectQuiet(t, 400*time.Millisecond)
	bump(t, filepath.Join(f.dir, "a.md"), "changed\n")
	s.expectReload(t, time.Second)
}

func TestEventsEmbeddedDrawingChangeReloads(t *testing.T) {
	f := newFixture(t)
	s := f.events("a.md")
	time.Sleep(100 * time.Millisecond)
	bump(t, filepath.Join(f.dir, filepath.FromSlash(drawingID)), sceneJSON+"\n")
	s.expectReload(t, time.Second)
}

func TestEventsUnrelatedChangeIsQuiet(t *testing.T) {
	f := newFixture(t)
	s := f.events("b.md")
	time.Sleep(100 * time.Millisecond)
	bump(t, filepath.Join(f.dir, "a.md"), "changed\n")
	s.expectQuiet(t, 600*time.Millisecond)
}

func TestEventsUnknownID(t *testing.T) {
	f := newFixture(t)
	if c, _ := f.get("/events?id=nope.md"); c != 404 {
		t.Fatalf("%d", c)
	}
}

func TestNotifyWakesPollers(t *testing.T) {
	f := newFixture(t, PollInterval(time.Hour))
	s := f.events("b.md")
	time.Sleep(100 * time.Millisecond)
	bump(t, filepath.Join(f.dir, "b.md"), "changed\n")
	s.expectQuiet(t, 200*time.Millisecond)
	f.srv.Notify()
	s.expectReload(t, time.Second)
}

func TestSharedLockHeldAroundWrites(t *testing.T) {
	lk := &countLock{}
	f := newFixture(t, WithLock(lk))
	resp := f.do("PUT", f.tok("/api/drawing/"+drawingID), strings.NewReader(sceneJSON), func(r *http.Request) {
		r.Header.Set("Origin", "http://"+f.srv.Addr())
		r.Header.Set("If-Match", f.mtime(drawingID))
	})
	if resp.Body.Close(); resp.StatusCode != 200 {
		t.Fatalf("%d", resp.StatusCode)
	}
	if lk.locks != 1 || lk.unlocks != 1 {
		t.Fatalf("lock/unlock %d/%d", lk.locks, lk.unlocks)
	}
}

type countLock struct{ locks, unlocks int }

func (c *countLock) Lock()   { c.locks++ }
func (c *countLock) Unlock() { c.unlocks++ }

const wantCSP = "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; connect-src 'self'; frame-src 'none'; worker-src 'self' blob:; object-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'none'"

func TestHTMLSecurityHeaders(t *testing.T) {
	f := newFixture(t)
	for _, route := range []string{"/n/a.md", "/n/b.md", "/d/" + drawingID} {
		resp := f.do("GET", f.tok(route), nil, nil)
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("%s: %d", route, resp.StatusCode)
		}
		csp := resp.Header.Get("Content-Security-Policy")
		if csp != wantCSP {
			t.Errorf("%s CSP:\n got %q\nwant %q", route, csp, wantCSP)
		}
		for _, bad := range []string{"http:", "https:", "ws:", "wss:", "*", "//"} {
			if strings.Contains(csp, bad) {
				t.Errorf("%s CSP contains %q", route, bad)
			}
		}
		if got := resp.Header.Get("Referrer-Policy"); got != "no-referrer" {
			t.Errorf("%s Referrer-Policy %q", route, got)
		}
		if got := resp.Header.Get("X-Content-Type-Options"); got != "nosniff" {
			t.Errorf("%s nosniff %q", route, got)
		}
		for _, m := range regexp.MustCompile(`(?is)<script\b[^>]*>`).FindAllString(string(body), -1) {
			if !strings.Contains(m, ` src="/assets/`) {
				t.Errorf("%s inline or foreign script %q", route, m)
			}
		}
		if regexp.MustCompile(`(?i)\son[a-z]+\s*=`).MatchString(string(body)) {
			t.Errorf("%s has inline event handler", route)
		}
	}
}

func TestFilesKeepSandboxCSP(t *testing.T) {
	f := newFixture(t)
	resp := f.do("GET", f.tok("/f/pic.png"), nil, nil)
	resp.Body.Close()
	if got := resp.Header.Get("Content-Security-Policy"); !strings.HasPrefix(got, "sandbox;") {
		t.Errorf("/f/ CSP %q", got)
	}
}

func TestBundleHasNoCDNFallback(t *testing.T) {
	dir := filepath.Join("..", "..", "web", "dist", "assets")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".js") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(b, []byte("esm.sh")) {
			t.Errorf("%s references esm.sh", e.Name())
		}
	}
}
