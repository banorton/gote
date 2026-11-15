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

  test("command palette has no AI / wireframe / text-to-diagram / collab entries", async ({ page }) => {
    await open(page);
    const palette = page.locator(".command-palette-dialog");
    const input = palette.locator("input");
    const probe = async (open) => {
      await open();
      try {
        await expect(input).toBeVisible({ timeout: 1_500 });
        return true;
      } catch {
        return false;
      }
    };
    // The editor does not currently mount Excalidraw's <CommandPalette/>, so both entry points are
    // no-ops. If it is ever mounted, its contents must stay clean, so check them whenever it opens.
    const viaMenu = await probe(async () => {
      await page.getByTestId("main-menu-trigger").click();
      await page.getByTestId("command-palette-button").click();
    });
    if (viaMenu) {
      await page.keyboard.press("Escape");
    }
    const viaKey = await probe(() => page.keyboard.press("ControlOrMeta+/"));
    if (!viaMenu && !viaKey) return;
    if (!viaKey) await probe(async () => {
      await page.getByTestId("main-menu-trigger").click();
      await page.getByTestId("command-palette-button").click();
    });
    expect(await palette.innerText()).not.toMatch(BANNED);
    for (const q of ["AI", "wireframe", "diagram", "live", "collab", "excalidraw+", "share", "link", "mermaid"]) {
      await input.fill(q);
      const text = await palette.innerText();
      expect(text, `palette query "${q}"`).not.toMatch(/wireframe to code|text to diagram|live collaboration|excalidraw\+|shareable|export to link|\bAI\b/i);
    }
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
