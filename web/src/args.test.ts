import { describe, expect, it } from "vitest";
import { buildArgs, connectHost, DEFAULT_CVARS, queryCommands } from "./args";

describe("queryCommands", () => {
  it("prefixes each command and splits words", () => {
    expect(queryCommands("?cg_fov 120&r_picmip 2")).toEqual(["+cg_fov", "120", "+r_picmip", "2"]);
  });
  it("decodes percent escapes", () => {
    expect(queryCommands("?set%20name%20bob")).toEqual(["+set", "name", "bob"]);
  });
  it("returns nothing for an empty query", () => {
    expect(queryCommands("")).toEqual([]);
    expect(queryCommands("?")).toEqual([]);
  });
});

describe("connectHost", () => {
  it("keeps an explicit port", () => expect(connectHost("a:8080", "http:")).toBe("a:8080"));
  it("adds 80 for http", () => expect(connectHost("a", "http:")).toBe("a:80"));
  it("adds 443 for https", () => expect(connectHost("a", "https:")).toBe("a:443"));
});

describe("buildArgs", () => {
  const base = { name: "bob", host: "h:80", search: "" };
  it("starts with name, cdn and connect", () => {
    expect(buildArgs(base).slice(0, 10)).toEqual(
      ["+set", "name", "bob", "+name", "bob", "+set", "fs_cdn", "h:80", "+connect", "h:80"]);
  });
  it("includes every default cvar", () => {
    const args = buildArgs(base);
    for (const [k, v] of DEFAULT_CVARS) {
      const i = args.indexOf(k);
      expect(args[i - 1]).toBe("+set");
      expect(args[i + 1]).toBe(v);
    }
  });
  it("lets query commands come after the defaults so they override", () => {
    const args = buildArgs({ ...base, search: "?cg_fov 90" });
    expect(args.lastIndexOf("+cg_fov")).toBeGreaterThan(args.indexOf("cg_fov"));
  });
  it("adds the password only when given", () => {
    expect(buildArgs(base)).not.toContain("password");
    expect(buildArgs({ ...base, password: "x" }).slice(-3)).toEqual(["+set", "password", "x"]);
  });
});

describe("queryCommands edge and bad input", () => {
  it("skips a malformed percent escape instead of throwing", () => {
    expect(queryCommands("?%E0%A4%A&cg_fov 90")).toEqual(["+cg_fov", "90"]);
  });
  it("ignores empty segments", () => {
    expect(queryCommands("?&&cg_fov 90&")).toEqual(["+cg_fov", "90"]);
  });
  it("splits on = like the original parser", () => {
    expect(queryCommands("?a=b")).toEqual(["+a", "+b"]);
  });
  it("accepts a query without a leading question mark", () => {
    expect(queryCommands("cg_fov 90")).toEqual(["+cg_fov", "90"]);
  });
  it("keeps a very long query intact", () => {
    const cmd = "x".repeat(10_000);
    expect(queryCommands(`?${cmd}`)).toEqual([`+${cmd}`]);
  });
});

describe("buildArgs edge cases", () => {
  it("treats an empty password as unset", () => {
    expect(buildArgs({ name: "a", host: "h:1", search: "", password: "" })).not.toContain("password");
  });
  it("passes a name with spaces as one argument", () => {
    expect(buildArgs({ name: "big bob", host: "h:1", search: "" })[2]).toBe("big bob");
  });
  it("has no duplicate default cvars", () => {
    const keys = DEFAULT_CVARS.map(([k]) => k);
    expect(new Set(keys).size).toBe(keys.length);
  });
});
