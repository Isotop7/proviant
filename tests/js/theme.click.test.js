import { describe, expect, it } from "vitest";
import { createMatchMedia, loadClassicScript } from "./helpers/loadScript.js";

/**
 * Every load of theme.js registers another document-level click listener, and
 * each of those listeners calls window.proviantTheme.cycle(). Loading the file
 * N times therefore cycles the theme N times for a single click, which makes a
 * delegation assertion depend on how many other tests ran first.
 *
 * Keeping the delegation test alone in its own file removes that coupling: this
 * is the only load, so exactly one listener exists.
 */
describe("theme.js click delegation", () => {
  it("cycles when the toggle is clicked, from either button", () => {
    const { exports: captured, realm } = loadClassicScript("theme.js", ["proviantTheme"], {
      prelude: "localStorage.clear(); document.body.innerHTML = '';",
      globals: { matchMedia: createMatchMedia(false) },
    });
    const theme = captured.proviantTheme;
    const doc = realm.document;

    const desktop = doc.createElement("button");
    desktop.id = "btnThemeToggle";
    desktop.innerHTML = '<i id="themeToggleIcon"></i>';
    doc.body.appendChild(desktop);

    const mobile = doc.createElement("button");
    mobile.id = "btnThemeToggleMobile";
    doc.body.appendChild(mobile);

    const unrelated = doc.createElement("button");
    unrelated.id = "someOtherButton";
    doc.body.appendChild(unrelated);

    expect(theme.get()).toBe("system");

    // A click on a child element must still match, via Element.closest.
    desktop.querySelector("i").dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    expect(theme.get()).toBe("light");

    mobile.dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    expect(theme.get()).toBe("dark");

    unrelated.dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    expect(theme.get()).toBe("dark");
  });
});