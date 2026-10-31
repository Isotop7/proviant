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
  <div class="dashboard-tools">
    <button type="button" id="dashboard-edit-layout" disabled
            data-edit-label="Edit dashboard" data-done-label="Done">
      <span class="dashboard-tools__label">Edit dashboard</span>
    </button>
    <button type="button" id="dashboard-reset-layout" disabled hidden>Reset</button>
  </div>
  <div class="bento" id="dashboard">
    <div class="bento__cell bento__cell--3" data-tile="hero">
      <div class="kpi"><div class="kpi__label">Active Products</div></div>
    </div>
    <div class="bento__cell bento__cell--2" data-tile="consumed">
      <div class="kpi"><div class="kpi__label">Total Consumed</div></div>
    </div>
    <div class="bento__cell bento__cell--6" data-tile="expiring-list" data-tile-label="Expiring">
      <div class="card"><div class="card-header">Expiring</div></div>
    </div>
    <div class="bento__cell bento__cell--6" data-tile="use-first">
      <div class="card"><div class="card-header">Use me first</div></div>
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
const animations = [];

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

/**
 * jsdom has no layout: every box measures zero. The stub gives the grid and
 * its cells a size, so the engine can measure them. The cells stand where
 * their order puts them.
 */
function stubGeometry() {
  const grid = realm.document.getElementById("dashboard");
  Object.defineProperty(grid, "clientWidth", { value: 1200, configurable: true });
  Object.defineProperty(grid, "clientHeight", { value: 600, configurable: true });
  grid.getBoundingClientRect = () => ({
    left: 0, top: 0, right: 1200, bottom: 600, width: 1200, height: 600, x: 0, y: 0,
  });

  ["hero", "consumed", "expiring-list", "use-first"].forEach((id) => {
    const cell = tile(id);
    const order = () => Number(cell.style.getPropertyValue("--tile-order")) || 0;
    Object.defineProperty(cell, "offsetLeft", { get: () => order() * 200, configurable: true });
    Object.defineProperty(cell, "offsetTop", { get: () => 0, configurable: true });
    Object.defineProperty(cell, "offsetWidth", { get: () => 200, configurable: true });
    Object.defineProperty(cell, "offsetHeight", { get: () => 120, configurable: true });
    // The rendered box stands somewhere else: the engine must not measure it.
    cell.getBoundingClientRect = () => ({
      left: 999, top: 999, right: 1199, bottom: 1119, width: 200, height: 120, x: 999, y: 999,
    });
  });
}

/**
 * jsdom has no Web Animations. The stub records the animations the engine
 * starts, so a test can see which cells are animated and which are not.
 */
function stubAnimations() {
  realm.Element.prototype.animate = function (keyframes, options) {
    const animation = { element: this, keyframes, options, cancelled: false };
    animation.cancel = () => { animation.cancelled = true; };
    animations.push(animation);
    return animation;
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
  stubAnimations();
});

beforeEach(() => {
  vi.useFakeTimers();
  phoneLayout = false;
  realm.document.body.innerHTML = PAGE_MARKUP;
  realm.localStorage.clear();
  realm.document.documentElement.style.removeProperty("--motion-base");
  animations.length = 0;
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

/** Switches the dashboard into the edit mode, where the grips live. */
function enterEditMode() {
  realm.document.getElementById("dashboard-edit-layout")
    .dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
}

const tile = (id) => realm.document.querySelector(`[data-tile="${id}"]`);
const span = (id, name) => tile(id).style.getPropertyValue(name);
const handles = (id) => tile(id).querySelectorAll(".bento__handle");

describe("dashboardLayout.js arrangement", () => {
  it("gives every tile its position, width and height", async () => {
    await boot();

    expect(span("hero", "--tile-order")).toBe("0");
    expect(span("hero", "--tile-w")).toBe("3");
    expect(span("hero", "--tile-h")).toBe("1");
    expect(span("consumed", "--tile-order")).toBe("1");
    expect(span("consumed", "--tile-w")).toBe("2");
    expect(span("expiring-list", "--tile-w")).toBe("6");
    expect(span("use-first", "--tile-w")).toBe("6");
    expect(span("use-first", "--tile-h")).toBe("2");
    expect(tile("hero").dataset.tileSpan).toBe("true");
  });

  it("gives a tile the dashboard did not know yet its default place", async () => {
    await boot();

    // The use-first tile joined the dashboard after the stored arrangement was
    // written: it lands on its default place and the other tiles keep theirs.
    expect(span("use-first", "--tile-order")).toBe("2");
    expect(span("hero", "--tile-order")).toBe("0");
    expect(span("consumed", "--tile-order")).toBe("1");
    expect(span("expiring-list", "--tile-order")).toBe("3");
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
    expect(realm.document.querySelectorAll("[data-tile]").length).toBe(4);
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

describe("dashboardLayout.js edit mode", () => {
  it("keeps the grips and the reset out of the way", async () => {
    await boot();

    expect(realm.document.getElementById("dashboard-edit-layout").disabled).toBe(false);
    expect(realm.document.getElementById("dashboard-reset-layout").hidden).toBe(true);
    expect(realm.document.getElementById("dashboard").classList.contains("bento--editing")).toBe(false);
    handles("hero").forEach((handle) => expect(handle.hidden).toBe(true));
  });

  it("names the grips after the tile, not after its content", async () => {
    await boot();

    // The tile label is what the grip announces. The content of the tile — a
    // count, a date — would make the name of the grip change with it.
    const move = tile("expiring-list").querySelector(".bento__handle--move");
    expect(move.getAttribute("aria-label")).toBe("Move Expiring");
  });

  it("shows the grips and the reset while the dashboard is edited", async () => {
    await boot();

    enterEditMode();

    expect(realm.document.getElementById("dashboard").classList.contains("bento--editing")).toBe(true);
    expect(realm.document.getElementById("dashboard-reset-layout").hidden).toBe(false);
    expect(realm.document.querySelector(".dashboard-tools__label").textContent).toBe("Done");
    handles("hero").forEach((handle) => expect(handle.hidden).toBe(false));
  });

  it("hides the grips and the reset again", async () => {
    await boot();

    enterEditMode();
    enterEditMode();

    expect(realm.document.getElementById("dashboard").classList.contains("bento--editing")).toBe(false);
    expect(realm.document.getElementById("dashboard-reset-layout").hidden).toBe(true);
    expect(realm.document.querySelector(".dashboard-tools__label").textContent).toBe("Edit dashboard");
    handles("hero").forEach((handle) => expect(handle.hidden).toBe(true));
  });

  it("keeps the resize grip on the desktop only", async () => {
    await boot();
    enterEditMode();

    const move = tile("hero").querySelector(".bento__handle--move");
    const resize = tile("hero").querySelector(".bento__handle--resize");
    expect(move.hidden).toBe(false);
    expect(resize.hidden).toBe(false);

    phoneLayout = true;
    realm.dispatchEvent(new realm.Event("resize"));

    expect(move.hidden).toBe(false);
    expect(resize.hidden).toBe(true);
  });
});

describe("dashboardLayout.js arranging", () => {
  it("moves a tile with the arrow keys and stores the arrangement", async () => {
    await boot();
    enterEditMode();

    const handle = tile("hero").querySelector(".bento__handle--move");
    handle.dispatchEvent(new realm.KeyboardEvent("keydown", { key: "ArrowRight", bubbles: true }));
    await vi.advanceTimersByTimeAsync(400);

    expect(span("hero", "--tile-order")).toBe("1");
    expect(span("consumed", "--tile-order")).toBe("0");
    expect(span("expiring-list", "--tile-order")).toBe("3");
    expect(saveDashboardLayout).toHaveBeenCalledTimes(1);
    expect(saveDashboardLayout.mock.calls[0][0].tiles.hero).toEqual({ order: 1, w: 3, h: 1 });
    expect(realm.document.getElementById("dashboard-status").textContent).not.toBe("");
  });

  it("resizes a tile with the arrow keys", async () => {
    await boot();
    enterEditMode();

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
    enterEditMode();

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
    expect(realm.document.getElementById("dashboard-reset-layout").hidden).toBe(true);
    expect(saveDashboardLayout).not.toHaveBeenCalled();
  });
});

describe("dashboardLayout.js pointer", () => {
  /** jsdom has no PointerEvent: the engine only reads the coordinates and the
   * pointer id, so a mouse event of the same type does the job. */
  function pointer(type, x = 0, y = 0) {
    return new realm.MouseEvent(type, { bubbles: true, clientX: x, clientY: y });
  }

  it("marks the place a dragged tile will land in", async () => {
    await boot();
    enterEditMode();
    stubGeometry();

    const handle = tile("hero").querySelector(".bento__handle--move");
    handle.dispatchEvent(pointer("pointerdown", 10, 10));

    const slot = realm.document.querySelector(".bento__slot");
    expect(slot).not.toBe(null);
    expect(slot.classList.contains("bento__slot--visible")).toBe(true);
    expect(tile("hero").classList.contains("bento__cell--dragging")).toBe(true);

    realm.document.dispatchEvent(pointer("pointermove", 120, 40));
    realm.document.dispatchEvent(pointer("pointerup", 120, 40));

    expect(tile("hero").classList.contains("bento__cell--dragging")).toBe(false);
    expect(tile("hero").classList.contains("bento__cell--landed")).toBe(true);

    await vi.advanceTimersByTimeAsync(400);
    expect(realm.document.querySelector(".bento__slot")).toBe(null);
  });

  it("puts the landing shadow on the place the tile will take", async () => {
    await boot();
    enterEditMode();
    stubGeometry();

    const handle = tile("hero").querySelector(".bento__handle--move");
    handle.dispatchEvent(pointer("pointerdown", 10, 10));

    const slot = realm.document.querySelector(".bento__slot");
    expect(slot.style.left).toBe("0px");
    expect(slot.style.top).toBe("0px");
    expect(slot.style.width).toBe("200px");
    expect(slot.style.height).toBe("120px");

    realm.document.dispatchEvent(pointer("pointermove", 250, 30));

    expect(slot.style.left).toBe("200px");
  });

  it("reorders while the pointer stays at the top of the tile", async () => {
    await boot();
    enterEditMode();
    stubGeometry();

    const handle = tile("hero").querySelector(".bento__handle--move");
    handle.dispatchEvent(pointer("pointerdown", 10, 10));
    // The grip sits at the top of the tile, so the pointer never enters the
    // row of the tiles: the tile itself decides where it lands.
    realm.document.dispatchEvent(pointer("pointermove", 250, 10));

    expect(span("hero", "--tile-order")).toBe("1");
  });

  it("does not animate the tile that is dragged", async () => {
    await boot();
    enterEditMode();
    stubGeometry();
    realm.document.documentElement.style.setProperty("--motion-base", "180ms");

    const handle = tile("hero").querySelector(".bento__handle--move");
    handle.dispatchEvent(pointer("pointerdown", 10, 10));
    realm.document.dispatchEvent(pointer("pointermove", 250, 30));

    expect(span("hero", "--tile-order")).toBe("1");
    const animated = animations.map((animation) => animation.element);
    expect(animated).not.toContain(tile("hero"));
    expect(animated).toContain(tile("consumed"));
  });

  it("measures the layout position and not the rendered one", async () => {
    await boot();
    enterEditMode();
    stubGeometry();
    realm.document.documentElement.style.setProperty("--motion-base", "180ms");

    const handle = tile("hero").querySelector(".bento__handle--move");
    handle.dispatchEvent(pointer("pointerdown", 10, 10));
    realm.document.dispatchEvent(pointer("pointermove", 250, 30));

    expect(animations).toHaveLength(1);
    expect(animations[0].element).toBe(tile("consumed"));
    expect(animations[0].keyframes[0].transform).toBe("translate(200px, 0px)");
  });

  it("cancels the reflow that is still running", async () => {
    await boot();
    enterEditMode();
    stubGeometry();
    realm.document.documentElement.style.setProperty("--motion-base", "180ms");

    const handle = tile("hero").querySelector(".bento__handle--move");
    handle.dispatchEvent(pointer("pointerdown", 10, 10));
    realm.document.dispatchEvent(pointer("pointermove", 250, 30));
    realm.document.dispatchEvent(pointer("pointermove", 450, 30));

    expect(animations.length).toBeGreaterThan(1);
    expect(animations[0].cancelled).toBe(true);
  });

  it("keeps the dragged tile inside the grid", async () => {
    await boot();
    enterEditMode();
    stubGeometry();

    const handle = tile("hero").querySelector(".bento__handle--move");
    handle.dispatchEvent(pointer("pointerdown", 10, 10));
    realm.document.dispatchEvent(pointer("pointermove", 5000, 5000));

    const match = /translate\((-?[\d.]+)px, (-?[\d.]+)px\)/.exec(tile("hero").style.transform);
    const layoutLeft = Number(tile("hero").style.getPropertyValue("--tile-order")) * 200;
    const right = layoutLeft + Number(match[1]) + 200;

    expect(right).toBeLessThanOrEqual(1200);
  });

  it("snaps into place without motion", async () => {
    await boot();
    enterEditMode();
    stubGeometry();
    realm.document.documentElement.style.setProperty("--motion-base", "0ms");

    const handle = tile("hero").querySelector(".bento__handle--move");
    handle.dispatchEvent(pointer("pointerdown", 10, 10));
    realm.document.dispatchEvent(pointer("pointermove", 250, 30));

    expect(span("hero", "--tile-order")).toBe("1");
    expect(animations).toHaveLength(0);
  });

  it("shows the size a resized tile will have", async () => {
    await boot();
    enterEditMode();

    const handle = tile("hero").querySelector(".bento__handle--resize");
    handle.dispatchEvent(pointer("pointerdown", 10, 10));

    expect(tile("hero").classList.contains("bento__cell--resizing")).toBe(true);

    realm.document.dispatchEvent(pointer("pointerup", 10, 10));

    expect(tile("hero").classList.contains("bento__cell--resizing")).toBe(false);
    expect(tile("hero").classList.contains("bento__cell--landed")).toBe(true);
  });
});
