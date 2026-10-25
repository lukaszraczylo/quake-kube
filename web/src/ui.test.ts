// @vitest-environment jsdom
import { beforeEach, describe, expect, it } from "vitest";
import { showError, showLoading } from "./ui";

let frame: HTMLElement;
const status = () => document.getElementById("status") as HTMLElement;
const tick = () => new Promise((r) => setTimeout(r, 0));

beforeEach(() => {
  document.body.innerHTML = '<div id="status" class="status"></div><div id="viewport-frame"></div>';
  frame = document.getElementById("viewport-frame") as HTMLElement;
});

describe("showError", () => {
  it("shows the message with a link back to the lobby", () => {
    showError("boom");
    expect(status().className).toBe("status error");
    expect(status().querySelector("p")?.textContent).toBe("boom");
    expect(status().querySelector("a")?.getAttribute("href")).toBe("/");
  });
  it("does not interpret markup in the message", () => {
    showError('<img src=x onerror="alert(1)">');
    expect(status().querySelector("img")).toBeNull();
    expect(status().querySelector("p")?.textContent).toContain("<img");
  });
  it("replaces a previous error instead of stacking", () => {
    showError("one");
    showError("two");
    expect(status().querySelectorAll("p")).toHaveLength(1);
    expect(status().textContent).toContain("two");
  });
  it("stringifies non-string errors", () => {
    showError(new Error("bad"));
    expect(status().textContent).toContain("bad");
    showError(undefined);
    expect(status().querySelector("p")?.textContent).toBe("undefined");
  });
  it("throws a clear error when #status is missing", () => {
    document.body.innerHTML = "";
    expect(() => showError("x")).toThrow("#status element missing");
  });
});

describe("showLoading", () => {
  it("shows the loading text", () => {
    showLoading(frame);
    expect(status().className).toBe("status loading");
    expect(status().textContent).toBe("Loading game files...");
  });
  it("stays visible while the engine adds no canvas", async () => {
    showLoading(frame);
    frame.appendChild(document.createElement("div"));
    await tick();
    expect(status().className).toBe("status loading");
  });
  it("hides once a canvas appears", async () => {
    showLoading(frame);
    frame.appendChild(document.createElement("canvas"));
    await tick();
    expect(status().className).toBe("status");
  });
  it("detects a canvas nested deeper in the frame", async () => {
    showLoading(frame);
    const wrap = document.createElement("div");
    frame.appendChild(wrap);
    await tick();
    wrap.appendChild(document.createElement("canvas"));
    await tick();
    expect(status().className).toBe("status");
  });
  it("does not clobber a later error after the canvas appeared", async () => {
    showLoading(frame);
    frame.appendChild(document.createElement("canvas"));
    await tick();
    showError("late");
    frame.appendChild(document.createElement("span"));
    await tick();
    expect(status().className).toBe("status error");
  });
});
