function statusBox(): HTMLElement {
  const box = document.getElementById("status");
  if (!box) throw new Error("#status element missing");
  return box;
}

/** Shows an engine error in the page; the server has no handler for POST /. */
export function showError(err: unknown): void {
  const box = statusBox();
  box.className = "status error";
  const msg = document.createElement("p");
  msg.textContent = String(err);
  const retry = document.createElement("a");
  retry.href = "/";
  retry.textContent = "Back to the lobby";
  box.replaceChildren(msg, retry);
}

/** Keeps feedback on screen until the engine adds its canvas. */
export function showLoading(frame: HTMLElement): void {
  const box = statusBox();
  box.className = "status loading";
  box.textContent = "Loading game files...";
  const observer = new MutationObserver(() => {
    if (frame.querySelector("canvas")) {
      box.className = "status";
      observer.disconnect();
    }
  });
  observer.observe(frame, { childList: true, subtree: true });
}
