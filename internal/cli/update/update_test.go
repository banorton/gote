package update

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/banorton/gote/internal/cli"
)

func TestCompareSemver(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"v1.10.0", "v1.9.0", 1},
		{"v1.9.0", "v1.10.0", -1},
		{"v1.1.0", "v1.1.0", 0},
		{"v2.0.0", "v1.99.99", 1},
		{"v1.2.3", "v1.2.4", -1},
		{"v0.0.1", "v0.0.0", 1},
	}
	for _, tc := range cases {
		got, err := compareSemver(tc.a, tc.b)
		if err != nil || got != tc.want {
			t.Errorf("compareSemver(%s,%s) = %d, %v; want %d", tc.a, tc.b, got, err, tc.want)
		}
	}
	for _, bad := range []string{"dev", "1.2.3", "v1.2", "v1.2.3-rc1", ""} {
		if _, err := compareSemver(bad, "v1.0.0"); err == nil {
			t.Errorf("compareSemver(%q) should error", bad)
		}
	}
}

type fixture struct {
	t        *testing.T
	srv      *httptest.Server
	latest   string
	binary   []byte
	sums     string // override SHA256SUMS body when non-empty
	noAsset  bool
	requests []*http.Request
	dir      string
	target   string
}

func script(version string) []byte {
	return []byte("#!/bin/sh\necho 'gote " + version + "'\n")
}

func sumLine(b []byte, name string) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:]) + "  " + name + "\n"
}

// newFixture starts a fake GitHub and installs a fake "current" binary reporting `current`.
func newFixture(t *testing.T, current, latest string) *fixture {
	t.Helper()
	f := &fixture{t: t, latest: latest, binary: script(latest)}
	assetName := fmt.Sprintf("gote-%s-%s", runtime.GOOS, runtime.GOARCH)
	mux := http.NewServeMux()
	mux.HandleFunc("/repos/banorton/gote/releases/latest", func(w http.ResponseWriter, r *http.Request) {
		f.requests = append(f.requests, r)
		assets := []map[string]string{
			{"name": "gote-plan9-mips", "browser_download_url": f.srv.URL + "/dl/other"},
			{"name": "SHA256SUMS", "browser_download_url": f.srv.URL + "/dl/SHA256SUMS"},
		}
		if !f.noAsset {
			assets = append(assets, map[string]string{"name": assetName, "browser_download_url": f.srv.URL + "/dl/redirect"})
		}
		json.NewEncoder(w).Encode(map[string]any{
			"tag_name": f.latest,
			"html_url": "https://github.com/banorton/gote/releases/tag/" + f.latest,
			"assets":   assets,
		})
	})
	mux.HandleFunc("/dl/redirect", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/dl/"+assetName, http.StatusFound)
	})
	mux.HandleFunc("/dl/"+assetName, func(w http.ResponseWriter, r *http.Request) { w.Write(f.binary) })
	mux.HandleFunc("/dl/SHA256SUMS", func(w http.ResponseWriter, r *http.Request) {
		if f.sums != "" {
			w.Write([]byte(f.sums))
			return
		}
		w.Write([]byte(sumLine([]byte("other"), "gote-plan9-mips") + sumLine(f.binary, assetName)))
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)

	f.dir = t.TempDir()
	f.target = filepath.Join(f.dir, "gote")
	if err := os.WriteFile(f.target, script(current), 0o755); err != nil {
		t.Fatal(err)
	}

	oldBase, oldAPI, oldDL, oldExe, oldVer := apiBase, apiClient, downloadClient, executable, cli.Version
	apiBase, apiClient, downloadClient = f.srv.URL, f.srv.Client(), f.srv.Client()
	executable = func() (string, error) { return f.target, nil }
	cli.Version = current
	t.Cleanup(func() {
		apiBase, apiClient, downloadClient, executable, cli.Version = oldBase, oldAPI, oldDL, oldExe, oldVer
	})
	return f
}

func runCmd(args ...string) (out, errOut string, err error) {
	var o, e bytes.Buffer
	c := &cli.Ctx{Out: &o, Err: &e}
	rest := args[:0:0]
	for _, a := range args {
		if a == "--json" {
			c.JSON = true
			continue
		}
		rest = append(rest, a)
	}
	err = run(c, rest)
	return o.String(), e.String(), err
}

func read(t *testing.T, p string) []byte {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func assertUntouched(t *testing.T, f *fixture, orig []byte) {
	t.Helper()
	if got := read(t, f.target); !bytes.Equal(got, orig) {
		t.Errorf("target changed: %q", got)
	}
	ents, _ := os.ReadDir(f.dir)
	if len(ents) != 1 {
		t.Errorf("stray files left in target dir: %v", ents)
	}
}

func TestCheckUpToDate(t *testing.T) {
	f := newFixture(t, "v1.1.0", "v1.1.0")
	out, _, err := runCmd("--check")
	if err != nil || strings.TrimSpace(out) != "gote v1.1.0 is up to date" {
		t.Fatalf("out=%q err=%v", out, err)
	}
	r := f.requests[0]
	if r.Header.Get("User-Agent") != "gote/v1.1.0" || r.Header.Get("Accept") != "application/vnd.github+json" {
		t.Errorf("headers: %v", r.Header)
	}
}

func TestCheckAvailable(t *testing.T) {
	newFixture(t, "v1.1.0", "v1.2.0")
	out, _, err := runCmd("--check")
	if err != nil || strings.TrimSpace(out) != "gote v1.1.0 → v1.2.0 available" {
		t.Fatalf("out=%q err=%v", out, err)
	}
}

func TestCheckJSON(t *testing.T) {
	newFixture(t, "v1.1.0", "v1.2.0")
	out, _, err := runCmd("--check", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var got struct {
		Current, Latest, URL string
		UpdateAvailable      bool `json:"update_available"`
	}
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	if got.Current != "v1.1.0" || got.Latest != "v1.2.0" || !got.UpdateAvailable ||
		got.URL != "https://github.com/banorton/gote/releases/tag/v1.2.0" {
		t.Errorf("%+v", got)
	}
}

func TestCheckHTTPErrorFails(t *testing.T) {
	newFixture(t, "v1.1.0", "v1.2.0")
	apiBase += "/nope"
	if _, _, err := runCmd("--check"); err == nil || !strings.Contains(err.Error(), "404") {
		t.Fatalf("err=%v", err)
	}
}

func TestDevBuildRefused(t *testing.T) {
	for _, v := range []string{"dev", "v1.1.0-3-gabc1234", "v1.1.0-dirty", "v1.1.0-3-gabc-dirty"} {
		f := newFixture(t, v, "v1.2.0")
		orig := read(t, f.target)
		_, _, err := runCmd()
		if err == nil || !strings.Contains(err.Error(), "development build") || !strings.Contains(err.Error(), "git pull && make build") {
			t.Errorf("%s: err=%v", v, err)
		}
		assertUntouched(t, f, orig)
	}
}

func TestDevBuildForce(t *testing.T) {
	f := newFixture(t, "dev", "v1.2.0")
	out, _, err := runCmd("--force")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(read(t, f.target), f.binary) {
		t.Error("target not replaced")
	}
	if !strings.Contains(out, "updated gote dev → v1.2.0") {
		t.Errorf("out=%q", out)
	}
}

func TestUpdateReplacesSymlinkTarget(t *testing.T) {
	f := newFixture(t, "v1.1.0", "v1.2.0")
	realDir := t.TempDir()
	real := filepath.Join(realDir, "gote-real")
	if err := os.Rename(f.target, real); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(f.dir, "gote")
	if err := os.Symlink(real, link); err != nil {
		t.Fatal(err)
	}
	executable = func() (string, error) { return link, nil }

	out, _, err := runCmd()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(read(t, real), f.binary) {
		t.Error("real file not replaced")
	}
	if dest, err := os.Readlink(link); err != nil || dest != real {
		t.Errorf("symlink changed: %q %v", dest, err)
	}
	if fi, _ := os.Stat(real); fi.Mode().Perm() != 0o755 {
		t.Errorf("mode %v", fi.Mode())
	}
	realResolved, _ := filepath.EvalSymlinks(real)
	if !strings.Contains(out, "updated gote v1.1.0 → v1.2.0 ("+realResolved+")") || !strings.Contains(out, "server") {
		t.Errorf("out=%q", out)
	}
	if ents, _ := os.ReadDir(realDir); len(ents) != 1 {
		t.Errorf("stray temp: %v", ents)
	}
}

func TestUpdateJSON(t *testing.T) {
	f := newFixture(t, "v1.1.0", "v1.2.0")
	out, _, err := runCmd("--json")
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]string
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatal(err)
	}
	want, _ := filepath.EvalSymlinks(f.target)
	if got["previous"] != "v1.1.0" || got["current"] != "v1.2.0" || got["path"] != want {
		t.Errorf("%v", got)
	}
}

func TestChecksumMismatch(t *testing.T) {
	f := newFixture(t, "v1.1.0", "v1.2.0")
	f.sums = sumLine([]byte("different"), fmt.Sprintf("gote-%s-%s", runtime.GOOS, runtime.GOARCH))
	orig := read(t, f.target)
	_, _, err := runCmd()
	if err == nil || !strings.Contains(err.Error(), "checksum") {
		t.Fatalf("err=%v", err)
	}
	assertUntouched(t, f, orig)
}

func TestChecksumLineMissing(t *testing.T) {
	f := newFixture(t, "v1.1.0", "v1.2.0")
	f.sums = sumLine([]byte("x"), "gote-plan9-mips")
	orig := read(t, f.target)
	_, _, err := runCmd()
	if err == nil || !strings.Contains(err.Error(), "SHA256SUMS") {
		t.Fatalf("err=%v", err)
	}
	assertUntouched(t, f, orig)
}

func TestMissingAsset(t *testing.T) {
	f := newFixture(t, "v1.1.0", "v1.2.0")
	f.noAsset = true
	orig := read(t, f.target)
	_, _, err := runCmd()
	if err == nil || !strings.Contains(err.Error(), "gote-plan9-mips") || !strings.Contains(err.Error(), runtime.GOOS) {
		t.Fatalf("err=%v", err)
	}
	assertUntouched(t, f, orig)
}

func TestSanityCheckFailure(t *testing.T) {
	f := newFixture(t, "v1.1.0", "v1.2.0")
	f.binary = script("v9.9.9")
	orig := read(t, f.target)
	_, _, err := runCmd()
	if err == nil || !strings.Contains(err.Error(), "v9.9.9") {
		t.Fatalf("err=%v", err)
	}
	assertUntouched(t, f, orig)
}

func TestSanityCheckNotExecutable(t *testing.T) {
	f := newFixture(t, "v1.1.0", "v1.2.0")
	f.binary = []byte("not a program")
	orig := read(t, f.target)
	if _, _, err := runCmd(); err == nil {
		t.Fatal("want error")
	}
	assertUntouched(t, f, orig)
}

func TestAlreadyLatestNoop(t *testing.T) {
	f := newFixture(t, "v1.2.0", "v1.2.0")
	orig := read(t, f.target)
	out, _, err := runCmd()
	if err != nil || !strings.Contains(out, "already up to date") {
		t.Fatalf("out=%q err=%v", out, err)
	}
	assertUntouched(t, f, orig)
}

func TestNewerThanLatestNoop(t *testing.T) {
	f := newFixture(t, "v1.10.0", "v1.9.0")
	orig := read(t, f.target)
	out, _, err := runCmd()
	if err != nil || !strings.Contains(out, "already up to date") {
		t.Fatalf("out=%q err=%v", out, err)
	}
	assertUntouched(t, f, orig)
}

func TestForceReinstallsSameVersion(t *testing.T) {
	f := newFixture(t, "v1.2.0", "v1.2.0")
	f.binary = append(script("v1.2.0"), []byte("# rebuilt\n")...)
	out, _, err := runCmd("--force")
	if err != nil || !strings.Contains(out, "updated gote v1.2.0 → v1.2.0") {
		t.Fatalf("out=%q err=%v", out, err)
	}
	if !bytes.Equal(read(t, f.target), f.binary) {
		t.Error("not reinstalled")
	}
}

func TestPermissionDenied(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root bypasses permissions")
	}
	f := newFixture(t, "v1.1.0", "v1.2.0")
	orig := read(t, f.target)
	os.Chmod(f.dir, 0o555)
	t.Cleanup(func() { os.Chmod(f.dir, 0o755) })
	_, _, err := runCmd()
	if err == nil || !strings.Contains(err.Error(), "permission") {
		t.Fatalf("err=%v", err)
	}
	os.Chmod(f.dir, 0o755)
	assertUntouched(t, f, orig)
}
