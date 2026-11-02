import { beforeAll, beforeEach, describe, expect, it } from "vitest";
import { loadClassicScript } from "./helpers/loadScript.js";

/**
 * The add-product dialog switches between manual entry and camera with a
 * segmented control. The active tab used to be painted from JavaScript with a
 * hardcoded "white" background: in dark mode the pill stayed light while its
 * label switched to the dark theme's ink, so the text vanished into the
 * background. The stylesheet owns the active state, the script only moves the
 * "active" class.
 */
const MODAL_HTML = `
  <dialog id="addProductModal">
    <div id="modalStepScan">
      <div class="tab-switcher">
        <button id="tabManualInput" type="button" class="tab-switcher-btn active"></button>
        <button id="tabCamera" type="button" class="tab-switcher-btn"></button>
      </div>
      <div id="manualInputSection"></div>
      <div id="cameraSection" style="display:none"></div>
      <button id="btnScan" type="button" data-action="scan"></button>
    </div>
    <div id="modalStepForm"></div>
    <div id="modalStepSuccess"></div>
    <div id="modalFooterScan"></div>
    <div id="modalFooterForm"></div>
    <div id="modalFooterSuccess"></div>
  </dialog>
`;

describe("products.js add-product mode switcher", () => {
  let realm;

  beforeAll(() => {
    const loaded = loadClassicScript("products.js", [], {
      prelude: `document.body.innerHTML = ${JSON.stringify(MODAL_HTML)};`,
      globals: {
        bootstrap: {
          Dialog: class {
            constructor(element) {
              this.element = element;
            }
            show() {}
            hide() {}
          },
        },
        proviant: { showFeedback: () => {} },
      },
    });
    realm = loaded.realm;
    realm.document.dispatchEvent(new realm.Event("DOMContentLoaded"));
  });

  beforeEach(() => {
    const manual = realm.document.getElementById("tabManualInput");
    const camera = realm.document.getElementById("tabCamera");
    manual.classList.add("active");
    manual.removeAttribute("style");
    camera.classList.remove("active");
    camera.removeAttribute("style");
    realm.document.getElementById("manualInputSection").style.display = "block";
    realm.document.getElementById("cameraSection").style.display = "none";
  });

  it("moves the active tab to the camera", () => {
    realm.document.getElementById("tabCamera").click();

    expect(realm.document.getElementById("tabCamera").classList.contains("active")).toBe(true);
    expect(realm.document.getElementById("tabManualInput").classList.contains("active")).toBe(false);
  });

  it("moves the active tab back to manual input", () => {
    realm.document.getElementById("tabCamera").click();
    realm.document.getElementById("tabManualInput").click();

    expect(realm.document.getElementById("tabManualInput").classList.contains("active")).toBe(true);
    expect(realm.document.getElementById("tabCamera").classList.contains("active")).toBe(false);
  });

  it("leaves the colour to the stylesheet so the tab follows the theme", () => {
    realm.document.getElementById("tabCamera").click();

    const camera = realm.document.getElementById("tabCamera");
    expect(camera.getAttribute("style")).toBeNull();
    expect(camera.style.background).toBe("");
    expect(camera.style.color).toBe("");
  });
});
