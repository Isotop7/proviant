import { describe, expect, it, vi } from "vitest";
import { loadClassicScript } from "./helpers/loadScript.js";

/**
 * Every load of userSettings.js registers another document-level click
 * listener, so a second load in the same realm would fire every handler twice
 * per click. Keeping the delegation test alone in its own file removes that
 * coupling: this is the only load, so exactly one listener exists.
 */
const PAGE_MARKUP = `
<div id="twoFactorSection">
  <span class="badge" id="twoFactorStatusBadge"></span>
  <p id="twoFactorStatusText"></p>
  <button type="button" id="btnStartTOTP">Enable</button>
  <button type="button" id="btnDisableTOTP" class="d-none">Disable</button>
</div>
<dialog id="twoFactorEnrolModal">
  <img id="twoFactorQr">
  <p id="twoFactorExpiry" class="d-none"></p>
  <input type="text" id="inputTwoFactorCode">
  <div id="twoFactorRecoveryCodes" class="d-none">
    <ul id="twoFactorRecoveryCodeList"></ul>
  </div>
  <button type="button" id="btnConfirmTOTP">Confirm</button>
</dialog>
<dialog id="twoFactorDisableModal">
  <input type="password" id="inputTwoFactorPassword">
  <div id="twoFactorDisableWarning" class="d-none"></div>
  <button type="button" id="btnConfirmDisableTOTP">Disable</button>
</dialog>
<button type="button" id="btnSaveRequireTwoFactor">Save</button>
<input type="checkbox" id="inputRequireTwoFactor" data-require-two-factor="false">
`;

const flush = () => new Promise((resolve) => setTimeout(resolve, 0));

describe("userSettings.js two-factor delegation", () => {
  it("enrols, shows the recovery codes once and disables the second factor", async () => {
    // The status call answers 404 once the flow is done, so the section hides
    // itself on a server that offers no second factor.
    let statusCode = 200;
    const fetchStub = vi.fn().mockImplementation((url, options = {}) => {
      if (url.endsWith("/api/v1/user/totp/confirm")) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: () => Promise.resolve({ twoFactorEnabled: true, recoveryCodes: ["AAAA1111", "BBBB2222"] }),
        });
      }
      if (url.endsWith("/api/v1/user/totp/disable")) {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: () => Promise.resolve({ message: "Two-factor authentication disabled" }),
        });
      }
      if (url.endsWith("/api/v1/user/totp") && options.method === "POST") {
        return Promise.resolve({
          ok: true,
          status: 200,
          json: () => Promise.resolve({ qr: "data:image/png;base64,AAAABBBB" }),
        });
      }
      return Promise.resolve({
        ok: statusCode === 200,
        status: statusCode,
        json: () =>
          Promise.resolve(
            statusCode === 200
              ? { enabled: false, pending: false, recoveryCodesRemaining: 0 }
              : { message: "two-factor authentication is disabled on this server" },
          ),
      });
    });

    const startTOTPEnrolment = vi.fn().mockResolvedValue({
      code: 200,
      message: { qr: "data:image/png;base64,AAAABBBB", expiresAt: "2026-10-27T18:00:00Z" },
    });
    const confirmTOTPEnrolment = vi.fn().mockResolvedValue({
      code: 200,
      message: { twoFactorEnabled: true, recoveryCodes: ["AAAA1111", "BBBB2222"] },
    });
    const disableTOTP = vi.fn().mockResolvedValue({ code: 200, message: "Two-factor authentication disabled" });
    const getTOTPStatus = vi.fn().mockResolvedValue({
      code: 200,
      message: { enabled: false, pending: false, recoveryCodesRemaining: 0 },
    });

    const { realm } = loadClassicScript("userSettings.js", [], {
      prelude: `document.body.innerHTML = ${JSON.stringify(PAGE_MARKUP)};`,
      globals: {
        fetch: fetchStub,
        proviant: {
          showFeedback: vi.fn(),
          showConfirm: vi.fn(),
          copyToClipboard: vi.fn(),
          getTOTPStatus,
          startTOTPEnrolment,
          confirmTOTPEnrolment,
          disableTOTP,
          getHouseholdUsers: vi.fn().mockResolvedValue({ code: 200, message: [] }),
          getPATs: vi.fn().mockResolvedValue({ code: 200, message: [] }),
          getCalendarTokenStatus: vi.fn().mockResolvedValue({ code: 200, message: {} }),
          getWebhooks: vi.fn().mockResolvedValue({ code: 200, message: [] }),
          getWebPushVAPIDPublicKey: vi.fn().mockResolvedValue({ code: 200, message: "" }),
          subscribeWebPush: vi.fn(),
          unsubscribeWebPush: vi.fn(),
        },
      },
    });
    const doc = realm.document;

    // ── Enrolment ────────────────────────────────────────────────────────
    doc.getElementById("btnStartTOTP").dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    await flush();

    expect(startTOTPEnrolment).toHaveBeenCalledTimes(1);
    expect(doc.getElementById("twoFactorQr").getAttribute("src")).toBe("data:image/png;base64,AAAABBBB");
    // The QR code only stays valid until the pending enrolment expires.
    expect(doc.getElementById("twoFactorExpiry").classList.contains("d-none")).toBe(false);
    expect(doc.getElementById("twoFactorExpiry").textContent).not.toBe("");

    // ── Confirmation shows the recovery codes exactly once ─────────────────
    doc.getElementById("inputTwoFactorCode").value = "123456";
    doc.getElementById("btnConfirmTOTP").dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    await flush();

    expect(confirmTOTPEnrolment).toHaveBeenCalledTimes(1);
    expect(confirmTOTPEnrolment).toHaveBeenCalledWith("123456");
    expect(doc.getElementById("twoFactorRecoveryCodes").classList.contains("d-none")).toBe(false);
    const items = doc.querySelectorAll("#twoFactorRecoveryCodeList li");
    expect(items.length).toBe(2);
    expect(items[0].textContent).toBe("AAAA1111");
    expect(items[1].textContent).toBe("BBBB2222");

    // ── Disabling needs the password ──────────────────────────────────────
    doc.getElementById("btnConfirmDisableTOTP").dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    await flush();

    expect(disableTOTP).not.toHaveBeenCalled();
    expect(doc.getElementById("inputTwoFactorPassword").classList.contains("is-invalid")).toBe(true);

    doc.getElementById("inputTwoFactorPassword").value = "correct-password";
    doc.getElementById("btnConfirmDisableTOTP").dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    await flush();

    expect(disableTOTP).toHaveBeenCalledTimes(1);
    expect(disableTOTP).toHaveBeenCalledWith("correct-password");

    // ── The household policy warns before the second factor is turned off ──
    // The policy comes from the rendered page, not from the status endpoint.
    doc.getElementById("twoFactorSection").dataset.householdRequired = "true";
    getTOTPStatus.mockResolvedValue({
      code: 200,
      message: { enabled: true, pending: false, recoveryCodesRemaining: 3 },
    });
    doc.dispatchEvent(new realm.Event("DOMContentLoaded"));
    await flush();
    await flush();

    doc.getElementById("btnDisableTOTP").dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    await flush();

    expect(doc.getElementById("twoFactorDisableWarning").classList.contains("d-none")).toBe(false);

    // ── A failed policy save puts the switch back ─────────────────────────
    const policyCall = vi.fn().mockResolvedValue({ code: 500, message: { message: "boom" } });
    // The helper is captured on the realm's proviant object.
    realm.proviant.updateHouseholdTwoFactorPolicy = policyCall;
    const toggle = doc.getElementById("inputRequireTwoFactor");

    // Save without touching the switch: the failure must leave it where the
    // server has it, not flip it.
    doc.getElementById("btnSaveRequireTwoFactor").dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    await flush();
    await flush();

    expect(policyCall).toHaveBeenCalledWith(false);
    expect(toggle.checked).toBe(false);

    // The user turns the policy on and the save fails: the switch goes back
    // to the value the server holds.
    toggle.checked = true;
    doc.getElementById("btnSaveRequireTwoFactor").dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    await flush();
    await flush();

    expect(policyCall).toHaveBeenCalledWith(true);
    expect(toggle.checked).toBe(false);

    // ── A saved policy reaches the two-factor card ─────────────────────────
    // The card renders the policy from the page, so a successful save has to
    // hand the new value to it.
    const policyOkCall = vi.fn().mockResolvedValue({ code: 200, message: {} });
    realm.proviant.updateHouseholdTwoFactorPolicy = policyOkCall;
    toggle.checked = true;
    doc.getElementById("btnSaveRequireTwoFactor").dispatchEvent(new realm.MouseEvent("click", { bubbles: true }));
    await flush();
    await flush();

    expect(policyOkCall).toHaveBeenCalledWith(true);
    expect(doc.getElementById("twoFactorSection").dataset.householdRequired).toBe("true");
    expect(toggle.dataset.requireTwoFactor).toBe("true");

    // ── A server that fails hides the section ─────────────────────────────
    // A broken status is not the same as "the feature is off", but the card
    // must not stay visible in an unknown state either.
    statusCode = 500;
    getTOTPStatus.mockResolvedValue({
      code: 500,
      message: { message: "internal server error" },
    });
    doc.dispatchEvent(new realm.Event("DOMContentLoaded"));
    await flush();
    await flush();

    expect(doc.getElementById("twoFactorSection").classList.contains("d-none")).toBe(true);
  });
});
