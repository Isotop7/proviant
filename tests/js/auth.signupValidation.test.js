import { readFileSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it, vi } from "vitest";
import { loadClassicScript } from "./helpers/loadScript.js";

/**
 * auth.js reads the password policy from the data attributes of #auth-password
 * at load time, so the markup has to be in the DOM before the script runs — the
 * prelude of loadClassicScript puts it there.
 *
 * The attributes demand every rule, while the shipped config templates demand
 * the length alone: the cases below cover the strict policy, which is the one
 * that exercises all four branches.
 *
 * auth.js registers its submit listener when it loads, so the load happens once
 * at module scope and every case drives that one instance. A second load in the
 * same file would register the listener twice and submit twice.
 */
const PAGE_MARKUP = `
<form id="authForm">
  <input type="hidden" id="auth-mode" value="signup">
  <input id="auth-name">
  <input id="auth-email">
  <input id="auth-password"
         data-min-length="12"
         data-require-uppercase="true"
         data-require-digit="true"
         data-require-special="true">
  <div id="auth-email-error" style="display:none;"><span id="auth-email-error-text"></span></div>
  <div id="auth-password-error" style="display:none;"><span id="auth-password-error-text"></span></div>
  <button type="submit" id="btn-auth-submit"><i id="btn-auth-icon"></i><span id="btn-auth-label"></span></button>
</form>
`;

// The wall-clock budget for a regex guard. A super-linear pattern needs
// minutes on the pathological inputs below, so the bound is loose enough not
// to flake on a loaded runner and tight enough to catch the regression.
const WALL_CLOCK_BUDGET_MS = 2000;

// The catalogue is read here instead of through the translator: the translator
// falls back to the key itself when a key is missing, so a comparison against
// it would hold even for a message that no longer exists.
const catalog = JSON.parse(
  readFileSync(
    resolve(dirname(fileURLToPath(import.meta.url)), "../../src/i18n/catalogs/en/jsAuth.json"),
    "utf8",
  ),
);

// The address shapes the check accepts. The check does not trim: Signup trims
// the field before it calls.
const ACCEPTED_EMAILS = [
  "user@example.com",
  "user.name@example.com",
  "user+tag@example.com",
  "user@sub.example.co.uk",
  "üser@example.com",
];

const REJECTED_EMAILS = [
  ["user", "no @"],
  ["user@localhost", "no dot in the domain"],
  ["user@example.", "a trailing dot leaves an empty label"],
  ["user@.example.com", "an empty first label"],
  ["user@example..com", "an empty label in the middle"],
  ["user name@example.com", "whitespace in the local part"],
  ["user@exa mple.com", "whitespace in a label"],
  ["user@@example.com", "a second @"],
  ["@example.com", "an empty local part"],
  ["user@exam@ple.com", "an @ inside the domain"],
  ["user@example.com ", "a trailing space"],
];

const signupUser = vi.fn().mockResolvedValue({ code: 200 });

const { exports: { isValidSignupEmail }, realm } = loadClassicScript("auth.js", ["isValidSignupEmail"], {
  prelude: `document.body.innerHTML = ${JSON.stringify(PAGE_MARKUP)};`,
  globals: {
    proviant: {
      signupUser,
      showFeedback: vi.fn(),
      showConfirm: vi.fn(),
      loginUser: vi.fn(),
      logoutUser: vi.fn(),
      getOnboardingState: vi.fn().mockResolvedValue({ code: 200, body: { onboardingCompleted: true } }),
      confirmTwoFactorSetup: vi.fn(),
    },
    bootstrap: {
      Dialog: class {
        show() {}
        hide() {}
      },
    },
  },
});

const doc = realm.document;

function submitSignup({ name = "Ada", email = "ada@example.com", password = "Str0ng!Passw0rd" } = {}) {
  doc.getElementById("auth-name").value = name;
  doc.getElementById("auth-email").value = email;
  doc.getElementById("auth-password").value = password;
  doc.getElementById("authForm").dispatchEvent(new realm.Event("submit", { bubbles: true, cancelable: true }));
}

function fieldError(field, messageKey) {
  expect(catalog, messageKey).toHaveProperty(messageKey);
  expect(doc.getElementById(`${field}-error`).style.display).toBe("flex");
  return doc.getElementById(`${field}-error-text`).textContent;
}

describe("auth.js signup validation", () => {
  it("accepts the address shapes the check allows", () => {
    for (const email of ACCEPTED_EMAILS) {
      expect(isValidSignupEmail(email), email).toBe(true);
    }
  });

  it("rejects the address shapes the check refuses", () => {
    for (const [email, why] of REJECTED_EMAILS) {
      expect(isValidSignupEmail(email), `${email} — ${why}`).toBe(false);
    }
  });

  it("answers a pathological address in linear time", () => {
    // One shape per super-linear regex this check has worn: "a@" repeated is
    // the input that /\S+@\S+\.\S+/ needs seconds for, "b." repeated is the
    // input that /^[^\s@]+@[^\s@]+\.[^@\s@]+$/ needs seconds for. Both run in
    // milliseconds now.
    //
    // The budget is generous on purpose: a super-linear regex needs minutes on
    // these inputs, so the guard does not have to be tight enough to flake on
    // a loaded runner.
    for (const input of ["a@".repeat(1_600), "a@" + "b.".repeat(32_000) + "@"]) {
      const started = performance.now();
      const accepted = isValidSignupEmail(input);
      const elapsed = performance.now() - started;
      expect(accepted).toBe(false);
      expect(elapsed).toBeLessThan(WALL_CLOCK_BUDGET_MS);
    }
  });

  it("signs up when every rule is met", () => {
    signupUser.mockClear();
    submitSignup();
    expect(signupUser).toHaveBeenCalledTimes(1);
    expect(signupUser).toHaveBeenCalledWith("Ada", "ada@example.com", "Str0ng!Passw0rd", "");
  });

  // Every password but the first is long enough to pass the length rule, so
  // the rule under test is the one that fires: the checks are an else-if chain.
  it.each([
    ["Sh0rt!A", "js.auth.passwordMinLength", 12],
    ["nouppercase1!", "js.auth.passwordRequiresUppercase"],
    ["NoDigitsHere!", "js.auth.passwordRequiresDigit"],
    ["NoSpecial12345", "js.auth.passwordRequiresSpecial"],
  ])("refuses the password %s", (password, messageKey, ...args) => {
    signupUser.mockClear();
    submitSignup({ password });
    expect(signupUser).not.toHaveBeenCalled();
    expect(fieldError("auth-password", messageKey)).toBe(realm.proviant.t(messageKey, ...args));
  });

  it("refuses an invalid address and names the field", () => {
    signupUser.mockClear();
    submitSignup({ email: "ada@example" });
    expect(signupUser).not.toHaveBeenCalled();
    expect(fieldError("auth-email", "js.auth.emailInvalid")).toBe(realm.proviant.t("js.auth.emailInvalid"));
  });

  it("refuses a missing address", () => {
    signupUser.mockClear();
    submitSignup({ email: "" });
    expect(signupUser).not.toHaveBeenCalled();
    expect(fieldError("auth-email", "js.auth.emailRequired")).toBe(realm.proviant.t("js.auth.emailRequired"));
  });
});
