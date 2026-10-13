import { describe, expect, it } from "vitest";
import { createMatchMedia, loadClassicScript } from "./helpers/loadScript.js";

/**
 * The loader is what every other spec depends on, so its failure modes matter
 * as much as the code it exercises. These are the messages a contributor meets
 * the first time a target file turns out not to be reachable — if they degrade,
 * the next person debugs a mystery instead of reading the hint.
 */
describe("loadClassicScript error reporting", () => {
  const fetchStub = async () => ({ status: 200, json: async () => ({}) });

  it("names the identifier when a target is not at top level", () => {
    expect(() => loadClassicScript("proviant.js", ["noSuchIdentifier"], { globals: { fetch: fetchStub } })).toThrow(
      /proviant\.js did not define \[noSuchIdentifier\]/,
    );
  });

  it("points at the IIFE seam when internals are private", () => {
    expect(() =>
      loadClassicScript("theme.js", ["applyTheme"], { globals: { matchMedia: createMatchMedia(false) } }),
    ).toThrow(/explicit export seam/);
  });

  it("rejects identifiers that are not valid JavaScript names", () => {
    expect(() => loadClassicScript("proviant.js", ["a; window.pwned = 1"])).toThrow(
      /not valid JavaScript identifiers/,
    );
    expect(globalThis.pwned).toBeUndefined();
  });

  it("reports a file that does not exist", () => {
    expect(() => loadClassicScript("does-not-exist.js", ["anything"])).toThrow(/ENOENT/);
  });

  it("reports a script that threw before it could be captured", () => {
    // Reproduces a source file that blows up during load. The realm needs its
    // own error handler first: jsdom rethrows a script error as an uncaught
    // exception on the window, which would otherwise fail the whole run for a
    // condition this test is deliberately provoking.
    const swallow = "window.addEventListener('error', function (e) { e.preventDefault(); });";
    expect(() => loadClassicScript("proviant.js", ["proviant"], { prelude: `${swallow}throw new Error('boom');` })).toThrow(
      /never reached its end/,
    );
  });

  it("leaves no carrier state behind on document", () => {
    loadClassicScript("proviant.js", ["proviant"], { globals: { fetch: fetchStub } });

    expect(document.__proviantSetup).toBeUndefined();
    expect(document.__proviantRealm).toBeUndefined();
    expect(document.__proviantCaptured).toBeUndefined();
  });
});

describe("loadClassicScript isolation", () => {
  const fetchStub = async () => ({ status: 200, json: async () => ({}) });

  it("gives each load a pristine copy of the module state", () => {
    const first = loadClassicScript("proviant.js", ["proviant"], { globals: { fetch: fetchStub } });
    first.exports.proviant._enqueueAmountDelta(7, 2);

    const second = loadClassicScript("proviant.js", ["proviant"], {
      prelude: "localStorage.clear();",
      globals: { fetch: fetchStub },
    });

    expect(second.exports.proviant._offlineQueue).toHaveLength(0);
  });

  it("does not accumulate script elements in the document head", () => {
    const before = document.head.querySelectorAll("script").length;

    loadClassicScript("proviant.js", ["proviant"], { globals: { fetch: fetchStub } });
    loadClassicScript("proviant.js", ["proviant"], { globals: { fetch: fetchStub } });

    expect(document.head.querySelectorAll("script")).toHaveLength(before);
  });
});