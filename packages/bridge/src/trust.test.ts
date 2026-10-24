import { describe, expect, test } from "bun:test";
import { isTrusted, type Trust } from "./trust";

const at = (url: string) => {
  const u = new URL(url);
  return { protocol: u.protocol, host: u.host, hostname: u.hostname };
};

// The cases of TestBridgeTrust in the Go package, which decides the same
// with isTrusted.
const rule: Trust = {
  schemes: ["about", "file", "mygo", "trusted"],
  origins: ["https://partner.example", "http://mygo.localhost"],
  loopback: false,
};

describe("isTrusted", () => {
  const cases: [string, boolean][] = [
    ["https://evil.example/", false],
    ["https://partner.example/page", true],
    ["HTTPS://Partner.Example/page", true],
    ["https://partner.example.evil.com/", false],
    ["http://partner.example/", false],
    ["trusted://localhost/", true],
    ["mygo://localhost/index.html", true],
    ["http://mygo.localhost/", true],
    ["http://other.localhost/", false],
    ["file:///tmp/index.html", true],
    ["about:blank", true],
    ["http://localhost:5173/", false],
    ["data:text/html,x", false],
  ];
  for (const [url, ok] of cases) {
    test(`${url} is ${ok ? "" : "not "}trusted`, () => expect(isTrusted(at(url), rule)).toBe(ok));
  }

  test("loopback servers", () => {
    const dev: Trust = { ...rule, loopback: true };
    expect(isTrusted(at("http://localhost:5173/"), dev)).toBe(true);
    expect(isTrusted(at("https://127.0.0.1:8443/"), dev)).toBe(true);
    expect(isTrusted(at("http://[::1]:3000/"), dev)).toBe(true);
    expect(isTrusted(at("http://192.168.1.2:5173/"), dev)).toBe(false);
    expect(isTrusted(at("ftp://localhost/"), dev)).toBe(false);
  });

  test("a star trusts every page", () => {
    const all: Trust = { schemes: [], origins: ["*"], loopback: false };
    expect(isTrusted(at("https://anything.example/"), all)).toBe(true);
  });
});
