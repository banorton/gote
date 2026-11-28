// Must evaluate before @excalidraw/excalidraw, which reads the path at import time.
import "./assetpath.js";
import "@excalidraw/excalidraw/index.css";
import React, { useCallback, useEffect, useRef, useState } from "react";
import { createRoot } from "react-dom/client";
import { Excalidraw, MainMenu, serializeAsJSON } from "@excalidraw/excalidraw";
import { apiPath, dark, loadDrawing, localOnlyProps } from "./common.js";

const AUTOSAVE_MS = 2000;

const signature = (elements, files) =>
  elements.map((e) => e.id + ":" + e.version).join(",") + "|" + Object.keys(files || {}).join(",");

function Editor({ id }) {
  const [initial, setInitial] = useState(null);
  const [status, setStatus] = useState("saved");
  const [error, setError] = useState("");
  const mtime = useRef("");
  const latest = useRef(null);
  const baseline = useRef(null);
  const timer = useRef(null);
  const saving = useRef(false);
  const conflict = useRef(false);

  useEffect(() => {
    loadDrawing(id)
      .then(({ scene, mtime: m }) => {
        mtime.current = String(m);
        setInitial({
          elements: scene.elements || [],
          appState: { ...(scene.appState || {}), collaborators: new Map() },
          files: scene.files || {},
          scrollToContent: true,
        });
      })
      .catch((e) => setError(String(e.message || e)));
  }, [id]);

  const save = useCallback(async () => {
    if (saving.current || conflict.current || !latest.current) return;
    clearTimeout(timer.current);
    const { elements, appState, files } = latest.current;
    const sig = signature(elements, files);
    saving.current = true;
    setStatus("saving");
    try {
      const body = serializeAsJSON(elements, appState, files, "local");
      const r = await fetch(apiPath(id), {
        method: "PUT",
        credentials: "same-origin",
        headers: { "If-Match": mtime.current, "Content-Type": "application/json" },
        body,
      });
      if (r.status === 409) {
        conflict.current = true;
        setStatus("conflict");
        return;
      }
      if (!r.ok) throw new Error(r.status + " " + (await r.text()));
      mtime.current = String((await r.json()).mtime_ns);
      baseline.current = sig;
      setStatus(signature(latest.current.elements, latest.current.files) === sig ? "saved" : "dirty");
      if (signature(latest.current.elements, latest.current.files) !== sig) schedule();
    } catch (e) {
      setError(String(e.message || e));
      setStatus("error");
    } finally {
      saving.current = false;
    }
  }, [id]);

  const schedule = useCallback(() => {
    clearTimeout(timer.current);
    timer.current = setTimeout(save, AUTOSAVE_MS);
  }, [save]);

  const onChange = useCallback(
    (elements, appState, files) => {
      latest.current = { elements, appState, files };
      const sig = signature(elements, files);
      if (baseline.current === null) {
        baseline.current = sig;
        return;
      }
      if (sig === baseline.current || conflict.current) return;
      setStatus((s) => (s === "saving" ? s : "dirty"));
      schedule();
    },
    [schedule],
  );

  // Presence only: the server counts open streams to decide whether `gote update`
  // may restart it. Events are ignored; the editor never reloads itself.
  useEffect(() => {
    const url = document.getElementById("root").dataset.events;
    if (!url) return undefined;
    const es = new EventSource(url);
    return () => es.close();
  }, []);

  useEffect(() => {
    const onKey = (e) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "s") {
        e.preventDefault();
        e.stopPropagation();
        save();
      }
    };
    window.addEventListener("keydown", onKey, true);
    return () => window.removeEventListener("keydown", onKey, true);
  }, [save]);

  if (error && !initial) return <pre className="gote-error">{error}</pre>;
  if (!initial) return null;
  return (
    <div style={{ height: "100vh" }}>
      <Excalidraw {...localOnlyProps} initialData={initial} onChange={onChange} theme={dark() ? "dark" : "light"}>
        <MainMenu>
          <MainMenu.DefaultItems.LoadScene />
          <MainMenu.DefaultItems.Export />
          <MainMenu.DefaultItems.SaveAsImage />
          <MainMenu.DefaultItems.SearchMenu />
          <MainMenu.DefaultItems.Help />
          <MainMenu.DefaultItems.ClearCanvas />
          <MainMenu.Separator />
          <MainMenu.DefaultItems.ToggleTheme />
          <MainMenu.DefaultItems.ChangeCanvasBackground />
        </MainMenu>
      </Excalidraw>
      <div
        id="gote-status"
        className={"gote-pill " + status}
        title={error}
        onClick={() => status === "conflict" && location.reload()}
      >
        {status === "conflict" ? "conflict - file changed on disk, click to reload" : status}
      </div>
    </div>
  );
}

const el = document.getElementById("root");
createRoot(el).render(<Editor id={el.dataset.id} />);
