package skill

import (
	"encoding/json"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

var customizeBlocks = []string{
	"layout", "note-types", "tags", "naming", "daily",
	"create-vs-append", "linking", "permissions", "integrations", "reporting",
}

var frontmatterFields = []string{"tags", "aliases", "created", "title", "related", "parent", "semantic"}

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func templateCore(t *testing.T) string {
	t.Helper()
	s := readFile(t, "template/SKILL.md")
	_, rest, ok := strings.Cut(s, "<!-- GOTE-CORE:START")
	if !ok {
		t.Fatal("template/SKILL.md has no GOTE-CORE:START marker")
	}
	core, _, ok := strings.Cut(rest, "<!-- GOTE-CORE:END")
	if !ok {
		t.Fatal("template/SKILL.md has no GOTE-CORE:END marker")
	}
	return core
}

func TestTemplateFrontmatterPlaceholders(t *testing.T) {
	s := readFile(t, "template/SKILL.md")
	if !strings.HasPrefix(s, "---\nname: <your-skill-name>\ndescription: ") {
		t.Fatal("frontmatter must start with name: <your-skill-name> and a description")
	}
	end := strings.Index(s[4:], "\n---\n")
	if end < 0 || !strings.Contains(s[:end], "<when your agent should use it") {
		t.Fatal("description must be a placeholder")
	}
}

func TestTemplateCommandTableCoversRegistry(t *testing.T) {
	core := templateCore(t)
	_, table, ok := strings.Cut(core, "| Command | Aliases |")
	if !ok {
		t.Fatal("core has no command table (header `| Command | Aliases |`)")
	}
	cmds := map[string]bool{}
	aliases := map[string]bool{}
	span := regexp.MustCompile("`([^`]+)`")
	for _, line := range strings.Split(table, "\n") {
		if !strings.HasPrefix(line, "|") {
			if len(cmds) > 0 {
				break
			}
			continue
		}
		cols := strings.Split(line, "|")
		if len(cols) < 4 {
			continue
		}
		for _, m := range span.FindAllStringSubmatch(cols[1], -1) {
			cmds[strings.Fields(m[1])[0]] = true
		}
		for _, m := range span.FindAllStringSubmatch(cols[2], -1) {
			aliases[m[1]] = true
		}
	}
	out, err := exec.Command(buildGote(t), "help", "--json").Output()
	if err != nil {
		t.Fatal(err)
	}
	var entries []struct {
		Name    string   `json:"name"`
		Aliases []string `json:"aliases"`
	}
	if err := json.Unmarshal(out, &entries); err != nil {
		t.Fatal(err)
	}
	registered := map[string]bool{}
	for _, e := range entries {
		registered[e.Name] = true
		if !cmds[e.Name] {
			t.Errorf("template command table is missing command `%s`", e.Name)
		}
		for _, a := range e.Aliases {
			if !aliases[a] {
				t.Errorf("template command table is missing alias `%s` of `%s`", a, e.Name)
			}
		}
	}
	for c := range cmds {
		if !registered[c] {
			t.Errorf("template command table lists `%s`, which is not a registered command", c)
		}
	}
}

func TestTemplateCoreDocumentsFrontmatterFields(t *testing.T) {
	core := templateCore(t)
	cfg := readFile(t, "../docs/config.md")
	if !strings.Contains(cfg, "`semantic: false`") {
		t.Fatal("docs/config.md no longer documents `semantic: false`; update frontmatterFields")
	}
	for _, f := range frontmatterFields {
		if !strings.Contains(core, "`"+f+"`") && !strings.Contains(core, "`"+f+":") {
			t.Errorf("core section does not document frontmatter field `%s`", f)
		}
	}
	if !strings.Contains(core, "`semantic: false`") {
		t.Error("core section must document `semantic: false`")
	}
}

func TestTemplateHasEveryCustomizeBlock(t *testing.T) {
	s := readFile(t, "template/SKILL.md")
	build := readFile(t, "template/BUILD.md")
	re := regexp.MustCompile(`<!-- CUSTOMIZE: ([a-z-]+)\b`)
	var found []string
	for _, m := range re.FindAllStringSubmatch(s, -1) {
		found = append(found, m[1])
	}
	if strings.Join(found, ",") != strings.Join(customizeBlocks, ",") {
		t.Errorf("CUSTOMIZE blocks = %v, want %v", found, customizeBlocks)
	}
	core := templateCore(t)
	if strings.Contains(core, "CUSTOMIZE:") {
		t.Error("core section must not contain CUSTOMIZE blocks")
	}
	for _, b := range customizeBlocks {
		if !strings.Contains(build, "`"+b+"`") {
			t.Errorf("BUILD.md does not cover block `%s`", b)
		}
	}
}

func TestTemplateCoreSafetyRules(t *testing.T) {
	core := templateCore(t)
	for _, want := range []string{"--json", "--no-server", "gote rename", ".trash", "--yes", "diagram check", "not_found", "`-`"} {
		if !strings.Contains(core, want) {
			t.Errorf("core section is missing %q", want)
		}
	}
}
