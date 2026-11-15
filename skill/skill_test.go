package skill

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func buildGote(t *testing.T) string {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "gote")
	cmd := exec.Command("go", "build", "-o", bin, "../cmd/gote")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	return bin
}

func captured(t *testing.T) map[string]string {
	t.Helper()
	out, err := exec.Command("bash", "capture.sh", buildGote(t)).Output()
	if err != nil {
		t.Fatalf("capture.sh: %v", err)
	}
	m := map[string]string{}
	for _, b := range strings.Split(string(out), "%%%\n")[1:] {
		b = strings.TrimRight(b, "\n")
		first, _, _ := strings.Cut(b, "\n")
		m[first] = b
	}
	return m
}

var fence = regexp.MustCompile("(?s)```console\n(.*?)\n```")

func TestSkillFrontmatter(t *testing.T) {
	data, err := os.ReadFile("gote/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	s := string(data)
	if !strings.HasPrefix(s, "---\nname: gote\ndescription: ") {
		t.Fatalf("frontmatter must start with name: gote and a description")
	}
	end := strings.Index(s[4:], "\n---\n")
	if end < 0 || !strings.Contains(s[:end], "notes") || !strings.Contains(s[:end], "drawing") {
		t.Fatal("description must trigger on notes and drawings")
	}
}

func TestSkillExamplesMatchCapturedOutput(t *testing.T) {
	data, err := os.ReadFile("gote/SKILL.md")
	if err != nil {
		t.Fatal(err)
	}
	want := captured(t)
	blocks := fence.FindAllStringSubmatch(string(data), -1)
	if len(blocks) < 15 {
		t.Fatalf("expected the skill to carry captured examples, found %d", len(blocks))
	}
	seen := map[string]bool{}
	for _, m := range blocks {
		first, _, _ := strings.Cut(m[1], "\n")
		if !strings.HasPrefix(first, "$ gote ") {
			t.Errorf("console block must start with a `$ gote` line: %q", first)
			continue
		}
		seen[first] = true
		w, ok := want[first]
		if !ok {
			t.Errorf("no capture for %q; add it to capture.sh", first)
			continue
		}
		if m[1] != w {
			t.Errorf("stale example %q; rerun skill/capture.sh <gote> and paste\n--- SKILL.md\n%s\n--- captured\n%s", first, m[1], w)
		}
	}
	for first := range want {
		if !seen[first] {
			t.Errorf("capture.sh runs %q but SKILL.md does not show it", first)
		}
	}
}
