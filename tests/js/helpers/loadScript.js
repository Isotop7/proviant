import { readFileSync, readdirSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

// tests/js/helpers/ -> tests/js/ -> tests/ -> project root
const projectRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..", "..");
const assetDir = resolve(projectRoot, "src", "assets", "js");
const catalogDir = resolve(projectRoot, "src", "i18n", "catalogs", "en");

// The server injects window.proviantI18n per page (see i18n.FuncMap); scripts
// under test read it through proviant.t. The merged English catalog stands in
// for that injection so tests exercise the real lookup path.
function loadMessageCatalog() {
  return readdirSync(catalogDir)
    .filter((name) => name.endsWith(".json"))
    .reduce((catalog, name) => {
      Object.assign(catalog, JSON.parse(readFileSync(resolve(catalogDir, name), "utf8")));
      return catalog;
    }, {});
}

// Mirrors proviant.t / proviant.tn from src/assets/js/proviant.js so partial
// proviant mocks in click tests resolve keys exactly like the shipped script.
function createTranslator(catalog) {
  const format = (text, args) =>
    args.reduce((out, arg) => out.replace(/%[sdv]/, String(arg)), text);
  const t = (key, ...args) =>
    format(Object.prototype.hasOwnProperty.call(catalog, key) ? catalog[key] : key, args);
  const tn = (key, n, ...args) => {
    const suffixed = key + (n === 1 ? "_one" : "_other");
    return t(Object.prototype.hasOwnProperty.call(catalog, suffixed) ? suffixed : key, n, ...args);
  };
  return { t, tn };
}

const CARRIER = "__proviantSetup";
const REALM = "__proviantRealm";
const CAPTURE = "__proviantCaptured";

// Mirrors the window.proviantUnits / window.proviantUnitTable /
// window.proviantUnitSystem injection in base.tmpl (util.UnitOptions,
// util.UnitConversionJSON and util.ResolveUnitSystem). Scripts read the unit
// list and factors from there, so the tests must supply them too.
const UNIT_TABLE = [
  { symbol: "g", kind: "mass", factor: 1 },
  { symbol: "kg", kind: "mass", factor: 1000 },
  { symbol: "oz", kind: "mass", factor: 28.349523125 },
  { symbol: "lb", kind: "mass", factor: 453.59237 },
  { symbol: "ml", kind: "volume", factor: 1 },
  { symbol: "l", kind: "volume", factor: 1000 },
  { symbol: "tsp", kind: "volume", factor: 5 },
  { symbol: "tbsp", kind: "volume", factor: 15 },
  { symbol: "cup", kind: "volume", factor: 240 },
  { symbol: "fl oz", kind: "volume", factor: 29.5735295625 },
  { symbol: "pcs", kind: "count", factor: 1 },
  { symbol: "pkg", kind: "count", factor: 1 },
  { symbol: "can", kind: "count", factor: 1 },
  { symbol: "box", kind: "count", factor: 1 },
  { symbol: "jar", kind: "count", factor: 1 },
  { symbol: "bottle", kind: "count", factor: 1 },
  { symbol: "bunch", kind: "count", factor: 1 },
  { symbol: "slice", kind: "count", factor: 1 },
];
const UNIT_SYMBOLS = UNIT_TABLE.map((unit) => unit.symbol);

// Mirrors util.unitAliases (rendered by util.UnitAliasJSON). The frontend
// resolves a stored spelling through it before looking the unit up, so the
// tests need the same table to exercise that resolution.
const UNIT_ALIASES = {
  g: "g", gr: "g", gram: "g", grams: "g", gramm: "g",
  kg: "kg", kilo: "kg", kilos: "kg", kilogram: "kg", kilograms: "kg",
  oz: "oz", ounce: "oz", ounces: "oz",
  lb: "lb", lbs: "lb", pound: "lb", pounds: "lb",
  ml: "ml", milliliter: "ml", milliliters: "ml", millilitre: "ml", millilitres: "ml",
  l: "l", liter: "l", liters: "l", litre: "l", litres: "l",
  tsp: "tsp", teaspoon: "tsp", teaspoons: "tsp",
  tbsp: "tbsp", tbs: "tbsp", tablespoon: "tbsp", tablespoons: "tbsp",
  cup: "cup", cups: "cup",
  "fl oz": "fl oz", floz: "fl oz", "fl. oz": "fl oz", "fluid ounce": "fl oz", "fluid ounces": "fl oz",
  pcs: "pcs", pc: "pcs", piece: "pcs", pieces: "pcs",
  pkg: "pkg", package: "pkg", packages: "pkg", pack: "pkg", packs: "pkg",
  can: "can", cans: "can",
  box: "box", boxes: "box",
  jar: "jar", jars: "jar",
  bottle: "bottle", bottles: "bottle",
  bunch: "bunch", bunches: "bunch",
  slice: "slice", slices: "slice",
};

/**
 * Load a shipped frontend script and capture the identifiers it declares.
 *
 * The frontend is served as classic `<script src>` bundles, not ES modules —
 * there is not a single `export` statement in src/assets/js. So a test cannot
 * `import { formatDate } from "../../src/assets/js/proviant.js"`. Instead the
 * file is evaluated the way a browser evaluates it and the identifiers it
 * leaves behind are read back.
 *
 * Three details make this work, each verified against jsdom rather than assumed:
 *
 * 1. The capture expression is *appended to the source*. A classic script's
 *    top-level `const proviant = {}` is a global lexical binding that is
 *    deliberately not a property of the global object, so reading
 *    `globalThis.proviant` yields undefined. Appending keeps the reference
 *    inside the lexical scope that declared it.
 *
 * 2. The source is wrapped in a function so repeated loads do not raise
 *    "Identifier 'proviant' has already been declared". Global lexical
 *    bindings live for the lifetime of the realm, so a bare re-evaluation of
 *    a file declaring `const proviant` is a SyntaxError. Function scope lets
 *    every test load a pristine copy.
 *
 * 3. Everything is exchanged through `document`. vitest's jsdom environment
 *    exposes the DOM on the test's global object, but an injected script runs
 *    inside jsdom's own realm, whose `window` is a *different* object from the
 *    test's `globalThis`. Assignments to `window` from inside the script are
 *    therefore invisible to the test. `document` is the one reference both
 *    sides genuinely share. For the same reason `realm` is returned: it is the
 *    script's real `window`, and the only handle a test has for that realm's
 *    `localStorage`, `fetch` and `document.cookie`.
 *
 * @param {string} fileName              e.g. "proviant.js"
 * @param {string[]} exportNames         top-level identifiers to hand back
 * @param {object}  [options]
 * @param {string}  [options.prelude]    extra source evaluated inside the realm before the script
 * @param {object}  [options.globals]    values assigned onto the realm's window before the script runs
 * @returns {{ exports: Record<string, unknown>, realm: Window }}
 */
export function loadClassicScript(fileName, exportNames, options = {}) {
  const { prelude = "", globals = {} } = options;
  const catalog = loadMessageCatalog();
  const environment = {
    proviantI18n: catalog,
    proviantLang: "en",
    proviantUnitSystem: "metric",
    proviantUnits: UNIT_SYMBOLS,
    proviantUnitTable: UNIT_TABLE,
    proviantUnitAliases: UNIT_ALIASES,
    ...globals,
  };
  if (environment.proviant && typeof environment.proviant === "object") {
    environment.proviant = { ...createTranslator(catalog), ...environment.proviant };
  }

  // These names are interpolated into generated source below. A malformed name
  // would produce broken or injected code, and the failure would surface as a
  // baffling jsdom parse error rather than as the typo it is.
  const malformed = exportNames.filter((name) => !/^[A-Za-z_$][A-Za-z0-9_$]*$/.test(name));
  if (malformed.length > 0) {
    throw new Error(`[${malformed.join(", ")}] are not valid JavaScript identifiers.`);
  }
  if (!document.head) {
    throw new Error(`${fileName} could not be loaded: the document has no <head> to inject into.`);
  }

  const source = readFileSync(resolve(assetDir, fileName), "utf8");

  if (Object.keys(environment).length > 0) {
    document[CARRIER] = environment;
  }

  // `typeof x` is the only reference to an identifier that stays quiet when the
  // identifier is undeclared. Without the guard a typo'd name raises a
  // ReferenceError inside the injected script, which jsdom reports as an
  // uncaught exception on the global rather than as a value this function can
  // inspect — the caller would lose the actionable message below.
  const pairs = exportNames
    .map((name) => `${name}: typeof ${name} === "undefined" ? undefined : ${name}`)
    .join(", ");

  // Leading newlines keep a trailing `//` comment from swallowing the prelude.
  const wrapped =
    `(function () {\n` +
    `document[${JSON.stringify(REALM)}] = window;\n` +
    // defineProperty rather than Object.assign: jsdom exposes globals such as
    // `navigator` as getter-only accessors, and a plain assignment to one of
    // those is either ignored or throws.
    (Object.keys(environment).length > 0
      ? `Object.keys(document[${JSON.stringify(CARRIER)}]).forEach(function (k) { ` +
        `Object.defineProperty(window, k, { ` +
        `value: document[${JSON.stringify(CARRIER)}][k], writable: true, configurable: true }); });\n`
      : "") +
    `${prelude}\n` +
    `${source}\n` +
    `document[${JSON.stringify(CAPTURE)}] = { ${pairs} };\n` +
    `})();`;

  const element = document.createElement("script");
  element.textContent = wrapped;
  document.head.appendChild(element);
  // Execution is synchronous on insertion, so the node has served its purpose.
  // Dropping it keeps a long test file from retaining every script it ran.
  element.remove();

  const exports_ = document[CAPTURE];
  const realm = document[REALM];

  delete document[CAPTURE];
  delete document[REALM];
  delete document[CARRIER];

  if (exports_ === undefined) {
    throw new Error(
      `${fileName} produced no capture object, so the script never reached its end. ` +
        "It most likely threw while loading; check for a global it needs stubbed.",
    );
  }

  const missing = exportNames.filter((name) => exports_[name] === undefined);
  if (missing.length > 0) {
    throw new Error(
      `${fileName} did not define [${missing.join(", ")}] at top level. ` +
        "An IIFE keeps its internals private — such a file needs an explicit export seam.",
    );
  }

  return { exports: exports_, realm };
}

/**
 * jsdom implements no `matchMedia`, and theme.js calls it during load to read
 * the system colour scheme. Pass the result as `globals.matchMedia`.
 *
 * @param {boolean} prefersDark value the stubbed dark-mode query reports
 */
export function createMatchMedia(prefersDark = false) {
  const mediaQueryList = {
    matches: prefersDark,
    media: "(prefers-color-scheme: dark)",
    addEventListener() {},
    removeEventListener() {},
    addListener() {},
    removeListener() {},
  };
  return () => mediaQueryList;
}