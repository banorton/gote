package web

import "html/template"

const baseCSS = `
:root{--bg:#fff;--fg:#1d1d1f;--muted:#6b6b73;--link:#0b57d0;--broken:#c5221f;--chip:#eef0f4;--chip-fg:#33363d;--rule:#d9dbe0;--code:#f4f5f7}
@media (prefers-color-scheme: dark){:root{--bg:#16171a;--fg:#e6e6e9;--muted:#9a9aa3;--link:#8ab4f8;--broken:#f28b82;--chip:#2a2c31;--chip-fg:#d2d3d8;--rule:#34363c;--code:#202226}}
html{background:var(--bg)}
body{margin:0;color:var(--fg);font:16px/1.6 -apple-system,BlinkMacSystemFont,"Segoe UI",Roboto,sans-serif}
main{max-width:46rem;margin:0 auto;padding:2rem 1.25rem 4rem}
a{color:var(--link)}
a.broken,span.broken{color:var(--broken);text-decoration:line-through dotted}
.note-title{font-size:.85rem;color:var(--muted);margin-bottom:.25rem}
.tags{display:flex;flex-wrap:wrap;gap:.4rem;margin:.25rem 0 1.25rem}
.tag{background:var(--chip);color:var(--chip-fg);border-radius:999px;padding:.05rem .65rem;font-size:.8rem}
pre,code{background:var(--code);border-radius:4px;font-family:ui-monospace,Menlo,Consolas,monospace;font-size:.9em}
code{padding:.1em .3em}pre{padding:.75rem 1rem;overflow:auto}pre code{padding:0;background:none}
blockquote{margin:1rem 0;padding:.1rem 1rem;border-left:3px solid var(--rule);color:var(--muted)}
table{border-collapse:collapse}th,td{border:1px solid var(--rule);padding:.3rem .6rem}
img{max-width:100%}hr{border:0;border-top:1px solid var(--rule)}
.gote-drawing{position:relative;height:420px;margin:1rem 0;border:1px solid var(--rule);border-radius:6px;overflow:hidden}
.gote-drawing-canvas{position:absolute;inset:0}
.gote-drawing .App-bottom-bar{display:none}
.gote-edit{position:absolute;top:.5rem;right:.5rem;z-index:5;font-size:.8rem;background:var(--chip);color:var(--chip-fg);padding:.1rem .6rem;border-radius:999px;text-decoration:none}
`

var notePage = template.Must(template.New("note").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>{{.Title}}</title>
<link rel="icon" href="data:,">
<style>` + baseCSS + `</style>
{{if .Drawings}}<link rel="stylesheet" href="/assets/viewer.css">{{end}}
</head><body><main>
<div class="note-title">{{.ID}}</div>
{{if .Tags}}<div class="tags">{{range .Tags}}<span class="tag">{{.}}</span>{{end}}</div>{{end}}
<article>{{.Body}}</article>
</main>
{{if .Drawings}}<script type="module" src="/assets/viewer.js"></script>{{end}}
<script>new EventSource({{.Events}}).addEventListener("reload",function(){location.reload()})</script>
</body></html>
`))

type editorPage struct{ ID, Title string }

var editorTmpl = template.Must(template.New("editor").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>{{.Title}}</title>
<link rel="icon" href="data:,">
<link rel="stylesheet" href="/assets/editor.css">
<style>
html,body{margin:0;height:100%;overflow:hidden}
.gote-pill{position:fixed;left:.75rem;bottom:.75rem;z-index:100;font:12px/1 -apple-system,sans-serif;padding:.4rem .7rem;border-radius:999px;background:#eef0f4;color:#33363d;box-shadow:0 1px 3px rgba(0,0,0,.25)}
.gote-pill.saved{background:#d9f2e0;color:#14532d}
.gote-pill.saving,.gote-pill.dirty{background:#fdf2c9;color:#6b4e00}
.gote-pill.conflict,.gote-pill.error{background:#fbd5d3;color:#8c1d18;cursor:pointer}
.gote-error{padding:1rem;white-space:pre-wrap}
</style>
</head><body>
<div id="root" data-id="{{.ID}}"></div>
<script type="module" src="/assets/editor.js"></script>
</body></html>
`))
