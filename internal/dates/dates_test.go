package dates

import (
	"testing"
	"time"
)

func d(y int, m time.Month, day, h, mi, s int) time.Time {
	return time.Date(y, m, day, h, mi, s, 0, time.Local)
}

func TestFormat(t *testing.T) {
	got := Format(d(2026, 11, 27, 14, 30, 0))
	if _, err := time.Parse(time.RFC3339, got); err != nil || got[:19] != "2026-11-27T14:30:00" {
		t.Fatalf("%q %v", got, err)
	}
	if len(got) < 25 && got[len(got)-1] != 'Z' {
		t.Fatalf("no offset in %q", got)
	}
}

func TestParseStored(t *testing.T) {
	cases := []struct {
		in   string
		want time.Time
	}{
		{"2026-11-27T14:30:00-05:00", time.Date(2026, 11, 27, 14, 30, 0, 0, time.FixedZone("", -5*3600))},
		{"2026-11-27T14:30:00Z", time.Date(2026, 11, 27, 14, 30, 0, 0, time.UTC)},
		{"2026-11-27", d(2026, 11, 27, 0, 0, 0)},
		{"2026-11-27T14:30:00", d(2026, 11, 27, 14, 30, 0)},
		{"261127143015", d(2026, 11, 27, 14, 30, 15)},
		{"000102030405", d(2000, 1, 2, 3, 4, 5)},
	}
	for _, c := range cases {
		got, ok := ParseStored(c.in)
		if !ok || !got.Equal(c.want) {
			t.Errorf("%q: got %v %v want %v", c.in, got, ok, c.want)
		}
	}
	for _, bad := range []string{"", "old", "2026-13-01", "26112714", "2611271430150", "2026-11-27T25:00:00"} {
		if _, ok := ParseStored(bad); ok {
			t.Errorf("%q should not parse", bad)
		}
	}
}

func TestPeriod(t *testing.T) {
	cases := []struct {
		in     string
		lo, hi time.Time
	}{
		{"2026", d(2026, 1, 1, 0, 0, 0), d(2027, 1, 1, 0, 0, 0)},
		{"2026-03", d(2026, 3, 1, 0, 0, 0), d(2026, 4, 1, 0, 0, 0)},
		{"2026-03-14", d(2026, 3, 14, 0, 0, 0), d(2026, 3, 15, 0, 0, 0)},
		{"2026-03-14T10", d(2026, 3, 14, 10, 0, 0), d(2026, 3, 14, 11, 0, 0)},
		{"2026-03-14T10:30", d(2026, 3, 14, 10, 30, 0), d(2026, 3, 14, 10, 31, 0)},
		{"2026-03-14T10:30:05", d(2026, 3, 14, 10, 30, 5), d(2026, 3, 14, 10, 30, 6)},
		{"2011-", d(2011, 1, 1, 0, 0, 0), d(2012, 1, 1, 0, 0, 0)},
		{"26", d(2026, 1, 1, 0, 0, 0), d(2027, 1, 1, 0, 0, 0)},
		{"2603", d(2026, 3, 1, 0, 0, 0), d(2026, 4, 1, 0, 0, 0)},
		{"2612", d(2026, 12, 1, 0, 0, 0), d(2027, 1, 1, 0, 0, 0)},
		{"260314", d(2026, 3, 14, 0, 0, 0), d(2026, 3, 15, 0, 0, 0)},
		{"26031415", d(2026, 3, 14, 15, 0, 0), d(2026, 3, 14, 16, 0, 0)},
		{"2603141530", d(2026, 3, 14, 15, 30, 0), d(2026, 3, 14, 15, 31, 0)},
		{"260314153045", d(2026, 3, 14, 15, 30, 45), d(2026, 3, 14, 15, 30, 46)},
		// 4 digits: valid month -> compact, else ISO year
		{"2026", d(2026, 1, 1, 0, 0, 0), d(2027, 1, 1, 0, 0, 0)},
		{"1999", d(1999, 1, 1, 0, 0, 0), d(2000, 1, 1, 0, 0, 0)},
		{"2000", d(2000, 1, 1, 0, 0, 0), d(2001, 1, 1, 0, 0, 0)},
		{"2011", d(2020, 11, 1, 0, 0, 0), d(2020, 12, 1, 0, 0, 0)},
		{"2012", d(2020, 12, 1, 0, 0, 0), d(2021, 1, 1, 0, 0, 0)},
	}
	for _, c := range cases {
		lo, hi, ok := Period(c.in)
		if !ok || !lo.Equal(c.lo) || !hi.Equal(c.hi) {
			t.Errorf("%q: got %v %v %v want %v %v", c.in, lo, hi, ok, c.lo, c.hi)
		}
	}
	for _, bad := range []string{"", "2", "260", "2603141516171", "abcd", "26x", "2613x", "260230", "2026-13", "2026-02-30", "2026-03-14T25", "2026-3", "2026-03-14T10:3", "-", "20261", "2026-03-14X10"} {
		if _, _, ok := Period(bad); ok {
			t.Errorf("%q should be invalid", bad)
		}
	}
}
