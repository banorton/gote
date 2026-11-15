package notes

import (
	"regexp"
	"strconv"

	"github.com/banorton/gote/internal/cli"
	opsnotes "github.com/banorton/gote/internal/ops/notes"
)

type openFlags struct {
	template  string
	d, dt, nt bool
}

func (o *openFlags) prefix() (string, error) {
	switch {
	case o.d && o.dt:
		return "", cli.Usagef("-d and -dt are mutually exclusive")
	case o.d:
		return "date", nil
	case o.dt:
		return "datetime", nil
	}
	return "", nil
}

func parseOpenFlags(c *cli.Ctx, name string, args []string) (openFlags, []string, error) {
	var o openFlags
	fs := c.FlagSet(name)
	fs.StringVar(&o.template, "t", "", "template name")
	fs.BoolVar(&o.d, "d", false, "prefix yymmdd_")
	fs.BoolVar(&o.dt, "dt", false, "prefix yymmddhhmmss_")
	fs.BoolVar(&o.nt, "nt", false, "no prefix (default)")
	pos, err := parseInterleaved(fs, args)
	if err != nil {
		return o, nil, err
	}
	if o.nt && (o.d || o.dt) {
		return o, nil, cli.Usagef("-nt cannot be combined with -d or -dt")
	}
	return o, pos, nil
}

type openResult struct {
	ID      string `json:"id"`
	Path    string `json:"path"`
	Created bool   `json:"created"`
}

func finishOpen(c *cli.Ctx, r openResult) error {
	if err := editNote(c, r.ID, r.Path); err != nil {
		return err
	}
	if c.JSON {
		return c.PrintJSON(r)
	}
	return nil
}

func create(c *cli.Ctx, name string, o openFlags) (openResult, error) {
	prefix, err := o.prefix()
	if err != nil {
		return openResult{}, err
	}
	var r opsnotes.CreateResp
	if err := call(c, "notes.create", opsnotes.CreateReq{Name: name, Template: o.template, Prefix: prefix}, &r); err != nil {
		return openResult{}, err
	}
	return openResult(r), nil
}

func openCmd(c *cli.Ctx, args []string) error {
	o, pos, err := parseOpenFlags(c, "open", args)
	if err != nil {
		return err
	}
	name, err := need(pos, "gote open [-t template] [-d|-dt|-nt] <note>")
	if err != nil {
		return err
	}
	if r, err := resolve(c, name); err == nil {
		return finishOpen(c, openResult{ID: r.ID, Path: r.Path})
	} else if name == "-" || !isCode(err, "not_found") {
		return err
	}
	r, err := create(c, name, o)
	if err != nil {
		return err
	}
	return finishOpen(c, r)
}

func newCmd(c *cli.Ctx, args []string) error {
	o, pos, err := parseOpenFlags(c, "new", args)
	if err != nil {
		return err
	}
	name, err := need(pos, "gote new [-t template] [-d|-dt|-nt] <name>")
	if err != nil {
		return err
	}
	r, err := create(c, name, o)
	if err != nil {
		return err
	}
	return finishOpen(c, r)
}

func quickCmd(c *cli.Ctx, args []string) error {
	if len(args) > 0 && args[0] == "save" {
		return quickSave(c, args[1:])
	}
	var r opsnotes.PathResp
	if err := call(c, "quick.open", nil, &r); err != nil {
		return err
	}
	if err := runEditor(c, r.Path); err != nil {
		return err
	}
	if c.JSON {
		return c.PrintJSON(r)
	}
	return nil
}

func quickSave(c *cli.Ctx, args []string) error {
	name, err := need(args, "gote quick save <name>")
	if err != nil {
		return err
	}
	var r opsnotes.CreateResp
	if err := call(c, "quick.save", opsnotes.QuickSaveReq{Name: name}, &r); err != nil {
		return err
	}
	return say(c, r, "saved quick note as %s", r.ID)
}

var offsetRe = regexp.MustCompile(`^[+-]?\d+$`)

func todayCmd(c *cli.Ctx, args []string) error {
	offset := 0
	switch {
	case len(args) == 1 && offsetRe.MatchString(args[0]):
		n, err := strconv.Atoi(args[0])
		if err != nil {
			return cli.Usagef("bad day offset %q", args[0])
		}
		offset = n
	case len(args) > 0:
		return cli.Usagef("usage: gote today [-N]")
	}
	var r opsnotes.CreateResp
	if err := call(c, "daily.open", opsnotes.DailyReq{Offset: offset}, &r); err != nil {
		return err
	}
	return finishOpen(c, openResult(r))
}

func init() {
	cli.Register(cli.Command{Name: "open", Summary: "open a note, creating it if missing",
		Usage: "gote open [-t template] [-d|-dt|-nt] <note>   (or: gote <note>)", Run: openCmd})
	cli.Register(cli.Command{Name: "new", Summary: "create a note and open it",
		Usage: "gote new [-t template] [-d|-dt|-nt] <name>", Run: newCmd})
	cli.Register(cli.Command{Name: "quick", Aliases: []string{"q"}, Summary: "open the quick note",
		Usage: "gote quick [save <name>]", Run: quickCmd})
	cli.Register(cli.Command{Name: "quick-save", Aliases: []string{"qs"}, Summary: "save the quick note as a note",
		Usage: "gote qs <name>", Run: quickSave})
	cli.Register(cli.Command{Name: "today", Summary: "open the daily note",
		Usage: "gote today [-N]", Run: todayCmd})
}
