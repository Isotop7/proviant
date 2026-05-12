package v1

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"time"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers"
	"codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/errors"
	apiModel "codeberg.org/isotop7/proviant/models/api"
	"codeberg.org/isotop7/proviant/models/configuration"
	"codeberg.org/isotop7/proviant/models/configuration/static"
	dbModel "codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/util"

	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

// ScanExpiryDate scans an uploaded image for expiry date
// @Summary       Scan expiry date from product photo
// @Description   Upload an image of product packaging; returns detected expiry date with confidence score
// @Tags          product
// @Accept        multipart/form-data
// @Produce       json
// @Param          image  formData  file  true  "Product packaging image"
// @Success       200  {object}  apiModel.ExpiryScanResponse
// @Failure       400  {object}  api.APIResponse
// @Failure       500  {object}  api.APIResponse
// @Router        /api/v1/products/scan-date [post]
func ScanExpiryDate(ctx *gin.Context, appCtx *AppContext) {
	logger := appCtx.Logger

	// Get uploaded image file
	file, err := ctx.FormFile("image")
	if err != nil {
		logger.Error().Msgf("Form file error: %s", err.Error())
		ctx.JSON(http.StatusBadRequest, api.Error(errors.ErrInvalidRequest))
		return
	}

	// Validate size (configured via config)
	proviantConfig, _ := ctx.MustGet(util.ContextKeyProviantConfig).(*configuration.ProviantConfiguration)
	if file.Size > int64(proviantConfig.Server.MaxUploadSizeMB)*1024*1024 {
		ctx.JSON(http.StatusBadRequest, api.Error(errors.ErrFileTooLarge))
		return
	}

	// Open and read file
	src, openErr := file.Open()
	if openErr != nil {
		logger.Error().Msgf("File open error: %s", openErr.Error())
		ctx.JSON(http.StatusInternalServerError, api.Error(errors.ErrInternalServer))
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
		ctx.JSON(http.StatusInternalServerError, api.Error(errors.ErrInternalServer))
		return
	}

	// Get OCR controller from context
	ocrController, ok := ctx.MustGet("ocrController").(*controllers.OCRControllerImpl)
	if !ok {
		logger.Error().Msg("OCR controller not found in context")
		ctx.JSON(http.StatusInternalServerError, api.Error(errors.ErrInternalServer))
		return
	}

	// Call OCR with timeout (goroutine pattern like barcode scanner)
	ctxTimeout, cancel := context.WithTimeout(ctx.Request.Context(), time.Duration(ocrController.Config.Timeout)*time.Second)
	defer cancel()

	resultChan := make(chan *apiModel.ExpiryScanResponse, 1)
	errChan := make(chan error, 1)

	go func() {
		resp, err := ocrController.ScanExpiryDate(imgBytes)
		if err != nil {
			errChan <- err
			return
		}
		resultChan <- resp
	}()

	select {
	case <-ctxTimeout.Done():
		ctx.JSON(http.StatusInternalServerError, api.Error(errors.ErrOCRTimeout))
		return
	case err := <-errChan:
		logger.Error().Msgf("OCR error: %s", err.Error())
		ctx.JSON(http.StatusInternalServerError, api.Error(errors.ErrOCRProcessing))
		return
	case resp := <-resultChan:
		if userID, ok := getCurrentUserID(ctx, logger); ok {
			go logExpiryScan(ctx, logger, userID, resp, imgBytes)
		}
		ctx.JSON(http.StatusOK, resp)
		return
	}
}

func logExpiryScan(ctx *gin.Context, logger *zerolog.Logger, userID uint, resp *apiModel.ExpiryScanResponse, imgBytes []byte) {
	repos, dbOk := ctx.MustGet(util.ContextKeyRepos).(*database.RepositoryContainer)
	if !dbOk {
		logger.Warn().Msg("repos not available for expiry scan logging")
		return
	}
	scan := &dbModel.ExpiryScan{
		UserID:       userID,
		ScannedAt:    time.Now(),
		DetectedDate: mustParseDate(resp.DetectedDate),
		Confidence:   resp.Confidence,
		RawText:      resp.RawText,
		ImageHash:    hashImage(imgBytes),
	}
	if err := repos.ExpiryScan.Create(scan); err != nil {
		logger.Warn().Msgf("Failed to store expiry scan: %s", err.Error())
	}
}

// getCurrentUserID extracts user ID from JWT claims or PAT context
func getCurrentUserID(ctx *gin.Context, logger *zerolog.Logger) (uint, bool) {
	// PAT path: PAT middleware injects userID directly into context
	if id, exists := ctx.Get(util.ContextKeyUserID); exists {
		if userID, ok := id.(uint); ok && userID > 0 {
			return userID, true
		}
	}

	// JWT path: extract from token claims
	claims := jwt.ExtractClaims(ctx)
	idClaim, ok := claims[static.TokenIdentityKey]
	if !ok {
		logger.Error().Msg(api.ResponseErrUserIDFromToken.Message)
		return 0, false
	}
	idFloat, ok := idClaim.(float64)
	if !ok {
		logger.Error().Msg(api.ResponseErrUserIDFromToken.Message)
		return 0, false
	}
	userID := uint(idFloat)
	if userID <= 0 {
		logger.Error().Msg(api.ResponseErrUserIDFromToken.Message)
		return 0, false
	}
	return userID, true
}

// hashImage computes SHA256 of image bytes
func hashImage(img []byte) string {
	sum := sha256.Sum256(img)
	return hex.EncodeToString(sum[:])
}

// mustParseDate converts ISO string to time.Time; returns zero time on failure
func mustParseDate(s string) time.Time {
	if t, err := time.Parse(util.DefaultDateFormatParseStr, s); err == nil {
		return t
	}
	return time.Time{}
}
