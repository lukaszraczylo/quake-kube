// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

const PAGE = `
  <div id="status" class="status"></div>
  <div id="main"><form id="join-form">
    <input type="text" id="playerName"><input type="password" id="password">
  </form></div>
  <div id="viewport-frame"></div>`;

type Stub = {
  canvas: HTMLCanvasElement | null;
  viewport: HTMLElement | null;
  elementPointerLock: boolean;
  exitHandler: ((e?: unknown) => void) | null;
  setCanvasSize: ReturnType<typeof vi.fn>;
  callMain: ReturnType<typeof vi.fn>;
};

const added: Array<[string, EventListenerOrEventListenerObject]> = [];
const realAdd = window.addEventListener.bind(window);

async function load(search = ""): Promise<Stub> {
  vi.resetModules();
  localStorage.clear();
  document.body.innerHTML = PAGE;
  window.history.replaceState({}, "", `/${search}`);
  const stub: Stub = {
    canvas: null, viewport: null, elementPointerLock: false, exitHandler: null,
    setCanvasSize: vi.fn(), callMain: vi.fn(),
  };
  vi.stubGlobal("ioq3", stub);
  window.addEventListener = ((type: string, fn: EventListenerOrEventListenerObject) => {
    added.push([type, fn]);
    realAdd(type, fn);
  }) as typeof window.addEventListener;
  await import("./main");
  window.dispatchEvent(new Event("load"));
  return stub;
}

const submit = () =>
  (document.getElementById("join-form") as HTMLFormElement).dispatchEvent(
    new Event("submit", { cancelable: true, bubbles: true }));

beforeEach(() => vi.unstubAllGlobals());
afterEach(() => {
  for (const [type, fn] of added.splice(0)) window.removeEventListener(type, fn);
  window.addEventListener = realAdd;
});

describe("page wiring", () => {
  it("defines Module before the engine loads", async () => {
    await load();
    expect(window.Module.TOTAL_MEMORY).toBe(384 * 1024 * 1024);
  });
  it("configures the engine on load", async () => {
    const ioq3 = await load();
    expect(ioq3.viewport?.id).toBe("viewport-frame");
    expect(ioq3.elementPointerLock).toBe(true);
  });
  it("starts the game with the typed name and removes the lobby", async () => {
    const ioq3 = await load("?cg_fov%2090");
    (document.getElementById("playerName") as HTMLInputElement).value = "bob";
    submit();
    expect(ioq3.callMain).toHaveBeenCalledOnce();
    const args = ioq3.callMain.mock.calls[0]?.[0] as string[];
    expect(args.slice(0, 3)).toEqual(["+set", "name", "bob"]);
    expect(args.slice(-2)).toEqual(["+cg_fov", "90"]);
    expect(document.getElementById("main")).toBeNull();
    expect(document.getElementById("status")?.className).toBe("status loading");
  });
  it("sends the password only when entered", async () => {
    const ioq3 = await load();
    (document.getElementById("password") as HTMLInputElement).value = "s3cret";
    submit();
    expect((ioq3.callMain.mock.calls[0]?.[0] as string[]).slice(-3)).toEqual(["+set", "password", "s3cret"]);
  });
  it("works on a lobby page without a password field", async () => {
    const ioq3 = await load();
    document.getElementById("password")?.remove();
    submit();
    expect(ioq3.callMain).toHaveBeenCalledOnce();
  });
  it("prevents the native form post", async () => {
    await load();
    const ev = new Event("submit", { cancelable: true, bubbles: true });
    document.getElementById("join-form")?.dispatchEvent(ev);
    expect(ev.defaultPrevented).toBe(true);
  });
});

describe("engine callbacks", () => {
  it("shows the engine error in the page", async () => {
    const ioq3 = await load();
    ioq3.exitHandler?.("no pak0");
    expect(document.getElementById("status")?.textContent).toContain("no pak0");
  });
  it("resizes the canvas to the viewport", async () => {
    const ioq3 = await load();
    ioq3.canvas = document.createElement("canvas");
    window.dispatchEvent(new Event("resize"));
    expect(ioq3.setCanvasSize).toHaveBeenCalledOnce();
  });
  it("ignores resize before the canvas exists", async () => {
    const ioq3 = await load();
    window.dispatchEvent(new Event("resize"));
    expect(ioq3.setCanvasSize).not.toHaveBeenCalled();
  });
  it("ignores resize while fullscreen", async () => {
    const ioq3 = await load();
    ioq3.canvas = document.createElement("canvas");
    Object.defineProperty(document, "fullscreenElement", { value: document.body, configurable: true });
    window.dispatchEvent(new Event("resize"));
    Reflect.deleteProperty(document, "fullscreenElement");
    expect(ioq3.setCanvasSize).not.toHaveBeenCalled();
  });
});

describe("flaky engine", () => {
  it("shows an error when callMain throws", async () => {
    const ioq3 = await load();
    ioq3.callMain.mockImplementation(() => { throw new Error("wasm trap"); });
    expect(() => submit()).not.toThrow();
    const box = document.getElementById("status");
    expect(box?.className).toBe("status error");
    expect(box?.textContent).toContain("wasm trap");
  });
});
