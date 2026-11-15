package fuzzy

import (
	"reflect"
	"testing"
)

func TestMatchOrdersLikeFzf(t *testing.T) {
	rs := Match("fb", []string{"xfxxb", "foo/bar.md", "nomatch"})
	if len(rs) != 2 || rs[0].Index != 1 || rs[1].Index != 0 {
		t.Fatalf("%+v", rs)
	}
	if rs[0].Score <= rs[1].Score {
		t.Fatalf("%+v", rs)
	}
}

func TestMatchPositions(t *testing.T) {
	rs := Match("fb", []string{"foo/bar.md"})
	if len(rs) != 1 || !reflect.DeepEqual(rs[0].Positions, []int{0, 4}) {
		t.Fatalf("%+v", rs)
	}
}

func TestMatchEmptyPatternKeepsOrder(t *testing.T) {
	rs := Match("", []string{"b", "a", "c"})
	if len(rs) != 3 || rs[0].Index != 0 || rs[1].Index != 1 || rs[2].Index != 2 {
		t.Fatalf("%+v", rs)
	}
	if len(Match("   ", []string{"a"})) != 1 {
		t.Fatal("whitespace-only pattern is empty")
	}
}

func TestMatchSmartCase(t *testing.T) {
	items := []string{"foo", "Foo"}
	if rs := Match("foo", items); len(rs) != 2 {
		t.Fatalf("lowercase pattern is case-insensitive: %+v", rs)
	}
	if rs := Match("Foo", items); len(rs) != 1 || rs[0].Index != 1 {
		t.Fatalf("uppercase pattern is case-sensitive: %+v", rs)
	}
}

func TestMatchMultipleTermsAnd(t *testing.T) {
	items := []string{"alpha  notes/one.md", "beta  notes/two.md", "alpha  other.md"}
	rs := Match("alp notes", items)
	if len(rs) != 1 || rs[0].Index != 0 {
		t.Fatalf("%+v", rs)
	}
	for i := 1; i < len(rs[0].Positions); i++ {
		if rs[0].Positions[i] <= rs[0].Positions[i-1] {
			t.Fatalf("positions not sorted/unique: %v", rs[0].Positions)
		}
	}
}

func TestMatchTieKeepsInputOrder(t *testing.T) {
	rs := Match("a", []string{"a", "a", "a"})
	if rs[0].Index != 0 || rs[1].Index != 1 || rs[2].Index != 2 {
		t.Fatalf("%+v", rs)
	}
}

func TestMatchUnicode(t *testing.T) {
	rs := Match("cafe", []string{"café notes", "cafe"})
	if len(rs) == 0 {
		t.Fatal("no match")
	}
	rs = Match("é", []string{"café"})
	if len(rs) != 1 || !reflect.DeepEqual(rs[0].Positions, []int{3}) {
		t.Fatalf("positions are rune indexes: %+v", rs)
	}
}
