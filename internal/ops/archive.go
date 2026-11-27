package ops

import "strings"

// ArchiveTag marks a note archived; there is no other archive state.
const ArchiveTag = "archive"

// IsArchiveTag reports whether tag is the archive tag (case-insensitive).
func IsArchiveTag(tag string) bool { return strings.EqualFold(strings.TrimSpace(tag), ArchiveTag) }

// NotArchived is the one SQL condition every archive-hiding listing ANDs into
// its WHERE clause; alias is the notes table alias (or name) in scope.
func NotArchived(alias string) string {
	return "NOT EXISTS (SELECT 1 FROM tags _arch WHERE _arch.note_id = " + alias + ".id AND lower(_arch.tag) = '" + ArchiveTag + "')"
}

// Visible returns the WHERE fragment for a listing: NotArchived unless the
// caller asked for everything.
func Visible(alias string, all bool) string {
	if all {
		return "1=1"
	}
	return NotArchived(alias)
}
