import { defineConfig } from "vitest/config";

// No coverage block on purpose. The suite evaluates src/assets/js by injecting
// it into jsdom's realm, which runs in a separate V8 context that the inspector
// never sees — every file reports 0%. The provider works (a normally imported
// module measures fine); the harness is what defeats it. A metric that is
// permanently zero is worse than no metric, so it is omitted until the loader
// can attribute its script to a filename.

export default defineConfig({
  test: {
    // The frontend ships as classic <script src> bundles, never ES modules.
    // jsdom is required: proviant.js and theme.js touch document.cookie,
    // localStorage, matchMedia and window.fetch at load time.
    environment: "jsdom",
    include: ["tests/js/**/*.test.js"],
    // Explicit imports only — no ambient describe/it globals, so the test
    // files lint identically to the rest of the tree.
    globals: false,
    // formatDate/colorExpiry build dates from local time. Pin the zone so
    // assertions hold identically on a dev laptop and on CI.
    env: { TZ: "UTC" },
  },
});