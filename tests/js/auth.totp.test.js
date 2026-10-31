import { describe, expect, it, vi } from "vitest";
import { loadClassicScript } from "./helpers/loadScript.js";

/**
 * Every load of auth.js registers another document-level click listener, so a
 * second load in the same realm would fire every handler twice per click.
 * Keeping the delegation test alone in its own file removes that coupling:
 * this is the only load, so exactly one listener exists.
 */
const PAGE_MARKUP = `
<form id="authForm">
  <input type="hidden" id="auth-mode" value="signin">
  <input class="form-control" type="text" id="auth-username">
  <input class="form-control" type="password" id="auth-password">
  <div class="invalid-feedback" id="auth-username-error" style="display:none;">
    <span id="auth-username-error-text"></span>
  </div>
  <div class="invalid-feedback" id="auth-password-error" style="display:none;">
    <span id="auth-password-error-text"></span>
  </div>
  <div class="invalid-feedback" id="auth-code-error" style="display:none;">
    <span id="auth-code-error-text"></span>
  </div>
  <div class="field" id="field-code" style="display:none;">
    <input class="form-control" type="text" id="auth-code">
  </div>
  <div class="field" id="field-totp-setup" style="display:none;">
    <img class="totp-qr" id="auth-totp-qr">
  </div>
  <button type="submit" class="btn" id="btn-auth-submit">
    <i class="bi" id="btn-auth-icon"></i>
    <span id="btn-auth-label">Sign in</span>
  </button>
</form>
<dialog id="twoFactorRecoveryModal">
  <ul id="auth-recovery-codes"></ul>
  <button type="button" id="btn-auth-recovery-continue">Continue</button>
</dialog>
`;

const flush = () => new Promise((resolve) => setTimeout(resolve, 0));

describe("auth.js two-factor sign-in", () => {
  it("asks for a code, walks through the setup and shows the recovery codes", async () => {
    const responses = [];
    const loginUser = vi.fn().mockImplementation(() => {
      const next = responses.shift();
      return Promise.resolve(next);
    });

    const showFeedback = vi.fn();
    const confirmTwoFactorSetup = vi.fn().mockResolvedValue({
      code: 200,
      message: "",
      body: { twoFactorEnabled: true, recoveryCodes: ["AAAA1111", "BBBB2222"] },
    });

    const { realm } = loadClassicScript("auth.js", [], {
      prelude: `document.body.innerHTML = ${JSON.stringify(PAGE_MARKUP)};`,
      globals: {
        proviant: {
          showFeedback,
          showConfirm: vi.fn(),
          loginUser,
          signupUser: vi.fn(),
          logoutUser: vi.fn(),
          getOnboardingState: vi.fn().mockResolvedValue({ code: 200, body: { onboardingCompleted: true } }),
          confirmTwoFactorSetup,
        },
      },
    });
    const doc = realm.document;

    doc.getElementById("auth-username").value = "alice";
    doc.getElementById("auth-password").value = "correct-password";

    // ── The account has a second factor: the code field appears ───────────
    responses.push({ code: 401, body: "two-factor authentication required", errorReason: "TWO_FACTOR_REQUIRED", qr: "" });
    doc.getElementById("authForm").dispatchEvent(new realm.Event("submit", { bubbles: true, cancelable: true }));
    await flush();

    expect(loginUser).toHaveBeenCalledTimes(1);
    expect(doc.getElementById("field-code").style.display).toBe("block");
    // The password stays in the form, so the user does not retype it.
    expect(doc.getElementById("auth-password").value).toBe("correct-password");

    // ── The code is sent with the next sign-in ───────────────────────────
    doc.getElementById("auth-code").value = "123456";
    responses.push({ code: 200, body: "", errorReason: "", qr: "" });
    doc.getElementById("authForm").dispatchEvent(new realm.Event("submit", { bubbles: true, cancelable: true }));
    await flush();

    expect(loginUser).toHaveBeenCalledTimes(2);
    expect(loginUser.mock.calls[1][2]).toBe("123456");

    // ── A rejected code keeps the password in the form ───────────────────
    responses.push({ code: 401, body: "invalid credentials", errorReason: "", qr: "" });
    doc.getElementById("authForm").dispatchEvent(new realm.Event("submit", { bubbles: true, cancelable: true }));
    await flush();

    expect(doc.getElementById("auth-password").value).toBe("correct-password");

    // ── The household demands a second factor the account does not have ───
    responses.push({
      code: 403,
      body: "two-factor authentication setup required",
      errorReason: "TWO_FACTOR_SETUP_REQUIRED",
      qr: "data:image/png;base64,AAAABBBB",
    });
    doc.getElementById("authForm").dispatchEvent(new realm.Event("submit", { bubbles: true, cancelable: true }));
    await flush();

    expect(doc.getElementById("field-totp-setup").style.display).toBe("block");
    expect(doc.getElementById("auth-totp-qr").getAttribute("src")).toBe("data:image/png;base64,AAAABBBB");

    // ── A rejected confirmation answers with the code-specific message ────
    confirmTwoFactorSetup.mockResolvedValue({ code: 401, message: "invalid credentials", body: {} });
    showFeedback.mockClear();
    doc.getElementById("auth-code").value = "000000";
    doc.getElementById("authForm").dispatchEvent(new realm.Event("submit", { bubbles: true, cancelable: true }));
    await flush();

    expect(confirmTwoFactorSetup).toHaveBeenCalledWith("alice", "correct-password", "000000");
    expect(showFeedback).toHaveBeenCalledWith("error", expect.any(String), expect.stringContaining("code"));

    // ── A household that demands no second factor says so ──────────────────
    confirmTwoFactorSetup.mockResolvedValue({ code: 400, message: "the two-factor setup is no longer valid", body: {} });
    showFeedback.mockClear();
    doc.getElementById("auth-code").value = "000000";
    doc.getElementById("authForm").dispatchEvent(new realm.Event("submit", { bubbles: true, cancelable: true }));
    await flush();

    expect(showFeedback).toHaveBeenCalledWith("error", expect.any(String), expect.stringContaining("no longer valid"));

    // ── The setup flow ends when the household drops the requirement ──────
    // The next submit has to be a plain sign-in again, not another confirm.
    loginUser.mockClear();
    responses.push({ code: 200, body: "", errorReason: "", qr: "" });
    doc.getElementById("authForm").dispatchEvent(new realm.Event("submit", { bubbles: true, cancelable: true }));
    await flush();

    expect(confirmTwoFactorSetup).toHaveBeenCalledTimes(2);
    expect(loginUser).toHaveBeenCalledTimes(1);
  });
});
