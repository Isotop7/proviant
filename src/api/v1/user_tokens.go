package v1

import (
	"net/http"
	"strconv"
	"time"

	v1api "codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers"
	"codeberg.org/isotop7/proviant/errors"
	"codeberg.org/isotop7/proviant/models/api"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

func CreateUserToken(ctx *gin.Context) {
	logger, _ := ctx.MustGet(util.ContextKeyLogger).(*zerolog.Logger)

	repos, ok := mustGetRepos(ctx, logger)
	if !ok {
		return
	}

	userID, ok := mustGetUserID(ctx, logger)
	if !ok {
		return
	}

	var req api.CreateTokenRequest
	if err := ctx.ShouldBind(&req); err != nil {
		ctx.JSON(http.StatusBadRequest, v1api.InvalidInputErrorWithDetail(err.Error()))
		return
	}

	rawToken, err := controllers.GeneratePAT()
	if err != nil {
		logger.Error().Msg(err.Error())
		ctx.JSON(http.StatusInternalServerError, v1api.InternalError())
		return
	}

	tokenHash := controllers.HashToken(rawToken)

	var expiresAt *time.Time
	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		parsed, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err != nil {
			ctx.JSON(http.StatusBadRequest, v1api.InvalidInputErrorWithDetail("expires_at must be in RFC3339 format"))
			return
		}
		expiresAt = &parsed
	}

	pat, err := repos.PATs.CreatePAT(userID, req.Name, tokenHash, expiresAt, req.Scopes)
	if err != nil {
		logger.Error().Msg(err.Error())
		ctx.JSON(http.StatusInternalServerError, v1api.InternalError())
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
	logger, _ := ctx.MustGet(util.ContextKeyLogger).(*zerolog.Logger)

	repos, ok := mustGetRepos(ctx, logger)
	if !ok {
		return
	}

	userID, ok := mustGetUserID(ctx, logger)
	if !ok {
		return
	}

	pats, err := repos.PATs.GetPATsByUserID(userID)
	if err != nil {
		ctx.JSON(http.StatusInternalServerError, v1api.InternalError())
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
	logger, _ := ctx.MustGet(util.ContextKeyLogger).(*zerolog.Logger)

	repos, ok := mustGetRepos(ctx, logger)
	if !ok {
		return
	}

	userID, ok := mustGetUserID(ctx, logger)
	if !ok {
		return
	}

	patIDStr := ctx.Param("id")
	patIDRaw, err := strconv.ParseUint(patIDStr, 10, 64)
	if err != nil {
		ctx.JSON(http.StatusBadRequest, v1api.InvalidInputErrorWithDetail("token ID must be a valid unsigned integer"))
		return
	}
	patID := uint(patIDRaw)

	if err := repos.PATs.DeletePAT(patID, userID); err != nil {
		if err == gorm.ErrRecordNotFound {
			ctx.JSON(http.StatusNotFound, v1api.Error(errors.ErrPATNotFound))
			return
		}
		logger.Error().Msg(err.Error())
		ctx.JSON(http.StatusInternalServerError, v1api.DeleteFailedError())
		return
	}

	ctx.JSON(http.StatusOK, v1api.APIResponse{Message: "token deleted"})
}
