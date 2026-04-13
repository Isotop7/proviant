package v1

import (
	"net/http"
	"time"

	v1api "codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers"
	"codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/api"
	jwt "github.com/appleboy/gin-jwt/v2"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

func CreateUserToken(ctx *gin.Context) {
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)
	dbHandle, dbErr := ctx.MustGet("dbHandle").(*gorm.DB)
	if !dbErr {
		ctx.JSON(http.StatusInternalServerError, v1api.APIResponse{Message: errors.ErrDatabaseContextNotFound.Error()})
		return
	}

	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims["id"].(float64))

	var req api.CreateTokenRequest
	if err := ctx.ShouldBind(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, v1api.APIResponse{Message: "invalid request: " + err.Error()})
		return
	}

	rawToken, err := controllers.GeneratePAT()
	if err != nil {
		logger.Error().Msg(err.Error())
		ctx.JSON(http.StatusInternalServerError, v1api.APIResponse{Message: "failed to generate token"})
		return
	}

	tokenHash := controllers.HashToken(rawToken)

	var expiresAt *time.Time
	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		parsed, parseErr := time.Parse(time.RFC3339, *req.ExpiresAt)
		if parseErr != nil {
			ctx.JSON(http.StatusBadRequest, v1api.APIResponse{Message: "invalid expires_at format, use RFC3339"})
			return
		}
		expiresAt = &parsed
	}

	patRepo := database.NewPATRepository(dbHandle)
	pat, err := patRepo.CreatePAT(userID, req.Name, tokenHash, expiresAt, req.Scopes)
	if err != nil {
		logger.Error().Msg(err.Error())
		ctx.JSON(http.StatusInternalServerError, v1api.APIResponse{Message: "failed to create token"})
		return
	}

	resp := api.CreateTokenResponse{
		Token:  rawToken,
		Name:   pat.Name,
		Scopes: pat.Scopes,
	}
	if pat.ExpiresAt != nil {
		formatted := pat.ExpiresAt.Format(time.RFC3339)
		resp.ExpiresAt = &formatted
	}

	ctx.JSON(http.StatusCreated, resp)
}

func ListUserTokens(ctx *gin.Context) {
	dbHandle, dbErr := ctx.MustGet("dbHandle").(*gorm.DB)
	if !dbErr {
		ctx.JSON(http.StatusInternalServerError, v1api.APIResponse{Message: errors.ErrDatabaseContextNotFound.Error()})
		return
	}

	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims["id"].(float64))

	patRepo := database.NewPATRepository(dbHandle)
	pats, err := patRepo.GetPATsByUserID(userID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, v1api.APIResponse{Message: "failed to list tokens"})
		return
	}

	tokens := make([]api.TokenResponse, 0, len(pats))
	for i := range pats {
		pat := &pats[i]
		token := api.TokenResponse{
			ID:     pat.ID,
			Name:   pat.Name,
			Scopes: pat.Scopes,
		}
		if pat.LastUsedAt != nil {
			formatted := pat.LastUsedAt.Format(time.RFC3339)
			token.LastUsedAt = &formatted
		}
		if pat.ExpiresAt != nil {
			formatted := pat.ExpiresAt.Format(time.RFC3339)
			token.ExpiresAt = &formatted
		}
		token.CreatedAt = pat.CreatedAt.Format(time.RFC3339)
		tokens = append(tokens, token)
	}

	ctx.JSON(http.StatusOK, tokens)
}

func DeleteUserToken(ctx *gin.Context) {
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)
	dbHandle, dbErr := ctx.MustGet("dbHandle").(*gorm.DB)
	if !dbErr {
		ctx.JSON(http.StatusInternalServerError, v1api.APIResponse{Message: errors.ErrDatabaseContextNotFound.Error()})
		return
	}

	claims := jwt.ExtractClaims(ctx)
	userID := uint(claims["id"].(float64))

	patIDStr := ctx.Param("id")
	var patID uint
	if _, err := parseUint(patIDStr, &patID); err != nil {
		ctx.JSON(http.StatusBadRequest, v1api.APIResponse{Message: "invalid token id"})
		return
	}

	patRepo := database.NewPATRepository(dbHandle)
	err := patRepo.DeletePAT(patID, userID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			ctx.JSON(http.StatusNotFound, v1api.APIResponse{Message: errors.ErrPATNotFound.Error()})
			return
		}
		logger.Error().Msg(err.Error())
		ctx.JSON(http.StatusInternalServerError, v1api.APIResponse{Message: "failed to delete token"})
		return
	}

	ctx.JSON(http.StatusOK, v1api.APIResponse{Message: "token deleted"})
}

func parseUint(s string, result *uint) (bool, error) {
	var val uint
	for _, c := range s {
		if c < '0' || c > '9' {
			return false, errors.ErrParseBody
		}
		val = val*10 + uint(c-'0')
	}
	*result = val
	return true, nil
}
