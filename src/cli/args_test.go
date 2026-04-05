package cli

import (
	"reflect"
	"testing"
)

func TestParseArgs_BoolFlags(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		wantFlag string
		want     bool
	}{
		{"short flag", []string{"-o"}, "o", true},
		{"long flag", []string{"--open"}, "open", true},
		{"missing flag", []string{"foo"}, "o", false},
		{"multiple flags", []string{"-o", "--verbose"}, "verbose", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := ParseArgs(tt.input)
			if got := args.Has(tt.wantFlag); got != tt.want {
				t.Errorf("Has(%q) = %v, want %v", tt.wantFlag, got, tt.want)
			}
		})
	}
}

func TestParseArgs_ValueFlags(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		flag     string
		wantStr  string
		wantInt  int
	}{
		{"short with value", []string{"-n", "5"}, "n", "5", 5},
		{"long with value", []string{"--limit", "10"}, "limit", "10", 10},
		{"flag without value", []string{"-n"}, "n", "", 0},
		{"missing flag", []string{"foo"}, "n", "", 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := ParseArgs(tt.input)
			if got := args.String(tt.flag); got != tt.wantStr {
				t.Errorf("String(%q) = %q, want %q", tt.flag, got, tt.wantStr)
			}
			if got := args.Int(tt.flag); got != tt.wantInt {
				t.Errorf("Int(%q) = %d, want %d", tt.flag, got, tt.wantInt)
			}
		})
	}
}

func TestParseArgs_IntOr(t *testing.T) {
	tests := []struct {
		name    string
		input   []string
		def     int
		flags   []string
		want    int
	}{
		{"has value", []string{"-n", "5"}, 10, []string{"n"}, 5},
		{"missing uses default", []string{"foo"}, 10, []string{"n"}, 10},
		{"invalid uses default", []string{"-n", "abc"}, 10, []string{"n"}, 10},
		{"multiple flag names", []string{"--limit", "3"}, 10, []string{"n", "limit"}, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := ParseArgs(tt.input)
			if got := args.IntOr(tt.def, tt.flags...); got != tt.want {
				t.Errorf("IntOr(%d, %v) = %d, want %d", tt.def, tt.flags, got, tt.want)
			}
		})
	}
}

func TestParseArgs_ListFlags(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		flag  string
		want  []string
	}{
		{"single value", []string{"-t", "foo"}, "t", []string{"foo"}},
		{"multiple values", []string{"-t", "foo", "bar", "baz"}, "t", []string{"foo", "bar", "baz"}},
		{"stops at next flag", []string{"-t", "foo", "bar", "-n", "5"}, "t", []string{"foo", "bar"}},
		{"empty list", []string{"-t"}, "t", []string{}},
		{"missing flag", []string{"foo"}, "t", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := ParseArgs(tt.input)
			got := args.List(tt.flag)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("List(%q) = %v, want %v", tt.flag, got, tt.want)
			}
		})
	}
}

func TestParseArgs_Positional(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{"no flags", []string{"foo", "bar"}, []string{"foo", "bar"}},
		{"positional before flag", []string{"foo", "-o"}, []string{"foo"}},
		{"positional before value flag", []string{"foo", "-n", "5"}, []string{"foo"}},
		{"empty", []string{}, []string{}},
		{"only flags", []string{"-o", "-n", "5"}, []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := ParseArgs(tt.input)
			if !reflect.DeepEqual(args.Positional, tt.want) {
				t.Errorf("Positional = %v, want %v", args.Positional, tt.want)
			}
		})
	}
}

func TestParseArgs_First(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  string
	}{
		{"has positional", []string{"foo", "bar"}, "foo"},
		{"empty", []string{}, ""},
		{"only flags", []string{"-o"}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := ParseArgs(tt.input)
			if got := args.First(); got != tt.want {
				t.Errorf("First() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseArgs_Rest(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []string
	}{
		{"multiple positional", []string{"foo", "bar", "baz"}, []string{"bar", "baz"}},
		{"single positional", []string{"foo"}, nil},
		{"empty", []string{}, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := ParseArgs(tt.input)
			if got := args.Rest(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Rest() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseArgs_Joined(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  string
	}{
		{"multiple words", []string{"my", "note", "name"}, "my note name"},
		{"with bool flag at end", []string{"my", "note", "-o"}, "my note"},
		{"with value flag at end", []string{"foo", "bar", "-n", "5"}, "foo bar"},
		{"empty", []string{}, ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			args := ParseArgs(tt.input)
			if got := args.Joined(); got != tt.want {
				t.Errorf("Joined() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseArgs_MultipleAliases(t *testing.T) {
	args := ParseArgs([]string{"-o", "--limit", "5"})

	// Should find flag with any alias
	if !args.Has("o", "open") {
		t.Error("Has(o, open) should be true")
	}
	if args.Int("n", "limit") != 5 {
		t.Errorf("Int(n, limit) = %d, want 5", args.Int("n", "limit"))
	}
}

func TestParseArgs_RealWorldExamples(t *testing.T) {
	t.Run("search with tags", func(t *testing.T) {
		args := ParseArgs([]string{"-t", "work", "project", "-n", "5", "-o"})

		if !args.Has("o", "open") {
			t.Error("should have open flag")
		}
		tags := args.List("t", "tags")
		if !reflect.DeepEqual(tags, []string{"work", "project"}) {
			t.Errorf("tags = %v, want [work project]", tags)
		}
		if args.IntOr(-1, "n", "limit") != 5 {
			t.Errorf("limit = %d, want 5", args.IntOr(-1, "n", "limit"))
		}
	})

	t.Run("search with query", func(t *testing.T) {
		args := ParseArgs([]string{"my", "search", "query", "-n", "10"})

		if args.Joined() != "my search query" {
			t.Errorf("query = %q, want 'my search query'", args.Joined())
		}
		if args.IntOr(-1, "n") != 10 {
			t.Errorf("limit = %d, want 10", args.IntOr(-1, "n"))
		}
	})

	t.Run("rename command", func(t *testing.T) {
		args := ParseArgs([]string{"old", "note", "name", "-n", "new", "note", "name"})

		if args.Joined() != "old note name" {
			t.Errorf("old name = %q, want 'old note name'", args.Joined())
		}
		newName := args.List("n", "name")
		if !reflect.DeepEqual(newName, []string{"new", "note", "name"}) {
			t.Errorf("new name = %v, want [new note name]", newName)
		}
	})

	t.Run("tag command", func(t *testing.T) {
		args := ParseArgs([]string{"my", "note", "-t", "tag1", "tag2"})

		if args.Joined() != "my note" {
			t.Errorf("note = %q, want 'my note'", args.Joined())
		}
		tags := args.List("t")
		if !reflect.DeepEqual(tags, []string{"tag1", "tag2"}) {
			t.Errorf("tags = %v, want [tag1 tag2]", tags)
		}
	})
}

func TestParseTagString(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{"simple tags", ".work.project", []string{"work", "project"}},
		{"single tag", ".work", []string{"work"}},
		{"tag with spaces", ".tag with spaces.another", []string{"tag with spaces", "another"}},
		{"empty string", "", []string{}},
		{"no leading dot", "work.project", []string{}},
		{"only dot", ".", []string{}},
		{"multiple dots", ".work..project", []string{"work", "project"}},
		{"mixed case normalizes", ".Work.PROJECT", []string{"work", "project"}},
		{"trims whitespace", ". work . project ", []string{"work", "project"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseTagString(tt.input)
			if len(got) == 0 && len(tt.want) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ParseTagString(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
