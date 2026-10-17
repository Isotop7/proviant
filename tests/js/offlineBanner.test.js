import { describe, expect, it, vi } from "vitest";
import { loadOutbox } from "./helpers/offlineQueueHarness.js";
import { loadClassicScript } from "./helpers/loadScript.js";

const BANNER_MARKUP = `
<div id="offlineBanner" class="offline-banner d-none">
  <span role="status" aria-live="polite">
    <i id="offlineBannerIcon" class="bi bi-wifi-off"></i>
    <span id="offlineBannerText"></span>
    <span id="offlineQueueCount" class="d-none"></span>
  </span>
  <button type="button" id="offlineQueueRetry" class="d-none"></button>
  <button type="button" id="offlineQueueDiscard" class="d-none"></button>
</div>
`;

const flush = () => new Promise((resolve) => setTimeout(resolve, 0));

async function waitUntil(predicate, description) {
  for (let attempt = 0; attempt < 100; attempt++) {
    if (await predicate()) return;
    await flush();
  }
  throw new Error(`timed out waiting for ${description}`);
}

const failing = () => ({ status: 500, ok: false, json: async () => ({ message: "boom" }) });

/**
 * offlineQueue.js registers a document-level click listener on every load, so
 * the banner controls are exercised from their own file with a single load:
 * a second load would make one click fire both handlers.
 */
describe("offlineQueue.js banner controls", () => {
  it("retries a stuck change and discards it after confirmation", async () => {
    const { proviant, realm } = await loadOutbox({
      online: true,
      responder: failing,
      prelude: `document.body.innerHTML = ${JSON.stringify(BANNER_MARKUP)};`,
    });
    const doc = realm.document;
    const retry = doc.getElementById("offlineQueueRetry");
    const discard = doc.getElementById("offlineQueueDiscard");

    await proviant.offline.enqueue({ type: "updateAmount", payload: { productID: 9, delta: 1 } });
    for (let attempt = 0; attempt < 5; attempt++) {
      await proviant.offline.sync();
    }

    expect(await proviant.offline.failedItems()).toHaveLength(1);
    expect(retry.classList.contains("d-none")).toBe(false);
    expect(discard.classList.contains("d-none")).toBe(false);

    retry.dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    await waitUntil(async () => (await proviant.offline.pendingItems()).length === 1, "the retried change to go back to pending");

    expect(await proviant.offline.failedItems()).toHaveLength(0);
    expect(retry.classList.contains("d-none")).toBe(true);

    for (let attempt = 0; attempt < 5; attempt++) {
      await proviant.offline.sync();
    }
    expect(await proviant.offline.failedItems()).toHaveLength(1);

    const showConfirm = vi.fn();
    proviant.showConfirm = showConfirm;
    discard.dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    await waitUntil(() => showConfirm.mock.calls.length > 0, "the discard confirmation");

    const [title, message, onConfirm, label, type] = showConfirm.mock.calls[0];
    expect(title).toBe("Discard unsynced changes");
    expect(message).toBe("Discard 1 unsynced change? It will not be saved.");
    expect(label).toBe("Discard");
    expect(type).toBe("danger");
    expect(await proviant.offline.failedItems()).toHaveLength(1);

    onConfirm();
    await waitUntil(async () => (await proviant.offline.failedItems()).length === 0, "the discarded change to be gone");

    expect(await proviant.offline.pendingItems()).toHaveLength(0);
    expect(discard.classList.contains("d-none")).toBe(true);
  });
});

describe("main.js banner state", () => {
  function loadBanner({ online, failedCount }) {
    const { exports: captured, realm } = loadClassicScript("main.js", ["updateOfflineBanner"], {
      prelude: `document.body.innerHTML = ${JSON.stringify(BANNER_MARKUP)};`,
      globals: {
        navigator: { onLine: online },
        proviant: { offline: { failedCount } },
      },
    });
    const doc = realm.document;
    captured.updateOfflineBanner();
    return {
      hidden: doc.getElementById("offlineBanner").classList.contains("d-none"),
      text: doc.getElementById("offlineBannerText").textContent,
      icon: doc.getElementById("offlineBannerIcon").className,
    };
  }

  it("names the network while offline", () => {
    const banner = loadBanner({ online: false, failedCount: 0 });

    expect(banner.hidden).toBe(false);
    expect(banner.text).toBe("You're offline — showing cached data. Changes will sync when reconnected.");
    expect(banner.icon).toContain("bi-wifi-off");
  });

  it("stays up with the stuck changes once back online", () => {
    const banner = loadBanner({ online: true, failedCount: 1 });

    expect(banner.hidden).toBe(false);
    expect(banner.text).toBe("Some changes could not be synced.");
    expect(banner.icon).toContain("bi-exclamation-triangle");
  });

  it("hides itself online without stuck changes", () => {
    expect(loadBanner({ online: true, failedCount: 0 }).hidden).toBe(true);
  });
});
