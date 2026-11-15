import { test, expect, env } from "./fixtures.mjs";

// The auto "guard" fixture in fixtures.mjs applies the same request/console assertions to every test.
// This test exercises every page type explicitly so the check cannot pass vacuously.
test("every page loads only from 127.0.0.1:<port> with no CSP violations or console errors", async ({ page, guard }) => {
  const { origin, token } = env();
  await page.goto(`${origin}/n/e2e-note.md?t=${token}`);
  await expect(page.locator(".gote-drawing canvas").first()).toBeVisible();
  await page.goto(`${origin}/d/diagrams/261101120000_diagram_architecture.excalidraw?t=${token}`);
  await expect(page.locator("canvas.interactive")).toBeVisible();
  await page.evaluate(() => document.fonts.ready);
  expect(guard.requests.length).toBeGreaterThan(5);
  expect(guard.requests.every((u) => u.startsWith(origin + "/") || u.startsWith("data:") || u.startsWith("blob:"))).toBe(true);
  expect(guard.bad).toEqual([]);
});
