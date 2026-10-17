// v1 implements version 1 of the proviant API
package v1

import (
	"encoding/json"
	stderrors "errors"
	"io"
	"net/http"
	"strings"

	"github.com/Isotop7/proviant/api"
	"github.com/Isotop7/proviant/controllers"
	"github.com/Isotop7/proviant/errors"
	apiModel "github.com/Isotop7/proviant/models/api"
	"github.com/Isotop7/proviant/models/authentication"
	"github.com/Isotop7/proviant/models/configuration"
	"github.com/Isotop7/proviant/util"

	"github.com/gin-gonic/gin"
)

// RestoreFullJSON replaces the caller's household inventory with the contents
// of a versioned backup document.
// @Summary      Restore household data from a JSON backup
// @Description  Accepts a backup document produced by the full JSON export or the scheduled backup job and replaces the household's products and storage locations in one transaction. Accounts are not recreated; product ownership is mapped onto members that still exist. The document's schema version is validated first.
// @Tags         import
// @Accept       multipart/form-data
// @Accept       application/json
// @Produce      json
// @Param        file  formData  file  false  "Backup JSON file"
// @Success      200   {object}  apiModel.RestoreResult
// @Failure      400   {object}  api.APIResponse
// @Failure      500   {object}  api.APIResponse
// @Router       /api/v1/products/restore/full.json [post]
func RestoreFullJSON(ctx *gin.Context, appCtx *AppContext) {
	if appCtx == nil || appCtx.Logger == nil {
		api.RespondError(ctx, http.StatusInternalServerError, errors.ErrInternalServer)
		return
	}
	logger := appCtx.Logger

	configValue, configOk := ctx.Get(util.ContextKeyProviantConfig)
	proviantConfig, isConfig := configValue.(*configuration.ProviantConfiguration)
	if !configOk || !isConfig || proviantConfig == nil {
		logger.Error().Msg("proviant config not found in context")
		api.RespondError(ctx, http.StatusInternalServerError, errors.ErrInternalServer)
		return
	}

	maxFileBytes := int64(proviantConfig.Server.MaxUploadSizeMB) * 1024 * 1024
	// A full household backup is much larger than an ordinary upload, so the
	// restore path uses its own cap. server.maxUploadSizeMB still bounds every
	// other upload; this only raises the ceiling for the document a restore must
	// be able to read back.
	if proviantConfig.Restore.MaxUploadSizeMB > 0 {
		maxFileBytes = int64(proviantConfig.Restore.MaxUploadSizeMB) * 1024 * 1024
	}
	// The body is capped before parsing for the same reason the CSV import does
	// it: ctx.FormFile buffers the multipart envelope before the file part is
	// returned, so the configured limit has to be enforced on the transport.
	ctx.Request.Body = http.MaxBytesReader(ctx.Writer, ctx.Request.Body, maxFileBytes+util.CsvImportMultipartSlackBytes)

	payload, readErr := readBackupUpload(ctx)
	if readErr != nil {
		if importBodyTooLarge(readErr) {
			api.RespondError(ctx, http.StatusBadRequest, errors.ErrFileTooLarge)
			return
		}
		if stderrors.Is(readErr, errors.ErrBackupMissingFile) {
			api.RespondError(ctx, http.StatusBadRequest, readErr)
			return
		}
		logger.Warn().Msgf("RestoreFullJSON: read: %s", readErr)
		api.RespondError(ctx, http.StatusBadRequest, errors.ErrBackupInvalidDocument)
		return
	}
	if len(payload) == 0 {
		api.RespondError(ctx, http.StatusBadRequest, errors.ErrBackupMissingFile)
		return
	}

	var backup apiModel.FullExportResponse
	if err := json.Unmarshal(payload, &backup); err != nil {
		logger.Warn().Msgf("RestoreFullJSON: decode: %s", err)
		api.RespondError(ctx, http.StatusBadRequest, errors.ErrBackupInvalidDocument)
		return
	}
	if backup.Version != apiModel.BackupSchemaVersion {
		api.RespondError(ctx, http.StatusBadRequest, errors.ErrBackupUnsupportedVersion)
		return
	}

	householdID, ok := restoreHouseholdID(ctx, appCtx)
	if !ok {
		return
	}
	if appCtx.DB == nil {
		logger.Error().Msg("RestoreFullJSON: database handle missing")
		api.RespondError(ctx, http.StatusInternalServerError, errors.ErrDatabaseContextNotFound)
		return
	}

	result, restoreErr := controllers.RestoreHouseholdBackup(appCtx.DB, householdID, appCtx.UserID, &backup)
	if restoreErr != nil {
		if stderrors.Is(restoreErr, errors.ErrBackupUnsupportedVersion) ||
			stderrors.Is(restoreErr, errors.ErrBackupInvalidDocument) ||
			stderrors.Is(restoreErr, errors.ErrInvalidOpenedLifecycle) ||
			stderrors.Is(restoreErr, errors.ErrBackupTooManyProducts) ||
			stderrors.Is(restoreErr, errors.ErrBackupTooManyLocations) {
			api.RespondError(ctx, http.StatusBadRequest, restoreErr)
			return
		}
		logger.Error().Msgf("RestoreFullJSON: restore: %s", restoreErr)
		api.RespondError(ctx, http.StatusInternalServerError, errors.ErrBackupRestoreFailed)
		return
	}

	ctx.JSON(http.StatusOK, result)
}

// readBackupUpload returns the uploaded backup bytes from either a multipart
// file part or a raw JSON body. Both shapes are accepted because the browser UI
// uploads a file while scripts POST the document they just downloaded.
func readBackupUpload(ctx *gin.Context) ([]byte, error) {
	if strings.HasPrefix(ctx.ContentType(), "multipart/form-data") {
		file, err := ctx.FormFile(importFormField)
		if err != nil {
			if importBodyTooLarge(err) {
				return nil, err
			}
			return nil, errors.ErrBackupMissingFile
		}
		src, openErr := file.Open()
		if openErr != nil {
			return nil, openErr
		}
		defer func() {
			_ = src.Close()
		}()
		return io.ReadAll(src)
	}
	return io.ReadAll(ctx.Request.Body)
}

// restoreHouseholdID resolves the household the restore replaces and verifies
// the caller may replace it. The route middleware already runs the role check,
// but the handler is exported and the operation is destructive, so it rechecks
// rather than trusting the route wiring.
func restoreHouseholdID(ctx *gin.Context, appCtx *AppContext) (uint, bool) {
	if appCtx == nil || appCtx.Repos == nil || appCtx.Repos.Users == nil {
		api.RespondError(ctx, http.StatusInternalServerError, errors.ErrDatabaseContextNotFound)
		return 0, false
	}
	role, roleErr := appCtx.Repos.Users.GetUserHouseholdRole(appCtx.UserID)
	if roleErr != nil {
		appCtx.Logger.Error().Msgf("RestoreFullJSON: role lookup: %s", roleErr)
		api.RespondError(ctx, http.StatusInternalServerError, errors.ErrInternalServer)
		return 0, false
	}
	if role != authentication.RoleAdmin {
		api.RespondError(ctx, http.StatusForbidden, errors.ErrInsufficientRole)
		return 0, false
	}

	if value, exists := ctx.Get(util.ContextKeyHouseholdID); exists {
		if householdID, ok := value.(uint); ok && householdID > 0 {
			return householdID, true
		}
	}
	householdID, err := appCtx.Repos.Users.GetUserHouseholdByID(appCtx.UserID)
	if err == nil && householdID > 0 {
		return householdID, true
	}
	if err != nil {
		appCtx.Logger.Error().Msgf("RestoreFullJSON: household lookup: %s", err)
	} else {
		appCtx.Logger.Error().Msg("RestoreFullJSON: user has no household")
	}
	api.RespondError(ctx, http.StatusBadRequest, errors.ErrUserHasNoHousehold)
	return 0, false
}
