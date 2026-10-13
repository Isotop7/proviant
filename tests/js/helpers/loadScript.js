import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

// tests/js/helpers/ -> tests/js/ -> tests/ -> project root
const projectRoot = resolve(dirname(fileURLToPath(import.meta.url)), "..", "..", "..");
const assetDir = resolve(projectRoot, "src", "assets", "js");

const CARRIER = "__proviantSetup";
const REALM = "__proviantRealm";
const CAPTURE = "__proviantCaptured";

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

  if (Object.keys(globals).length > 0) {
    document[CARRIER] = globals;
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
    (Object.keys(globals).length > 0
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