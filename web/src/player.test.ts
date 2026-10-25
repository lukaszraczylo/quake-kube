// @vitest-environment jsdom
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { placeholder, resolveName, storedName } from "./player";

beforeEach(() => localStorage.clear());
afterEach(() => vi.restoreAllMocks());

describe("resolveName", () => {
  it("uses and stores the typed name", () => {
    expect(resolveName("bob")).toBe("bob");
    expect(localStorage.getItem("playerName")).toBe("bob");
  });
  it("trims the typed name", () => expect(resolveName("  bob  ")).toBe("bob"));
  it("falls back to the stored name when input is empty or blank", () => {
    localStorage.setItem("playerName", "old");
    expect(resolveName("")).toBe("old");
    expect(resolveName("   ")).toBe("old");
  });
  it("generates and stores a default when nothing is known", () => {
    const name = resolveName("");
    expect(name).toMatch(/^Player_\d{1,3}$/);
    expect(localStorage.getItem("playerName")).toBe(name);
  });
  it("overwrites the stored name with a new typed one", () => {
    localStorage.setItem("playerName", "old");
    resolveName("new");
    expect(storedName()).toBe("new");
  });
});

describe("placeholder", () => {
  it("shows the stored name", () => {
    localStorage.setItem("playerName", "bob");
    expect(placeholder()).toBe("bob");
  });
  it("shows the hint when nothing is stored", () => expect(placeholder()).toBe("enter player name"));
});

describe("blocked storage (flaky environments)", () => {
  it("still returns a name when reads and writes throw", () => {
    vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => {
      throw new DOMException("denied", "SecurityError");
    });
    vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {
      throw new DOMException("quota", "QuotaExceededError");
    });
    expect(resolveName("bob")).toBe("bob");
    expect(resolveName("")).toMatch(/^Player_\d+$/);
    expect(storedName()).toBeNull();
    expect(placeholder()).toBe("enter player name");
  });
  it("recovers when storage works again", () => {
    const get = vi.spyOn(Storage.prototype, "getItem").mockImplementationOnce(() => {
      throw new Error("transient");
    });
    expect(storedName()).toBeNull();
    get.mockRestore();
    localStorage.setItem("playerName", "bob");
    expect(storedName()).toBe("bob");
  });
});
