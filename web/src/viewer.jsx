import "@excalidraw/excalidraw/index.css";
import React, { useEffect, useState } from "react";
import { createRoot } from "react-dom/client";
import { Excalidraw } from "@excalidraw/excalidraw";
import { dark, loadDrawing } from "./common.js";

window.EXCALIDRAW_ASSET_PATH = "/assets/";

function Viewer({ id }) {
  const [initial, setInitial] = useState(null);
  const [error, setError] = useState("");
  useEffect(() => {
    loadDrawing(id)
      .then(({ scene }) =>
        setInitial({
          elements: scene.elements || [],
          appState: { ...(scene.appState || {}), collaborators: new Map() },
          files: scene.files || {},
          scrollToContent: true,
        }),
      )
      .catch((e) => setError(String(e.message || e)));
  }, [id]);
  if (error) return <pre className="gote-error">{error}</pre>;
  if (!initial) return null;
  return <Excalidraw initialData={initial} viewModeEnabled zenModeEnabled theme={dark() ? "dark" : "light"} />;
}

document.querySelectorAll(".gote-drawing").forEach((el) => {
  createRoot(el.querySelector(".gote-drawing-canvas")).render(<Viewer id={el.dataset.id} />);
});
