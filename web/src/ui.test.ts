import assert from "node:assert/strict";
import { beforeEach, describe, it } from "node:test";
import { showError, showLoading } from "./ui.ts";
import { FakeElement, install, tick, type Env } from "./testdom.ts";

let env: Env;
const status = () => env.byId.get("status") as FakeElement;
const frame = () => env.byId.get("viewport-frame") as unknown as HTMLElement;
const add = (parent: FakeElement, tag: string) => parent.appendChild(new FakeElement(tag));

beforeEach(() => {
  env = install();
});

describe("showError", () => {
  it("shows the message with a link back to the join screen", () => {
    showError("boom");
    assert.equal(status().className, "status error");
    assert.equal(status().querySelector("p")?.textContent, "boom");
    assert.equal(status().querySelector("a")?.href, "/");
  });
  it("uses textContent only, so markup stays text", () => {
    showError('<img src=x onerror="alert(1)">'); // the fake throws on innerHTML
    assert.equal(status().querySelector("img"), null);
    assert.match(status().textContent, /<img/);
  });
  it("replaces a previous error instead of stacking", () => {
    showError("one");
    showError("two");
    assert.equal(status().children.filter((c) => c.tag === "p").length, 1);
    assert.match(status().textContent, /two/);
  });
  it("stringifies non-string errors", () => {
    showError(new Error("bad"));
    assert.match(status().textContent, /bad/);
    showError(undefined);
    assert.equal(status().querySelector("p")?.textContent, "undefined");
  });
  it("throws a clear error when #status is missing", () => {
    env.byId.delete("status");
    assert.throws(() => showError("x"), /#status element missing/);
  });
});

describe("showLoading", () => {
  it("shows the loading text", () => {
    showLoading(frame());
    assert.equal(status().className, "status loading");
    assert.equal(status().textContent, "Loading game files...");
  });
  it("stays visible while the engine adds no canvas", async () => {
    showLoading(frame());
    add(env.byId.get("viewport-frame") as FakeElement, "div");
    await tick();
    assert.equal(status().className, "status loading");
  });
  it("hides once a canvas appears", async () => {
    showLoading(frame());
    add(env.byId.get("viewport-frame") as FakeElement, "canvas");
    await tick();
    assert.equal(status().className, "status");
  });
  it("detects a canvas nested deeper in the frame", async () => {
    showLoading(frame());
    const wrap = add(env.byId.get("viewport-frame") as FakeElement, "div");
    await tick();
    add(wrap, "canvas");
    await tick();
    assert.equal(status().className, "status");
  });
  it("does not clobber a later error after the canvas appeared", async () => {
    showLoading(frame());
    add(env.byId.get("viewport-frame") as FakeElement, "canvas");
    await tick();
    showError("late");
    add(env.byId.get("viewport-frame") as FakeElement, "span");
    await tick();
    assert.equal(status().className, "status error");
  });
});

describe("showLoading with the engine's own panels", () => {
  const frameEl = () => env.byId.get("viewport-frame") as FakeElement;

  function enginePanel(): { panel: FakeElement; label: FakeElement; bar: FakeElement; eula: FakeElement } {
    const mk = (tag: string, cls = "", id = "") => {
      const e = new FakeElement(tag);
      e.className = cls;
      e.id = id;
      return e;
    };
    const loading = mk("div", "", "loading");
    const progress = mk("div", "", "loading-progress");
    const label = mk("div", "description");
    const bar = mk("div", "bar");
    progress.appendChild(label);
    progress.appendChild(bar);
    loading.appendChild(progress);
    const eula = mk("div", "", "eula-frame");
    frameEl().appendChild(loading);
    frameEl().appendChild(eula);
    return { panel: loading, label, bar, eula };
  }

  it("hides file names and shows one overall percentage", async () => {
    showLoading(frame());
    const { panel, label, bar } = enginePanel();
    panel.style.display = "block";
    bar.style.width = "37.4%";
    label.textContent = "loading baseq3/pak0.pk3";
    await tick();
    assert.equal(label.textContent, "Downloading game files… 37%");
    assert.ok(!label.textContent.includes(".pk3"));
  });
  it("follows the bar as the next file starts", async () => {
    showLoading(frame());
    const { panel, label, bar } = enginePanel();
    panel.style.display = "block";
    bar.style.width = "10%";
    label.textContent = "loading baseq3/pak1.pk3";
    await tick();
    bar.style.width = "62%";
    label.textContent = "loading baseq3/pak2.pk3";
    await tick();
    assert.equal(label.textContent, "Downloading game files… 62%");
  });
  it("shows 0% when the bar has no width yet", async () => {
    showLoading(frame());
    const { panel, label } = enginePanel();
    panel.style.display = "block";
    label.textContent = "loading baseq3/pak0.pk3";
    await tick();
    assert.equal(label.textContent, "Downloading game files… 0%");
  });
  it("steps aside while the engine panel shows and returns after", async () => {
    showLoading(frame());
    const { panel } = enginePanel();
    panel.style.display = "block";
    await tick();
    assert.equal(status().className, "status");
    panel.style.display = "none";
    await tick();
    assert.equal(status().className, "status loading");
  });
  it("steps aside for the licence prompt", async () => {
    showLoading(frame());
    const { eula } = enginePanel();
    eula.style.display = "block";
    await tick();
    assert.equal(status().className, "status");
    eula.style.display = "none";
    await tick();
    assert.equal(status().className, "status loading");
  });
  it("leaves the label alone when the panel is empty", async () => {
    showLoading(frame());
    const { label } = enginePanel();
    label.textContent = "";
    await tick();
    assert.equal(label.textContent, "");
  });
  it("stops reacting once the canvas exists", async () => {
    showLoading(frame());
    const { panel } = enginePanel();
    add(frameEl(), "canvas");
    await tick();
    panel.style.display = "block";
    await tick();
    assert.equal(status().className, "status");
  });
});
