import assert from "node:assert/strict";
import { beforeEach, describe, it } from "node:test";
import { placeholder, resolveName, storedName } from "./player.ts";
import { install, type Env } from "./testdom.ts";

let env: Env;
beforeEach(() => {
  env = install();
});

describe("resolveName", () => {
  it("uses and stores the typed name", () => {
    assert.equal(resolveName("bob"), "bob");
    assert.equal(env.storage.getItem("playerName"), "bob");
  });
  it("trims the typed name", () => assert.equal(resolveName("  bob  "), "bob"));
  it("falls back to the stored name when input is empty or blank", () => {
    env.storage.setItem("playerName", "old");
    assert.equal(resolveName(""), "old");
    assert.equal(resolveName("   "), "old");
  });
  it("generates and stores a default when nothing is known", () => {
    const name = resolveName("");
    assert.match(name, /^Player_\d{1,3}$/);
    assert.equal(env.storage.getItem("playerName"), name);
  });
  it("overwrites the stored name with a new typed one", () => {
    env.storage.setItem("playerName", "old");
    resolveName("new");
    assert.equal(storedName(), "new");
  });
});

describe("placeholder", () => {
  it("shows the stored name", () => {
    env.storage.setItem("playerName", "bob");
    assert.equal(placeholder(), "bob");
  });
  it("shows the hint when nothing is stored", () => assert.equal(placeholder(), "enter player name"));
});

describe("blocked storage (flaky environments)", () => {
  it("still returns a name when reads and writes throw", () => {
    env.storage.failGet = true;
    env.storage.failSet = true;
    assert.equal(resolveName("bob"), "bob");
    assert.match(resolveName(""), /^Player_\d+$/);
    assert.equal(storedName(), null);
    assert.equal(placeholder(), "enter player name");
  });
  it("recovers when storage works again", () => {
    env.storage.setItem("playerName", "bob");
    env.storage.failGet = true;
    assert.equal(storedName(), null);
    env.storage.failGet = false;
    assert.equal(storedName(), "bob");
  });
});
