package notes

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/term"

	"github.com/banorton/gote/internal/cli"
	"github.com/banorton/gote/internal/ops"
	opsnotes "github.com/banorton/gote/internal/ops/notes"
)

var stdin io.Reader = os.Stdin

func call(c *cli.Ctx, op string, req, resp any) error {
	return c.Client.Call(context.Background(), op, req, resp)
}

func isCode(err error, code string) bool {
	var oe *ops.Error
	return errors.As(err, &oe) && oe.Code == code
}

func resolve(c *cli.Ctx, ref string) (opsnotes.Ref, error) {
	var r opsnotes.Ref
	return r, call(c, "notes.resolve", opsnotes.ResolveReq{Ref: ref}, &r)
}

// parseInterleaved parses flags anywhere among the arguments and returns the
// positionals.
func parseInterleaved(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := cli.ParseFlags(fs, args); err != nil {
			return nil, err
		}
		rest := fs.Args()
		if len(rest) == 0 {
			return pos, nil
		}
		pos = append(pos, rest[0])
		args = rest[1:]
	}
}

func joined(args []string) string { return strings.TrimSpace(strings.Join(args, " ")) }

func need(args []string, usage string) (string, error) {
	n := joined(args)
	if n == "" {
		return "", cli.Usagef("usage: %s", usage)
	}
	return n, nil
}

// splitCommand splits an editor command with shell word rules (quotes and
// backslash escapes) so values like `code -w` work.
func splitCommand(s string) ([]string, error) {
	var words []string
	var cur strings.Builder
	inWord := false
	var quote rune
	rs := []rune(s)
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		switch {
		case quote == '\'':
			if r == '\'' {
				quote = 0
			} else {
				cur.WriteRune(r)
			}
		case quote == '"':
			switch {
			case r == '"':
				quote = 0
			case r == '\\' && i+1 < len(rs) && strings.ContainsRune("\"\\$`", rs[i+1]):
				i++
				cur.WriteRune(rs[i])
			default:
				cur.WriteRune(r)
			}
		case r == '\'' || r == '"':
			quote, inWord = r, true
		case r == '\\' && i+1 < len(rs):
			i++
			cur.WriteRune(rs[i])
			inWord = true
		case r == ' ' || r == '\t':
			if inWord {
				words = append(words, cur.String())
				cur.Reset()
				inWord = false
			}
		default:
			cur.WriteRune(r)
			inWord = true
		}
	}
	if quote != 0 {
		return nil, fmt.Errorf("unterminated quote in editor command %q", s)
	}
	if inWord {
		words = append(words, cur.String())
	}
	if len(words) == 0 {
		return nil, fmt.Errorf("empty editor command")
	}
	return words, nil
}

func editorCmd(c *cli.Ctx) string {
	if c.Cfg != nil && c.Cfg.Editor != "" {
		return c.Cfg.Editor
	}
	if e := os.Getenv("EDITOR"); e != "" {
		return e
	}
	return "vi"
}

func runEditor(c *cli.Ctx, file string) error {
	words, err := splitCommand(editorCmd(c))
	if err != nil {
		return err
	}
	cmd := exec.Command(words[0], append(words[1:], file)...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, c.Out, c.Err
	if c.JSON {
		cmd.Stdout = c.Err
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor %q: %w", words[0], err)
	}
	return nil
}

// editNote opens the note in the editor, then records the visit and
// re-indexes whatever the editor changed.
func editNote(c *cli.Ctx, id, path string) error {
	if err := runEditor(c, path); err != nil {
		return err
	}
	if err := call(c, "index.reconcile", nil, nil); err != nil {
		return err
	}
	return call(c, "notes.visit", opsnotes.IDReq{ID: id}, nil)
}

var isTTY = func(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && term.IsTerminal(int(f.Fd()))
}

// headless reports whether the editor must not be launched: --json or a
// non-terminal stdout (agents, pipes).
func headless(c *cli.Ctx) bool { return c.JSON || !isTTY(c.Out) }

func prompt(c *cli.Ctx, msg, def string) (string, error) {
	if def != "" {
		fmt.Fprintf(c.Err, "%s [%s]: ", msg, def)
	} else {
		fmt.Fprintf(c.Err, "%s: ", msg)
	}
	line, err := bufio.NewReader(stdin).ReadString('\n')
	if err != nil && line == "" {
		return "", fmt.Errorf("read answer: %w", err)
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return def, nil
	}
	return line, nil
}

func absPath(p string) (string, error) {
	if filepath.IsAbs(p) {
		return p, nil
	}
	return filepath.Abs(p)
}

func say(c *cli.Ctx, v any, format string, args ...any) error {
	if c.JSON {
		return c.PrintJSON(v)
	}
	fmt.Fprintf(c.Out, format+"\n", args...)
	return nil
}
