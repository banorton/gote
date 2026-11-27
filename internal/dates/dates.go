// Package dates owns gote's timestamp forms: ISO 8601 with offset for stored
// and printed values, legacy compact yymmddhhmmss accepted on read, and the
// prefix periods the created:/modified: filters expand to.
package dates

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Format renders t as ISO 8601 with its offset, seconds precision.
func Format(t time.Time) string { return t.Format(time.RFC3339) }

// ParseStored reads a frontmatter created value: RFC 3339, date-only or
// offset-less ISO (local time), or the legacy 12-digit yymmddhhmmss.
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
	if len(s) == 12 && allDigits(s) {
		if lo, _, ok := compact(s); ok {
			return lo, true
		}
	}
	return time.Time{}, false
}

var isoRe = regexp.MustCompile(`^(\d{4})(?:-(\d{2})(?:-(\d{2})(?:T(\d{2})(?::(\d{2})(?::(\d{2}))?)?)?)?)?$`)

// Period expands a filter date token into the half-open local-time interval it
// denotes. A token containing '-', 'T' or ':' is an ISO prefix (2026, 2026-03,
// 2026-03-14, 2026-03-14T10, 2026-03-14T10:30[:05]); a trailing '-' alone
// forces an ISO year (2011-). Digits only is compact yymmddhhmmss (even length
// 2-12), except a 4-digit token whose month part is not 01-12, which is an ISO
// year.
func Period(tok string) (start, end time.Time, ok bool) {
	if strings.ContainsAny(tok, "-T:") {
		return iso(strings.TrimSuffix(tok, "-"))
	}
	if !allDigits(tok) {
		return start, end, false
	}
	if len(tok) == 4 {
		if m, _ := strconv.Atoi(tok[2:]); m < 1 || m > 12 {
			return iso(tok)
		}
	}
	return compact(tok)
}

func allDigits(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
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
	return build(parts, n)
}

func compact(tok string) (start, end time.Time, ok bool) {
	n := len(tok)
	if n < 2 || n > 12 || n%2 != 0 || !allDigits(tok) {
		return start, end, false
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
