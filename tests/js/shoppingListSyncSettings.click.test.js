import { describe, expect, it, vi } from "vitest";
import { loadClassicScript } from "./helpers/loadScript.js";

const PAGE_MARKUP = `
<div>
  <input type="checkbox" id="toggleIntegration-todoist" data-integration-toggle="todoist">
  <div id="integrationFields-todoist" class="d-none">
    <input type="text" id="inputIntegrationListId-todoist" value="proj-1">
    <input type="text" id="inputIntegrationListName-todoist" value="Groceries">
    <input type="password" id="inputIntegrationToken-todoist" value="secret-tok">
    <input type="checkbox" id="clearIntegrationToken-todoist">
    <div id="tokenHint-todoist"></div>
    <div id="integrationStatus-todoist" aria-live="polite"></div>
    <button type="button" id="btnSaveIntegration-todoist" data-integration-action="save" data-provider="todoist">Save</button>
    <button type="button" id="btnTestIntegration-todoist" data-integration-action="test" data-provider="todoist">Test</button>
    <button type="button" id="btnSyncIntegration-todoist" data-integration-action="sync" data-provider="todoist">Sync</button>
    <button type="button" id="btnDeleteIntegration-todoist" data-integration-action="delete" data-provider="todoist">Disconnect</button>
  </div>
</div>
`;

const flush = () => new Promise((resolve) => setTimeout(resolve, 0));

describe("shoppingListSyncSettings.js delegation", () => {
  it("toggles fields, saves credentials write-only, tests, syncs, and disconnects", async () => {
    const fetchStub = vi.fn().mockImplementation((url) => {
      if (url.endsWith("/sync")) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: () => Promise.resolve({ lastSyncAt: "2026-10-14T10:00:00Z", accessTokenConfigured: true }),
        });
      }
      if (url.endsWith("/test")) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: () => Promise.resolve({ message: "Connection test succeeded" }),
        });
      }
      return Promise.resolve({
        ok: true,
        status: 201,
        json: () => Promise.resolve({ accessTokenConfigured: true, lastSyncAt: "2026-10-14T09:00:00Z" }),
      });
    });

    const showFeedback = vi.fn();
    const showConfirm = vi.fn((_title, _msg, onConfirm) => onConfirm());

    const { realm } = loadClassicScript("shoppingListSyncSettings.js", [], {
      prelude: `document.body.innerHTML = ${JSON.stringify(PAGE_MARKUP)};`,
      globals: {
        fetch: fetchStub,
        proviant: {
          showFeedback,
          showConfirm,
        },
      },
    });
    const doc = realm.document;

    const toggle = doc.getElementById("toggleIntegration-todoist");
    toggle.checked = true;
    toggle.dispatchEvent(new realm.Event("change", { bubbles: true }));
    expect(doc.getElementById("integrationFields-todoist").classList.contains("d-none")).toBe(false);

    doc.getElementById("btnSaveIntegration-todoist").dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    await flush();
    await flush();

    expect(fetchStub).toHaveBeenCalledWith(
      "/api/v1/household/integrations/todoist",
      expect.objectContaining({ method: "POST" })
    );
    expect(doc.getElementById("inputIntegrationToken-todoist").value).toBe("");
    expect(doc.getElementById("tokenHint-todoist").textContent).toContain("Token stored");

    doc.getElementById("btnTestIntegration-todoist").dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    await flush();
    await flush();
    expect(fetchStub).toHaveBeenCalledWith(
      "/api/v1/household/integrations/todoist/test",
      expect.objectContaining({ method: "POST" })
    );

    doc.getElementById("btnSyncIntegration-todoist").dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    await flush();
    await flush();
    expect(fetchStub).toHaveBeenCalledWith(
      "/api/v1/household/integrations/todoist/sync",
      expect.objectContaining({ method: "POST" })
    );

    doc.getElementById("btnDeleteIntegration-todoist").dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    await flush();
    await flush();
    expect(showConfirm).toHaveBeenCalledTimes(1);
    expect(fetchStub).toHaveBeenCalledWith(
      "/api/v1/household/integrations/todoist",
      expect.objectContaining({ method: "DELETE" })
    );
  });
});
