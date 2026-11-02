import { describe, expect, it } from "vitest";
import { loadClassicScript } from "./helpers/loadScript.js";

describe("userSettings.js server error mapping", () => {
  it("maps the server's codes to their catalog text and falls back otherwise", () => {
    const { exports: { serverErrorMessage }, realm } = loadClassicScript("userSettings.js", ["serverErrorMessage"], {
      prelude: "document.body.innerHTML = '';",
      globals: {
        fetch: () => Promise.resolve({ status: 200, json: () => Promise.resolve({}) }),
        // The helper merges the catalog translator into this object.
        proviant: {},
      },
    });

    // The server names the cause with a stable code; the text behind it is
    // prose and may be reworded without breaking the mapping.
    const disabled = serverErrorMessage("TWO_FACTOR_DISABLED", "fallback");
    expect(disabled).not.toBe("fallback");
    expect(disabled).toContain("two-factor");

    const notEnabled = serverErrorMessage("TWO_FACTOR_NOT_ENABLED", "fallback");
    expect(notEnabled).not.toBe("fallback");

    const credentials = serverErrorMessage("INVALID_CREDENTIALS", "fallback");
    expect(credentials).not.toBe("fallback");

    // An unknown code falls back to the caller's text.
    expect(serverErrorMessage("SOMETHING_ELSE", "fallback")).toBe("fallback");
    expect(serverErrorMessage(undefined, "fallback")).toBe("fallback");

    // The mapping must never show the raw server sentence.
    expect(serverErrorMessage("TWO_FACTOR_DISABLED", "fallback")).not.toContain("two-factor authentication is disabled on this server");

    expect(realm.document).toBeDefined();
  });
});
