import { describe, expect, it, vi } from "vitest";
import { loadClassicScript } from "./helpers/loadScript.js";

/**
 * Every load of savedRecipes.js registers another document-level click
 * listener, so a second load in the same realm would fire every handler twice
 * per click. Keeping the parser test alone in its file removes that coupling:
 * this is the only load, so exactly one listener exists.
 */

// The wall-clock budget for a regex guard. A super-linear pattern needs
// minutes on the pathological inputs below, so the bound is loose enough not
// to flake on a loaded runner and tight enough to catch the regression.
const WALL_CLOCK_BUDGET_MS = 2000;
const { exports: { parseSuggestionIngredient } } = loadClassicScript("savedRecipes.js", ["parseSuggestionIngredient"], {
  globals: {
    proviant: {
      getProducts: vi.fn().mockResolvedValue({ code: 200, products: [] }),
      getRecipes: vi.fn().mockResolvedValue({ code: 200, recipes: [] }),
      showFeedback: vi.fn(),
    },
  },
});

describe("savedRecipes.js ingredient parser", () => {
  it.each([
    ["200 g flour", { name: "flour", portion: "200 g" }],
    ["2 eggs", { name: "eggs", portion: "2" }],
    ["salt", { name: "salt", portion: "" }],
    ["1,5 kg sugar", { name: "sugar", portion: "1,5 kg" }],
    ["200 g of flour", { name: "flour", portion: "200 g" }],
    ["200 h flour", { name: "h flour", portion: "200" }],
  ])("splits %s", (raw, expected) => {
    expect(parseSuggestionIngredient(raw)).toEqual({ ...expected, productId: null });
  });

  it("copes with a missing ingredient", () => {
    expect(parseSuggestionIngredient(undefined)).toEqual({ name: "", portion: "", productId: null });
  });

  // The two bounds guard the two regexes this parser wears against a rewrite
  // into a super-linear shape: a long digit run and a long letter run are the
  // inputs such a rewrite backtracks on. The shape the sweep removed only
  // blows up on a trailing line terminator, which trim() takes away.
  //
  // The budget is generous on purpose: a super-linear regex needs minutes on
  // these inputs, so the guard does not have to be tight enough to flake on a
  // loaded runner.
  it("answers a pathological amount in linear time", () => {
    const digits = "1".repeat(100_000);
    const started = performance.now();
    const parsed = parseSuggestionIngredient(digits);
    expect(performance.now() - started).toBeLessThan(WALL_CLOCK_BUDGET_MS);
    expect(parsed.name).toBe(digits);
  });

  it("answers a pathological name in linear time", () => {
    const name = "x".repeat(100_000);
    const input = "1 " + name;
    const started = performance.now();
    const parsed = parseSuggestionIngredient(input);
    expect(performance.now() - started).toBeLessThan(WALL_CLOCK_BUDGET_MS);
    expect(parsed.portion).toBe("1");
    expect(parsed.name).toBe(name);
  });
});
