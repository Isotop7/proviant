import js from "@eslint/js";
import globals from "globals";

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
    },
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
    },
  },
];
