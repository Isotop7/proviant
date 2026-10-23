import js from "@eslint/js";
import globals from "globals";
import { superLinearRegex } from "./scripts/eslint-rules/super-linear-regex.mjs";

const localPlugin = {
    rules: {
        "super-linear-regex": superLinearRegex,
    },
};

export default [
  {
    ignores: [
      "node_modules/",
      "src/assets/js/bootstrap.bundle.min.js",
      "src/assets/js/bootstrap.bundle.min.js.map",
      "src/assets/js/chart.umd.min.js",
      "src/assets/js/html5-qrcode.min.js",
    ],
  },
  {
    ...js.configs.recommended,
    files: ["src/assets/js/*.js"],
    languageOptions: {
      ecmaVersion: 2022,
      sourceType: "script",
      globals: {
        ...globals.browser,
        ...globals.serviceworker,
        bootstrap: "readonly",
        Chart: "readonly",
        Html5Qrcode: "readonly",
        Html5QrcodeSupportedFormats: "readonly",
        Html5QrcodeScannerState: "readonly",
        proviant: "readonly",
        proviantChart: "readonly",
        // Top-level function declarations in classic scripts become window
        // properties, so these are genuine cross-file globals at runtime:
        // clearBulkSelectionAndReload lives in proviant.js but is called from
        // products.js, and updateBulkSelected is the other way around.
        clearBulkSelectionAndReload: "readonly",
        updateBulkSelected: "readonly",
      },
    },
    // rules must re-spread the recommended set: a bare `rules:` key after the
    // config spread *replaces* its rules wholesale, silently dropping no-undef
    // and friends while leaving the block looking like it lints recommended.
    rules: {
      ...js.configs.recommended.rules,
      "no-unused-vars": ["error", { "caughtErrorsIgnorePattern": "^_" }],
      "local/super-linear-regex": "error",
    },
    plugins: { local: localPlugin },
  },
  {
    // The one ES module in src/assets/js: it re-exports the Bootstrap 6 bundle
    // as the global the classic scripts expect. Bootstrap 6 ships ESM only,
    // so this file has to be a module too.
    ...js.configs.recommended,
    files: ["src/assets/js/bootstrap-globals.js"],
    languageOptions: {
      ecmaVersion: 2022,
      sourceType: "module",
      globals: {
        ...globals.browser,
      },
    },
    rules: {
      ...js.configs.recommended.rules,
      "no-unused-vars": ["error", { "caughtErrorsIgnorePattern": "^_" }],
      "local/super-linear-regex": "error",
    },
    plugins: { local: localPlugin },
  },
  {
    // Each file that *defines* one of the cross-file globals needs it off for
    // itself, or the definition trips no-redeclare — same reason as below.
    files: ["src/assets/js/proviant.js"],
    languageOptions: {
      globals: { proviant: "off", clearBulkSelectionAndReload: "off" },
    },
  },
  {
    files: ["src/assets/js/products.js"],
    languageOptions: {
      globals: { updateBulkSelected: "off" },
    },
  },
  {
    // Build-time tooling lives in scripts/ and runs under Node as ES modules.
    // It never ships to the browser, so it gets its own globals and its own
    // source type.
    ...js.configs.recommended,
    files: ["scripts/**/*.mjs"],
    languageOptions: {
      ecmaVersion: 2022,
      sourceType: "module",
      globals: {
        ...globals.node,
      },
    },
    rules: {
      ...js.configs.recommended.rules,
      "no-unused-vars": ["error", { "caughtErrorsIgnorePattern": "^_" }],
      "local/super-linear-regex": "error",
    },
    plugins: { local: localPlugin },
  },
  {
    // Vitest specs live outside src/ and reach into jsdom globals that the
    // shipped code never touches. describe/it/expect come in via explicit
    // imports, so nothing has to be declared as an ambient global here.
    ...js.configs.recommended,
    files: ["tests/js/**/*.js"],
    languageOptions: {
      ecmaVersion: 2022,
      sourceType: "module",
      globals: {
        ...globals.browser,
        ...globals.node,
      },
    },
    rules: {
      ...js.configs.recommended.rules,
      "no-unused-vars": ["error", { "caughtErrorsIgnorePattern": "^_" }],
      "local/super-linear-regex": "error",
    },
    plugins: { local: localPlugin },
  },
];
