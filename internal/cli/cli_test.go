package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func fake(name string, aliases ...string) Command {
	return Command{Name: name, Aliases: aliases, NoConfig: true, Run: func(c *Ctx, args []string) error {
		c.Out.Write([]byte(name + ":" + strings.Join(args, ",")))
		return nil
	}}
}

func TestRegisterCollisionPanics(t *testing.T) {
	cases := map[string][]Command{
		"name/name":   {fake("a"), fake("a")},
		"name/alias":  {fake("a"), fake("b", "a")},
		"alias/name":  {fake("a", "x"), fake("x")},
		"alias/alias": {fake("a", "x"), fake("b", "x")},
	}
	for name, cmds := range cases {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("%s: expected panic", name)
				}
			}()
			r := newRegistry()
			for _, c := range cmds {
				r.register(c)
			}
		}()
	}
}

func runWith(r *registry, args ...string) (int, string, string) {
	var out, errb bytes.Buffer
	code := r.run(args, &out, &errb)
	return code, out.String(), errb.String()
}

func TestResolution(t *testing.T) {
	r := newRegistry()
	r.register(fake("help"))
	if code, out, _ := runWith(r); code != 0 || out != "help:" {
		t.Fatalf("no args without quick -> help: %d %q", code, out)
	}
	if code, _, errs := runWith(r, "something"); code != 2 || !strings.Contains(errs, "something") {
		t.Fatalf("unknown without open -> usage: %d %q", code, errs)
	}
	r.register(fake("quick"))
	r.register(fake("open"))
	r.register(fake("status", "st"))
	if _, out, _ := runWith(r); out != "quick:" {
		t.Fatal(out)
	}
	if _, out, _ := runWith(r, "st", "x"); out != "status:x" {
		t.Fatal(out)
	}
	if _, out, _ := runWith(r, "my", "note"); out != "open:my,note" {
		t.Fatal(out)
	}
	if _, out, _ := runWith(r, "-"); out != "open:-" {
		t.Fatal(out)
	}
	if _, out, _ := runWith(r, "-d", "name"); out != "open:-d,name" {
		t.Fatal(out)
	}
	if _, out, _ := runWith(r, "status", "open"); out != "status:open" {
		t.Fatal("registered names win only in command position: " + out)
	}
}

func TestGlobalFlags(t *testing.T) {
	r := newRegistry()
	var seen *Ctx
	r.register(Command{Name: "x", NoConfig: true, Run: func(c *Ctx, args []string) error {
		seen = c
		c.Out.Write([]byte(strings.Join(args, ",")))
		return nil
	}})
	_, out, _ := runWith(r, "--json", "x", "a", "--no-server", "b")
	if !seen.JSON || !seen.NoServer || out != "a,b" {
		t.Fatalf("%+v %q", seen, out)
	}
	Version = "9.9.9"
	if code, out, _ := runWith(r, "--version"); code != 0 || !strings.Contains(out, "9.9.9") {
		t.Fatalf("%d %q", code, out)
	}
	if code, out, _ := runWith(r, "--help"); code != 0 || !strings.Contains(out, "x") {
		t.Fatalf("%d %q", code, out)
	}
}

func TestUsageErrorExit2AndErrorExit1(t *testing.T) {
	r := newRegistry()
	r.register(Command{Name: "u", NoConfig: true, Run: func(c *Ctx, args []string) error { return Usagef("bad args") }})
	r.register(Command{Name: "e", NoConfig: true, Run: func(c *Ctx, args []string) error { return os.ErrNotExist }})
	if code, _, errs := runWith(r, "u"); code != 2 || !strings.Contains(errs, "bad args") {
		t.Fatalf("%d %q", code, errs)
	}
	if code, _, errs := runWith(r, "e"); code != 1 || !strings.Contains(errs, "file does not exist") {
		t.Fatalf("%d %q", code, errs)
	}
}

func TestFlagParsingErrorIsUsage(t *testing.T) {
	r := newRegistry()
	r.register(Command{Name: "f", NoConfig: true, Run: func(c *Ctx, args []string) error {
		fs := c.FlagSet("f")
		fs.Bool("v", false, "")
		return ParseFlags(fs, args)
	}})
	if code, _, _ := runWith(r, "f", "-zzz"); code != 2 {
		t.Fatal(code)
	}
	if code, _, _ := runWith(r, "f", "-v"); code != 0 {
		t.Fatal(code)
	}
}

func env(t *testing.T) (notes string) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "cfg"))
	t.Setenv("XDG_STATE_HOME", filepath.Join(dir, "state"))
	notes = filepath.Join(dir, "notes")
	os.MkdirAll(notes, 0o755)
	return
}

func writeConfig(t *testing.T, notes string) {
	t.Helper()
	p := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "gote", "config.yaml")
	os.MkdirAll(filepath.Dir(p), 0o755)
	os.WriteFile(p, []byte("notes_dir: "+notes+"\n"), 0o644)
}

func TestMissingConfigExit1NamesPath(t *testing.T) {
	env(t)
	code, _, errs := runWith(std, "status")
	want := filepath.Join(os.Getenv("XDG_CONFIG_HOME"), "gote", "config.yaml")
	if code != 1 || !strings.Contains(errs, want) {
		t.Fatalf("%d %q", code, errs)
	}
}

func TestStatusAndReindex(t *testing.T) {
	notes := env(t)
	writeConfig(t, notes)
	os.WriteFile(filepath.Join(notes, "a.md"), []byte("[[b]] [[nope]]"), 0o644)
	os.WriteFile(filepath.Join(notes, "b.md"), []byte("x"), 0o644)

	code, out, errs := runWith(std, "status", "--json")
	if code != 0 {
		t.Fatalf("%d %s", code, errs)
	}
	var st struct{ Notes, Links, Unresolved int }
	if err := json.Unmarshal([]byte(out), &st); err != nil || st.Notes != 2 || st.Links != 2 || st.Unresolved != 1 {
		t.Fatalf("%v %s", err, out)
	}

	os.WriteFile(filepath.Join(notes, "c.md"), []byte("x"), 0o644)
	code, out, errs = runWith(std, "reindex", "--json")
	var rs struct{ Scanned, Added int }
	if code != 0 || json.Unmarshal([]byte(out), &rs) != nil || rs.Scanned != 3 || rs.Added != 3 {
		t.Fatalf("%d %s %s", code, out, errs)
	}
	if code, out, _ := runWith(std, "idx"); code != 0 || !strings.Contains(out, "3") {
		t.Fatalf("%d %s", code, out)
	}
	if code, out, _ := runWith(std, "status"); code != 0 || !strings.Contains(out, "notes") {
		t.Fatalf("%d %s", code, out)
	}
}

func TestVersionAndHelpWorkWithoutConfig(t *testing.T) {
	env(t)
	if code, out, _ := runWith(std, "version"); code != 0 || !strings.Contains(out, "gote") {
		t.Fatalf("%d %s", code, out)
	}
	if code, out, _ := runWith(std, "help"); code != 0 || !strings.Contains(out, "reindex") || !strings.Contains(out, "idx") {
		t.Fatalf("%d %s", code, out)
	}
	if code, out, _ := runWith(std, "man", "status"); code != 0 || !strings.Contains(out, "status") {
		t.Fatalf("%d %s", code, out)
	}
}
