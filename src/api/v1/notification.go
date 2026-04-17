package v1

import (
	"fmt"
	"net/http"

	"codeberg.org/isotop7/proviant/api"
	"codeberg.org/isotop7/proviant/controllers/database"
	apiModel "codeberg.org/isotop7/proviant/models/api"

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

	dbHandle, ok := mustGetDB(ctx, logger)
	if !ok {
		return
	}

	userID, ok := mustGetUserID(ctx, logger)
	if !ok {
		return
	}

	userRepo := database.NewUserRepository(dbHandle)
	householdRepo := database.NewHouseholdRepository(dbHandle)
	invitationRepo := database.NewInvitationRepository(dbHandle)
	items := []apiModel.NotificationItem{}

	user, err := userRepo.GetUserByID(userID)
	if err != nil {
		logger.Error().Msgf("Error fetching user %d: %s", userID, err)
		ctx.JSON(http.StatusInternalServerError, api.Error(err))
		return
	}

	if user.HouseholdID != 0 {
		// Pending invitations sent by any member of the user's household
		invitations, invErr := invitationRepo.GetPendingInvitationsForHousehold(user.HouseholdID)
		if invErr != nil {
			logger.Error().Msgf("Error fetching pending invitations: %s", invErr)
		} else {
			for i := range invitations {
				inv := &invitations[i]
				items = append(items, apiModel.NotificationItem{
					ID:        inv.ID,
					Type:      "invitation",
					Title:     "Invitation pending: " + inv.Email,
					CreatedAt: inv.CreatedAt.Format("2006-01-02"),
				})
			}
		}

		// If the user is the household admin, surface incoming join requests
		household, householdErr := householdRepo.GetHouseholdByID(user.HouseholdID)
		if householdErr == nil && household.AdminID == userID {
			applications, appErr := householdRepo.GetPendingApplicationsForAdmin(userID)
			if appErr != nil {
				logger.Error().Msgf("Error fetching pending applications for admin: %s", appErr)
			} else {
				for _, app := range applications {
					applicantName := fmt.Sprintf("User #%d", app.ApplicantID)
					if applicant, uErr := userRepo.GetUserByID(app.ApplicantID); uErr == nil {
						applicantName = applicant.EffectiveName()
					}
					items = append(items, apiModel.NotificationItem{
						ID:        app.ID,
						Type:      "application_incoming",
						Title:     applicantName + " wants to join your household",
						CreatedAt: app.CreatedAt.Format("2006-01-02"),
					})
				}
			}
		}
	}

	// User's own outgoing pending applications to other households
	ownApplications, ownErr := householdRepo.GetPendingApplicationsForApplicant(userID)
	if ownErr != nil {
		logger.Error().Msgf("Error fetching user's pending applications: %s", ownErr)
	} else {
		for _, app := range ownApplications {
			householdName := fmt.Sprintf("Household #%d", app.HouseholdID)
			if h, hErr := householdRepo.GetHouseholdByID(app.HouseholdID); hErr == nil {
				householdName = h.Name
			}
			items = append(items, apiModel.NotificationItem{
				ID:        app.ID,
				Type:      "application_outgoing",
				Title:     "Awaiting approval to join " + householdName,
				CreatedAt: app.CreatedAt.Format("2006-01-02"),
			})
		}
	}

	ctx.JSON(http.StatusOK, apiModel.NotificationsResponse{
		Total: len(items),
		Items: items,
	})
}
