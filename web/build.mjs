import { build } from "esbuild";
import { cpSync, rmSync, mkdirSync } from "node:fs";
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
