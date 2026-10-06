package v1

import (
	"context"
	stderrors "errors"
	"io"
	"net/http"
	"strings"
	"time"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers"
	"codeberg.org/isotop7/proviant/errors"
	apiModel "codeberg.org/isotop7/proviant/models/api"
	"codeberg.org/isotop7/proviant/models/configuration"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

// ScanReceipt extracts product line items from an uploaded receipt photo via
// a vision LLM. Stateless: nothing is persisted server-side.
// @Summary       Scan receipt photo for line items
// @Description   Upload a receipt photo; returns extracted product drafts (name, amount, unit, price). An empty items list means the scan succeeded but nothing was recognized.
// @Tags          product
// @Accept        multipart/form-data
// @Produce       json
// @Param         image  formData  file  true  "Receipt photo"
// @Success       200  {object}  apiModel.ReceiptScanResponse
// @Failure       400  {object}  api.APIResponse
// @Failure       403  {object}  api.APIResponse
// @Failure       500  {object}  api.APIResponse
// @Failure       502  {object}  api.APIResponse
// @Failure       504  {object}  api.APIResponse
// @Router        /api/v1/products/scan-receipt [post]
func ScanReceipt(ctx *gin.Context, appCtx *AppContext) {
	logger := appCtx.Logger

	proviantConfig, _ := ctx.MustGet(util.ContextKeyProviantConfig).(*configuration.ProviantConfiguration)
	if proviantConfig == nil || !proviantConfig.OCR.Receipt.Enabled || strings.TrimSpace(proviantConfig.OCR.Receipt.Model) == "" {
		logger.Warn().Msg("Receipt scan requested while feature is disabled")
		api.RespondError(ctx, http.StatusForbidden, errors.ErrReceiptOCRDisabled)
		return
	}

	// ctx.FormFile parses the whole multipart body before the file part is
	// returned, so the cap has to be enforced on the body itself — otherwise
	// an oversized upload is fully buffered (memory + temp file) before any
	// size check ever runs.
	uploadLimitMB := proviantConfig.Server.MaxUploadSizeMB
	if uploadLimitMB <= 0 {
		uploadLimitMB = util.DefaultMaxUploadSizeMB
	}
	maxUploadBytes := int64(uploadLimitMB) * 1024 * 1024
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, maxUploadBytes+util.ReceiptScanMultipartSlackBytes)

	file, err := ctx.FormFile("image")
	if err != nil {
		logger.Warn().Msgf("Form file error: %s", err.Error())
		if importBodyTooLarge(err) {
			api.RespondError(ctx, http.StatusBadRequest, errors.ErrFileTooLarge)
			return
		}
		api.RespondError(ctx, http.StatusBadRequest, errors.ErrInvalidRequest)
		return
	}

	if file.Size > maxUploadBytes {
		api.RespondError(ctx, http.StatusBadRequest, errors.ErrFileTooLarge)
		return
	}

	src, openErr := file.Open()
	if openErr != nil {
		logger.Error().Msgf("File open error: %s", openErr.Error())
		api.RespondError(ctx, http.StatusInternalServerError, errors.ErrInternalServer)
		return
	}
	defer func() {
		if closeErr := src.Close(); closeErr != nil {
			logger.Warn().Msgf("Error closing file: %s", closeErr.Error())
		}
	}()

	imgBytes, readErr := io.ReadAll(src)
	if readErr != nil {
		logger.Error().Msgf("File read error: %s", readErr.Error())
		api.RespondError(ctx, http.StatusInternalServerError, errors.ErrInternalServer)
		return
	}

	imageMIME := controllers.ReceiptImageMIME(imgBytes)
	if imageMIME == "" {
		logger.Warn().
			Str("declaredType", file.Header.Get("Content-Type")).
			Str("detectedType", http.DetectContentType(imgBytes)).
			Msg("Receipt scan rejected: payload is not a supported image")
		api.RespondError(ctx, http.StatusBadRequest, errors.ErrInvalidImageType)
		return
	}

	receiptCtrl, ok := ctx.MustGet(util.ContextKeyReceiptCtrl).(controllers.ReceiptScanController)
	if !ok {
		logger.Error().Msg("Receipt scan controller not found in context")
		api.RespondError(ctx, http.StatusInternalServerError, errors.ErrInternalServer)
		return
	}

	logger.Debug().
		Int("imageBytes", len(imgBytes)).
		Str("contentType", imageMIME).
		Msg("Receipt scan request received")

	// The timeout itself is layered inside the controller (config timeout +
	// client timeout); the handler only observes its expiry via the scan
	// context. A config that skipped startup validation (tests, embedders)
	// is zero-guarded there, so no spurious instantly-expired deadline here.
	// No deadline is layered on this context itself (the controller owns the
	// timeout), so its Done branch below is reachable only when the client
	// disconnects and cancels the request context.
	scanCtx, cancel := context.WithCancel(ctx.Request.Context())
	defer cancel()

	resultChan := make(chan *apiModel.ReceiptScanResponse, 1)
	errChan := make(chan error, 1)
	startedAt := time.Now()

	go func() {
		// Gin's recovery middleware does not cover goroutines: a panic inside
		// ScanReceipt would take down the whole server. Contain it — the
		// client gets an error response instead of a dead connection.
		defer func() {
			if r := recover(); r != nil {
				logger.Error().Msgf("Receipt scan worker panicked: %v", r)
				errChan <- errors.ErrOCRProcessing
			}
		}()
		resp, scanErr := receiptCtrl.ScanReceipt(scanCtx, imgBytes)
		if scanErr != nil {
			errChan <- scanErr
			return
		}
		resultChan <- resp
	}()

	select {
	case scanErr := <-errChan:
		respondScanError(ctx, logger, scanErr)
		return
	case resp := <-resultChan:
		respondScanSuccess(ctx, logger, startedAt, resp)
		return
	case <-scanCtx.Done():
		// The deadline can fire in the same select round as — or a few
		// milliseconds before — a ready result. Wait briefly for the actual
		// outcome before answering 504: a scan that finished just past the
		// deadline is still a successful scan for the user.
		graceTimer := time.NewTimer(receiptScanGracePeriod)
		defer graceTimer.Stop()
		for {
			select {
			case scanErr := <-errChan:
				respondScanError(ctx, logger, scanErr)
				return
			case resp := <-resultChan:
				respondScanSuccess(ctx, logger, startedAt, resp)
				return
			case <-graceTimer.C:
				api.RespondError(ctx, http.StatusGatewayTimeout, errors.ErrOCRTimeout)
				return
			}
		}
	}
}

// receiptScanGracePeriod extends the wait past the scan deadline for a result
// that is already in flight. Short enough not to hang the request noticeably
// longer on a real timeout; a result arriving later than this is answered 504
// even though the scan eventually succeeds — the alternative (waiting for the
// full config timeout again) doubles the worst-case request duration.
const receiptScanGracePeriod = 500 * time.Millisecond

func respondScanError(ctx *gin.Context, logger *zerolog.Logger, scanErr error) {
	// The controller's HTTP client timeout and the request context deadline
	// expire at the same instant, so a genuine timeout usually arrives as a
	// wrapped context.DeadlineExceeded instead of the 504 grace path.
	if stderrors.Is(scanErr, context.DeadlineExceeded) {
		logger.Warn().Msgf("Receipt scan timed out: %s", scanErr.Error())
		api.RespondError(ctx, http.StatusGatewayTimeout, errors.ErrOCRTimeout)
		return
	}
	// A client disconnect surfaces as a wrapped context.Canceled. The
	// connection is gone, so no response can be delivered; log at Warn per
	// log policy (user-caused) and skip the 5xx path to keep error monitoring
	// free of client-side noise.
	if stderrors.Is(scanErr, context.Canceled) {
		logger.Warn().Msgf("Receipt scan aborted: client disconnected: %s", scanErr.Error())
		return
	}
	// Upstream failures (unreachable endpoint, non-2xx status, endpoint-reported
	// errors such as 401/403 auth or config problems) are not internal failures
	// of this server and must not surface as 500.
	var endpointErr *errors.ReceiptEndpointError
	if stderrors.As(scanErr, &endpointErr) {
		logger.Error().Msgf("Receipt scan endpoint error: %s", scanErr.Error())
		api.RespondError(ctx, http.StatusBadGateway, errors.ErrReceiptEndpoint)
		return
	}
	logger.Error().Msgf("Receipt scan error: %s", scanErr.Error())
	api.RespondError(ctx, http.StatusInternalServerError, errors.ErrOCRProcessing)
}

func respondScanSuccess(ctx *gin.Context, logger *zerolog.Logger, startedAt time.Time, resp *apiModel.ReceiptScanResponse) {
	logger.Info().
		Int("items", len(resp.Items)).
		Str("duration", time.Since(startedAt).String()).
		Msg("Receipt scan successful")
	ctx.JSON(http.StatusOK, resp)
}
