package v1

import (
	"fmt"
	"net/http"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers/database"
	"codeberg.org/isotop7/proviant/models/authentication"
	apiModel "codeberg.org/isotop7/proviant/models/api"
	"codeberg.org/isotop7/proviant/util"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

// GetNotifications returns actionable notification items for the current user:
// pending invitations from their household, incoming join requests (admin only),
// and outgoing join requests the user submitted.
// @Summary Get notifications
// @Description Returns pending invitations and household join requests for the current user
// @Tags Notifications
// @Produce json
// @Success 200 {object} apiModel.NotificationsResponse
// @Failure 400 {object} api.APIResponse
// @Failure 500 {object} api.APIResponse
// @Router /api/v1/notifications [get]
func GetNotifications(ctx *gin.Context) {
	logger, _ := ctx.MustGet("logger").(*zerolog.Logger)

	repos, ok := mustGetRepos(ctx, logger)
	if !ok {
		return
	}

	userID, ok := mustGetUserID(ctx, logger)
	if !ok {
		return
	}

	user, err := repos.Users.GetUserByID(userID)
	if err != nil {
		logger.Error().Msgf("Error fetching user %d: %s", userID, err)
		ctx.JSON(http.StatusInternalServerError, api.Error(err))
		return
	}

	items := buildNotificationItems(repos, logger, user, userID)

	ctx.JSON(http.StatusOK, apiModel.NotificationsResponse{
		Total: len(items),
		Items: items,
	})
}

func buildNotificationItems(repos *database.RepositoryContainer, logger *zerolog.Logger, user authentication.User, userID uint) []apiModel.NotificationItem {
	items := []apiModel.NotificationItem{}

	if user.HouseholdID != 0 {
		items = append(items, invitationNotificationItems(repos, logger, user.HouseholdID)...)
		items = append(items, incomingApplicationNotificationItems(repos, logger, user.HouseholdID, userID)...)
	}

	items = append(items, outgoingApplicationNotificationItems(repos, logger, userID)...)
	return items
}

func invitationNotificationItems(repos *database.RepositoryContainer, logger *zerolog.Logger, householdID uint) []apiModel.NotificationItem {
	invitations, err := repos.Invitations.GetPendingInvitationsForHousehold(householdID)
	if err != nil {
		logger.Error().Msgf("Error fetching pending invitations: %s", err)
		return nil
	}
	items := make([]apiModel.NotificationItem, 0, len(invitations))
	for i := range invitations {
		inv := &invitations[i]
		items = append(items, apiModel.NotificationItem{
			ID:        inv.ID,
			Type:      "invitation",
			Title:     "Invitation pending: " + inv.Email,
			CreatedAt: inv.CreatedAt.Format(util.DefaultDateFormatParseStr),
		})
	}
	return items
}

func incomingApplicationNotificationItems(repos *database.RepositoryContainer, logger *zerolog.Logger, householdID uint, userID uint) []apiModel.NotificationItem {
	household, err := repos.Households.GetHouseholdByID(householdID)
	if err != nil || household.AdminID != userID {
		return nil
	}
	applications, err := repos.Households.GetPendingApplicationsForAdmin(userID)
	if err != nil {
		logger.Error().Msgf("Error fetching pending applications for admin: %s", err)
		return nil
	}
	items := make([]apiModel.NotificationItem, 0, len(applications))
	for _, app := range applications {
		applicantName := fmt.Sprintf("User #%d", app.ApplicantID)
		if applicant, uErr := repos.Users.GetUserByID(app.ApplicantID); uErr == nil {
			applicantName = applicant.EffectiveName()
		}
		items = append(items, apiModel.NotificationItem{
			ID:        app.ID,
			Type:      "application_incoming",
			Title:     applicantName + " wants to join your household",
			CreatedAt: app.CreatedAt.Format(util.DefaultDateFormatParseStr),
		})
	}
	return items
}

func outgoingApplicationNotificationItems(repos *database.RepositoryContainer, logger *zerolog.Logger, userID uint) []apiModel.NotificationItem {
	applications, err := repos.Households.GetPendingApplicationsForApplicant(userID)
	if err != nil {
		logger.Error().Msgf("Error fetching user's pending applications: %s", err)
		return nil
	}
	items := make([]apiModel.NotificationItem, 0, len(applications))
	for _, app := range applications {
		householdName := fmt.Sprintf("Household #%d", app.HouseholdID)
		if h, hErr := repos.Households.GetHouseholdByID(app.HouseholdID); hErr == nil {
			householdName = h.Name
		}
		items = append(items, apiModel.NotificationItem{
			ID:        app.ID,
			Type:      "application_outgoing",
			Title:     "Awaiting approval to join " + householdName,
			CreatedAt: app.CreatedAt.Format(util.DefaultDateFormatParseStr),
		})
	}
	return items
}
