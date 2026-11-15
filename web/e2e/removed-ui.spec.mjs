import { test, expect, env } from "./fixtures.mjs";

const editorURL = () => `${env().origin}/d/diagrams/261101120000_diagram_architecture.excalidraw?t=${env().token}`;
const BANNED = /live collaboration|excalidraw\+|excalidraw plus|discord|twitter|github|follow us|sign up|shareable link|export to link|wireframe to code|text to diagram|magic frame|\bAI\b/i;

async function open(page) {
  await page.goto(editorURL());
  await expect(page.locator("canvas.interactive")).toBeVisible();
  await page.locator("canvas.interactive").click({ position: { x: 5, y: 5 } });
}

test.describe("removed UI", () => {
  test("main menu has no collab / Excalidraw+ / socials", async ({ page }) => {
    await open(page);
    await page.getByTestId("main-menu-trigger").click();
    const menu = page.locator(".dropdown-menu");
    await expect(menu).toBeVisible();
    const text = await menu.innerText();
    expect(text).not.toMatch(BANNED);
    expect(text).toMatch(/Export/i);
    await expect(menu.locator("a[href^='http']")).toHaveCount(0);
  });

  test("export dialog has no shareable link option", async ({ page }) => {
    await open(page);
    await page.keyboard.press("ControlOrMeta+Shift+e");
    const dlg = page.locator(".Modal, .Dialog").first();
    await expect(dlg).toBeVisible();
    const text = await dlg.innerText();
    expect(text).toMatch(/export/i);
    expect(text).not.toMatch(/link|share|excalidraw\+/i);
  });

  test("command palette is not offered: no menu item, shortcut opens nothing", async ({ page }) => {
    await open(page);
    await page.getByTestId("main-menu-trigger").click();
    await expect(page.locator(".main-menu .dropdown-menu-container, .dropdown-menu").first()).toBeVisible();
    await expect(page.getByTestId("command-palette-button")).toHaveCount(0);
    await page.keyboard.press("Escape");
    await page.keyboard.press("ControlOrMeta+/");
    await expect(page.locator(".command-palette-dialog")).toHaveCount(0);
  });

  test("toolbar and library sidebar: no AI tools, no Browse libraries", async ({ page }) => {
    await open(page);
    expect(await page.locator(".App-toolbar").innerText()).not.toMatch(BANNED);
    await expect(page.getByText(/wireframe to code|text to diagram/i)).toHaveCount(0);
    await page.getByRole("checkbox", { name: "Library" }).check({ force: true });
    const side = page.locator(".layer-ui__library, .sidebar").first();
    await expect(side).toBeVisible();
    // The link is present in the DOM but hidden with CSS (never rendered, so it cannot be clicked).
    await expect(page.getByText(/browse libraries/i)).not.toBeVisible();
  });
});
