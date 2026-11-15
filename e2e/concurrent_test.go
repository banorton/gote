package e2e

import (
	"fmt"
	"strings"
	"sync"
	"testing"
)

// runParallel launches every argv at once and fails on any non-zero exit or
// raw SQLite busy error.
func runParallel(t *testing.T, e *env, argvs [][]string) {
	t.Helper()
	res := make([]result, len(argvs))
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i, a := range argvs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			res[i] = e.run(a...)
		}()
	}
	close(start)
	wg.Wait()
	for i, r := range res {
		if r.code != 0 || strings.Contains(r.err+r.out, "SQLITE_BUSY") || strings.Contains(r.err, "locked") {
			t.Errorf("gote %v: code %d\nstdout: %s\nstderr: %s", argvs[i], r.code, r.out, r.err)
		}
	}
}

func TestConcurrentServerlessFreshState(t *testing.T) {
	const rounds = 5
	for r := 0; r < rounds; r++ {
		e := newEnv(t)
		for i := 0; i < 20; i++ {
			e.write(fmt.Sprintf("n%d.md", i), fmt.Sprintf("note %d [[n%d]]\n", i, (i+1)%20))
		}
		var argvs [][]string
		for i := 0; i < 8; i++ {
			switch i % 4 {
			case 0:
				argvs = append(argvs, []string{"--no-server", "status", "--json"})
			case 1:
				argvs = append(argvs, []string{"--no-server", "reindex"})
			case 2:
				argvs = append(argvs, []string{"--no-server", "new", "--json", fmt.Sprintf("created %d", i)})
			case 3:
				argvs = append(argvs, []string{"--no-server", "search", "-t", "note"})
			}
		}
		runParallel(t, e, argvs)
		if t.Failed() {
			return
		}
	}
}

// First run on a large vault: a server's initial reconcile races serverless
// reconciles of the same db.
func TestConcurrentServerlessVsServerFirstReconcile(t *testing.T) {
	if testing.Short() {
		t.Skip("large vault")
	}
	e := newEnv(t)
	body := strings.Repeat("lorem ipsum dolor sit amet [[n1]] #tag\n\n", 20)
	for i := 0; i < 1500; i++ {
		e.write(fmt.Sprintf("d%d/n%d.md", i%10, i), fmt.Sprintf("# n%d\n%s", i, body))
	}
	argvs := [][]string{{"status", "--json"}, {"status", "--json"}}
	for i := 0; i < 4; i++ {
		argvs = append(argvs, []string{"--no-server", "status", "--json"})
	}
	runParallel(t, e, argvs)
	if n := noteCount(t, e); n != 1500 {
		t.Fatalf("notes = %d", n)
	}
}
