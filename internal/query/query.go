package query

import (
	"fmt"
	"strings"
	"time"

	"github.com/banorton/gote/internal/dates"
)

const (
	KindTag      = "tag"
	KindCreated  = "created"
	KindModified = "modified"
	KindField    = "field"
	KindNoteKind = "kind"
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

// UnixBounds returns the range as unix seconds for comparing against stored
// created/modified times; an unbounded side is nil.
func (r Range) UnixBounds() (lo, hi *int64) {
	if r.HasLo {
		v := r.Lo.Unix()
		lo = &v
	}
	if r.HasHi {
		v := r.Hi.Unix()
		hi = &v
	}
	return lo, hi
}

// MentionsArchive reports whether the query asks for archived notes with a
// positive tag:archive filter.
func (q Query) MentionsArchive() bool {
	for _, f := range q.Filters {
		if f.Kind == KindTag && !f.Neg && strings.EqualFold(f.Value, "archive") {
			return true
		}
	}
	return false
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
	case "kind":
		v := strings.ToLower(val)
		if v != "note" && v != "diagram" {
			return Filter{}, errf("invalid token %q: kind must be note or diagram", token)
		}
		return Filter{Kind: KindNoteKind, Value: v}, nil
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
		return errf("invalid token %q: date must be ISO (2026, 2026-03, 2026-03-14, 2026-03-14T10:30) or compact digits (2-12, even count), optionally >d, <d or d..d", token)
	}
	switch {
	case strings.HasPrefix(v, ">"):
		_, end, ok := dates.Period(v[1:])
		if !ok {
			return Range{}, bad()
		}
		return Range{Lo: end, HasLo: true}, nil
	case strings.HasPrefix(v, "<"):
		start, _, ok := dates.Period(v[1:])
		if !ok {
			return Range{}, bad()
		}
		return Range{Hi: start, HasHi: true}, nil
	}
	if a, b, found := strings.Cut(v, ".."); found {
		start, _, ok1 := dates.Period(a)
		_, end, ok2 := dates.Period(b)
		if !ok1 || !ok2 {
			return Range{}, bad()
		}
		return Range{Lo: start, Hi: end, HasLo: true, HasHi: true}, nil
	}
	start, end, ok := dates.Period(v)
	if !ok {
		return Range{}, bad()
	}
	return Range{Lo: start, Hi: end, HasLo: true, HasHi: true}, nil
}

// Quote wraps s as an FTS5 string literal so no FTS syntax is interpreted.
func Quote(s string) string { return `"` + strings.ReplaceAll(s, `"`, `""`) + `"` }
