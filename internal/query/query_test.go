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
		got, err := Parse(c.in)
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
		in     string
		lo, hi time.Time
		hasLo  bool
		hasHi  bool
	}{
		{"created:26", d(2026, 1, 1, 0, 0, 0), d(2027, 1, 1, 0, 0, 0), true, true},
		{"created:2603", d(2026, 3, 1, 0, 0, 0), d(2026, 4, 1, 0, 0, 0), true, true},
		{"created:2612", d(2026, 12, 1, 0, 0, 0), d(2027, 1, 1, 0, 0, 0), true, true},
		{"created:260314", d(2026, 3, 14, 0, 0, 0), d(2026, 3, 15, 0, 0, 0), true, true},
		{"created:26031415", d(2026, 3, 14, 15, 0, 0), d(2026, 3, 14, 16, 0, 0), true, true},
		{"created:260314153045", d(2026, 3, 14, 15, 30, 45), d(2026, 3, 14, 15, 30, 46), true, true},
		{"created:>2603", d(2026, 4, 1, 0, 0, 0), time.Time{}, true, false},
		{"created:<2603", time.Time{}, d(2026, 3, 1, 0, 0, 0), false, true},
		{"created:2601..2603", d(2026, 1, 1, 0, 0, 0), d(2026, 4, 1, 0, 0, 0), true, true},
		{"modified:2603", d(2026, 3, 1, 0, 0, 0), d(2026, 4, 1, 0, 0, 0), true, true},
	}
	for _, c := range cases {
		q, err := Parse(c.in)
		if err != nil {
			t.Errorf("%q: %v", c.in, err)
			continue
		}
		if len(q.Filters) != 1 || q.Filters[0].Range == nil {
			t.Errorf("%q: %+v", c.in, q)
			continue
		}
		r := q.Filters[0].Range
		if r.HasLo != c.hasLo || r.HasHi != c.hasHi || !r.Lo.Equal(c.lo) || !r.Hi.Equal(c.hi) {
			t.Errorf("%q: got %+v want lo=%v hi=%v", c.in, r, c.lo, c.hi)
		}
	}
	q, _ := Parse("-created:26")
	if !q.Filters[0].Neg || q.Filters[0].Kind != KindCreated {
		t.Errorf("%+v", q)
	}
}

func TestRangeStrings(t *testing.T) {
	q, _ := Parse("created:2603")
	lo, hi := q.Filters[0].Range.CreatedBounds()
	if lo != "260301000000" || hi != "260401000000" {
		t.Fatalf("%q %q", lo, hi)
	}
	q, _ = Parse("created:<2603")
	lo, hi = q.Filters[0].Range.CreatedBounds()
	if lo != "" || hi != "260301000000" {
		t.Fatalf("%q %q", lo, hi)
	}
}

func TestParseErrors(t *testing.T) {
	cases := map[string]string{
		"":                      "empty",
		"   ":                   "empty",
		"created:260":           "created:260",
		"created:2":             "created:2",
		"created:2603141516171": "created:2603141516171",
		"created:abcd":          "created:abcd",
		"created:>26x":          "created:>26x",
		"created:2613":          "created:2613",
		"created:260230":        "created:260230",
		"created:2601..":        "created:2601..",
		"created:..2603":        "created:..2603",
		"modified:123":          "modified:123",
		"created:":              "created:",
		"bad.field:x":           "bad.field:x",
		":x":                    ":x",
		"a b:c d":               "",
		`"unterminated`:         "unterminated",
		"tag:":                  "tag:",
		"sta$tus:x":             "sta$tus:x",
	}
	for in, want := range cases {
		if in == "a b:c d" {
			continue
		}
		_, err := Parse(in)
		if err == nil {
			t.Errorf("%q: want error", in)
			continue
		}
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%q: error %q does not name %q", in, err, want)
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
