package fuzzy

import (
	"sort"
	"strings"
	"unicode"

	"github.com/junegunn/fzf/src/algo"
	"github.com/junegunn/fzf/src/util"
)

func init() { algo.Init("default") }

type Result struct {
	Index     int
	Score     int
	Positions []int
}

// Match filters items by a whitespace-separated pattern (every term must match,
// fzf smart-case) and returns them best score first; ties keep input order.
// Positions are ascending rune indexes into the item.
func Match(pattern string, items []string) []Result {
	terms := strings.Fields(pattern)
	if len(terms) == 0 {
		out := make([]Result, len(items))
		for i := range items {
			out[i] = Result{Index: i}
		}
		return out
	}
	type term struct {
		runes []rune
		cs    bool
	}
	ts := make([]term, len(terms))
	for i, s := range terms {
		cs := strings.IndexFunc(s, unicode.IsUpper) >= 0
		if !cs {
			s = strings.ToLower(s)
		}
		ts[i] = term{[]rune(s), cs}
	}
	slab := util.MakeSlab(100*1024, 2048)
	var out []Result
	for idx, item := range items {
		chars := util.ToChars([]byte(item))
		total := 0
		var pos []int
		ok := true
		for _, t := range ts {
			res, p := algo.FuzzyMatchV2(t.cs, false, true, &chars, t.runes, true, slab)
			if res.Start < 0 {
				ok = false
				break
			}
			total += res.Score
			if p != nil {
				pos = append(pos, *p...)
			}
		}
		if !ok {
			continue
		}
		sort.Ints(pos)
		pos = dedupe(pos)
		out = append(out, Result{Index: idx, Score: total, Positions: pos})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out
}

func dedupe(s []int) []int {
	if len(s) < 2 {
		return s
	}
	out := s[:1]
	for _, v := range s[1:] {
		if v != out[len(out)-1] {
			out = append(out, v)
		}
	}
	return out
}
