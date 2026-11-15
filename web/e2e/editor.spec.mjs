import fs from "node:fs";
import path from "node:path";
import { test, expect, env } from "./fixtures.mjs";

const file = (name) => path.join(env().notes, "diagrams", `261101130001_${name}.excalidraw`);
const url = (name) => `${env().origin}/d/diagrams/261101130001_${name}.excalidraw?t=${env().token}`;
const scene = (name) => JSON.parse(fs.readFileSync(file(name), "utf8"));
const live = (s) => s.elements.filter((e) => !e.isDeleted);

async function openEditor(page, name) {
  await page.goto(url(name));
  const canvas = page.locator("canvas.interactive");
  await expect(canvas).toBeVisible();
  await expect(page.locator("#gote-status")).toHaveText("saved");
  return canvas;
}

async function drawRect(page, canvas, dx = 0) {
  const box = await canvas.boundingBox();
  await canvas.click({ position: { x: 5, y: 5 } }); // focus canvas
  await page.keyboard.press("r");
  const x = box.x + 400 + dx;
  const y = box.y + 300;
  await page.mouse.move(x, y);
  await page.mouse.down();
  await page.mouse.move(x + 120, y + 80, { steps: 5 });
  await page.mouse.up();
}

test.describe("editor", () => {
  test("draw rectangle + text, autosave, Ctrl/Cmd-S, persists on reload", async ({ page }) => {
    const canvas = await openEditor(page, "e2e_edit");
    const count = (type) => live(scene("e2e_edit")).filter((e) => e.type === type).length;
    const rects = count("rectangle");

    await drawRect(page, canvas);
    // Autosave: no key press, just the debounce.
    await expect.poll(() => count("rectangle"), { timeout: 5_000 }).toBe(rects + 1);
    await expect(page.locator("#gote-status")).toHaveText("saved");

    const box = await canvas.boundingBox();
    await page.keyboard.press("t");
    await page.mouse.click(box.x + 600, box.y + 500);
    await page.keyboard.type("hello-e2e");
    await page.keyboard.press("Escape");
    await page.keyboard.press("ControlOrMeta+s");
    await expect.poll(() => live(scene("e2e_edit")).some((e) => e.type === "text" && e.text === "hello-e2e"), {
      timeout: 5_000,
    }).toBe(true);
    await expect(page.locator("#gote-status")).toHaveText("saved");

    await page.reload();
    await expect(page.locator("#gote-status")).toHaveText("saved");
    const c2 = page.locator("canvas.interactive");
    await expect(c2).toBeVisible();
    // Clicking the new rectangle's left edge selects it only if it was reloaded.
    await c2.click({ position: { x: 400, y: 340 } });
    await expect(page.getByText("Stroke", { exact: true }).first()).toBeVisible();
  });

  test("conflict: disk change then browser edit shows conflict and keeps disk file", async ({ page, guard }) => {
    // The browser logs the expected 409 from the rejected save as a console error.
    guard.allow(/status of 409 \(Conflict\)/);
    const canvas = await openEditor(page, "e2e_conflict");
    const f = file("e2e_conflict");
    const s = scene("e2e_conflict");
    s.elements[0].x += 7;
    fs.writeFileSync(f, JSON.stringify(s));
    const future = new Date(Date.now() + 5_000);
    fs.utimesSync(f, future, future);
    const onDisk = fs.readFileSync(f, "utf8");

    await drawRect(page, canvas, -100);
    await expect(page.locator("#gote-status")).toHaveClass(/conflict/, { timeout: 8_000 });
    await expect(page.locator("#gote-status")).toContainText("conflict");
    expect(fs.readFileSync(f, "utf8")).toBe(onDisk);
  });
});
