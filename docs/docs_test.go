package docs

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"regexp"
	"strings"
	"testing"

	"github.com/banorton/gote/internal/config"
)

type helpEntry struct {
	Name    string   `json:"name"`
	Aliases []string `json:"aliases"`
}

func registry(t *testing.T) []helpEntry {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "gote")
	cmd := exec.Command("go", "build", "-o", bin, "../cmd/gote")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("build: %v\n%s", err, out)
	}
	out, err := exec.Command(bin, "help", "--json").Output()
	if err != nil {
		t.Fatal(err)
	}
	var entries []helpEntry
	if err := json.Unmarshal(out, &entries); err != nil {
		t.Fatal(err)
	}
	return entries
}

func read(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

var span = regexp.MustCompile("`([^`\n]+)`")

var codeFence = regexp.MustCompile("(?s)```.*?```")

func stripFences(s string) string { return codeFence.ReplaceAllString(s, "") }

func TestREADMEListsEveryRegisteredCommandAndAlias(t *testing.T) {
	readme := stripFences(read(t, "../README.md"))
	cells := map[string]bool{}
	for _, m := range span.FindAllStringSubmatch(readme, -1) {
		cells[m[1]] = true
	}
	for _, e := range registry(t) {
		for _, name := range append([]string{e.Name}, e.Aliases...) {
			if !cells[name] {
				t.Errorf("README does not mention `%s` in backticks", name)
			}
		}
	}
}

func TestREADMETableCommandsExist(t *testing.T) {
	readme := stripFences(read(t, "../README.md"))
	registered := map[string]bool{}
	for _, e := range registry(t) {
		registered[e.Name] = true
	}
	rows, inTable := 0, false
	for _, line := range strings.Split(readme, "\n") {
		if strings.HasPrefix(line, "| Command |") {
			inTable = true
			continue
		}
		if !strings.HasPrefix(line, "|") {
			inTable = false
		}
		if !inTable || !strings.HasPrefix(line, "| `") {
			continue
		}
		cols := strings.Split(line, "|")
		m := span.FindStringSubmatch(cols[1])
		if m == nil {
			continue
		}
		rows++
		name := strings.Fields(m[1])[0]
		if !registered[name] {
			t.Errorf("README table documents `%s`, which is not a registered command", name)
		}
	}
	if rows < 20 {
		t.Fatalf("command table not found: %d rows", rows)
	}
}

func TestConfigDocListsEveryKey(t *testing.T) {
	doc := read(t, "config.md")
	typ := reflect.TypeOf(config.Config{})
	for i := 0; i < typ.NumField(); i++ {
		key := strings.Split(typ.Field(i).Tag.Get("yaml"), ",")[0]
		if !strings.Contains(doc, "`"+key+"`") {
			t.Errorf("docs/config.md does not document `%s`", key)
		}
	}
	for _, key := range []string{
		"semantic.provider", "semantic.base_url", "semantic.model", "semantic.dimensions",
		"semantic.api_key", "semantic.api_key_env", "semantic.api_key_cmd", "semantic.embed_cmd",
		"semantic.batch_size", "semantic.exclude_tags", "semantic.price_per_mtok",
	} {
		if !strings.Contains(doc, "`"+key+"`") {
			t.Errorf("docs/config.md does not document `%s`", key)
		}
	}
}
