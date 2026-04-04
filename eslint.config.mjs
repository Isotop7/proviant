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
      },
    },
    rules: {
      "no-unused-vars": ["error", { "caughtErrorsIgnorePattern": "^_" }],
    },
  },
];
