import { execFileSync } from "node:child_process";
import { test as base, expect, request as pwRequest } from "@playwright/test";

export { expect };

export const env = () => {
  const port = process.env.E2E_PORT;
  return {
    port,
    token: process.env.E2E_TOKEN,
    origin: `http://127.0.0.1:${port}`,
    notes: process.env.E2E_NOTES,
  };
};

// Console errors that are known benign. Empty on purpose; add entries with a justification.
const BENIGN_CONSOLE = [
  // Excalidraw registers a window "unload" handler; recent Chromium disallows unload by default and
  // logs this on page teardown/reload. The editor saves via autosave and Ctrl/Cmd-S, not on unload.
  /Permissions policy violation: unload is not allowed/i,
];

export const test = base.extend({
  // Every test records all requests and console output and asserts they are local and clean.
  guard: [
    async ({ context, page }, use) => {
      const { origin } = env();
      const requests = [];
      const bad = [];
      const allowed = [];
      context.on("request", (r) => requests.push(r.url()));
      page.on("console", (m) => {
        const text = m.text();
        const isCsp = /content security policy|refused to (load|connect|execute|apply|frame)/i.test(text);
        if ((isCsp || m.type() === "error") && !BENIGN_CONSOLE.concat(allowed).some((re) => re.test(text))) {
          bad.push(`[${m.type()}] ${text}`);
        }
      });
      page.on("pageerror", (e) => bad.push(`[pageerror] ${e.message}`));
      await use({ requests, bad, allow: (re) => allowed.push(re) });
      const foreign = requests.filter((u) => !u.startsWith(origin + "/") && !u.startsWith("data:") && !u.startsWith("blob:"));
      expect(foreign, "requests outside 127.0.0.1:<port>").toEqual([]);
      expect(bad, "console errors / CSP violations").toEqual([]);
    },
    { auto: true },
  ],
  api: async ({}, use) => {
    const ctx = await pwRequest.newContext();
    await use(ctx);
    await ctx.dispose();
  },
});

// Open web-view streams as the server reports them (`gote server status --json`).
export const webClients = () => {
  const out = execFileSync(process.env.E2E_BIN, ["server", "status", "--json"], {
    env: { PATH: process.env.PATH, HOME: process.env.E2E_ROOT, XDG_CONFIG_HOME: process.env.E2E_CFG, XDG_STATE_HOME: process.env.E2E_STATE },
    encoding: "utf8",
  });
  return JSON.parse(out).web_clients;
};
