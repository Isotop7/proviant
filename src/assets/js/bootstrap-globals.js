// Bootstrap 6 is ESM-only: the dist bundle has no UMD wrapper and defines no
// global. The rest of the frontend ships as classic `<script src>` bundles and
// reaches for `bootstrap.Dialog`, `bootstrap.Menu` and friends, so this module
// re-exports the bundle as the global those scripts expect.
//
// This is the only ES module under src/assets/js and it must be loaded with
// `type="module"`. Module scripts are deferred, so it runs before
// DOMContentLoaded — every Bootstrap call in the app happens from an event
// handler or a DOMContentLoaded listener, both of which see the global.
import * as bootstrap from "./bootstrap.bundle.min.js";

window.bootstrap = bootstrap;
