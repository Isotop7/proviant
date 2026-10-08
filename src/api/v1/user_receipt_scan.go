package v1

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"codeberg.org/isotop7/proviant/api"
	apperrors "codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/configuration"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
)

const receiptScanMaxTimeout = 900

// ReceiptScanEffectiveTimeout returns the timeout that scans actually use:
// unset (zero or negative) values fall back to the default, mirroring the
// fallback in controllers.NewReceiptScanController.
func ReceiptScanEffectiveTimeout(timeout int) int {
	if timeout <= 0 {
		return util.ReceiptScanDefaultTimeout
	}
	return timeout
}

type receiptScanDefaultsResponse struct {
	Endpoint         string `json:"endpoint"`
	Model            string `json:"model"`
	Timeout          int    `json:"timeout"`
	APIKeyConfigured bool   `json:"apiKeyConfigured"`
}

type receiptScanSettingsResponse struct {
	OverrideEnabled  bool                        `json:"overrideEnabled"`
	Endpoint         string                      `json:"endpoint"`
	Model            string                      `json:"model"`
	Timeout          int                         `json:"timeout"`
	APIKeyConfigured bool                        `json:"apiKeyConfigured"`
	Defaults         receiptScanDefaultsResponse `json:"defaults"`
}

type receiptScanSettingsRequest struct {
	OverrideEnabled bool   `json:"overrideEnabled"`
	Endpoint        string `json:"endpoint"`
	APIKey          string `json:"apiKey"`
	ClearAPIKey     bool   `json:"clearApiKey"`
	Model           string `json:"model"`
	Timeout         int    `json:"timeout"`
}

// GetUserReceiptScanSettings gets a user's receipt scan settings
// @Summary			Gets a user's receipt scan settings
// @Description		Retrieves the current user's receipt scan vision model override settings and the app-level defaults. API keys are never returned.
// @Tags          	user
// @Accept        	json
// @Produce       	json
// @Success       	200  {object}  receiptScanSettingsResponse
// @Failure       	500  {object}  api.APIResponse
// @Router        	/api/v1/user/receipt-scan-settings [get]
func GetUserReceiptScanSettings(ctx *gin.Context, appCtx *AppContext) {
	user, getErr := appCtx.Repos.Users.GetUserByID(appCtx.UserID)
	if getErr != nil {
		appCtx.Logger.Error().Msgf("Error getting user: %s", getErr)
		api.RespondError(ctx, http.StatusInternalServerError, getErr)
		return
	}

	proviantConfig, _ := ctx.MustGet(util.ContextKeyProviantConfig).(*configuration.ProviantConfiguration)
	appReceipt := configuration.ReceiptOCRConfiguration{}
	if proviantConfig != nil {
		appReceipt = proviantConfig.OCR.Receipt
	}

	prefs := user.ReceiptScanPreferences
	resp := receiptScanSettingsResponse{
		OverrideEnabled:  prefs.OverrideEnabled,
		Endpoint:         prefs.Endpoint,
		Model:            prefs.Model,
		Timeout:          prefs.Timeout,
		APIKeyConfigured: prefs.APIKey != "",
		Defaults: receiptScanDefaultsResponse{
			Endpoint:         appReceipt.Endpoint,
			Model:            appReceipt.Model,
			Timeout:          ReceiptScanEffectiveTimeout(appReceipt.Timeout),
			APIKeyConfigured: appReceipt.APIKey != "",
		},
	}
	ctx.JSON(http.StatusOK, resp)
}

// UpdateUserReceiptScanSettings updates a user's receipt scan settings
// @Summary			Updates a user's receipt scan settings
// @Description		Updates the current user's receipt scan vision model override settings. An empty apiKey keeps the stored key; clearApiKey wipes it.
// @Tags          	user
// @Accept        	json
// @Produce       	json
// @Param         	settings  body      receiptScanSettingsRequest  true  "Receipt scan settings"
// @Success       	200  {object}  api.APIResponse
// @Failure       	400  {object}  api.APIResponse
// @Failure       	500  {object}  api.APIResponse
// @Router        	/api/v1/user/receipt-scan-settings [post]
func UpdateUserReceiptScanSettings(ctx *gin.Context, appCtx *AppContext) {
	var req receiptScanSettingsRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		appCtx.Logger.Warn().Msgf(apperrors.FormatGenericError, apperrors.ErrParseBody.Error(), err.Error())
		api.RespondError(ctx, http.StatusBadRequest, err)
		return
	}

	req.Endpoint = strings.TrimSpace(req.Endpoint)
	req.Model = strings.TrimSpace(req.Model)
	req.APIKey = strings.TrimSpace(req.APIKey)

	if req.Endpoint != "" {
		if err := validateReceiptScanEndpoint(req.Endpoint); err != nil {
			api.RespondError(ctx, http.StatusBadRequest, err)
			return
		}
	}

	if req.Timeout < 0 || req.Timeout > receiptScanMaxTimeout {
		api.RespondError(ctx, http.StatusBadRequest, apperrors.ErrReceiptScanInvalidTimeout)
		return
	}

	user, getErr := appCtx.Repos.Users.GetUserByID(appCtx.UserID)
	if getErr != nil {
		appCtx.Logger.Error().Msgf("Error getting user: %s", getErr)
		api.RespondError(ctx, http.StatusInternalServerError, getErr)
		return
	}

	prefs := authentication.ReceiptScanPreferences{
		OverrideEnabled: req.OverrideEnabled,
		Endpoint:        req.Endpoint,
		Model:           req.Model,
		Timeout:         req.Timeout,
	}

	if prefs.OverrideEnabled && prefs.Endpoint != "" {
		warnIfInsecureReceiptEndpoint(appCtx, prefs.Endpoint)
	}

	// The API key column is written only when the request explicitly sets or
	// clears it (apiKey == nil keeps the stored key): reading the stored key
	// and writing it back would let a concurrent save replay a stale value and
	// resurrect a cleared key. Preferences and key commit in one transaction
	// (see UpdateUserReceiptScanSettings), so a failure can never leave the
	// new endpoint active with the old key.
	var apiKey *string
	if req.ClearAPIKey || req.APIKey != "" {
		key := req.APIKey
		if req.ClearAPIKey {
			key = ""
		}
		apiKey = &key
	}

	updateErr := appCtx.Repos.Users.UpdateUserReceiptScanSettings(user.ID, prefs, apiKey)
	if updateErr != nil {
		appCtx.Logger.Error().Msgf("Error updating receipt scan settings: %s", updateErr)
		api.RespondError(ctx, http.StatusInternalServerError, updateErr)
		return
	}

	ctx.JSON(http.StatusOK, api.APIResponse{Message: "Receipt scan settings updated successfully"})
}

// validateReceiptScanEndpoint checks that the endpoint is an absolute
// http/https URL with a non-empty host. Literal private/reserved IPs are
// rejected. Hostnames are deliberately not resolved here: ssrfGuardDialContext
// re-validates every connection at request time, and a save-time lookup would
// let the distinct error responses be used to probe which internal hostnames
// resolve on the server's network (DNS oracle).
func validateReceiptScanEndpoint(endpoint string) error {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return apperrors.ErrReceiptScanInvalidEndpoint
	}
	if parsed.Host == "" {
		return apperrors.ErrReceiptScanInvalidEndpoint
	}
	scheme := strings.ToLower(parsed.Scheme)
	if scheme != "http" && scheme != "https" {
		return apperrors.ErrReceiptScanInvalidEndpoint
	}

	if ip := net.ParseIP(parsed.Hostname()); ip != nil && isPrivateOrReservedIP(ip) {
		return apperrors.ErrReceiptScanPrivateIP
	}

	return nil
}

// warnIfInsecureReceiptEndpoint logs a warning when the effective endpoint
// uses plaintext HTTP, mirroring the startup warning for the app-level config.
func warnIfInsecureReceiptEndpoint(appCtx *AppContext, endpoint string) {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return
	}
	if strings.EqualFold(parsed.Scheme, "http") {
		appCtx.Logger.Warn().Msgf(
			"receipt scan endpoint %q uses http://; the API key and receipt photos are sent unencrypted. "+
				"Use https:// unless the endpoint is reachable only over a trusted network",
			redactReceiptEndpoint(endpoint))
	}
}

func redactReceiptEndpoint(endpoint string) string {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "<invalid endpoint>"
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

// ssrfGuardDialContext wraps net.Dialer.DialContext with a private/reserved
// IP check on every connection. This closes the DNS-rebinding TOCTOU window
// left by save-time validation in validateReceiptScanEndpoint.
func ssrfGuardDialContext(ctx context.Context, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}

	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	if ip := net.ParseIP(host); ip != nil {
		if isPrivateOrReservedIP(ip) {
			return nil, apperrors.ErrReceiptScanPrivateIP
		}
		return dialer.DialContext(ctx, network, addr)
	}

	ips, err := dialer.Resolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, err
	}
	if len(ips) == 0 {
		return nil, apperrors.ErrReceiptScanInvalidEndpoint
	}
	for _, ipAddr := range ips {
		if isPrivateOrReservedIP(ipAddr.IP) {
			return nil, apperrors.ErrReceiptScanPrivateIP
		}
	}

	return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].IP.String(), port))
}

// ssrfGuardTransportMu guards ssrfGuardTransport. Production code never
// writes it; tests swap it via setSSRFGuardTransport to reach loopback
// httptest servers, and the mutex keeps those swaps race-free if tests in
// this package ever run in parallel.
var ssrfGuardTransportMu sync.RWMutex

// ssrfGuardTransport is a shared http.RoundTripper with an SSRF-guarded
// DialContext. Connection pooling is per-host so different endpoints never
// share connections.
var ssrfGuardTransport http.RoundTripper = &http.Transport{
	DialContext:           ssrfGuardDialContext,
	ForceAttemptHTTP2:     true,
	MaxIdleConns:          100,
	IdleConnTimeout:       90 * time.Second,
	TLSHandshakeTimeout:   10 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
}

// ssrfGuardTransportFor returns the currently installed SSRF-guarded
// transport.
func ssrfGuardTransportFor() http.RoundTripper {
	ssrfGuardTransportMu.RLock()
	defer ssrfGuardTransportMu.RUnlock()
	return ssrfGuardTransport
}
