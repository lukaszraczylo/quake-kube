import assert from "node:assert/strict";
import { describe, it } from "node:test";
import { buildArgs, connectHost, DEFAULT_CVARS, queryCommands } from "./args.ts";

describe("queryCommands", () => {
  it("prefixes each command and splits words", () => {
    assert.deepEqual(queryCommands("?cg_fov 120&r_picmip 2"), ["+cg_fov", "120", "+r_picmip", "2"]);
  });
  it("decodes percent escapes", () => {
    assert.deepEqual(queryCommands("?set%20name%20bob"), ["+set", "name", "bob"]);
  });
  it("returns nothing for an empty query", () => {
    assert.deepEqual(queryCommands(""), []);
    assert.deepEqual(queryCommands("?"), []);
  });
  it("skips a malformed percent escape instead of throwing", () => {
    assert.deepEqual(queryCommands("?%E0%A4%A&cg_fov 90"), ["+cg_fov", "90"]);
  });
  it("ignores empty segments", () => {
    assert.deepEqual(queryCommands("?&&cg_fov 90&"), ["+cg_fov", "90"]);
  });
  it("splits on = like the original parser", () => {
    assert.deepEqual(queryCommands("?a=b"), ["+a", "+b"]);
  });
  it("accepts a query without a leading question mark", () => {
    assert.deepEqual(queryCommands("cg_fov 90"), ["+cg_fov", "90"]);
  });
  it("keeps a very long query intact", () => {
    const cmd = "x".repeat(10_000);
    assert.deepEqual(queryCommands(`?${cmd}`), [`+${cmd}`]);
  });
});

describe("connectHost", () => {
  it("keeps an explicit port", () => assert.equal(connectHost("a:8080", "http:"), "a:8080"));
  it("adds 80 for http", () => assert.equal(connectHost("a", "http:"), "a:80"));
  it("adds 443 for https", () => assert.equal(connectHost("a", "https:"), "a:443"));
});

describe("buildArgs", () => {
  const base = { name: "bob", host: "h:80", search: "" };
  it("starts with name, cdn and connect", () => {
    assert.deepEqual(buildArgs(base).slice(0, 10),
      ["+set", "name", "bob", "+name", "bob", "+set", "fs_cdn", "h:80", "+connect", "h:80"]);
  });
  it("includes every default cvar", () => {
    const args = buildArgs(base);
    for (const [k, v] of DEFAULT_CVARS) {
      const i = args.indexOf(k);
      assert.equal(args[i - 1], "+set");
      assert.equal(args[i + 1], v);
    }
  });
  it("lets query commands come after the defaults so they override", () => {
    const args = buildArgs({ ...base, search: "?cg_fov 90" });
    assert.ok(args.lastIndexOf("+cg_fov") > args.indexOf("cg_fov"));
  });
  it("adds the password only when given", () => {
    assert.ok(!buildArgs(base).includes("password"));
    assert.deepEqual(buildArgs({ ...base, password: "x" }).slice(-3), ["+set", "password", "x"]);
  });
  it("treats an empty password as unset", () => {
    assert.ok(!buildArgs({ ...base, password: "" }).includes("password"));
  });
  it("passes a name with spaces as one argument", () => {
    assert.equal(buildArgs({ ...base, name: "big bob" })[2], "big bob");
  });
  it("has no duplicate default cvars", () => {
    const keys = DEFAULT_CVARS.map(([k]) => k);
    assert.equal(new Set(keys).size, keys.length);
  });
});
