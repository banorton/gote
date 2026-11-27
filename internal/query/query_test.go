package query

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

func d(y int, m time.Month, day, h, mi, s int) time.Time {
	return time.Date(y, m, day, h, mi, s, 0, time.Local)
}

func TestParseTokens(t *testing.T) {
	cases := []struct {
		in   string
		want Query
	}{
		{"foo bar", Query{Terms: []Term{{Text: "foo"}, {Text: "bar"}}}},
		{`"hello world"`, Query{Terms: []Term{{Text: "hello world", Phrase: true}}}},
		{"-foo", Query{Terms: []Term{{Text: "foo", Neg: true}}}},
		{`-"a b"`, Query{Terms: []Term{{Text: "a b", Phrase: true, Neg: true}}}},
		{"tag:go", Query{Filters: []Filter{{Kind: KindTag, Value: "go"}}}},
		{"-tag:go", Query{Filters: []Filter{{Kind: KindTag, Value: "go", Neg: true}}}},
		{`tag:"two words"`, Query{Filters: []Filter{{Kind: KindTag, Value: "two words"}}}},
		{"status:draft", Query{Filters: []Filter{{Kind: KindField, Field: "status", Value: "draft"}}}},
		{"my-field_1:x", Query{Filters: []Filter{{Kind: KindField, Field: "my-field_1", Value: "x"}}}},
		{"foo tag:a -bar", Query{
			Terms:   []Term{{Text: "foo"}, {Text: "bar", Neg: true}},
			Filters: []Filter{{Kind: KindTag, Value: "a"}},
		}},
		{"  spaced   out ", Query{Terms: []Term{{Text: "spaced"}, {Text: "out"}}}},
		{"-", Query{Terms: []Term{{Text: "-"}}}},
	}
	for _, c := range cases {
		got, err := Parse(c.in, "")
		if err != nil {
			t.Errorf("%q: %v", c.in, err)
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("%q:\n got %+v\nwant %+v", c.in, got, c.want)
		}
	}
}

func TestParseDates(t *testing.T) {
	cases := []struct {
		format, in string
		lo, hi     time.Time
		hasLo      bool
		hasHi      bool
	}{
		{"", "created:2026", d(2026, 1, 1, 0, 0, 0), d(2027, 1, 1, 0, 0, 0), true, true},
		{"iso", "created:2026-03", d(2026, 3, 1, 0, 0, 0), d(2026, 4, 1, 0, 0, 0), true, true},
		{"iso", "created:2026-03-14", d(2026, 3, 14, 0, 0, 0), d(2026, 3, 15, 0, 0, 0), true, true},
		{"iso", "created:2026-03-14T10", d(2026, 3, 14, 10, 0, 0), d(2026, 3, 14, 11, 0, 0), true, true},
		{"iso", "created:2026-03-14T10:30", d(2026, 3, 14, 10, 30, 0), d(2026, 3, 14, 10, 31, 0), true, true},
		{"iso", "created:>2026-03", d(2026, 4, 1, 0, 0, 0), time.Time{}, true, false},
		{"iso", "modified:<2026-03-14", time.Time{}, d(2026, 3, 14, 0, 0, 0), false, true},
		{"iso", "created:2026-01..2026-03", d(2026, 1, 1, 0, 0, 0), d(2026, 4, 1, 0, 0, 0), true, true},
		{"iso", "modified:2026-03", d(2026, 3, 1, 0, 0, 0), d(2026, 4, 1, 0, 0, 0), true, true},
		{"compact", "created:26", d(2026, 1, 1, 0, 0, 0), d(2027, 1, 1, 0, 0, 0), true, true},
		{"compact", "created:2603", d(2026, 3, 1, 0, 0, 0), d(2026, 4, 1, 0, 0, 0), true, true},
		{"compact", "created:260314", d(2026, 3, 14, 0, 0, 0), d(2026, 3, 15, 0, 0, 0), true, true},
		{"compact", "created:260314153045", d(2026, 3, 14, 15, 30, 45), d(2026, 3, 14, 15, 30, 46), true, true},
		{"compact", "created:>2603", d(2026, 4, 1, 0, 0, 0), time.Time{}, true, false},
		{"compact", "created:<2603", time.Time{}, d(2026, 3, 1, 0, 0, 0), false, true},
		{"compact", "created:2601..2603", d(2026, 1, 1, 0, 0, 0), d(2026, 4, 1, 0, 0, 0), true, true},
	}
	for _, c := range cases {
		q, err := Parse(c.in, c.format)
		if err != nil {
			t.Errorf("%s %q: %v", c.format, c.in, err)
			continue
		}
		if len(q.Filters) != 1 || q.Filters[0].Range == nil {
			t.Errorf("%q: %+v", c.in, q)
			continue
		}
		r := q.Filters[0].Range
		if r.HasLo != c.hasLo || r.HasHi != c.hasHi || !r.Lo.Equal(c.lo) || !r.Hi.Equal(c.hi) {
			t.Errorf("%s %q: got %+v want lo=%v hi=%v", c.format, c.in, r, c.lo, c.hi)
		}
	}
	q, _ := Parse("-created:2026", "iso")
	if !q.Filters[0].Neg || q.Filters[0].Kind != KindCreated {
		t.Errorf("%+v", q)
	}
}

func TestRangeUnix(t *testing.T) {
	q, _ := Parse("created:2603", "compact")
	lo, hi := q.Filters[0].Range.UnixBounds()
	if lo == nil || hi == nil || *lo != d(2026, 3, 1, 0, 0, 0).Unix() || *hi != d(2026, 4, 1, 0, 0, 0).Unix() {
		t.Fatalf("%v %v", lo, hi)
	}
	q, _ = Parse("created:<2026-03", "iso")
	lo, hi = q.Filters[0].Range.UnixBounds()
	if lo != nil || hi == nil || *hi != d(2026, 3, 1, 0, 0, 0).Unix() {
		t.Fatalf("%v %v", lo, hi)
	}
}

func TestMentionsArchive(t *testing.T) {
	for in, want := range map[string]bool{"tag:archive": true, "TAG:Archive x": true, "-tag:archive": false, "tag:other": false, "archive": false} {
		q, err := Parse(in, "")
		if err != nil {
			t.Fatal(in, err)
		}
		if got := q.MentionsArchive(); got != want {
			t.Errorf("%q: got %v", in, got)
		}
	}
}

func TestParseErrors(t *testing.T) {
	type tc struct{ format, in, want string }
	cases := []tc{
		{"iso", "created:2603", "created:2603"},
		{"iso", "created:260314", "(date_format: iso)"},
		{"iso", "created:2011-", "created:2011-"},
		{"iso", "created:2026-13", "created:2026-13"},
		{"iso", "created:2026-02-30", "created:2026-02-30"},
		{"iso", "created:2026-03-14T25", "created:2026-03-14T25"},
		{"iso", "created:2026-3", "created:2026-3"},
		{"iso", "created:20261", "created:20261"},
		{"iso", "created:>26x", "created:>26x"},
		{"iso", "created:2026..", "created:2026.."},
		{"iso", "created:..2026", "created:..2026"},
		{"", "modified:123", "modified:123"},
		{"compact", "created:2026-03", "(date_format: compact)"},
		{"compact", "created:2026", "created:2026"},
		{"compact", "created:260", "created:260"},
		{"compact", "created:2", "created:2"},
		{"compact", "created:2603141516171", "created:2603141516171"},
		{"compact", "created:2613", "created:2613"},
		{"compact", "created:260230", "created:260230"},
		{"compact", "created:2601..", "created:2601.."},
		{"", "", "empty"},
		{"", "   ", "empty"},
		{"", "created:", "created:"},
		{"", "bad.field:x", "bad.field:x"},
		{"", ":x", ":x"},
		{"", `"unterminated`, "unterminated"},
		{"", "tag:", "tag:"},
		{"", "sta$tus:x", "sta$tus:x"},
	}
	for _, c := range cases {
		_, err := Parse(c.in, c.format)
		if err == nil {
			t.Errorf("%s %q: want error", c.format, c.in)
			continue
		}
		if !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s %q: error %q does not contain %q", c.format, c.in, err, c.want)
		}
	}
}

func TestQuote(t *testing.T) {
	if got := Quote(`a"b`); got != `"a""b"` {
		t.Fatal(got)
	}
	if got := Quote("NEAR"); got != `"NEAR"` {
		t.Fatal(got)
	}
}
