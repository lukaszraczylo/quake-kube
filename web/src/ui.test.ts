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
