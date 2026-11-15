import { test, expect, env } from "./fixtures.mjs";

const paths = () => ["/n/e2e-note.md", "/d/diagrams/261101120000_diagram_architecture.excalidraw", "/api/drawing/diagrams/261101120000_diagram_architecture.excalidraw", "/f/pic.png", "/events?id=e2e-note.md"];

test.describe("auth", () => {
  test("no token -> 403", async ({ api }) => {
    for (const p of paths()) {
      const r = await api.get(env().origin + p);
      expect(r.status(), p).toBe(403);
    }
  });

  test("wrong token -> 403", async ({ api }) => {
    const r = await api.get(`${env().origin}/n/e2e-note.md?t=${"0".repeat(64)}`);
    expect(r.status()).toBe(403);
  });

  test("token works and wrong Host -> 403", async ({ api }) => {
    const ok = await api.get(`${env().origin}/n/e2e-note.md?t=${env().token}`);
    expect(ok.status()).toBe(200);
    for (const host of ["evil.example", `evil.example:${env().port}`, "127.0.0.1:1"]) {
      const r = await api.get(`${env().origin}/n/e2e-note.md?t=${env().token}`, { headers: { Host: host } });
      expect(r.status(), host).toBe(403);
    }
  });
});
