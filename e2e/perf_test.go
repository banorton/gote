package e2e

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestBigNote(t *testing.T) {
	if testing.Short() {
		t.Skip("5 MB note")
	}
	e := newEnv(t)
	e.write("small.md", "tiny note\n")
	e.ok("--no-server", "status") // warm: schema created, small note indexed

	var b strings.Builder
	for i := 0; b.Len() < 5<<20; i++ {
		fmt.Fprintf(&b, "## Section %d\n\nParagraph %d with filler text about topic%d and a link to [[small]] and [[missing %d]].\n\n", i, i, i%50, i)
	}
	b.WriteString("\nneedleunicorn appears once at the very end.\n")
	e.write("big.md", b.String())

	start := time.Now()
	out := e.ok("--no-server", "status", "--json")
	indexTime := time.Since(start)
	if !strings.Contains(out, `"notes": 2`) {
		t.Fatalf("status: %s", out)
	}
	if indexTime > 2*time.Second {
		t.Errorf("index of 5 MB note took %v, budget 2s (incl. process start)", indexTime)
	}

	e.ok("--no-server", "search", "-t", "needleunicorn") // warm
	start = time.Now()
	res := e.ok("--no-server", "search", "-t", "--json", "needleunicorn")
	searchTime := time.Since(start)
	if !strings.Contains(res, "big.md") {
		t.Fatalf("search: %s", res)
	}
	if searchTime > 200*time.Millisecond {
		t.Errorf("warm search took %v, budget 200ms (incl. process start)", searchTime)
	}
	t.Logf("index %v, warm search %v", indexTime, searchTime)
}
