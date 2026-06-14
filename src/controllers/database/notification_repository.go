package database

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"codeberg.org/isotop7/proviant/models"
	"codeberg.org/isotop7/proviant/models/authentication"
	"codeberg.org/isotop7/proviant/models/database"
	"codeberg.org/isotop7/proviant/util"

	"github.com/rs/zerolog"
	"gorm.io/gorm"
)

type NotificationRepository struct {
	DB     *gorm.DB
	Logger *zerolog.Logger
}

func NewNotificationRepository(db *gorm.DB) *NotificationRepository {
	return &NotificationRepository{DB: db}
}

func NewNotificationRepositoryWithLogger(db *gorm.DB, logger *zerolog.Logger) *NotificationRepository {
	return &NotificationRepository{DB: db, Logger: logger}
}

type NotificationRepositoryInterface interface {
	GetProductsExpiredAndNotificationPending(sleepInterval time.Duration, maxLookAheadDays int) ([]database.Product, error)
	GetMaxNotificationThresholdDays() int
	GetHouseholdMembersMailAddressesByID(householdID uint) ([]string, error)
	GetHouseholdMembersNotificationPreferences(householdID uint) ([]models.NotificationRecipientInfo, error)
	SetProductNotifiedAt(productID uint) error
	CreateInvitation(householdID, inviterID uint, email string) (database.HouseholdInvitation, error)
	GetInvitationsForHousehold(householdID, inviterID uint) ([]database.HouseholdInvitation, error)
	GetInvitationByToken(token string) (database.HouseholdInvitation, error)
	AcceptInvitation(token, email string, userID uint) error
	CancelInvitation(invitationID, userID uint) error
	GetUserByID(userID uint) (authentication.User, error)
	GetHouseholdByID(householdID uint) (database.Household, error)
	GetPendingInvitationsNotSent(retryInterval time.Duration) ([]database.HouseholdInvitation, error)
	MarkInvitationSent(invitationID uint) error
	MarkInvitationSentTx(tx *gorm.DB, invitationID uint) error
	MarkInvitationSendFailed(invitationID uint) error
	GetHouseholdsWithMonthlyWasteReportEnabled() ([]models.HouseholdReportTarget, error)
	GetHouseholdsWithMailDigestEnabled() ([]models.HouseholdMailDigestTarget, error)
	GetWasteStatsForHousehold(householdID uint, month time.Time) (models.WasteStats, error)
	GetOnboardingState(userID uint) (database.OnboardingState, error)
	MarkNotificationsSetup(userID uint) error
	MarkHouseholdStepDone(userID uint) error
	MarkOnboardingComplete(userID uint) error
	GetPublicHouseholds(excludeHouseholdID uint) ([]database.HouseholdWithMemberCount, error)
	FindUserByTelegramLinkToken(token string) (authentication.User, error)
	SetTelegramChatID(userID uint, chatID string) error
	SetTelegramLinkToken(userID uint, token string) error
	SetTelegramBotUsername(userID uint, username string) error
	GetAllUsersWithTelegramBotToken() ([]authentication.User, error)
	SaveWebPushSubscription(userID uint, subscriptionJSON string) error
	DeleteWebPushSubscription(userID uint) error
	GetVAPIDKeys() (publicKey, privateKey string, err error)
	GenerateMailDigestUnsubscribeToken(userID uint) (string, error)
	GetUserByMailDigestUnsubscribeToken(token string) (authentication.User, error)
	DeleteMailDigestUnsubscribeToken(token string) error
}

func (r *NotificationRepository) GetProductsExpiredAndNotificationPending(sleepInterval time.Duration, maxLookAheadDays int) ([]database.Product, error) {
	now := time.Now()
	// SQL filter: include any product whose printed expiry is in the
	// window, OR whose opened-shelf-life rule could put it in the window.
	// The exact effective-date filter is applied in Go below to keep the
	// query portable across SQLite and MariaDB.
	//
	// The lower bound for the SQL candidate set is now - maxDaysAfterOpening
	// (with a 1-day safety margin) rather than the zero time. Without this
	// bound, effectiveExpiryCandidateScope's opened branch would admit
	// every product that has ever been opened (a near-full-table scan on
	// the scheduler's hot path). The Go fine-filter below still drops any
	// row whose effective expiry is actually outside the window.
	horizon := now.AddDate(0, 0, maxLookAheadDays)
	candidateStart := now.AddDate(0, 0, -(maxDaysAfterOpening + 1))
	var notificationProducts []database.Product
	getError := r.DB.
		Scopes(effectiveExpiryCandidateScope(candidateStart, horizon)).
		Where("notified_at < ?", now.Add(-(sleepInterval))).
		Find(&notificationProducts)

	if getError.Error != nil {
		return []database.Product{}, getError.Error
	}

	// Fine filter: only keep products whose effective expiry (printed or
	// opened+days) is actually within the window. This is where the secondary
	// expiry rule takes effect for the scheduler. The `notified_at < sleepInterval`
	// cooldown is already enforced by the SQL filter above, so no extra
	// notified_at check is needed here.
	filtered := make([]database.Product, 0, len(notificationProducts))
	for i := range notificationProducts {
		p := &notificationProducts[i]
		eff := p.EffectiveExpireAt()
		if eff.IsZero() || eff.After(horizon) {
			continue
		}
		filtered = append(filtered, *p)
	}
	return filtered, nil
}

func (r *NotificationRepository) GetMaxNotificationThresholdDays() int {
	var maxThreshold int
	r.DB.Model(&authentication.User{}).
		Select("COALESCE(MAX(notification_threshold_days), 0)").
		Scan(&maxThreshold)
	return maxThreshold
}

func (r *NotificationRepository) GetHouseholdMembersMailAddressesByID(householdID uint) ([]string, error) {
	var mailAddresses []string
	_, householdErr := r.GetHouseholdByID(householdID)
	if householdErr != nil {
		return mailAddresses, householdErr
	}

	var users []*authentication.User
	findErr := r.DB.Where(util.QueryHouseholdId, householdID).Find(&users)
	if findErr != nil {
		return mailAddresses, findErr.Error
	}

	for idx := range users {
		mailAddresses = append(mailAddresses, users[idx].MailAddress)
	}
	return mailAddresses, nil
}

func (r *NotificationRepository) GetHouseholdMembersNotificationPreferences(householdID uint) ([]models.NotificationRecipientInfo, error) {
	var preferences []models.NotificationRecipientInfo

	_, householdErr := r.GetHouseholdByID(householdID)
	if householdErr != nil {
		return preferences, householdErr
	}

	var users []*authentication.User
	findErr := r.DB.Where(util.QueryHouseholdId, householdID).Find(&users)
	if findErr.Error != nil {
		return preferences, findErr.Error
	}

	for idx := range users {
		user := users[idx]
		preferences = append(preferences, models.NotificationRecipientInfo{
			EmailAddress:              user.MailAddress,
			EmailEnabled:              user.NotificationPreferences.EmailEnabled,
			NtfyEnabled:               user.NotificationPreferences.NtfyEnabled,
			NtfyURL:                   user.NotificationPreferences.NtfyURL,
			NtfyTopic:                 user.NotificationPreferences.NtfyTopic,
			NtfyToken:                 user.NotificationPreferences.NtfyToken,
			TelegramEnabled:           user.NotificationPreferences.TelegramEnabled,
			TelegramChatID:            user.NotificationPreferences.TelegramChatID,
			TelegramBotToken:          user.NotificationPreferences.TelegramBotToken,
			NotificationThresholdDays: user.NotificationPreferences.NotificationThresholdDays,
			WebPushEnabled:            user.NotificationPreferences.WebPushEnabled,
			WebPushSubscriptionJSON:   user.NotificationPreferences.WebPushSubscriptionJSON,
		})
	}

	return preferences, nil
}

func (r *NotificationRepository) SetProductNotifiedAt(productID uint) error {
	var dbProduct database.Product
	getError := r.DB.First(&dbProduct, productID)
	if getError.Error != nil {
		return getError.Error
	}

	dbProduct.NotifiedAt = time.Now()
	saveResult := r.DB.Save(&dbProduct)
	if saveResult.Error != nil {
		return saveResult.Error
	}
	return nil
}

func (r *NotificationRepository) CreateInvitation(householdID, inviterID uint, email string) (database.HouseholdInvitation, error) {
	invRepo := NewInvitationRepository(r.DB)
	return invRepo.CreateInvitation(householdID, inviterID, email)
}

func (r *NotificationRepository) GetInvitationsForHousehold(householdID, inviterID uint) ([]database.HouseholdInvitation, error) {
	invRepo := NewInvitationRepository(r.DB)
	return invRepo.GetInvitationsForHousehold(householdID, inviterID)
}

func (r *NotificationRepository) GetInvitationByToken(token string) (database.HouseholdInvitation, error) {
	invRepo := NewInvitationRepository(r.DB)
	return invRepo.GetInvitationByToken(token)
}

func (r *NotificationRepository) AcceptInvitation(token, email string, userID uint) error {
	invRepo := NewInvitationRepository(r.DB)
	return invRepo.AcceptInvitation(token, email, userID)
}

func (r *NotificationRepository) CancelInvitation(invitationID, userID uint) error {
	invRepo := NewInvitationRepository(r.DB)
	return invRepo.CancelInvitation(invitationID, userID)
}

func (r *NotificationRepository) GetUserByID(userID uint) (authentication.User, error) {
	userRepo := NewUserRepository(r.DB)
	return userRepo.GetUserByID(userID)
}

func (r *NotificationRepository) GetHouseholdByID(householdID uint) (database.Household, error) {
	householdRepo := NewHouseholdRepository(r.DB)
	return householdRepo.GetHouseholdByID(householdID)
}

func (r *NotificationRepository) GetPendingInvitationsNotSent(retryInterval time.Duration) ([]database.HouseholdInvitation, error) {
	invRepo := NewInvitationRepository(r.DB)
	return invRepo.GetPendingInvitationsNotSent(retryInterval)
}

func (r *NotificationRepository) MarkInvitationSent(invitationID uint) error {
	invRepo := NewInvitationRepository(r.DB)
	return invRepo.MarkInvitationSent(invitationID)
}

func (r *NotificationRepository) MarkInvitationSentTx(tx *gorm.DB, invitationID uint) error {
	invRepo := NewInvitationRepository(r.DB)
	return invRepo.MarkInvitationSentTx(tx, invitationID)
}

func (r *NotificationRepository) MarkInvitationSendFailed(invitationID uint) error {
	invRepo := NewInvitationRepository(r.DB)
	return invRepo.MarkInvitationSendFailed(invitationID)
}

func (r *NotificationRepository) GetOnboardingState(userID uint) (database.OnboardingState, error) {
	userRepo := NewUserRepository(r.DB)
	return userRepo.GetOnboardingState(userID)
}

func (r *NotificationRepository) MarkNotificationsSetup(userID uint) error {
	userRepo := NewUserRepository(r.DB)
	return userRepo.MarkNotificationsSetup(userID)
}

func (r *NotificationRepository) MarkHouseholdStepDone(userID uint) error {
	userRepo := NewUserRepository(r.DB)
	return userRepo.MarkHouseholdStepDone(userID)
}

func (r *NotificationRepository) MarkOnboardingComplete(userID uint) error {
	userRepo := NewUserRepository(r.DB)
	return userRepo.MarkOnboardingComplete(userID)
}

func (r *NotificationRepository) GetPublicHouseholds(excludeHouseholdID uint) ([]database.HouseholdWithMemberCount, error) {
	householdRepo := NewHouseholdRepository(r.DB)
	return householdRepo.GetPublicHouseholds(excludeHouseholdID)
}

func (r *NotificationRepository) GetHouseholdsWithMonthlyWasteReportEnabled() ([]models.HouseholdReportTarget, error) {
	var users []authentication.User
	if err := r.DB.
		Where("monthly_waste_report_enabled = ?", true).
		Find(&users).Error; err != nil {
		return nil, err
	}

	index := map[uint]*models.HouseholdReportTarget{}
	for i := range users {
		user := &users[i]
		if _, ok := index[user.HouseholdID]; !ok {
			name := fmt.Sprintf("Household #%d", user.HouseholdID)
			if h, err := r.GetHouseholdByID(user.HouseholdID); err == nil {
				name = h.Name
			}
			index[user.HouseholdID] = &models.HouseholdReportTarget{
				HouseholdID:   user.HouseholdID,
				HouseholdName: name,
			}
		}
		if user.MailAddress != "" && user.NotificationPreferences.EmailEnabled {
			index[user.HouseholdID].Recipients = append(index[user.HouseholdID].Recipients, user.MailAddress)
		}
		if user.NotificationPreferences.TelegramEnabled && user.NotificationPreferences.TelegramChatID != "" && user.NotificationPreferences.TelegramBotToken != "" {
			index[user.HouseholdID].TelegramRecipients = append(index[user.HouseholdID].TelegramRecipients, models.TelegramRecipient{
				ChatID:   user.NotificationPreferences.TelegramChatID,
				BotToken: user.NotificationPreferences.TelegramBotToken,
			})
		}
	}

	targets := make([]models.HouseholdReportTarget, 0, len(index))
	for _, t := range index {
		targets = append(targets, *t)
	}
	return targets, nil
}

func (r *NotificationRepository) FindUserByTelegramLinkToken(token string) (authentication.User, error) {
	var user authentication.User
	err := r.DB.Where("telegram_link_token = ?", token).First(&user).Error
	return user, err
}

func (r *NotificationRepository) SetTelegramChatID(userID uint, chatID string) error {
	return r.DB.Model(&authentication.User{}).
		Where(util.QueryId, userID).
		Updates(map[string]interface{}{
			"telegram_chat_id":    chatID,
			"telegram_link_token": "",
			"telegram_enabled":    true,
		}).Error
}

func (r *NotificationRepository) SetTelegramLinkToken(userID uint, token string) error {
	return r.DB.Model(&authentication.User{}).
		Where(util.QueryId, userID).
		Update("telegram_link_token", token).Error
}

func (r *NotificationRepository) SetTelegramBotUsername(userID uint, username string) error {
	return r.DB.Model(&authentication.User{}).
		Where(util.QueryId, userID).
		Update("telegram_bot_username", username).Error
}

func (r *NotificationRepository) GetAllUsersWithTelegramBotToken() ([]authentication.User, error) {
	var users []authentication.User
	err := r.DB.Where("telegram_bot_token != ''").Find(&users).Error
	return users, err
}

func (r *NotificationRepository) SaveWebPushSubscription(userID uint, subscriptionJSON string) error {
	return r.DB.Model(&authentication.User{}).
		Where(util.QueryId, userID).
		Updates(map[string]any{
			"web_push_enabled":           true,
			"web_push_subscription_json": subscriptionJSON,
		}).Error
}

func (r *NotificationRepository) DeleteWebPushSubscription(userID uint) error {
	return r.DB.Model(&authentication.User{}).
		Where(util.QueryId, userID).
		Updates(map[string]any{
			"web_push_enabled":           false,
			"web_push_subscription_json": "",
		}).Error
}

func (r *NotificationRepository) GetVAPIDKeys() (publicKey, privateKey string, err error) {
	var config database.WebPushConfig
	findErr := r.DB.First(&config).Error
	if findErr == nil {
		return config.PublicKey, config.PrivateKey, nil
	}
	if !errors.Is(findErr, gorm.ErrRecordNotFound) {
		return "", "", findErr
	}

	privateKeyBytes, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return "", "", err
	}

	publicKeyBytes := elliptic.MarshalCompressed(elliptic.P256(), privateKeyBytes.X, privateKeyBytes.Y)

	config = database.WebPushConfig{
		PublicKey:  base64.URLEncoding.EncodeToString(publicKeyBytes),
		PrivateKey: base64.URLEncoding.EncodeToString(privateKeyBytes.D.Bytes()),
	}
	if saveErr := r.DB.Create(&config).Error; saveErr != nil {
		return "", "", saveErr
	}

	return config.PublicKey, config.PrivateKey, nil
}

func (r *NotificationRepository) GetHouseholdsWithMailDigestEnabled() ([]models.HouseholdMailDigestTarget, error) {
	var users []authentication.User
	if err := r.DB.
		Where("mail_digest_frequency IN ?", []string{authentication.MailDigestFrequencyDaily, authentication.MailDigestFrequencyWeekly}).
		Where("email_enabled = ?", true).
		Where("mail_address != ''").
		Find(&users).Error; err != nil {
		return nil, err
	}

	index := map[uint]*models.HouseholdMailDigestTarget{}
	for i := range users {
		user := &users[i]
		if _, ok := index[user.HouseholdID]; !ok {
			name := fmt.Sprintf("Household #%d", user.HouseholdID)
			if h, err := r.GetHouseholdByID(user.HouseholdID); err == nil {
				name = h.Name
			}
			index[user.HouseholdID] = &models.HouseholdMailDigestTarget{
				HouseholdID:   user.HouseholdID,
				HouseholdName: name,
			}
		}

		token, tokErr := r.getMailDigestUnsubscribeTokenForUser(user.ID)
		if tokErr != nil && !errors.Is(tokErr, gorm.ErrRecordNotFound) {
			r.Logger.Warn().Msgf("Mail digest: failed to get unsubscribe token for user %d: %s", user.ID, tokErr)
			continue
		}

		index[user.HouseholdID].Users = append(index[user.HouseholdID].Users, models.MailDigestUser{
			UserID:              user.ID,
			Email:               user.MailAddress,
			MailDigestFrequency: user.NotificationPreferences.MailDigestFrequency,
			MailDigestToken:     token,
		})
	}

	targets := make([]models.HouseholdMailDigestTarget, 0, len(index))
	for _, t := range index {
		targets = append(targets, *t)
	}
	return targets, nil
}

func (r *NotificationRepository) getMailDigestUnsubscribeTokenForUser(userID uint) (string, error) {
	var tokenRecord database.MailDigestUnsubscribeToken
	err := r.DB.Where("user_id = ?", userID).First(&tokenRecord).Error
	return tokenRecord.MailDigestToken, err
}

func (r *NotificationRepository) GenerateMailDigestUnsubscribeToken(userID uint) (string, error) {
	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", err
	}
	tokenStr := base64.URLEncoding.EncodeToString(tokenBytes)

	ut := database.MailDigestUnsubscribeToken{
		MailDigestToken: tokenStr,
		UserID:          userID,
		CreatedAt:       time.Now(),
	}
	if err := r.DB.Create(&ut).Error; err != nil {
		return "", err
	}
	return tokenStr, nil
}

func (r *NotificationRepository) GetUserByMailDigestUnsubscribeToken(tokenStr string) (authentication.User, error) {
	var ut database.MailDigestUnsubscribeToken
	if err := r.DB.Where("mail_digest_token = ?", tokenStr).First(&ut).Error; err != nil {
		return authentication.User{}, err
	}
	return r.GetUserByID(ut.UserID)
}

func (r *NotificationRepository) DeleteMailDigestUnsubscribeToken(tokenStr string) error {
	return r.DB.Where("mail_digest_token = ?", tokenStr).Delete(&database.MailDigestUnsubscribeToken{}).Error
}

func (r *NotificationRepository) GetWasteStatsForHousehold(householdID uint, month time.Time) (models.WasteStats, error) {
	start := time.Date(month.Year(), month.Month(), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, 0)
	prevStart := start.AddDate(0, -1, 0)

	countDeleted := func(from, to time.Time) (int64, error) {
		var n int64
		err := r.DB.Unscoped().Model(&database.Product{}).
			Where(util.QueryHouseholdId, householdID).
			Where("deleted_at >= ? AND deleted_at < ?", from, to).
			Count(&n).Error
		return n, err
	}

	// Products still in pantry whose best-before date fell within the window.
	countExpired := func(from, to time.Time) (int64, error) {
		var n int64
		err := r.DB.Model(&database.Product{}).
			Where(util.QueryHouseholdId, householdID).
			Where("expire_at >= ? AND expire_at < ?", from, to).
			Count(&n).Error
		return n, err
	}

	// Products active at any point in the window (denominator for waste rate).
	countActive := func(from, to time.Time) (int64, error) {
		var n int64
		err := r.DB.Unscoped().Model(&database.Product{}).
			Where(util.QueryHouseholdId, householdID).
			Where("created_at < ?", to).
			Where("deleted_at IS NULL OR deleted_at >= ?", from).
			Count(&n).Error
		return n, err
	}

	deleted, err := countDeleted(start, end)
	if err != nil {
		return models.WasteStats{}, err
	}
	expired, err := countExpired(start, end)
	if err != nil {
		return models.WasteStats{}, err
	}
	active, err := countActive(start, end)
	if err != nil {
		return models.WasteStats{}, err
	}
	prevDeleted, err := countDeleted(prevStart, start)
	if err != nil {
		return models.WasteStats{}, err
	}
	prevExpired, err := countExpired(prevStart, start)
	if err != nil {
		return models.WasteStats{}, err
	}
	prevActive, err := countActive(prevStart, start)
	if err != nil {
		return models.WasteStats{}, err
	}

	wasted := int(deleted + expired)
	prevWasted := int(prevDeleted + prevExpired)

	rate := func(w int, a int64) float64 {
		if a == 0 {
			return 0
		}
		return float64(w) / float64(a) * 100
	}

	wasteRate := rate(wasted, active)
	prevWasteRate := rate(prevWasted, prevActive)
	delta := wasteRate - prevWasteRate

	deltaColor := "#6A9580"
	deltaSymbol := "="
	if delta > 0 {
		deltaColor = "#DC2626"
		deltaSymbol = "&#9650;"
	} else if delta < 0 {
		deltaColor = "#2D9B4F"
		deltaSymbol = "&#9660;"
	}

	return models.WasteStats{
		Month:            start,
		MonthLabel:       start.Format("January 2006"),
		HouseholdID:      householdID,
		DeletedCount:     int(deleted),
		ExpiredCount:     int(expired),
		WastedCount:      wasted,
		PrevWastedCount:  prevWasted,
		WasteRatePct:     wasteRate,
		PrevWasteRatePct: prevWasteRate,
		Delta:            delta,
		DeltaColor:       deltaColor,
		DeltaSymbol:      deltaSymbol,
	}, nil
}

var _ NotificationRepositoryInterface = (*NotificationRepository)(nil)
