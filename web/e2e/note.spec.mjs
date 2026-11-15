import fs from "node:fs";
import path from "node:path";
import { test, expect, env } from "./fixtures.mjs";

const noteURL = (id = "e2e-note.md") => `${env().origin}/n/${id}?t=${env().token}`;

test.describe("note page", () => {
  test("wikilinks resolve to /n/ or /d/ hrefs and navigate; unresolved is broken", async ({ page }) => {
    await page.goto(noteURL());
    const ideas = page.getByRole("link", { name: "ideas", exact: true });
    await expect(ideas).toHaveAttribute("href", /^\/(n|d)\//);
    await expect(page.getByRole("link", { name: "design", exact: true })).toHaveAttribute("href", /^\/(n|d)\//);

    const broken = page.locator(".broken", { hasText: "no-such-note-xyz" });
    await expect(broken).toHaveCount(1);
    await expect(broken).toHaveClass(/broken/);
    expect(await broken.getAttribute("href")).toBeNull();

    await ideas.click();
    await expect(page).toHaveURL(/\/n\/ideas\.md/);
    await expect(page.locator("h1")).toBeVisible();
  });

  test("embedded drawing renders a canvas; image loads from /f/", async ({ page }) => {
    await page.goto(noteURL());
    await expect(page.locator(".gote-drawing canvas").first()).toBeVisible();
    const img = page.locator('img[src^="/f/"]');
    await expect(img).toHaveCount(1);
    await expect.poll(() => img.evaluate((i) => i.complete && i.naturalWidth > 0)).toBe(true);
  });

  test("editing the note on disk reloads the page via SSE", async ({ page }) => {
    await page.goto(noteURL());
    await expect(page.getByText("marker-initial")).toBeVisible();
    const f = path.join(env().notes, "e2e-note.md");
    fs.writeFileSync(f, fs.readFileSync(f, "utf8").replace("marker-initial", "marker-updated"));
    await expect(page.getByText("marker-updated")).toBeVisible({ timeout: 4_000 });
  });
});
