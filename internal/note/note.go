package note

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
	Created               string
	Tags, Aliases         []string
	Fields                map[string]any
	Semantic              bool
	Body                  string
	Headings              []Heading
	Links                 []Link
	Tasks                 []Task
	ParseErr              string
}
