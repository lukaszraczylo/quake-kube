import { buildArgs, connectHost } from "./args.ts";
import { placeholder, resolveName, storedName } from "./player.ts";
import { showError, showLoading } from "./ui.ts";

const WASM_MEMORY = 384 * 1024 * 1024;

// ioquake3.js reads Module when it starts, and both scripts are deferred in
// document order, so this assignment runs first.
window.Module = {
  TOTAL_MEMORY: WASM_MEMORY,
  canvas: null,
  printErr: (text) => console.error(text),
  setStatus: (text) => console.log(text),
};

function fullscreenActive(): boolean {
  const d = document as Document & Record<string, unknown>;
  return ["webkitFullScreenElement", "webkitFullscreenElement", "mozFullScreenElement",
    "mozFullscreenElement", "fullScreenElement", "fullscreenElement"].some((k) => d[k]);
}

function byId<T extends HTMLElement>(id: string): T | null {
  return document.getElementById(id) as T | null;
}

function join(): void {
  const nameInput = byId<HTMLInputElement>("playerName");
  const frame = byId("viewport-frame");
  const main = byId("main");
  if (!nameInput || !frame) return;

  const args = buildArgs({
    name: resolveName(nameInput.value),
    host: connectHost(document.location.host, window.location.protocol),
    search: window.location.search,
    password: byId<HTMLInputElement>("password")?.value,
  });
  main?.remove();
  showLoading(frame);
  try {
    ioq3.callMain(args);
  } catch (err) {
    showError(err);
  }
}

function initForm(): void {
  const nameInput = byId<HTMLInputElement>("playerName");
  if (nameInput) {
    nameInput.placeholder = placeholder();
    nameInput.value = storedName() ?? "";
  }
  byId<HTMLFormElement>("join-form")?.addEventListener("submit", (e) => {
    e.preventDefault();
    join();
  });
}

function initEngine(): void {
  const frame = byId("viewport-frame");
  ioq3.viewport = frame;
  ioq3.elementPointerLock = true;
  ioq3.exitHandler = (err) => {
    if (err) {
      showError(err);
      return;
    }
    window.location.href = "/";
  };
  window.addEventListener("resize", () => {
    if (!ioq3.canvas || !ioq3.viewport || fullscreenActive()) return;
    ioq3.setCanvasSize(ioq3.viewport.offsetWidth, ioq3.viewport.offsetHeight);
  });
}

initForm();
window.addEventListener("load", initEngine);
