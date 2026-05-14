package v1

import (
	"net/http"

	"codeberg.org/isotop7/proviant/api"
	"github.com/gin-gonic/gin"
)

type WebPushSubscriptionRequest struct {
	Endpoint string `json:"endpoint" binding:"required"`
	Keys      struct {
		P256dh string `json:"p256dh" binding:"required"`
		Auth   string `json:"auth" binding:"required"`
	} `json:"keys" binding:"required"`
}

type WebPushVAPIDPublicKeyResponse struct {
	PublicKey string `json:"publicKey"`
}

func GetWebPushVAPIDPublicKey(ctx *gin.Context, appCtx *AppContext) {
	publicKey, _, err := appCtx.Repos.Notifications.GetVAPIDKeys()
	if err != nil {
		appCtx.Logger.Error().Msgf("Error getting VAPID keys: %s", err)
		api.RespondError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, WebPushVAPIDPublicKeyResponse{PublicKey: publicKey})
}

func SubscribeWebPushNotifications(ctx *gin.Context, appCtx *AppContext) {
	var req WebPushSubscriptionRequest
	if !bindJSON(ctx, appCtx.Logger, &req) {
		return
	}

	subscriptionJSON := constructWebPushSubscriptionJSON(req.Endpoint, req.Keys.P256dh, req.Keys.Auth)
	if err := appCtx.Repos.Notifications.SaveWebPushSubscription(appCtx.UserID, subscriptionJSON); err != nil {
		appCtx.Logger.Error().Msgf("Error saving webpush subscription: %s", err)
		api.RespondError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, api.APIResponse{Message: "ok"})
}

func UnsubscribeWebPushNotifications(ctx *gin.Context, appCtx *AppContext) {
	if err := appCtx.Repos.Notifications.DeleteWebPushSubscription(appCtx.UserID); err != nil {
		appCtx.Logger.Error().Msgf("Error deleting push subscription: %s", err)
		api.RespondError(ctx, http.StatusInternalServerError, err)
		return
	}

	ctx.JSON(http.StatusOK, api.APIResponse{Message: "ok"})
}

func constructWebPushSubscriptionJSON(endpoint, p256dh, auth string) string {
	return `{"endpoint":"` + endpoint + `","keys":{"p256dh":"` + p256dh + `","auth":"` + auth + `"}}`
}