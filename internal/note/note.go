package note

import (
	"strings"
	"time"

	"golang.org/x/text/unicode/norm"
)

const (
	KindNote    = "note"
	KindDiagram = "diagram"
)

const (
	LinkWiki    = "wiki"
	LinkEmbed   = "embed"
	LinkMD      = "md"
	LinkRelated = "related"
	LinkParent  = "parent"
)

type Heading struct {
	Level int
	Text  string
	Line  int
}

type Link struct {
	Target   string
	Fragment string
	Alias    string
	Kind     string
	Line     int
}

type Task struct {
	Line int
	Text string
	Done bool
}

type Note struct {
	ID, Kind, Title, Stem string
	Created               time.Time // zero when absent or unparseable
	Tags, Aliases         []string
	Fields                map[string]any
	Semantic              bool
	Body                  string
	Headings              []Heading
	Links                 []Link
	Tasks                 []Task
	ParseErr              string
}

// Fold is the comparison key for names and link targets: lowercase, NFC.
// macOS filesystems hand back NFD names while users type NFC.
func Fold(s string) string { return norm.NFC.String(strings.ToLower(s)) }
