import { describe, expect, it } from "vitest";
import { loadClassicScript } from "./helpers/loadScript.js";

/**
 * The chart theme is the one place where the design system's chart rules are
 * translated into Chart.js options. It is loaded as a classic script, so the
 * test reads the API back from the realm the script ran in.
 */
function loadChartTheme() {
  const { realm } = loadClassicScript("chartTheme.js", [], {
    prelude: "localStorage.clear(); document.body.innerHTML = '';",
  });
  return realm.proviantChart;
}

describe("chartTheme.js month labels", () => {
  it("shortens a month to its name and a two-digit year", () => {
    const chart = loadChartTheme();

    const label = chart.monthLabel("2026-10");

    expect(label).toMatch(/^[^\s]+ '26$/);
  });

  it("spells the month out for a tooltip", () => {
    const chart = loadChartTheme();

    const title = chart.monthTitle("2026-10");

    expect(title).toMatch(/2026$/);
  });

  it("passes a value that is not a month through unchanged", () => {
    const chart = loadChartTheme();

    expect(chart.monthLabel("not-a-month")).toBe("not-a-month");
    expect(chart.monthTitle("")).toBe("");
  });
});

describe("chartTheme.js colour helpers", () => {
  it("turns a hex colour into a translucent one", () => {
    const chart = loadChartTheme();

    expect(chart.withAlpha("#418069", 0.5)).toBe("rgba(65, 128, 105, 0.5)");
  });

  it("turns a short hex colour into a translucent one", () => {
    const chart = loadChartTheme();

    expect(chart.withAlpha("#abc", 0.25)).toBe("rgba(170, 187, 204, 0.25)");
  });

  it("replaces the alpha of an rgb colour", () => {
    const chart = loadChartTheme();

    expect(chart.withAlpha("rgb(65, 128, 105)", 0.25)).toBe("rgba(65, 128, 105, 0.25)");
  });

  it("keeps a colour it cannot read", () => {
    const chart = loadChartTheme();

    expect(chart.withAlpha("currentColor", 0.25)).toBe("currentColor");
  });
});

describe("chartTheme.js plugins", () => {
  it("registers the centre text and bar value plugins", () => {
    const chart = loadChartTheme();

    expect(chart.centerTextPlugin.id).toBe("proviantCenterText");
    expect(chart.barValuePlugin.id).toBe("proviantBarValues");
  });
});

describe("chartTheme.js donut legend", () => {
  it("carries the count and the share into the legend text", () => {
    const chart = loadChartTheme();

    const items = chart.shareLegend({
      data: { labels: ["Expired", "Fresh"], datasets: [{ data: [1, 29], backgroundColor: ["#A24A4A", "#418069"] }] },
      options: { plugins: { legend: { labels: { color: "#4A4A4A" } } } },
    });

    expect(items.map((item) => item.text)).toEqual(["Expired · 1 (3%)", "Fresh · 29 (97%)"]);
  });

  it("gives every item the legend's own font colour", () => {
    const chart = loadChartTheme();

    const items = chart.shareLegend({
      data: { labels: ["Expired"], datasets: [{ data: [1], backgroundColor: ["#A24A4A"] }] },
      options: { plugins: { legend: { labels: { color: "#A79E92" } } } },
    });

    expect(items[0].fontColor).toBe("#A79E92");
  });
});
