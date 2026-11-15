export const dark = () => window.matchMedia("(prefers-color-scheme: dark)").matches;

export async function loadDrawing(id) {
  const r = await fetch("/api/drawing/" + id.split("/").map(encodeURIComponent).join("/"), { credentials: "same-origin" });
  if (!r.ok) throw new Error("load " + id + ": " + r.status + " " + (await r.text()));
  const { scene, mtime_ns } = await r.json();
  return { scene, mtime: mtime_ns };
}

export function apiPath(id) {
  return "/api/drawing/" + id.split("/").map(encodeURIComponent).join("/");
}
