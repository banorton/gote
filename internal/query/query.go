package query

import (
	"fmt"
	"strings"
	"time"
)

const (
	KindTag      = "tag"
	KindCreated  = "created"
	KindModified = "modified"
	KindField    = "field"
)

type Term struct {
	Text   string
	Phrase bool
	Neg    bool
}

// Range is the half-open interval [Lo, Hi) in local time; an unset side is unbounded.
type Range struct {
	Lo, Hi       time.Time
	HasLo, HasHi bool
}

type Filter struct {
	Kind  string
	Field string
	Value string
	Range *Range
	Neg   bool
}

type Query struct {
	Terms   []Term
	Filters []Filter
}

type Error struct{ Msg string }

func (e *Error) Error() string { return e.Msg }

func errf(format string, args ...any) error { return &Error{Msg: fmt.Sprintf(format, args...)} }

const createdLayout = "060102150405"

// CreatedBounds renders the range as fixed-width yymmddhhmmss strings; an
// unbounded side is "". Fixed width makes the string range equal the time range.
func (r Range) CreatedBounds() (lo, hi string) {
	if r.HasLo {
		lo = createdString(r.Lo)
	}
	if r.HasHi {
		hi = createdString(r.Hi)
	}
	return lo, hi
}

func createdString(t time.Time) string {
	if t.Year() > 2099 {
		return "a"
	}
	if t.Year() < 2000 {
		return "0"
	}
	return t.Format(createdLayout)
}

func validName(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '_', r == '-':
		default:
			return false
		}
	}
	return true
}

func isSpace(r rune) bool { return r == ' ' || r == '\t' || r == '\n' || r == '\r' }

func Parse(s string) (Query, error) {
	var q Query
	rs := []rune(s)
	i := 0
	for {
		for i < len(rs) && isSpace(rs[i]) {
			i++
		}
		if i >= len(rs) {
			break
		}
		neg := false
		if rs[i] == '-' && i+1 < len(rs) && !isSpace(rs[i+1]) {
			neg = true
			i++
		}
		if rs[i] == '"' {
			text, next, err := readQuoted(rs, i)
			if err != nil {
				return Query{}, err
			}
			i = next
			if strings.TrimSpace(text) == "" {
				return Query{}, errf("empty phrase")
			}
			q.Terms = append(q.Terms, Term{Text: text, Phrase: true, Neg: neg})
			continue
		}
		start := i
		colon := -1
		for i < len(rs) && !isSpace(rs[i]) {
			if rs[i] == ':' {
				colon = i
				break
			}
			i++
		}
		if colon < 0 {
			q.Terms = append(q.Terms, Term{Text: string(rs[start:i]), Neg: neg})
			continue
		}
		key := string(rs[start:colon])
		i = colon + 1
		var val string
		if i < len(rs) && rs[i] == '"' {
			text, next, err := readQuoted(rs, i)
			if err != nil {
				return Query{}, err
			}
			val, i = text, next
		} else {
			vs := i
			for i < len(rs) && !isSpace(rs[i]) {
				i++
			}
			val = string(rs[vs:i])
		}
		token := key + ":" + val
		if !validName(key) {
			return Query{}, errf("invalid token %q: field name must match [A-Za-z0-9_-]+", token)
		}
		if val == "" {
			return Query{}, errf("invalid token %q: empty value", token)
		}
		f, err := makeFilter(key, val, token)
		if err != nil {
			return Query{}, err
		}
		f.Neg = neg
		q.Filters = append(q.Filters, f)
	}
	if len(q.Terms) == 0 && len(q.Filters) == 0 {
		return Query{}, errf("empty query")
	}
	return q, nil
}

func readQuoted(rs []rune, i int) (string, int, error) {
	for j := i + 1; j < len(rs); j++ {
		if rs[j] == '"' {
			return string(rs[i+1 : j]), j + 1, nil
		}
	}
	return "", 0, errf("unterminated quote in %q", string(rs[i:]))
}

func makeFilter(key, val, token string) (Filter, error) {
	switch strings.ToLower(key) {
	case KindTag:
		return Filter{Kind: KindTag, Value: val}, nil
	case KindCreated, KindModified:
		r, err := parseRange(val, token)
		if err != nil {
			return Filter{}, err
		}
		return Filter{Kind: strings.ToLower(key), Range: &r}, nil
	}
	return Filter{Kind: KindField, Field: key, Value: val}, nil
}

func parseRange(v, token string) (Range, error) {
	bad := func() error {
		return errf("invalid token %q: date must be 2-12 digits (even count), optionally >d, <d or d..d", token)
	}
	switch {
	case strings.HasPrefix(v, ">"):
		_, end, ok := period(v[1:])
		if !ok {
			return Range{}, bad()
		}
		return Range{Lo: end, HasLo: true}, nil
	case strings.HasPrefix(v, "<"):
		start, _, ok := period(v[1:])
		if !ok {
			return Range{}, bad()
		}
		return Range{Hi: start, HasHi: true}, nil
	}
	if a, b, found := strings.Cut(v, ".."); found {
		start, _, ok1 := period(a)
		_, end, ok2 := period(b)
		if !ok1 || !ok2 {
			return Range{}, bad()
		}
		return Range{Lo: start, Hi: end, HasLo: true, HasHi: true}, nil
	}
	start, end, ok := period(v)
	if !ok {
		return Range{}, bad()
	}
	return Range{Lo: start, Hi: end, HasLo: true, HasHi: true}, nil
}

// period expands a yymmddhhmmss prefix into the local-time interval it denotes.
func period(d string) (start, end time.Time, ok bool) {
	n := len(d)
	if n < 2 || n > 12 || n%2 != 0 {
		return start, end, false
	}
	parts := [6]int{0, 1, 1, 0, 0, 0}
	for i := 0; i < n/2; i++ {
		a, b := d[2*i], d[2*i+1]
		if a < '0' || a > '9' || b < '0' || b > '9' {
			return start, end, false
		}
		parts[i] = int(a-'0')*10 + int(b-'0')
	}
	start = time.Date(2000+parts[0], time.Month(parts[1]), parts[2], parts[3], parts[4], parts[5], 0, time.Local)
	if start.Year() != 2000+parts[0] || int(start.Month()) != parts[1] || start.Day() != parts[2] ||
		start.Hour() != parts[3] || start.Minute() != parts[4] || start.Second() != parts[5] {
		return start, end, false
	}
	switch n / 2 {
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

// Quote wraps s as an FTS5 string literal so no FTS syntax is interpreted.
func Quote(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }
