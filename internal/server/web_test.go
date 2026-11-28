package server

import (
	"context"
	"time"

	"encoding/json"
	"github.com/banorton/gote/internal/client"
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

func TestVersionReportsWebClients(t *testing.T) {
	d := newDirs(t)
	os.WriteFile(filepath.Join(d.notes, "a.md"), []byte("hello web\n"), 0o644)
	o := d.opts("1")
	start(t, o)
	b, err := os.ReadFile(filepath.Join(d.state, "web.json"))
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
	clients := func() int {
		info, err := client.Probe(o.SocketPath)
		if err != nil {
			t.Fatal(err)
		}
		return info.WebClients
	}
	if n := clients(); n != 0 {
		t.Fatalf("web_clients = %d, want 0", n)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, "GET", "http://127.0.0.1:"+strconv.Itoa(w.Port)+"/events?id=a.md&t="+w.Token, nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	waitClients := func(want int) {
		t.Helper()
		deadline := time.Now().Add(2 * time.Second)
		for clients() != want {
			if time.Now().After(deadline) {
				t.Fatalf("web_clients = %d, want %d", clients(), want)
			}
			time.Sleep(10 * time.Millisecond)
		}
	}
	waitClients(1)
	cancel()
	waitClients(0)
}
