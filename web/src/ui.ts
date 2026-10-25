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

const LOADING_LABEL = "Downloading game files";

function shown(frame: HTMLElement, selector: string): boolean {
  return frame.querySelector<HTMLElement>(selector)?.style.display === "block";
}

/** Swaps the engine's per-file names for one overall percentage. */
function tidyDownloadLabel(frame: HTMLElement): void {
  const progress = frame.querySelector("#loading-progress");
  const label = progress?.querySelector(".description");
  const bar = progress?.querySelector<HTMLElement>(".bar");
  if (!label || !bar || !label.textContent) return;
  const text = `${LOADING_LABEL}\u2026 ${Math.round(parseFloat(bar.style.width) || 0)}%`;
  if (label.textContent !== text) label.textContent = text;
}

/**
 * Keeps feedback on screen until the engine adds its canvas. The engine draws
 * its own download panel and licence prompt, so our text steps aside for both.
 */
export function showLoading(frame: HTMLElement): void {
  const box = statusBox();
  box.className = "status loading";
  box.textContent = "Loading game files...";
  const observer = new MutationObserver(() => {
    if (frame.querySelector("canvas")) {
      box.className = "status";
      observer.disconnect();
      return;
    }
    tidyDownloadLabel(frame);
    const engineBusy = shown(frame, "#loading") || shown(frame, "#eula-frame");
    box.className = engineBusy ? "status" : "status loading";
  });
  observer.observe(frame, { childList: true, subtree: true, attributes: true, characterData: true });
}
