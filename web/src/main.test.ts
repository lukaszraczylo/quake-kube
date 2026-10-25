import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { FakeElement, install, type Env } from "./testdom.ts";

interface Stub {
  canvas: unknown;
  viewport: FakeElement | null;
  elementPointerLock: boolean;
  exitHandler: ((e?: unknown) => void) | null;
  calls: string[][];
  sizes: number[][];
  callMain: (a: string[]) => void;
  setCanvasSize: (w: number, h: number) => void;
}

let seq = 0;

async function load(opts: { search?: string; password?: boolean } = {}): Promise<{ env: Env; ioq3: Stub }> {
  const env = install(opts);
  const ioq3: Stub = {
    canvas: null, viewport: null, elementPointerLock: false, exitHandler: null, calls: [], sizes: [],
    callMain(a) { this.calls.push(a); },
    setCanvasSize(w, h) { this.sizes.push([w, h]); },
  };
  (globalThis as Record<string, unknown>).ioq3 = ioq3;
  await import(`./main.ts?run=${++seq}`);
  env.win.dispatch("load");
  return { env, ioq3 };
}

const el = (env: Env, id: string) => env.byId.get(id) as FakeElement;
const join = (env: Env) => el(env, "join-form").dispatch("submit");

describe("page wiring", () => {
  it("defines Module before the engine loads", async () => {
    const { env } = await load();
    assert.equal((env.win.Module as { TOTAL_MEMORY: number }).TOTAL_MEMORY, 384 * 1024 * 1024);
  });
  it("configures the engine on load", async () => {
    const { env, ioq3 } = await load();
    assert.equal(ioq3.viewport, el(env, "viewport-frame"));
    assert.equal(ioq3.elementPointerLock, true);
  });
  it("starts the game with the typed name and removes the join screen", async () => {
    const { env, ioq3 } = await load({ search: "?cg_fov%2090" });
    el(env, "playerName").value = "bob";
    join(env);
    assert.equal(ioq3.calls.length, 1);
    const args = ioq3.calls[0] as string[];
    assert.deepEqual(args.slice(0, 3), ["+set", "name", "bob"]);
    assert.deepEqual(args.slice(-2), ["+cg_fov", "90"]);
    assert.equal(el(env, "main").parent, null);
    assert.equal(el(env, "status").className, "status loading");
  });
  it("sends the password only when entered", async () => {
    const { env, ioq3 } = await load({ password: true });
    el(env, "password").value = "s3cret";
    join(env);
    assert.deepEqual((ioq3.calls[0] as string[]).slice(-3), ["+set", "password", "s3cret"]);
  });
  it("works on a page without a password field", async () => {
    const { env, ioq3 } = await load();
    join(env);
    assert.equal(ioq3.calls.length, 1);
  });
  it("prevents the native form post", async () => {
    const { env } = await load();
    assert.equal(join(env).defaultPrevented, true);
  });
  it("prefills the name from storage", async () => {
    const env0 = install();
    env0.storage.setItem("playerName", "old");
    const ioq3 = { viewport: null, calls: [] as string[][], callMain() {}, setCanvasSize() {} };
    (globalThis as Record<string, unknown>).ioq3 = ioq3;
    await import(`./main.ts?run=${++seq}`);
    assert.equal(el(env0, "playerName").value, "old");
    assert.equal(el(env0, "playerName").placeholder, "old");
  });
});

describe("engine callbacks", () => {
  it("shows the engine error in the page", async () => {
    const { env, ioq3 } = await load();
    ioq3.exitHandler?.("no pak0");
    assert.match(el(env, "status").textContent, /no pak0/);
  });
  it("returns to the join screen on a clean exit", async () => {
    const { env, ioq3 } = await load();
    ioq3.exitHandler?.();
    assert.equal(env.win.location.href, "/");
  });
  it("resizes the canvas to the viewport", async () => {
    const { env, ioq3 } = await load();
    ioq3.canvas = {};
    env.win.dispatch("resize");
    assert.deepEqual(ioq3.sizes, [[800, 600]]);
  });
  it("ignores resize before the canvas exists", async () => {
    const { env, ioq3 } = await load();
    env.win.dispatch("resize");
    assert.equal(ioq3.sizes.length, 0);
  });
  it("ignores resize while fullscreen", async () => {
    const { env, ioq3 } = await load();
    ioq3.canvas = {};
    env.doc.fullscreenElement = {};
    env.win.dispatch("resize");
    assert.equal(ioq3.sizes.length, 0);
  });
});

describe("flaky engine", () => {
  it("shows an error when callMain throws", async () => {
    const { env, ioq3 } = await load();
    ioq3.callMain = () => { throw new Error("wasm trap"); };
    join(env);
    assert.equal(el(env, "status").className, "status error");
    assert.match(el(env, "status").textContent, /wasm trap/);
  });
});
