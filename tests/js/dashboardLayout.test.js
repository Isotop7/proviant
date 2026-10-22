import { beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { loadClassicScript } from "./helpers/loadScript.js";

/**
 * dashboardLayout.js registers its document-level listeners once per load and
 * its grid-level listeners per grid. The script is therefore loaded once for
 * the whole file; every test rebuilds the grid and boots the engine again
 * through the events the dashboard sends.
 */
const PAGE_MARKUP = `
  <div aria-live="polite" class="visually-hidden" id="dashboard-status"></div>
  <button type="button" id="dashboard-reset-layout" disabled>Reset</button>
  <div class="bento" id="dashboard">
    <div class="bento__cell bento__cell--3" data-tile="hero">
      <div class="kpi"><div class="kpi__label">Active Products</div></div>
    </div>
    <div class="bento__cell bento__cell--2" data-tile="consumed">
      <div class="kpi"><div class="kpi__label">Total Consumed</div></div>
    </div>
    <div class="bento__cell bento__cell--6" data-tile="expiring-list">
      <div class="card"><div class="card-header">Expiring</div></div>
    </div>
  </div>
`;

const STORED_LAYOUT = {
  version: 1,
  tiles: {
    hero: { order: 1, w: 6, h: 3 },
    consumed: { order: 0, w: 2, h: 1 },
    "expiring-list": { order: 2, w: 6, h: 2 },
  },
};

let realm;
let getDashboardLayout;
let saveDashboardLayout;
let phoneLayout = false;

/**
 * jsdom has no matchMedia. The stub answers the phone breakpoint only while a
 * test asks for it, so the engine can be checked at both grid widths.
 */
function matchMediaStub(query) {
  return {
    matches: phoneLayout && query.indexOf("575.98px") !== -1,
    media: query,
    addEventListener() {},
    removeEventListener() {},
    addListener() {},
    removeListener() {},
  };
}

beforeAll(() => {
  getDashboardLayout = vi.fn();
  saveDashboardLayout = vi.fn().mockResolvedValue({ code: 200, message: "Dashboard layout updated" });
  const loaded = loadClassicScript("dashboardLayout.js", [], {
    prelude: `document.body.innerHTML = ${JSON.stringify(PAGE_MARKUP)};`,
    globals: {
      matchMedia: matchMediaStub,
      proviant: {
        getDashboardLayout,
        saveDashboardLayout,
        showFeedback: vi.fn(),
      },
    },
  });
  realm = loaded.realm;
});

beforeEach(() => {
  vi.useFakeTimers();
  phoneLayout = false;
  realm.document.body.innerHTML = PAGE_MARKUP;
  realm.localStorage.clear();
  getDashboardLayout.mockReset();
  getDashboardLayout.mockResolvedValue({ code: 200, message: { version: 1, tiles: {} } });
  saveDashboardLayout.mockClear();
});

/** Boots the engine the way the dashboard does: the layout arrives, then the
 * tiles announce that they stand in the grid. */
async function boot() {
  realm.document.dispatchEvent(new realm.Event("DOMContentLoaded"));
  realm.document.dispatchEvent(new realm.Event("proviant:dashboardready"));
  await vi.advanceTimersByTimeAsync(0);
}

const tile = (id) => realm.document.querySelector(`[data-tile="${id}"]`);
const span = (id, name) => tile(id).style.getPropertyValue(name);

describe("dashboardLayout.js arrangement", () => {
  it("gives every tile its position, width and height", async () => {
    await boot();

    expect(span("hero", "--tile-order")).toBe("0");
    expect(span("hero", "--tile-w")).toBe("3");
    expect(span("hero", "--tile-h")).toBe("1");
    expect(span("consumed", "--tile-order")).toBe("1");
    expect(span("consumed", "--tile-w")).toBe("2");
    expect(span("expiring-list", "--tile-w")).toBe("6");
    expect(tile("hero").dataset.tileSpan).toBe("true");
  });

  it("applies the stored arrangement", async () => {
    getDashboardLayout.mockResolvedValue({ code: 200, message: STORED_LAYOUT });

    await boot();

    expect(span("consumed", "--tile-order")).toBe("0");
    expect(span("hero", "--tile-order")).toBe("1");
    expect(span("hero", "--tile-w")).toBe("6");
    expect(span("hero", "--tile-h")).toBe("3");
    expect(span("expiring-list", "--tile-h")).toBe("2");
  });

  it("ignores a stored tile the dashboard does not know", async () => {
    getDashboardLayout.mockResolvedValue({
      code: 200,
      message: {
        version: 1,
        tiles: {
          ...STORED_LAYOUT.tiles,
          "a-tile-that-is-gone": { order: 0, w: 1, h: 1 },
        },
      },
    });

    await boot();

    expect(span("consumed", "--tile-order")).toBe("0");
    expect(span("hero", "--tile-order")).toBe("1");
    expect(realm.document.querySelectorAll("[data-tile]").length).toBe(3);
  });

  it("scales the width for the narrower grids", async () => {
    await boot();
    phoneLayout = true;

    realm.dispatchEvent(new realm.Event("resize"));

    expect(span("hero", "--tile-w")).toBe("2");
    expect(span("consumed", "--tile-w")).toBe("2");
    expect(span("expiring-list", "--tile-w")).toBe("2");
  });
});

describe("dashboardLayout.js arranging", () => {
  it("moves a tile with the arrow keys and stores the arrangement", async () => {
    await boot();

    const handle = tile("hero").querySelector(".bento__handle--move");
    handle.dispatchEvent(new realm.KeyboardEvent("keydown", { key: "ArrowRight", bubbles: true }));
    await vi.advanceTimersByTimeAsync(400);

    expect(span("hero", "--tile-order")).toBe("1");
    expect(span("consumed", "--tile-order")).toBe("0");
    expect(span("expiring-list", "--tile-order")).toBe("2");
    expect(saveDashboardLayout).toHaveBeenCalledTimes(1);
    expect(saveDashboardLayout.mock.calls[0][0].tiles.hero).toEqual({ order: 1, w: 3, h: 1 });
    expect(realm.document.getElementById("dashboard-status").textContent).not.toBe("");
  });

  it("resizes a tile with the arrow keys", async () => {
    await boot();

    const handle = tile("hero").querySelector(".bento__handle--resize");
    handle.dispatchEvent(new realm.KeyboardEvent("keydown", { key: "ArrowRight", bubbles: true }));
    handle.dispatchEvent(new realm.KeyboardEvent("keydown", { key: "ArrowDown", bubbles: true }));
    await vi.advanceTimersByTimeAsync(400);

    expect(span("hero", "--tile-w")).toBe("4");
    expect(span("hero", "--tile-h")).toBe("2");
    expect(saveDashboardLayout.mock.calls[0][0].tiles.hero).toEqual({ order: 0, w: 4, h: 2 });
  });

  it("puts the dashboard back to its default arrangement", async () => {
    getDashboardLayout.mockResolvedValue({ code: 200, message: STORED_LAYOUT });
    await boot();

    const reset = realm.document.getElementById("dashboard-reset-layout");
    expect(reset.disabled).toBe(false);
    reset.dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    await vi.advanceTimersByTimeAsync(400);

    expect(span("hero", "--tile-order")).toBe("0");
    expect(span("hero", "--tile-w")).toBe("3");
    expect(reset.disabled).toBe(true);
    expect(saveDashboardLayout).toHaveBeenCalledWith({ version: 1, tiles: {} });
  });

  it("keeps the reset out of the way while the dashboard is untouched", async () => {
    await boot();

    expect(realm.document.getElementById("dashboard-reset-layout").disabled).toBe(true);
    expect(saveDashboardLayout).not.toHaveBeenCalled();
  });
});
