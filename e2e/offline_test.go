package e2e

import (
	"net"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

// No outbound traffic: every command runs with proxy env vars pointing at a
// listener that records connections. Limitation: this only catches clients that
// honor proxy settings, not raw-socket dials; a Docker `--network none` run
// will cover those later. Semantic search is unconfigured here.
func TestNoOutboundConnections(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var conns []string
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			conns = append(conns, c.RemoteAddr().String())
			mu.Unlock()
			c.Close()
		}
	}()
	t.Cleanup(func() { ln.Close() })
	proxy := "http://" + ln.Addr().String()

	e := newEnv(t)
	e.extra = []string{"HTTP_PROXY=" + proxy, "HTTPS_PROXY=" + proxy, "ALL_PROXY=" + proxy,
		"http_proxy=" + proxy, "https_proxy=" + proxy, "all_proxy=" + proxy, "NO_PROXY=", "no_proxy="}
	e.write("a.md", "alpha [[b]]\n- [ ] a task\n")
	e.write("b.md", "bravo\n")
	fx, err := os.ReadFile("testdata/valid.excalidraw")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(e.dir, "d.excalidraw"), fx, 0o644); err != nil {
		t.Fatal(err)
	}

	for _, args := range [][]string{
		{"status"},
		{"search", "alp"},
		{"search", "-t", "bravo"},
		{"graph", "a"},
		{"view", "-T", "a"},
		{"diagram", "read", "d"},
		{"tasks"},
		{"new", "--json", "fresh"},
		{"--no-server", "status"},
		{"server", "status"},
	} {
		e.ok(args...)
	}
	e.stopServer()

	mu.Lock()
	defer mu.Unlock()
	if len(conns) != 0 {
		t.Fatalf("outbound connections: %v", conns)
	}
}
