import { beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { loadClassicScript } from "./helpers/loadScript.js";

/**
 * homeStats.js reads the stats of the household and builds the dashboard
 * tiles from them. The script registers its DOMContentLoaded listener once per
 * load, so the file loads it once and every test rebuilds the markup and boots
 * it again through the event the browser sends.
 */
const PAGE_MARKUP = `
  <div aria-live="polite" class="visually-hidden" id="dashboard-status"></div>
  <div class="bento" id="dashboard">
    <div class="bento__cell bento__cell--6" data-tile="expiring-list" id="dashboard-list"></div>
    <div class="bento__cell bento__cell--6" data-tile="use-first" id="dashboard-use-first"></div>
    <div class="bento__cell bento__cell--3" data-tile="chart-waste">
      <canvas id="chartWaste"></canvas>
    </div>
    <div class="bento__cell bento__cell--3" data-tile="chart-categories">
      <canvas id="chartCategories"></canvas>
    </div>
    <div class="bento__cell bento__cell--6" data-tile="chart-expiry">
      <canvas id="chartExpiryTrend"></canvas>
    </div>
  </div>
`;

const USE_FIRST = [
  {
    id: 1,
    productName: "Greek Yogurt",
    expireAt: "2099-01-02",
    quantity: 2,
    packSize: "250g",
    storageLocationName: "Fridge",
    storageLocationIcon: "🧊",
  },
  {
    id: 2,
    productName: "Flour",
    expireAt: "",
    quantity: 1,
    packSize: "",
    storageLocationName: "Cupboard",
    storageLocationIcon: "🥫",
  },
];

const STATS = {
  wasteCount: 0,
  wastePercent: 0,
  totalActive: 12,
  totalArchived: 0,
  uniqueArchived: 0,
  lastInsertedProduct: "",
  expiringSoon: [],
  expiringSoonDays: 7,
  useFirst: USE_FIRST,
  categories: {},
  expiryTrend: [],
};

/** Chart.js draws the charts of the dashboard; the tiles are what this file
 *  is about, so the charts get a stub that draws nothing. */
class ChartStub {
  destroy() {}
  static register() {}
}

const chartThemeStub = {
  applyTheme: () => ({
    statusExpired: "#8c1d18",
    statusFresh: "#1d5c3a",
    fg: "#22201d",
    muted: "#6f6f6f",
    fg2: "#4a4642",
    accent: "#418069",
    accentOn: "#ffffff",
    border: "#e2ded7",
  }),
  baseOptions: () => ({ plugins: { legend: { labels: {} } } }),
  areaGradient: () => "rgba(65, 128, 105, 0.28)",
  monthLabel: (month) => month,
  monthTitle: (month) => month,
  shareLegend: () => [],
  centerTextPlugin: {},
  barValuePlugin: {},
};

let realm;
let getProductStats;

beforeAll(() => {
  getProductStats = vi.fn().mockResolvedValue({ code: 200, message: STATS });

  const loaded = loadClassicScript("homeStats.js", [], {
    prelude: `document.body.innerHTML = ${JSON.stringify(PAGE_MARKUP)};`,
    globals: {
      Chart: ChartStub,
      proviantChart: chartThemeStub,
      proviant: {
        getProductStats,
        getStreak: vi.fn().mockResolvedValue({ code: 200, message: { currentStreak: 0, longestStreak: 0 } }),
        getSavingsStats: vi.fn().mockResolvedValue({ code: 200, message: null }),
        getHouseholdSettings: vi.fn().mockResolvedValue({ code: 200, message: {} }),
        getActivityFeed: vi.fn().mockResolvedValue({ code: 200, message: { activities: [] } }),
      },
    },
  });
  realm = loaded.realm;
});

beforeEach(() => {
  realm.document.body.innerHTML = PAGE_MARKUP;
  getProductStats.mockReset();
  getProductStats.mockResolvedValue({ code: 200, message: STATS });
});

/** Boots the dashboard the way the browser does. */
async function render() {
  realm.document.dispatchEvent(new realm.Event("DOMContentLoaded"));
  await new Promise((resolve) => setTimeout(resolve, 0));
}

const tile = () => realm.document.getElementById("dashboard-use-first");
const rows = () => Array.from(tile().querySelectorAll(".list-group-item"));

describe("homeStats.js use-first tile", () => {
  it("lists the products of the household", async () => {
    await render();

    const names = tile().querySelectorAll(".list-group-item__title");
    expect(names.length).toBe(2);
    expect(names[0].textContent).toBe("Greek Yogurt");
  });

  it("names the place, the pack count and the pack size of every product", async () => {
    await render();

    const lines = Array.from(tile().querySelectorAll(".list-group-item__meta"))
      .map((element) => element.textContent);
    expect(lines).toContain("Fridge · 2 packs · 250g");
    expect(lines).toContain("Cupboard · 1 pack");
  });

  it("links into the products list with the same order", async () => {
    await render();

    const header = tile().querySelector(".card-header");
    const links = Array.from(header.querySelectorAll("a"));
    expect(links.length).toBeGreaterThan(0);
    for (const link of links) {
      expect(link.getAttribute("href")).toBe("/web/products?sort=effective_expire_at");
    }
  });

  it("keeps the products without a date in their own group at the end", async () => {
    await render();

    const items = rows();
    const dividerIndex = items.findIndex((item) => item.querySelector(".list-group-item__meta")
      && !item.querySelector(".list-group-item__title"));
    expect(dividerIndex).toBe(1);
    expect(items[dividerIndex + 1].querySelector(".list-group-item__title").textContent).toBe("Flour");
    expect(items[dividerIndex + 1].querySelector(".expiry-text")).toBe(null);
    expect(items[dividerIndex + 1].querySelector(".list-group-item__stripe--neutral")).not.toBe(null);
  });

  it("leads to the product every row names", async () => {
    await render();

    const links = Array.from(tile().querySelectorAll(".list-group-item__link"));
    expect(links.length).toBe(2);
    expect(links[0].getAttribute("href")).toBe("/web/products/1/edit");
    expect(links[1].getAttribute("href")).toBe("/web/products/2/edit");
  });

  it("names the tile, not its content", async () => {
    await render();

    // The move and resize handles take their accessible name from the tile
    // label. The count in the header would end up in that name.
    expect(tile().dataset.tileLabel).toBe("Use me first");
    expect(realm.document.getElementById("dashboard-list").dataset.tileLabel).toBe("Expiring within next 7 Days");
  });

  it("escapes the name of a product", async () => {
    getProductStats.mockResolvedValue({
      code: 200,
      message: {
        ...STATS,
        useFirst: [{ id: 9, productName: "<script>x</script>", expireAt: "2099-01-02", quantity: 1 }],
      },
    });

    await render();

    expect(tile().querySelectorAll("script").length).toBe(0);
    expect(tile().querySelector(".list-group-item__title").textContent).toBe("<script>x</script>");
  });
});
