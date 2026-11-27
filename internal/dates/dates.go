// Package dates owns gote's timestamp forms: ISO 8601 with offset for stored
// and printed values, and the prefix periods the created:/modified: filters
// expand to in the configured date_format (iso or compact).
package dates

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Values of the date_format config key.
const (
	ISO     = "iso"
	Compact = "compact"
)

// Format renders t as ISO 8601 with its offset, seconds precision.
func Format(t time.Time) string { return t.Format(time.RFC3339) }

// ParseStored reads a frontmatter created value: RFC 3339, or date-only or
// offset-less ISO (local time). Anything else is not a date.
func ParseStored(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, true
	}
	for _, layout := range []string{"2006-01-02T15:04:05", "2006-01-02"} {
		if t, err := time.ParseInLocation(layout, s, time.Local); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// ValidFormat reports whether f is a date_format value; "" means iso.
func ValidFormat(f string) bool { return f == "" || f == ISO || f == Compact }

// Expected describes the filter date syntax of format f, for error messages.
func Expected(f string) string {
	if f == Compact {
		return "expected compact date like 2603 or 260314 (yymmddhhmmss prefix, even length 2-12) (date_format: compact)"
	}
	return "expected ISO date like 2026-03 or 2026-03-14T10:30 (date_format: iso)"
}

var isoRe = regexp.MustCompile(`^(\d{4})(?:-(\d{2})(?:-(\d{2})(?:T(\d{2})(?::(\d{2})(?::(\d{2}))?)?)?)?)?$`)

// Period expands a filter date token into the half-open local-time interval it
// denotes, in the given format. iso: 2026, 2026-03, 2026-03-14, 2026-03-14T10,
// 2026-03-14T10:30[:05]. compact: yymmddhhmmss prefix of even length 2-12.
func Period(tok, format string) (start, end time.Time, ok bool) {
	if format == Compact {
		return compact(tok)
	}
	return iso(tok)
}

func iso(tok string) (start, end time.Time, ok bool) {
	m := isoRe.FindStringSubmatch(tok)
	if m == nil {
		return start, end, false
	}
	parts := [6]int{0, 1, 1, 0, 0, 0}
	n := 0
	for i := 1; i <= 6; i++ {
		if m[i] == "" {
			break
		}
		parts[i-1], _ = strconv.Atoi(m[i])
		n = i
	}
	// A year far outside any real note is almost always a compact value
	// (2603 for March 2026) typed under the wrong date_format.
	if parts[0] < 1900 || parts[0] > 2199 {
		return start, end, false
	}
	return build(parts, n)
}

func compact(tok string) (start, end time.Time, ok bool) {
	n := len(tok)
	if n < 2 || n > 12 || n%2 != 0 {
		return start, end, false
	}
	for i := 0; i < n; i++ {
		if tok[i] < '0' || tok[i] > '9' {
			return start, end, false
		}
	}
	parts := [6]int{0, 1, 1, 0, 0, 0}
	for i := 0; i < n/2; i++ {
		parts[i] = int(tok[2*i]-'0')*10 + int(tok[2*i+1]-'0')
	}
	parts[0] += 2000
	return build(parts, n/2)
}

// build makes the interval for the first n components of parts (year, month,
// day, hour, minute, second), rejecting values that do not round-trip.
func build(parts [6]int, n int) (start, end time.Time, ok bool) {
	start = time.Date(parts[0], time.Month(parts[1]), parts[2], parts[3], parts[4], parts[5], 0, time.Local)
	if start.Year() != parts[0] || int(start.Month()) != parts[1] || start.Day() != parts[2] ||
		start.Hour() != parts[3] || start.Minute() != parts[4] || start.Second() != parts[5] {
		return start, end, false
	}
	switch n {
	case 1:
		end = start.AddDate(1, 0, 0)
	case 2:
		end = start.AddDate(0, 1, 0)
	case 3:
		end = start.AddDate(0, 0, 1)
	case 4:
		end = start.Add(time.Hour)
	case 5:
		end = start.Add(time.Minute)
	case 6:
		end = start.Add(time.Second)
	}
	return start, end, true
}
