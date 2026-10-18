// ── Password policy (read from data attributes set by Go template) ────────────
const pwdInput = document.getElementById('auth-password');
const PWD_MIN_LENGTH        = Number.parseInt(pwdInput?.dataset.minLength || '12', 10);
const PWD_REQUIRE_UPPERCASE = pwdInput?.dataset.requireUppercase === 'true';
const PWD_REQUIRE_DIGIT     = pwdInput?.dataset.requireDigit     === 'true';
const PWD_REQUIRE_SPECIAL   = pwdInput?.dataset.requireSpecial   === 'true';

// ── Strength meter ────────────────────────────────────────────────────────────
const STRENGTH_COLORS = [
    'var(--status-expired)',
    'var(--status-critical)',
    'var(--status-soon)',
    'var(--status-fresh)',
];
const STRENGTH_LABEL_KEYS = ['js.auth.strengthWeak', 'js.auth.strengthFair', 'js.auth.strengthGood', 'js.auth.strengthStrong'];

function getStrengthScore(value) {
    let score = 0;
    if (value.length >= PWD_MIN_LENGTH)  score++;
    if (/[A-Z]/.test(value))             score++;
    if (/[0-9]/.test(value))             score++;
    if (/[^A-Za-z0-9]/.test(value))      score++;
    return score;
}

function updateStrengthMeter(value) {
    const bars  = document.querySelectorAll('#auth-strength-meter .strength-bar');
    const label = document.getElementById('auth-strength-label');
    if (!bars.length) return;

    const score = getStrengthScore(value);
    bars.forEach((bar, i) => {
        bar.style.background = i < score ? STRENGTH_COLORS[score - 1] : '';
    });
    if (label) {
        label.textContent  = score > 0 ? proviant.t(STRENGTH_LABEL_KEYS[score - 1]) : '';
        label.style.color  = score > 0 ? STRENGTH_COLORS[score - 1] : 'var(--fg-3)';
    }
}

function updateRequirements(value) {
    function setReq(id, met) {
        const el = document.getElementById(id);
        if (!el) return;
        const icon = el.querySelector('i');
        if (icon) {
            icon.className = met
                ? 'bi bi-check-circle'
                : 'bi bi-circle';
        }
        el.classList.toggle('req-met', met);
    }
    setReq('req-length',  value.length >= PWD_MIN_LENGTH);
    setReq('req-upper',   /[A-Z]/.test(value));
    setReq('req-digit',   /[0-9]/.test(value));
    setReq('req-special', /[^A-Za-z0-9]/.test(value));
}

// ── Password show/hide toggle ─────────────────────────────────────────────────
function togglePassword() {
    if (!pwdInput) return;
    const icon = document.getElementById('pwd-eye-icon');
    const btn  = document.getElementById('pwd-toggle-btn');
    const showing = pwdInput.type === 'text';
    pwdInput.type      = showing ? 'password' : 'text';
    if (icon) icon.className = showing ? 'bi bi-eye' : 'bi bi-eye-slash';
    if (btn)  btn.setAttribute('aria-pressed', String(!showing));
}

// ── Field error helpers ───────────────────────────────────────────────────────
function showFieldError(fieldId, message) {
    const input = document.getElementById(fieldId);
    const errEl = document.getElementById(fieldId + '-error');
    const textEl = document.getElementById(fieldId + '-error-text');
    if (input)  input.classList.add('error');
    if (textEl) textEl.textContent = message;
    if (errEl)  errEl.style.display = 'flex';
}

function clearFieldError(fieldId) {
    const input = document.getElementById(fieldId);
    const errEl = document.getElementById(fieldId + '-error');
    if (input)  input.classList.remove('error');
    if (errEl)  errEl.style.display = 'none';
}

function clearAllErrors() {
    ['auth-username', 'auth-name', 'auth-email', 'auth-password'].forEach(clearFieldError);
}

// ── Mode (tab) switching ──────────────────────────────────────────────────────
function switchMode(mode) {
    const modeInput = document.getElementById('auth-mode');
    if (modeInput) modeInput.value = mode;

    // Tabs
    document.getElementById('tab-signin')?.classList.toggle('active', mode === 'signin');
    document.getElementById('tab-signup')?.classList.toggle('active', mode === 'signup');
    document.getElementById('tab-signin')?.setAttribute('aria-selected', String(mode === 'signin'));
    document.getElementById('tab-signup')?.setAttribute('aria-selected', String(mode === 'signup'));

    // Fields
    document.getElementById('field-username')?.style.setProperty('display', mode === 'signin' ? '' : 'none');
    document.getElementById('field-name')?.style.setProperty('display',     mode === 'signup' ? '' : 'none');
    document.getElementById('field-email')?.style.setProperty('display',    mode === 'signup' ? '' : 'none');

    // Signin-only elements
    document.getElementById('auth-forgot-link')?.style.setProperty('display', mode === 'signin' ? 'flex' : 'none');

    // Signup-only elements
    document.getElementById('auth-strength-wrap')?.style.setProperty('display',              mode === 'signup' ? '' : 'none');
    document.getElementById('auth-password-requirements')?.style.setProperty('display',      mode === 'signup' ? '' : 'none');
    document.getElementById('auth-terms-note')?.style.setProperty('display',                 mode === 'signup' ? '' : 'none');

    // Password autocomplete
    if (pwdInput) {
        pwdInput.autocomplete = mode === 'signin' ? 'current-password' : 'new-password';
    }

    // Heading
    const title = document.getElementById('auth-heading-title');
    const sub   = document.getElementById('auth-heading-sub');
    if (title) title.textContent = mode === 'signin' ? proviant.t('auth.welcomeBack')       : proviant.t('js.auth.getStartedFree');
    if (sub)   sub.textContent   = mode === 'signin' ? proviant.t('auth.signInSubtitle')    : proviant.t('js.auth.createHouseholdAccount');

    // Submit button
    const icon  = document.getElementById('btn-auth-icon');
    const label = document.getElementById('btn-auth-label');
    if (icon)  icon.className  = mode === 'signin' ? 'bi bi-box-arrow-in-right' : 'bi bi-person-plus';
    if (label) label.textContent = mode === 'signin' ? proviant.t('auth.signIn') : proviant.t('auth.createAccount');

    clearAllErrors();

    // Clear password value and meter when switching
    if (pwdInput) {
        pwdInput.value = '';
        updateStrengthMeter('');
        updateRequirements('');
    }
}

// ── Submit button state ───────────────────────────────────────────────────────
function setSubmitLoading(loading) {
    const btn   = document.getElementById('btn-auth-submit');
    const icon  = document.getElementById('btn-auth-icon');
    const label = document.getElementById('btn-auth-label');
    if (!btn) return;
    btn.disabled = loading;
    if (loading) {
        if (icon)  icon.className   = 'bi bi-arrow-clockwise spin-icon';
        if (label) label.textContent = proviant.t('js.auth.pleaseWait');
    } else {
        const mode = document.getElementById('auth-mode')?.value || 'signin';
        if (icon)  icon.className   = mode === 'signin' ? 'bi bi-box-arrow-in-right' : 'bi bi-person-plus';
        if (label) label.textContent = mode === 'signin' ? proviant.t('auth.signIn') : proviant.t('auth.createAccount');
    }
}

// ── Modal helpers (kept from original) ───────────────────────────────────────
function showEmailVerificationModal(email) {
    const modal = document.getElementById('emailVerificationModal');
    if (!modal) return;
    const addressEl = document.getElementById('emailVerificationAddress');
    if (addressEl) addressEl.textContent = email || proviant.t('js.auth.yourEmailAddress');
    new bootstrap.Modal(modal).show();
}

// ── Login ─────────────────────────────────────────────────────────────────────
function Login() {
    clearAllErrors();

    const username = document.getElementById('auth-username')?.value?.trim() || '';
    const password = pwdInput?.value || '';

    let valid = true;

    if (!username) {
        showFieldError('auth-username', proviant.t('js.auth.usernameRequired'));
        valid = false;
    }
    if (!password) {
        showFieldError('auth-password', proviant.t('js.auth.passwordRequired'));
        valid = false;
    }
    if (!valid) return;

    setSubmitLoading(true);

    proviant.loginUser(username, password).then((response) => {
        setSubmitLoading(false);
        switch (response.code) {
            case 200:
                proviant.getOnboardingState().then((stateResponse) => {
                    if (stateResponse.code === 200 && stateResponse.body && !stateResponse.body.onboardingCompleted) {
                        globalThis.location.href = `${globalThis.location.protocol}//${globalThis.location.host}/web/onboarding`;
                    } else {
                        globalThis.location.href = `${globalThis.location.protocol}//${globalThis.location.host}/web`;
                    }
                }).catch(() => {
                    globalThis.location.href = `${globalThis.location.protocol}//${globalThis.location.host}/web`;
                });
                break;
            case 401:
                proviant.showFeedback('error', proviant.t('js.auth.loginFailed'), response.body || proviant.t('js.auth.invalidCredentials'));
                if (pwdInput) pwdInput.value = '';
                break;
            case 403:
                if (response.body?.toLowerCase().includes('verify')) {
                    showEmailVerificationModal(username);
                } else {
                    proviant.showFeedback('error', proviant.t('js.auth.loginFailed'), response.body || proviant.t('js.auth.accessDenied'));
                }
                if (pwdInput) pwdInput.value = '';
                break;
            case 429: {
                const mins = response.retryAfter ? Math.ceil(Number.parseInt(response.retryAfter, 10) / 60) : null;
                const msg = mins
                    ? proviant.tn('js.auth.accountLockedRetry', mins)
                    : (response.body || proviant.t('js.auth.accountLockedTemporary'));
                proviant.showFeedback('error', proviant.t('js.auth.accountLockedTitle'), msg);
                if (pwdInput) pwdInput.value = '';
                break;
            }
            default:
                proviant.showFeedback('error', proviant.t('js.auth.loginFailed'), response.body || proviant.t('js.auth.loginFailedMessage'));
                if (pwdInput) pwdInput.value = '';
                break;
        }
    }).catch(() => {
        setSubmitLoading(false);
        proviant.showFeedback('error', proviant.t('js.auth.loginFailed'), proviant.t('js.auth.networkError'));
    });
}

// ── Signup ────────────────────────────────────────────────────────────────────
function Signup() {
    clearAllErrors();

    const name     = document.getElementById('auth-name')?.value?.trim()  || '';
    const email    = document.getElementById('auth-email')?.value?.trim() || '';
    const password = pwdInput?.value || '';
    const inviteToken = document.getElementById('auth-invite-token')?.value || '';

    let valid = true;

    if (!name) {
        showFieldError('auth-name', proviant.t('js.auth.displayNameRequired'));
        valid = false;
    }
    if (!email) {
        showFieldError('auth-email', proviant.t('js.auth.emailRequired'));
        valid = false;
    } else if (!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
        showFieldError('auth-email', proviant.t('js.auth.emailInvalid'));
        valid = false;
    }
    if (!password) {
        showFieldError('auth-password', proviant.t('js.auth.passwordRequired'));
        valid = false;
    } else if (password.length < PWD_MIN_LENGTH) {
        showFieldError('auth-password', proviant.t('js.auth.passwordMinLength', PWD_MIN_LENGTH));
        valid = false;
    } else if (PWD_REQUIRE_UPPERCASE && !/[A-Z]/.test(password)) {
        showFieldError('auth-password', proviant.t('js.auth.passwordRequiresUppercase'));
        valid = false;
    } else if (PWD_REQUIRE_DIGIT && !/[0-9]/.test(password)) {
        showFieldError('auth-password', proviant.t('js.auth.passwordRequiresDigit'));
        valid = false;
    } else if (PWD_REQUIRE_SPECIAL && !/[^A-Za-z0-9]/.test(password)) {
        showFieldError('auth-password', proviant.t('js.auth.passwordRequiresSpecial'));
        valid = false;
    }
    if (!valid) return;

    setSubmitLoading(true);

    proviant.signupUser(name, email, password, inviteToken).then((response) => {
        setSubmitLoading(false);
        switch (response.code) {
            case 200:
                showEmailVerificationModal(email);
                document.getElementById('auth-name') && (document.getElementById('auth-name').value  = '');
                document.getElementById('auth-email') && (document.getElementById('auth-email').value = '');
                if (pwdInput) pwdInput.value = '';
                updateStrengthMeter('');
                updateRequirements('');
                break;
            case 400:
                proviant.showFeedback('error', proviant.t('js.auth.signupFailed'), proviant.t('js.auth.invalidData', response.body));
                if (pwdInput) pwdInput.value = '';
                break;
            default:
                proviant.showFeedback('error', proviant.t('js.auth.signupFailed'), response.body || proviant.t('js.auth.signupFailedMessage'));
                if (pwdInput) pwdInput.value = '';
                break;
        }
    }).catch(() => {
        setSubmitLoading(false);
        proviant.showFeedback('error', proviant.t('js.auth.signupFailed'), proviant.t('js.auth.networkError'));
    });
}

// ── Event listeners ───────────────────────────────────────────────────────────

// Tab switching
document.getElementById('tab-signin')?.addEventListener('click', () => switchMode('signin'));
document.getElementById('tab-signup')?.addEventListener('click', () => switchMode('signup'));

// Password toggle
document.getElementById('pwd-toggle-btn')?.addEventListener('click', togglePassword);

// Password input — strength meter + requirement indicators + clear error
if (pwdInput) {
    pwdInput.addEventListener('input', () => {
        const mode = document.getElementById('auth-mode')?.value || 'signin';
        if (mode === 'signup') {
            updateStrengthMeter(pwdInput.value);
            updateRequirements(pwdInput.value);
        }
        if (pwdInput.value) clearFieldError('auth-password');
    });
}

// Clear errors on input for all fields
['auth-username', 'auth-name', 'auth-email'].forEach((id) => {
    document.getElementById(id)?.addEventListener('input', function () {
        if (this.value) clearFieldError(id);
    });
});

// Form submit
document.getElementById('authForm')?.addEventListener('submit', (event) => {
    event.preventDefault();
    const mode = document.getElementById('auth-mode')?.value || 'signin';
    if (mode === 'signup') {
        Signup();
    } else {
        Login();
    }
});

// Logout button (present on other pages but auth.js is auth-only; kept for safety)
document.addEventListener('click', (event) => {
    if (event.target.closest('#btnLogout')) {
        event.preventDefault();
        void proviant.logoutUser().then(() => {
            window.location.href = '/web';
        });
    }
});
