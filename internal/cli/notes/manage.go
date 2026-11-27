package notes

import (
	"fmt"
	"os"
	"strings"

	"github.com/banorton/gote/internal/cli"
	"github.com/banorton/gote/internal/config"
	opsnotes "github.com/banorton/gote/internal/ops/notes"
)

func pinID(c *cli.Ctx, id string, pin bool) error {
	op, verb := "pins.add", "pinned"
	if !pin {
		op, verb = "pins.remove", "unpinned"
	}
	if err := call(c, op, opsnotes.IDReq{ID: id}, nil); err != nil {
		return err
	}
	return say(c, map[string]string{"id": id, "status": verb}, "%s %s", verb, id)
}

func deleteID(c *cli.Ctx, id string) error {
	var r opsnotes.DeleteResp
	if err := call(c, "notes.delete", opsnotes.IDReq{ID: id}, &r); err != nil {
		return err
	}
	return say(c, r, "moved %s to trash", id)
}

func renameID(c *cli.Ctx, id, newName string) error {
	var r opsnotes.RenameResp
	if err := call(c, "notes.rename", opsnotes.RenameReq{ID: id, New: newName}, &r); err != nil {
		return err
	}
	return say(c, r, "renamed %s to %s (%d links rewritten)", id, r.ID, len(r.Rewritten))
}

func withNote(usage string, fn func(c *cli.Ctx, r opsnotes.Ref) error) func(*cli.Ctx, []string) error {
	return func(c *cli.Ctx, args []string) error {
		name, err := need(args, usage)
		if err != nil {
			return err
		}
		r, err := resolve(c, name)
		if err != nil {
			return err
		}
		return fn(c, r)
	}
}

func pinnedCmd(c *cli.Ctx, args []string) error {
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		return pinCmd(c, args)
	}
	return listAndPick(c, listPinned, args, "open")
}

var pinCmd = withNote("gote pin <note>", func(c *cli.Ctx, r opsnotes.Ref) error { return pinID(c, r.ID, true) })
var unpinCmd = withNote("gote unpin <note>", func(c *cli.Ctx, r opsnotes.Ref) error { return pinID(c, r.ID, false) })
var deleteCmd = withNote("gote delete <note>", func(c *cli.Ctx, r opsnotes.Ref) error { return deleteID(c, r.ID) })

func renameCmd(c *cli.Ctx, args []string) error {
	fs := c.FlagSet("rename")
	newName := fs.String("n", "", "new name")
	pos, err := parseInterleaved(fs, args)
	if err != nil {
		return err
	}
	var ref string
	switch {
	case *newName != "":
		ref = joined(pos)
	case len(pos) == 2:
		ref, *newName = pos[0], pos[1]
	default:
		return cli.Usagef("usage: gote rename <note> -n <new>   |   gote rename <note> <new>")
	}
	if ref == "" {
		return cli.Usagef("usage: gote rename <note> -n <new>")
	}
	r, err := resolve(c, ref)
	if err != nil {
		return err
	}
	return renameID(c, r.ID, *newName)
}

func duplicateCmd(c *cli.Ctx, args []string) error {
	if len(args) < 2 {
		return cli.Usagef("usage: gote duplicate <note> <new>")
	}
	r, err := resolve(c, args[0])
	if err != nil {
		return err
	}
	var out opsnotes.CreateResp
	if err := call(c, "notes.duplicate", opsnotes.DupReq{ID: r.ID, New: joined(args[1:])}, &out); err != nil {
		return err
	}
	return say(c, out, "duplicated %s to %s", r.ID, out.ID)
}

var infoCmd = withNote("gote info <note>", func(c *cli.Ctx, r opsnotes.Ref) error {
	var i opsnotes.InfoResp
	if err := call(c, "notes.info", opsnotes.IDReq{ID: r.ID}, &i); err != nil {
		return err
	}
	if c.JSON {
		return c.PrintJSON(i)
	}
	fmt.Fprintf(c.Out, "id:        %s\ntitle:     %s\npath:      %s\ncreated:   %s\nmodified:  %s\nsize:      %d\ntags:      %s\naliases:   %s\nlinks:     %d out, %d in\npinned:    %v\n",
		i.ID, i.Title, i.Path, i.Created, i.Modified, i.Size, strings.Join(i.Tags, ", "), strings.Join(i.Aliases, ", "), i.Links, i.Backlinks, i.Pinned)
	if i.ParseError != "" {
		fmt.Fprintf(c.Out, "warning:   %s\n", i.ParseError)
	}
	return nil
})

func archiveEdit(op, verb string) func(c *cli.Ctx, args []string) error {
	return func(c *cli.Ctx, args []string) error {
		if len(args) == 0 {
			return cli.Usagef("usage: gote %s <note...>", strings.TrimPrefix(op, "notes."))
		}
		ids := make([]string, 0, len(args))
		for _, ref := range args {
			r, err := resolve(c, ref)
			if err != nil {
				return err
			}
			ids = append(ids, r.ID)
		}
		changed := []string{}
		if err := call(c, op, opsnotes.IDsReq{IDs: ids}, &changed); err != nil {
			return err
		}
		return say(c, changed, "%s %d notes", verb, len(changed))
	}
}

func tagCmd(c *cli.Ctx, args []string) error {
	if len(args) == 0 {
		var tl []opsnotes.TagCount
		if err := call(c, "tags.list", nil, &tl); err != nil {
			return err
		}
		if tl == nil {
			tl = []opsnotes.TagCount{}
		}
		if c.JSON {
			return c.PrintJSON(tl)
		}
		for _, t := range tl {
			fmt.Fprintf(c.Out, "%4d  %s\n", t.Count, t.Tag)
		}
		return nil
	}
	switch args[0] {
	case "add":
		return tagEdit(c, "tags.add", "added", args[1:])
	case "rm", "remove":
		return tagEdit(c, "tags.remove", "removed", args[1:])
	}
	return listAndPick(c, listTagged, args, "open")
}

func tagEdit(c *cli.Ctx, op, verb string, args []string) error {
	fs := c.FlagSet("tag")
	query := fs.String("query", "", "select target notes with a search query")
	pos, err := parseInterleaved(fs, args)
	if err != nil {
		return err
	}
	usage := cli.Usagef("usage: gote tag add|rm <tag> <note...>   |   gote tag add|rm <tag> --query <q>")
	if len(pos) == 0 {
		return usage
	}
	tag, refs := pos[0], pos[1:]
	var ids []string
	switch {
	case *query != "" && len(refs) == 0:
		hits, err := searchQuery(c, *query)
		if err != nil {
			return err
		}
		for _, h := range hits {
			ids = append(ids, h.ID)
		}
	case *query == "" && len(refs) > 0:
		for _, ref := range refs {
			r, err := resolve(c, ref)
			if err != nil {
				return err
			}
			ids = append(ids, r.ID)
		}
	default:
		return usage
	}
	changed := []string{}
	if len(ids) > 0 {
		if err := call(c, op, opsnotes.TagsEditReq{Tag: tag, IDs: ids}, &changed); err != nil {
			return err
		}
	}
	return say(c, changed, "%s tag %q on %d notes", verb, tag, len(changed))
}

func trashCmd(c *cli.Ctx, args []string) error {
	if len(args) > 0 {
		if args[0] != "empty" || len(args) > 1 {
			return cli.Usagef("usage: gote trash [empty]")
		}
		var r opsnotes.TrashEmptyResp
		if err := call(c, "notes.trash_empty", nil, &r); err != nil {
			return err
		}
		return say(c, r, "removed %d items from trash", r.Removed)
	}
	var es []opsnotes.TrashEntry
	if err := call(c, "notes.trash", nil, &es); err != nil {
		return err
	}
	if c.JSON {
		return c.PrintJSON(es)
	}
	for _, e := range es {
		fmt.Fprintf(c.Out, "%s  %s\n", e.Deleted, e.OriginalID)
	}
	return nil
}

func recoverCmd(c *cli.Ctx, args []string) error {
	name, err := need(args, "gote recover <name>")
	if err != nil {
		return err
	}
	var r opsnotes.RecoverResp
	err = call(c, "notes.recover", opsnotes.RecoverReq{OriginalID: name}, &r)
	if isCode(err, "not_found") && !strings.HasSuffix(name, ".md") {
		err = call(c, "notes.recover", opsnotes.RecoverReq{OriginalID: name + ".md"}, &r)
	}
	if err != nil {
		return err
	}
	return say(c, r, "recovered %s", r.ID)
}

func templateCmd(c *cli.Ctx, args []string) error {
	sub, rest := "list", []string(nil)
	if len(args) > 0 {
		sub, rest = args[0], args[1:]
	}
	switch sub {
	case "list":
		var l []string
		if err := call(c, "templates.list", nil, &l); err != nil {
			return err
		}
		if l == nil {
			l = []string{}
		}
		if c.JSON {
			return c.PrintJSON(l)
		}
		for _, n := range l {
			fmt.Fprintln(c.Out, n)
		}
		return nil
	case "rename":
		if len(rest) != 2 {
			return cli.Usagef("usage: gote template rename <name> <new>")
		}
		var r opsnotes.TemplateResp
		if err := call(c, "templates.rename", opsnotes.TemplateRenameReq{Name: rest[0], New: rest[1]}, &r); err != nil {
			return err
		}
		return say(c, r, "renamed template %s to %s", rest[0], r.Name)
	case "delete":
		if len(rest) != 1 {
			return cli.Usagef("usage: gote template delete <name>")
		}
		if err := call(c, "templates.delete", opsnotes.TemplateReq{Name: rest[0]}, nil); err != nil {
			return err
		}
		return say(c, map[string]string{"deleted": rest[0]}, "deleted template %s", rest[0])
	case "open":
		sub = joined(rest)
	}
	if sub == "" {
		return cli.Usagef("usage: gote template [list|open <name>|rename <name> <new>|delete <name>]")
	}
	var r opsnotes.TemplateResp
	if err := call(c, "templates.ensure", opsnotes.TemplateReq{Name: sub}, &r); err != nil {
		return err
	}
	return runEditor(c, r.Path)
}

func configCmd(c *cli.Ctx, args []string) error {
	if len(args) == 1 && args[0] == "edit" {
		return configEditCmd(c, nil)
	}
	if len(args) > 0 {
		return cli.Usagef("usage: gote config [edit]")
	}
	p := config.Path()
	data, err := os.ReadFile(p)
	if err != nil {
		return fmt.Errorf("read config %s: %w", p, err)
	}
	if c.JSON {
		return c.PrintJSON(map[string]string{"path": p, "text": string(data)})
	}
	fmt.Fprintf(c.Out, "# %s\n%s", p, data)
	return nil
}

func configEditCmd(c *cli.Ctx, args []string) error { return runEditor(c, config.Path()) }

func exportCmd(c *cli.Ctx, args []string) error {
	p := "gote-export.tar.gz"
	if len(args) > 1 {
		return cli.Usagef("usage: gote export [path]")
	}
	if len(args) == 1 {
		p = args[0]
	}
	p, err := absPath(p)
	if err != nil {
		return err
	}
	var r opsnotes.ExportResp
	if err := call(c, "export", opsnotes.ExportReq{Path: p}, &r); err != nil {
		return err
	}
	return say(c, r, "exported %d files to %s", r.Files, r.Path)
}

func importCmd(c *cli.Ctx, args []string) error {
	fs := c.FlagSet("import")
	force := fs.Bool("force", false, "import into a non-empty notes dir")
	pos, err := parseInterleaved(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return cli.Usagef("usage: gote import <path> [--force]")
	}
	p, err := absPath(pos[0])
	if err != nil {
		return err
	}
	var r opsnotes.ImportResp
	if err := call(c, "import", opsnotes.ImportReq{Path: p, Force: *force}, &r); err != nil {
		return err
	}
	return say(c, r, "imported %d files", r.Files)
}

func init() {
	cli.Register(cli.Command{Name: "pinned", Aliases: []string{"p"}, Summary: "menu of pinned notes (with args: pin a note)",
		Usage: "gote pinned [--all]   |   gote p <note>", Run: pinnedCmd})
	cli.Register(cli.Command{Name: "pin", Summary: "pin a note", Usage: "gote pin <note>", Run: pinCmd})
	cli.Register(cli.Command{Name: "unpin", Aliases: []string{"u", "up"}, Summary: "unpin a note", Usage: "gote unpin <note>", Run: unpinCmd})
	cli.Register(cli.Command{Name: "archive", Summary: "archive notes (adds the archive tag; hidden from listings and search)",
		Usage: "gote archive <note...>", Run: archiveEdit("notes.archive", "archived")})
	cli.Register(cli.Command{Name: "unarchive", Summary: "unarchive notes (removes the archive tag)",
		Usage: "gote unarchive <note...>", Run: archiveEdit("notes.unarchive", "unarchived")})
	cli.Register(cli.Command{Name: "tag", Aliases: []string{"t"}, Summary: "list tags, find notes by tag, add or remove tags",
		Usage: "gote tag [--all] [<tag>...]   |   gote tag add|rm <tag> <note...|--query q>", Run: tagCmd})
	cli.Register(cli.Command{Name: "delete", Aliases: []string{"d", "del"}, Summary: "move a note to the trash", Usage: "gote delete <note>", Run: deleteCmd})
	cli.Register(cli.Command{Name: "trash", Summary: "list the trash, or `trash empty`", Usage: "gote trash [empty]", Run: trashCmd})
	cli.Register(cli.Command{Name: "recover", Summary: "restore a note from the trash", Usage: "gote recover <name>", Run: recoverCmd})
	cli.Register(cli.Command{Name: "rename", Aliases: []string{"mv", "rn"}, Summary: "rename a note, rewriting inbound links",
		Usage: "gote rename <note> -n <new>", Run: renameCmd})
	cli.Register(cli.Command{Name: "duplicate", Aliases: []string{"dup", "cp"}, Summary: "copy a note", Usage: "gote duplicate <note> <new>", Run: duplicateCmd})
	cli.Register(cli.Command{Name: "info", Aliases: []string{"i"}, Summary: "show note details", Usage: "gote info <note>", Run: infoCmd})
	cli.Register(cli.Command{Name: "template", Aliases: []string{"tmpl"}, Summary: "list, open, rename or delete templates",
		Usage: "gote template [list|open <name>|rename <name> <new>|delete <name>]", Run: templateCmd})
	cli.Register(cli.Command{Name: "config", Aliases: []string{"c"}, Summary: "show the config file", Usage: "gote config", NoConfig: true, Run: configCmd})
	cli.Register(cli.Command{Name: "config-edit", Aliases: []string{"ce"}, Summary: "edit the config file", Usage: "gote config edit", NoConfig: true, Run: configEditCmd})
	cli.Register(cli.Command{Name: "export", Aliases: []string{"exp"}, Summary: "export notes, templates and pins as tar.gz", Usage: "gote export [path]", Run: exportCmd})
	cli.Register(cli.Command{Name: "import", Aliases: []string{"imp"}, Summary: "import an export archive", Usage: "gote import <path> [--force]", Run: importCmd})
}
