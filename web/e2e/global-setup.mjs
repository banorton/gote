import { spawnSync } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import { fileURLToPath } from "node:url";

const here = path.dirname(fileURLToPath(import.meta.url));
const repo = path.resolve(here, "../..");

// 1x1 PNG
const PNG = Buffer.from(
  "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==",
  "base64",
);

const NOTE = `---
title: E2E Note
tags: [e2e]
---

# E2E Note

Links: [[ideas]] and [[design]] and [[no-such-note-xyz]].

![[261101120000_diagram_architecture]]

![[pic.png]]

marker-initial
`;

function gote(bin, env, args) {
  const r = spawnSync(bin, args, { env, encoding: "utf8" });
  if (r.status !== 0) throw new Error(`gote ${args.join(" ")} failed: ${r.stderr || r.stdout}`);
  return r.stdout;
}

export default async function globalSetup() {
  const bin = process.env.GOTE_BIN ? path.resolve(process.env.GOTE_BIN) : path.join(repo, "gote");
  if (!fs.existsSync(bin)) throw new Error(`gote binary not found at ${bin}; run make e2e`);

  // Short root: the unix socket path limit is ~104 bytes.
  const root = fs.mkdtempSync("/tmp/gote-e2e-");
  const notes = path.join(root, "n");
  const cfg = path.join(root, "c");
  const state = path.join(root, "s");
  fs.cpSync(path.join(repo, "skill/fixture"), notes, { recursive: true });
  fs.mkdirSync(path.join(cfg, "gote"), { recursive: true });
  fs.mkdirSync(state);
  fs.writeFileSync(path.join(cfg, "gote/config.yaml"), `notes_dir: ${notes}\nserver: true\n`);

  const arch = fs.readFileSync(path.join(notes, "diagrams/261101120000_diagram_architecture.excalidraw"));
  for (const name of ["e2e_edit", "e2e_conflict"]) {
    fs.writeFileSync(path.join(notes, `diagrams/261101130001_${name}.excalidraw`), arch);
  }
  fs.writeFileSync(path.join(notes, "pic.png"), PNG);
  fs.writeFileSync(path.join(notes, "e2e-note.md"), NOTE);

  const env = { PATH: process.env.PATH, HOME: root, XDG_CONFIG_HOME: cfg, XDG_STATE_HOME: state };
  // Any command spawns the background server; this also indexes the copy.
  gote(bin, env, ["search", "gote"]);
  const status = gote(bin, env, ["server", "status"]);
  const pid = Number(/pid (\d+)/.exec(status)?.[1]);
  if (!pid) throw new Error(`cannot parse server pid from: ${status}`);

  const web = JSON.parse(fs.readFileSync(path.join(state, "gote/web.json"), "utf8"));
  if (!web.port || !web.token) throw new Error("web.json lacks port/token");


  Object.assign(process.env, {
    E2E_ROOT: root,
    E2E_NOTES: notes,
    E2E_PORT: String(web.port),
    E2E_TOKEN: web.token,
  });

  return async () => {
    try {
      gote(bin, env, ["server", "stop"]);
      let alive = true;
      for (let i = 0; i < 50 && alive; i++) {
        try {
          process.kill(pid, 0);
          await new Promise((r) => setTimeout(r, 100));
        } catch {
          alive = false;
        }
      }
      if (alive) throw new Error(`gote serve pid ${pid} still running after server stop`);
      if (fs.existsSync(path.join(state, "gote/gote.sock"))) throw new Error("socket remains after server stop");
    } finally {
      fs.rmSync(root, { recursive: true, force: true });
    }
  };
}
