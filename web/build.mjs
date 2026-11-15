import { build } from "esbuild";
import { cpSync, rmSync, mkdirSync, readdirSync, readFileSync, writeFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";

const root = dirname(fileURLToPath(import.meta.url));
const out = join(root, "dist", "assets");

rmSync(join(root, "dist"), { recursive: true, force: true });
mkdirSync(out, { recursive: true });

await build({
  entryPoints: {
    editor: join(root, "src/editor.jsx"),
    viewer: join(root, "src/viewer.jsx"),
    reload: join(root, "src/reload.js"),
  },
  outdir: out,
  bundle: true,
  splitting: true,
  format: "esm",
  minify: true,
  target: "es2020",
  jsx: "automatic",
  legalComments: "none",
  conditions: ["production"],
  define: { "process.env.NODE_ENV": '"production"', "process.env.IS_PREACT": '"false"' },
  loader: { ".woff2": "file", ".svg": "file", ".png": "file" },
  logLevel: "info",
});

// Xiaolai (CJK, 12 MB) is left out; CJK text falls back to system fonts.
cpSync(join(root, "node_modules/@excalidraw/excalidraw/dist/prod/fonts"), join(out, "fonts"), {
  recursive: true,
  filter: (src) => !src.split("/").includes("Xiaolai"),
});

// Excalidraw falls back to https://esm.sh/ when a local asset lookup fails (Xiaolai is not
// shipped). Point the fallback at a same-origin dead path so no CDN is ever contacted.
let patched = 0;
for (const f of readdirSync(out)) {
  if (!f.endsWith(".js")) continue;
  const p = join(out, f);
  const src = readFileSync(p, "utf8");
  if (!src.includes("`https://esm.sh/")) continue;
  writeFileSync(p, src.replaceAll("`https://esm.sh/", "`${location.origin}/__no_cdn/"));
  patched++;
}
if (patched === 0) throw new Error("esm.sh fallback not found in bundle; update build.mjs patch");
