package server

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestRunStartsWebAndRemovesStateFile(t *testing.T) {
	d := newDirs(t)
	os.WriteFile(filepath.Join(d.notes, "a.md"), []byte("hello web\n"), 0o644)
	r := start(t, d.opts("1"))
	p := filepath.Join(d.state, "web.json")
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var w struct {
		Port  int
		Token string
	}
	if err := json.Unmarshal(b, &w); err != nil {
		t.Fatal(err)
	}
	resp, err := http.Get("http://127.0.0.1:" + strconv.Itoa(w.Port) + "/n/a.md?t=" + w.Token)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("web via server: %d", resp.StatusCode)
	}
	r.stop(t)
	if _, err := os.Stat(p); !os.IsNotExist(err) {
		t.Fatalf("web.json left behind: %v", err)
	}
}
