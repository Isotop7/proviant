package v1

import (
	"net/http"
	"strings"
	"time"

	"codeberg.org/isotop7/proviant/controllers"
	"codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/configuration"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
)

// ResolveReceiptScanConfiguration merges per-user receipt scan preferences
// into the app-level configuration. Enabled and Provider always come from the
// app config; each overridable field falls back to the app value when the
// user's field is empty/zero and the override toggle is on. When the override
// toggle is off, the app config is returned unchanged. Exception: the app
// API key is never used with a user-controlled endpoint — only with the
// app-configured endpoint — so a user cannot harvest the server credential
// by redirecting scans to a host they control.
func ResolveReceiptScanConfiguration(app *configuration.ReceiptOCRConfiguration, prefs authentication.ReceiptScanPreferences) configuration.ReceiptOCRConfiguration {
	effective := *app
	if !prefs.OverrideEnabled {
		return effective
	}
	if strings.TrimSpace(prefs.Endpoint) != "" {
		effective.Endpoint = strings.TrimSpace(prefs.Endpoint)
	}
	if strings.TrimSpace(prefs.APIKey) != "" {
		effective.APIKey = prefs.APIKey
	} else if effective.Endpoint != app.Endpoint {
		effective.APIKey = ""
	}
	if strings.TrimSpace(prefs.Model) != "" {
		effective.Model = strings.TrimSpace(prefs.Model)
	}
	if prefs.Timeout > 0 {
		effective.Timeout = prefs.Timeout
	}
	return effective
}

// receiptScanControllerForUser loads the user's receipt scan preferences and
// returns a controller configured with the effective settings. When the user
// has no override or the effective config equals the app config, the base
// controller is returned unchanged (no per-request allocation). When the base
// controller is a *controllers.ReceiptScanControllerImpl, a copy with the
// effective config is returned, using ssrfGuardTransport so that every
// connection is re-validated against private/reserved IPs at dial time
// (closes the DNS-rebinding TOCTOU left by save-time validation).
// Test mocks that are not *ReceiptScanControllerImpl pass through untouched.
//
// The function fails closed: if the app config, the repositories, or the
// user record cannot be loaded, it returns an error instead of silently
// scanning with the app-configured provider. A user who redirected scans to
// their own endpoint must not have receipt photos (and credentials) delivered
// to the app endpoint just because the preference lookup failed.
func receiptScanControllerForUser(ctx *gin.Context, appCtx *AppContext, base controllers.ReceiptScanController) (controllers.ReceiptScanController, error) {
	proviantConfigVal, _ := ctx.Get(util.ContextKeyProviantConfig)
	proviantConfig, _ := proviantConfigVal.(*configuration.ProviantConfiguration)
	if proviantConfig == nil {
		return nil, errors.ErrInternalServer
	}

	if appCtx.Repos == nil || appCtx.Repos.Users == nil {
		return nil, errors.ErrInternalServer
	}

	user, err := appCtx.Repos.Users.GetUserByID(appCtx.UserID)
	if err != nil {
		appCtx.Logger.Error().Msgf("Receipt scan: failed to load user %d: %s", appCtx.UserID, err)
		return nil, err
	}

	if !user.ReceiptScanPreferences.OverrideEnabled {
		return base, nil
	}

	effective := ResolveReceiptScanConfiguration(&proviantConfig.OCR.Receipt, user.ReceiptScanPreferences)
	if receiptScanConfigEqual(&effective, &proviantConfig.OCR.Receipt) {
		return base, nil
	}

	impl, ok := base.(*controllers.ReceiptScanControllerImpl)
	if !ok {
		return base, nil
	}

	timeout := effective.Timeout
	if timeout <= 0 {
		timeout = util.ReceiptScanDefaultTimeout
	}
	transport := impl.Client.Transport
	if effective.Endpoint != proviantConfig.OCR.Receipt.Endpoint {
		transport = ssrfGuardTransportFor()
	}
	client := &http.Client{
		Timeout:   time.Duration(timeout) * time.Second,
		Transport: transport,
	}
	return &controllers.ReceiptScanControllerImpl{
		Logger: impl.Logger,
		Config: effective,
		Client: client,
	}, nil
}

// receiptScanConfigEqual reports whether two ReceiptOCRConfiguration values
// are equivalent for scanning purposes.
func receiptScanConfigEqual(a, b *configuration.ReceiptOCRConfiguration) bool {
	return a.Enabled == b.Enabled &&
		a.Provider == b.Provider &&
		a.APIKey == b.APIKey &&
		a.Endpoint == b.Endpoint &&
		a.Model == b.Model &&
		a.Timeout == b.Timeout
}
