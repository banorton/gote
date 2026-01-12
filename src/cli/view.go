package cli

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer/html"

	"gote/src/data"
)

// ViewCommand opens a markdown preview of a note in the browser
func ViewCommand(rawArgs []string) {
	args := ParseArgs(rawArgs)
	noteName := args.Joined()

	_, ui, ok := LoadConfigAndUI()
	if !ok {
		return
	}

	if noteName == "" {
		fmt.Println("Usage: gote view <note>")
		return
	}

	noteName, err := ResolveNoteName(noteName)
	if err != nil {
		ui.Error(err.Error())
		return
	}

	// Find the note
	index, err := data.LoadIndex()
	if err != nil {
		ui.Error("Error loading index: " + err.Error())
		return
	}

	meta, exists := index[noteName]
	if !exists {
		ui.Error("Note not found: " + noteName)
		return
	}

	ViewNoteInBrowser(meta.FilePath, noteName)
}

// ViewNoteInBrowser opens a note's markdown content as HTML in the browser
func ViewNoteInBrowser(filePath, title string) error {
	// Read the note content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("error reading note: %w", err)
	}

	// Convert markdown to HTML
	htmlContent, err := markdownToHTML(content)
	if err != nil {
		return fmt.Errorf("error converting markdown: %w", err)
	}

	// Create full HTML document
	fullHTML := wrapInHTMLTemplate(title, htmlContent)

	// Write to temp file
	tempFile := filepath.Join(os.TempDir(), "gote-view.html")
	if err := os.WriteFile(tempFile, []byte(fullHTML), 0644); err != nil {
		return fmt.Errorf("error writing view file: %w", err)
	}

	// Open in browser
	if err := openInBrowser(tempFile); err != nil {
		return fmt.Errorf("error opening browser: %w", err)
	}

	return nil
}

func markdownToHTML(source []byte) (string, error) {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM, // GitHub Flavored Markdown (tables, strikethrough, etc.)
		),
		goldmark.WithRendererOptions(
			html.WithUnsafe(), // Allow raw HTML in markdown
		),
	)

	var buf bytes.Buffer
	if err := md.Convert(source, &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func wrapInHTMLTemplate(title, content string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>%s</title>
    <style>
        :root {
            --bg: #1a1a2e;
            --fg: #eaeaea;
            --accent: #64ffda;
            --muted: #888;
            --code-bg: #0f0f1a;
            --border: #333;
        }

        @media (prefers-color-scheme: light) {
            :root {
                --bg: #ffffff;
                --fg: #1a1a1a;
                --accent: #0066cc;
                --muted: #666;
                --code-bg: #f5f5f5;
                --border: #ddd;
            }
        }

        * {
            box-sizing: border-box;
        }

        body {
            font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, Oxygen, Ubuntu, sans-serif;
            line-height: 1.6;
            max-width: 800px;
            margin: 0 auto;
            padding: 2rem;
            background: var(--bg);
            color: var(--fg);
        }

        h1, h2, h3, h4, h5, h6 {
            color: var(--accent);
            margin-top: 1.5em;
            margin-bottom: 0.5em;
            font-weight: 600;
        }

        h1 { font-size: 2rem; border-bottom: 2px solid var(--border); padding-bottom: 0.3em; }
        h2 { font-size: 1.5rem; border-bottom: 1px solid var(--border); padding-bottom: 0.2em; }
        h3 { font-size: 1.25rem; }

        a {
            color: var(--accent);
            text-decoration: none;
        }

        a:hover {
            text-decoration: underline;
        }

        code {
            font-family: 'SF Mono', Monaco, 'Cascadia Code', 'Roboto Mono', Consolas, monospace;
            background: var(--code-bg);
            padding: 0.2em 0.4em;
            border-radius: 4px;
            font-size: 0.9em;
        }

        pre {
            background: var(--code-bg);
            padding: 1rem;
            border-radius: 8px;
            overflow-x: auto;
            border: 1px solid var(--border);
        }

        pre code {
            background: none;
            padding: 0;
        }

        blockquote {
            border-left: 4px solid var(--accent);
            margin: 1em 0;
            padding: 0.5em 1em;
            background: var(--code-bg);
            border-radius: 0 8px 8px 0;
        }

        blockquote p {
            margin: 0;
        }

        ul, ol {
            padding-left: 1.5em;
        }

        li {
            margin: 0.25em 0;
        }

        table {
            border-collapse: collapse;
            width: 100%%;
            margin: 1em 0;
        }

        th, td {
            border: 1px solid var(--border);
            padding: 0.5em 1em;
            text-align: left;
        }

        th {
            background: var(--code-bg);
            font-weight: 600;
        }

        hr {
            border: none;
            border-top: 1px solid var(--border);
            margin: 2em 0;
        }

        img {
            max-width: 100%%;
            height: auto;
            border-radius: 8px;
        }

        .title {
            color: var(--muted);
            font-size: 0.9rem;
            margin-bottom: 2rem;
            padding-bottom: 1rem;
            border-bottom: 1px solid var(--border);
        }
    </style>
</head>
<body>
    <div class="title">%s</div>
    %s
</body>
</html>`, title, title, content)
}

func openInBrowser(path string) error {
	var cmd *exec.Cmd

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", path)
	case "linux":
		cmd = exec.Command("xdg-open", path)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", path)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return cmd.Start()
}
