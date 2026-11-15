package frontmatter

import "testing"

func TestSplit(t *testing.T) {
	cases := []struct {
		name, src, fm, body string
		has                 bool
		bodyLine            int
	}{
		{"none", "hello\n", "", "hello\n", false, 0},
		{"empty", "---\n---\nbody\n", "", "body\n", true, 2},
		{"basic", "---\na: 1\n---\nbody\n", "a: 1\n", "body\n", true, 3},
		{"unterminated", "---\na: 1\nbody\n", "", "---\na: 1\nbody\n", false, 0},
		{"crlf", "---\r\na: 1\r\n---\r\nbody", "a: 1\r\n", "body", true, 3},
	}
	for _, c := range cases {
		fm, body, has, line := Split([]byte(c.src))
		if string(fm) != c.fm || string(body) != c.body || has != c.has || line != c.bodyLine {
			t.Errorf("%s: got %q %q %v %d", c.name, fm, body, has, line)
		}
	}
}

const sample = "---\n# top comment\ntitle: Hello   # why\nowner: bob\ntags:\n  - a\n  - b\n# before created\nother:\n  nested: 1\n---\nbody\n"

func TestUpdateSetsPreservesRest(t *testing.T) {
	out, err := Update([]byte(sample), func(m *Map) {
		m.Set("tags", []string{"x", "z"})
		m.Set("created", Quoted("260102030405"))
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "---\n# top comment\ntitle: Hello   # why\nowner: bob\ntags:\n  - x\n  - z\n# before created\nother:\n  nested: 1\ncreated: \"260102030405\"\n---\nbody\n"
	if string(out) != want {
		t.Fatalf("got:\n%s\nwant:\n%s", out, want)
	}
}

func TestUpdateReplaceCreatedInPlace(t *testing.T) {
	src := "---\ncreated: 1\nz: 2 # c\n---\nb"
	out, err := Update([]byte(src), func(m *Map) { m.Set("created", Quoted("260102030405")) })
	if err != nil {
		t.Fatal(err)
	}
	if want := "---\ncreated: \"260102030405\"\nz: 2 # c\n---\nb"; string(out) != want {
		t.Fatalf("got %q", out)
	}
}

func TestUpdateInsertsBlock(t *testing.T) {
	out, err := Update([]byte("body\n"), func(m *Map) { m.Set("tags", []string{"a"}) })
	if err != nil {
		t.Fatal(err)
	}
	if want := "---\ntags:\n  - a\n---\nbody\n"; string(out) != want {
		t.Fatalf("got %q", out)
	}
}

func TestUpdateEmptyBlock(t *testing.T) {
	out, err := Update([]byte("---\n---\nbody\n"), func(m *Map) { m.Set("tags", []string{"a"}) })
	if err != nil {
		t.Fatal(err)
	}
	if want := "---\ntags:\n  - a\n---\nbody\n"; string(out) != want {
		t.Fatalf("got %q", out)
	}
}

func TestUpdateDelete(t *testing.T) {
	out, err := Update([]byte(sample), func(m *Map) { m.Delete("tags") })
	if err != nil {
		t.Fatal(err)
	}
	want := "---\n# top comment\ntitle: Hello   # why\nowner: bob\n# before created\nother:\n  nested: 1\n---\nbody\n"
	if string(out) != want {
		t.Fatalf("got:\n%s", out)
	}
}

func TestUpdateInvalidYAML(t *testing.T) {
	if _, err := Update([]byte("---\na: [\n---\nb"), func(m *Map) { m.Set("a", "x") }); err == nil {
		t.Fatal("want error")
	}
}

func TestUpdateDropsReplacedKeyInlineComment(t *testing.T) {
	src := "---\ntags: [a] # keep me?\nz: 1 # other\n---\nb"
	out, err := Update([]byte(src), func(m *Map) { m.Set("tags", []string{"b"}) })
	if err != nil {
		t.Fatal(err)
	}
	if want := "---\ntags:\n  - b\nz: 1 # other\n---\nb"; string(out) != want {
		t.Fatalf("got %q", out)
	}
}
