import { describe, expect, it } from "vitest";
import { createMatchMedia, loadClassicScript } from "./helpers/loadScript.js";

/**
 * theme.js reads localStorage during load and writes data-bs-theme onto the
 * document element, both of which persist on jsdom's realm for the whole file.
 * The prelude resets both so every test starts from the shipped default, and
 * optionally seeds a stored choice before the source runs.
 */
const RESET_REALM =
  "localStorage.clear();" +
  " document.body.innerHTML = '';" +
  " document.documentElement.removeAttribute('data-bs-theme');";

function freshTheme({ prefersDark = false, stored = null } = {}) {
  const seed = stored === null ? "" : `localStorage.setItem('proviant_theme', ${JSON.stringify(stored)});`;
  const { exports: captured, realm } = loadClassicScript("theme.js", ["proviantTheme"], {
    prelude: `${RESET_REALM}${seed}`,
    globals: { matchMedia: createMatchMedia(prefersDark) },
  });
  return { theme: captured.proviantTheme, realm };
}

const applied = (realm) => realm.document.documentElement.getAttribute("data-bs-theme");

describe("theme.js stored and effective theme", () => {
  it("defaults to following the system", () => {
    const { theme } = freshTheme();

    expect(theme.get()).toBe("system");
  });

  it("resolves the system theme to light when the OS prefers light", () => {
    const { theme } = freshTheme({ prefersDark: false });

    expect(theme.getEffective()).toBe("light");
  });

  it("resolves the system theme to dark when the OS prefers dark", () => {
    const { theme } = freshTheme({ prefersDark: true });

    expect(theme.getEffective()).toBe("dark");
  });

  it("reports an explicitly stored theme verbatim", () => {
    const { theme } = freshTheme({ stored: "dark", prefersDark: false });

    expect(theme.get()).toBe("dark");
    expect(theme.getEffective()).toBe("dark");
  });
});

describe("theme.js applying a theme", () => {
  it("writes the effective theme to the document element", () => {
    const { realm } = freshTheme({ stored: "dark" });

    expect(applied(realm)).toBe("dark");
  });

  it("applies the resolved system theme rather than the literal 'system'", () => {
    const { realm } = freshTheme({ prefersDark: true });

    expect(applied(realm)).toBe("dark");
  });

  it("persists an explicit choice", () => {
    const { theme, realm } = freshTheme();

    theme.set("dark");

    expect(realm.localStorage.getItem("proviant_theme")).toBe("dark");
    expect(applied(realm)).toBe("dark");
  });

  it("rejects a theme that is not one of the three known values", () => {
    const { theme, realm } = freshTheme();

    theme.set("chartreuse");

    expect(theme.get()).toBe("system");
    expect(realm.localStorage.getItem("proviant_theme")).toBeNull();
  });

  it("tolerates a missing theme-color meta tag", () => {
    const { theme } = freshTheme();

    expect(() => theme.set("dark")).not.toThrow();
  });
});

describe("theme.js toggle icons", () => {
  function withToggles({ prefersDark = false, stored = null } = {}) {
    const loaded = freshTheme({ prefersDark, stored });
    const doc = loaded.realm.document;

    const desktop = doc.createElement("i");
    desktop.id = "themeToggleIcon";
    doc.body.appendChild(desktop);

    const mobile = doc.createElement("i");
    mobile.id = "themeToggleIconMobile";
    doc.body.appendChild(mobile);

    // applyTheme already ran during load, before these elements existed, so
    // re-apply to give the freshly attached icons a starting class.
    loaded.theme.set(loaded.theme.get());

    return { ...loaded, desktop, mobile };
  }

  it("maps each theme to its icon", () => {
    const { theme, desktop, mobile } = withToggles({});

    theme.set("dark");
    expect(desktop.className).toBe("bi bi-moon-fill");
    expect(mobile.className).toBe("bi me-2 bi-moon-fill");

    theme.set("light");
    expect(desktop.className).toBe("bi bi-sun-fill");
    expect(mobile.className).toBe("bi me-2 bi-sun-fill");

    theme.set("system");
    expect(desktop.className).toBe("bi bi-circle-half");
    expect(mobile.className).toBe("bi me-2 bi-circle-half");
  });

  it("shows the system icon while following the system", () => {
    const { desktop } = withToggles({ stored: "system", prefersDark: true });

    // The icon reflects the user's choice ("system"), not the resolved value.
    expect(desktop.className).toBe("bi bi-circle-half");
  });

  it("updates the browser chrome colour to match", () => {
    const { theme, realm } = withToggles({});
    const meta = realm.document.createElement("meta");
    meta.name = "theme-color";
    realm.document.head.appendChild(meta);

    theme.set("dark");
    expect(meta.getAttribute("content")).toBe("#2a2620");

    theme.set("light");
    expect(meta.getAttribute("content")).toBe("#3D7A5C");
  });
});

describe("theme.js cycling", () => {
  it("walks system -> light -> dark -> system", () => {
    const { theme } = freshTheme({ prefersDark: false });
    const seen = [theme.get()];

    theme.cycle();
    seen.push(theme.get());
    theme.cycle();
    seen.push(theme.get());
    theme.cycle();
    seen.push(theme.get());

    expect(seen).toEqual(["system", "light", "dark", "system"]);
  });

  it("wraps around from dark back to system", () => {
    const { theme } = freshTheme({ stored: "dark" });

    theme.cycle();

    expect(theme.get()).toBe("system");
  });
});

describe("theme.js onChange", () => {
  it("reports the effective and stored theme on every change", () => {
    const { theme } = freshTheme({ prefersDark: false });
    const seen = [];
    theme.onChange((effective, storedValue) => seen.push([effective, storedValue]));

    theme.set("dark");
    theme.set("light");

    expect(seen).toEqual([
      ["dark", "dark"],
      ["light", "light"],
    ]);
  });

  it("resolves a system change to the OS preference", () => {
    const { theme } = freshTheme({ prefersDark: true });
    const seen = [];
    theme.onChange((effective) => seen.push(effective));

    theme.set("system");

    expect(seen).toEqual(["dark"]);
  });
});